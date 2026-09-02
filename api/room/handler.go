package room

import (
	"bytes"
	"crypto/sha1"
	"encoding/binary"
	"log"
	"math/rand/v2"
	"net/http"
	"slices"
	"strconv"
	"sync"
	"sync/atomic"

	"github.com/google/uuid"
	"github.com/lxzan/gws"
	"github.com/minishd/minnatropolis/api/room/filters"
	pt "github.com/minishd/minnatropolis/api/room/protocol"
	"github.com/minishd/minnatropolis/datastore"
)

type room struct {
	sync.RWMutex
	members []*User
}

// Shared handler for room websocket events
type Handler struct {
	guardPSK []byte

	ds      *datastore.DataStore
	filters *filters.Filters

	rooms   map[int32]*room
	users   map[uuid.UUID]*User
	usersMu sync.RWMutex

	// Increments by 1 for each
	// connection opened
	cIDCounter atomic.Int32
}

func NewHandler(ds *datastore.DataStore, guardPSK []byte, filters *filters.Filters) *Handler {
	rooms := make(map[int32]*room)
	for roomID := range filters.GetMaps() {
		rooms[roomID] = &room{}
	}

	users := make(map[uuid.UUID]*User)

	return &Handler{
		guardPSK: guardPSK,

		ds:      ds,
		filters: filters,

		rooms: rooms,
		users: users,
	}
}

// The values that clients will assume
// if they aren't specified.
//
// Used so our assumptions match theirs
// and we don't send unnecessary updates.
const (
	defaultXY     = -1
	defaultFacing = 2
	defaultSpeed  = 4

	defaultTransparency = 0
	defaultHidden       = false
	defaultSprite       = ""
	defaultSpriteIndex  = -1
	defaultSysName      = ""
)

func (h *Handler) Authorize(r *http.Request, session gws.SessionStorage) bool {
	// Get room ID
	roomID_, err := strconv.Atoi(r.URL.Query().Get("id"))
	if err != nil {
		return false
	}
	roomID := int32(roomID_)
	if !h.hasRoom(roomID) {
		// where are you going?
		return false
	}

	// Get token
	token := r.URL.Query().Get("token")

	// Player fields
	username := "" // set it empty, guests are name-less.
	accountUUID := uuid.New()
	loggedIn := false
	blocklist := make(map[uuid.UUID]struct{})

	// Look up token, if present
	ctx := r.Context()
	var st *datastore.SessionToken
	if token != "" {
		st, err = h.ds.LookupSessionToken(ctx, token)
		if err != nil {
			log.Println("session token lookup failed:", err)
			// we'll let them through still,
			// but they will be a guest
		}
	}

	// Get player fields
	if st != nil {
		username = st.ForUser.Username
		accountUUID = st.ForUser.ID
		loggedIn = true

		// Also look up blocklist
		users, err := h.ds.GetBlockedUsers(ctx, st.ForUser.ID)
		if err != nil {
			log.Println("blocklist lookup failed:", err)
			// still let them through
		}
		for _, user := range users {
			blocklist[user.ID] = struct{}{}
		}
	}

	// Don't allow the same user to connect twice
	h.usersMu.RLock()
	if _, ok := h.users[accountUUID]; ok {
		return false
	}
	h.usersMu.RUnlock()

	// Make guard key
	guardKey := rand.Uint32()
	guardKeyBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(guardKeyBytes, guardKey)

	// Set up data
	session.Store(kClientData, &clientData{
		cID:           h.cIDCounter.Add(1),
		name:          username,
		accountUUID:   accountUUID,
		loggedIn:      loggedIn,
		blocklist:     blocklist,
		guardKey:      guardKey,
		guardKeyBytes: guardKeyBytes,

		roomID: roomID,
		x:      defaultXY, y: defaultXY,
		facing: defaultFacing,
		speed:  defaultSpeed,

		transparency: defaultTransparency,
		hidden:       defaultHidden,
		sprite:       defaultSprite,
		spriteIndex:  defaultSpriteIndex,
		sysName:      defaultSysName,

		activePictures: make(map[int32]pt.Picture),
	})

	// Authorize connection
	return true
}

func (h *Handler) hasRoom(roomID int32) bool {
	_, ok := h.rooms[roomID]
	return ok
}

func (h *Handler) unsetRoom(m *User) {
	d := m.getData()

	// Remove them from their room
	room := h.rooms[d.roomID]
	room.Lock()
	room.members = slices.DeleteFunc(room.members, func(rm *User) bool { return rm.getData().cID == d.cID })
	room.Unlock()
}

func (h *Handler) setRoom(m *User, roomID int32) {
	// Remove them from old room
	h.unsetRoom(m)

	// Put them in new room
	room := h.rooms[roomID]
	room.Lock()
	room.members = append(room.members, m)
	room.Unlock()

	// Set their room ID
	m.getData().roomID = roomID
}

// Update a user's blocklist, showing & hiding players
// as needed
func (h *Handler) UpdateBlockList(accountUUID uuid.UUID, blocked []*datastore.User) {
	// Find the user
	u, ok := h.users[accountUUID]
	if !ok {
		// Seems not online
		return
	}

	// Lock blocklist
	// We don't want another update to come in
	// as we're dispatching connect/disconnect packets
	// That could cause invalid states
	d := u.getData()
	d.blocklistMu.Lock()
	defer d.blocklistMu.Unlock()

	// Make new blocklist
	blocklistNew := make(map[uuid.UUID]struct{}, len(blocked))
	for _, user := range blocked {
		blocklistNew[user.ID] = struct{}{}
	}

	// Lock users
	h.usersMu.RLock()
	defer h.usersMu.RUnlock()

	// Handle disconnections
	for id, _ := range blocklistNew {
		// Skip if already blocked
		if _, ok := d.blocklist[id]; ok {
			continue
		}
		// Get user
		user, ok := h.users[id]
		if !ok {
			// Not online
			continue
		}
		// Skip if not in same room
		data := user.getData()
		if d.roomID != data.roomID {
			continue
		}
		// Hide players from eachother
		u.Send(pt.DisconnectS2C{ID: data.cID})
		user.Send(pt.DisconnectS2C{ID: d.cID})
	}

	// Handle connections
	for id, _ := range d.blocklist {
		// Skip if still blocked
		if _, ok := blocklistNew[id]; ok {
			continue
		}
		// Get user
		user, ok := h.users[id]
		if !ok {
			// Not here..
			continue
		}
		// Skip if not in same room
		if d.roomID != user.getData().roomID {
			continue
		}
		// Show them
		u.Send(user.GetIntroMessages()...)
		user.Send(u.GetIntroMessages()...)
	}

	// Set blocklist
	d.blocklist = blocklistNew
}

// Whether or not we should skip packets in a room.
func (h *Handler) arePacketsSkippedMap(us *clientData) bool {
	return h.filters.IsMapSingleplayer(us.roomID)
}

// Whether or not we should skip packets about a player.
func (h *Handler) arePacketsSkippedPlayer(us *clientData, them *clientData) bool {
	// Is it ourselves? We already know what we sent
	if us.cID == them.cID {
		return true
	}

	// Lock blocklists so we can check safely
	us.blocklistMu.RLock()
	them.blocklistMu.RLock()
	defer us.blocklistMu.RUnlock()
	defer them.blocklistMu.RUnlock()

	// Did we block them?
	if _, ok := us.blocklist[them.accountUUID]; ok {
		// Yes, don't replicate..
		return true
	}
	// Did they block us?
	if _, ok := them.blocklist[us.accountUUID]; ok {
		// Yes, also don't replicate
		return true
	}

	// Nobody blocked eachother :)
	return false
}

// Send a message to everyone else inrack the room.
func (h *Handler) shareToRoom(d *clientData, msgs ...any) {
	// Skip if it's a room where we don't
	// want to network players (singleplayer)
	if h.arePacketsSkippedMap(d) {
		return
	}

	// Serialize and create [gws.Broadcaster]
	msgBytes := pt.Serialize(msgs...)
	bc := gws.NewBroadcaster(gws.OpcodeBinary, msgBytes)

	// Send to room members
	room := h.rooms[d.roomID]
	room.RLock()
	for _, m := range room.members {
		if h.arePacketsSkippedPlayer(d, m.getData()) {
			continue
		}

		// Send the message
		_ = bc.Broadcast(m.Conn(), nil)
	}
	room.RUnlock()
}

// Change from one room to another.
func (h *Handler) changeRoom(u *User, newID int32) {
	d := u.getData()

	// If the two rooms are different,
	// we need to handle leaving the other room
	if newID != d.roomID {
		// Tell other players we left
		h.shareToRoom(d, pt.DisconnectS2C{ID: d.cID})
	}

	// Introduce to new room
	u.Send(pt.RoomInfoS2C{RoomID: newID})
	h.setRoom(u, newID)

	// Tell us that everyone is here,
	// if it is not a singleplayer map
	if !h.arePacketsSkippedMap(d) {
		var introMsgs []any
		room := h.rooms[newID]
		room.RLock()
		for _, m := range room.members {
			if h.arePacketsSkippedPlayer(d, m.getData()) {
				continue
			}
			introMsgs = append(introMsgs, m.GetIntroMessages()...)
		}
		u.Send(introMsgs...)
		room.RUnlock()
	}

	// Tell everyone else we're here
	h.shareToRoom(d, u.GetIntroMessages()...)
}

func (h *Handler) OnOpen(c *gws.Conn) {
	s := NewUser(c)
	log.Println("open cID=", s.getData().cID)

	// Add to user registry
	// ..
	// We don't do it in [Handler.Authorize] because
	// it's called before there's a [gws.Conn]
	d := s.getData()
	h.usersMu.Lock()

	// Double-check that they didn't connect
	// in between [Handler.Authorize]'s check
	// and now..
	if _, ok := h.users[d.accountUUID]; ok {
		// They did, so close this connection
		log.Println("early close cID=", s.getData().cID)
		s.Conn().WriteClose(1000, nil)
		return
	}

	// Seems ok so add them
	h.users[d.accountUUID] = s
	h.usersMu.Unlock()

	// Set up initial packets
	// ..
	initial := []any{
		pt.SyncPlayerDataS2C{
			HostID:     d.cID,
			Key:        d.guardKey,
			UUID:       d.accountUUID,
			Rank:       d.rank,
			IsLoggedIn: d.loggedIn,
			Badge:      d.badge,
		},
	}

	// We only want to send filter list packets
	// if there's anything at all we want the client
	// to sync
	// That is because if you send an empty pic. prefix
	// sync list, the client will begin to sync every picture
	// I am assuming that is an engine bug..
	if picNames := h.filters.GetPictureNames(); len(picNames) != 0 {
		initial = append(initial, pt.PictureSyncListS2C{
			Type: pt.PictureListName,
			List: picNames,
		})
	}
	if picPrefixes := h.filters.GetPicturePrefixes(); len(picPrefixes) != 0 {
		initial = append(initial, pt.PictureSyncListS2C{
			Type: pt.PictureListPrefix,
			List: picPrefixes,
		})
	}
	if battleAnimIDs := h.filters.GetBattleAnimIDs(); len(battleAnimIDs) != 0 {
		initial = append(initial, pt.BattleAnimSyncListS2C{
			IDs: battleAnimIDs,
		})
	}

	// Send initial packet..
	s.Send(initial...)

	// Add to room
	h.changeRoom(s, d.roomID)
}

func (h *Handler) removePicture(d *clientData, picID int32) {
	delete(d.activePictures, picID)
	h.shareToRoom(d, pt.ErasePictureS2C{ID: d.cID, PicID: picID})
}

func (h *Handler) updatePicture(d *clientData, pic pt.Picture) {
	// If it's not a one-shot effect,
	// we will keep track of it
	if !pic.SpritesheetPlayOnce {
		d.activePictures[pic.PicID] = pic
	}
}

func (h *Handler) processMessage(u *User, m any) {
	d := u.getData()

	switch m := m.(type) {

	case pt.SwitchRoomC2S:
		log.Println("change to room", m.RoomID)
		h.changeRoom(u, m.RoomID)

	case pt.MainPlayerPosC2S:
		d.x = m.X
		d.y = m.Y
		h.shareToRoom(d, pt.MainPlayerPosS2C{ID: d.cID, X: d.x, Y: d.y})
	case pt.TeleportC2S:
		d.x = m.X
		d.y = m.Y
		h.shareToRoom(d, pt.MainPlayerPosS2C{ID: d.cID, X: d.x, Y: d.y})
	case pt.JumpC2S:
		d.x = m.X
		d.y = m.Y
		h.shareToRoom(d, pt.JumpS2C{ID: d.cID, X: d.x, Y: d.y})

	case pt.SpeedC2S:
		d.speed = m.Speed
		h.shareToRoom(d, pt.SpeedS2C{ID: d.cID, Speed: d.speed})

	case pt.SpriteC2S:
		d.sprite = m.Name
		d.spriteIndex = m.Index
		h.shareToRoom(d, pt.SpriteS2C{ID: d.cID, Name: d.sprite, Index: d.spriteIndex})

	case pt.FacingC2S:
		d.facing = m.Direction
		h.shareToRoom(d, pt.FacingS2C{ID: d.cID, Direction: d.facing})

	case pt.HiddenC2S:
		d.hidden = m.Hidden
		h.shareToRoom(d, pt.HiddenS2C{ID: d.cID, Hidden: d.hidden})

	case pt.SysNameC2S:
		d.sysName = m.Name
		h.shareToRoom(d, pt.SysNameS2C{ID: d.cID, Name: d.sysName})

	case pt.TransparencyC2S:
		d.transparency = m.Transparency
		h.shareToRoom(d, pt.TransparencyS2C{ID: d.cID, Transparency: d.transparency})

	case pt.SoundEffectC2S:
		h.shareToRoom(d, pt.SoundEffectS2C{ID: d.cID, Name: m.Name, Volume: m.Volume, Tempo: m.Tempo, Balance: m.Balance})

	case pt.FlashC2S:
		h.shareToRoom(d, pt.FlashS2C{ID: d.cID, Flash: m.Flash})
	case pt.RepeatingFlashC2S:
		d.flash = &m.Flash
		h.shareToRoom(d, pt.RepeatingFlashS2C{ID: d.cID, Flash: m.Flash})
	case pt.RemoveRepeatingFlashC2S:
		d.flash = nil
		h.shareToRoom(d, pt.RemoveRepeatingFlashS2C{ID: d.cID})

	case pt.ShowPlayerBattleAnimC2S:
		h.shareToRoom(d, pt.ShowPlayerBattleAnimS2C{ID: d.cID, AnimID: m.AnimID})

	case pt.ShowPictureC2S:
		// If there is already a picture
		// with that ID, we will remove it
		_, ok := d.activePictures[m.PicID]
		if ok {
			h.removePicture(d, m.PicID)
		}

		h.updatePicture(d, m.Picture)
		h.shareToRoom(d, pt.ShowPictureS2C{ID: d.cID, Picture: m.Picture})
	case pt.MovePictureC2S:
		pic, ok := d.activePictures[m.PicID]
		if !ok {
			// No such picture?
			// That's invalid but I won't do
			// anything about it for now
			return
		}
		pic.BasePicture = m.BasePicture

		h.updatePicture(d, pic)
		h.shareToRoom(d, pt.MovePictureS2C{ID: d.cID, BasePicture: m.BasePicture, Duration: m.Duration})

	case pt.ErasePictureC2S:
		_, ok := d.activePictures[m.PicID]
		if !ok {
			// Also no such picture..
			return
		}
		h.removePicture(d, m.PicID)

	default:
		// If we registered a message type,
		// we should also be handling it
		panic("unhandled message type")
	}
}

func (h *Handler) OnMessage(c *gws.Conn, msg *gws.Message) {
	defer msg.Close()

	s := NewUser(c)
	d := s.getData()

	m := msg.Bytes()
	if len(m) < 8 {
		// Missing guard data
		return
	}

	// Verify HMAC
	// ..
	hash := sha1.New()
	hash.Write(h.guardPSK)
	hash.Write(d.guardKeyBytes)
	hash.Write(m[4:])
	if !bytes.Equal(hash.Sum(nil)[:4], m[:4]) {
		// Invalid HMAC
		return
	}

	// Verify counter
	// ..
	count := binary.BigEndian.Uint32(m[4:8])
	if count <= d.guardCount {
		// The sent count should only increase
		log.Println("declined count")
		return
	}
	d.guardCount = count

	// Message handling
	// ..
	msgs, err := pt.Deserialize(m[8:])
	if err != nil {
		log.Println("invalid packet:", err)
		return
	}

	// Validate everything first so a packet is applied entirely
	// or not at all since a legitimate client will never send an
	// invalid packet.
	for _, msg := range msgs {
		if err := h.validateMessage(msg); err != nil {
			log.Println("invalid message:", err)
			return
		}
	}

	for _, msg := range msgs {
		h.processMessage(s, msg)
	}
}

func (h *Handler) OnClose(c *gws.Conn, err error) {
	s := NewUser(c)
	log.Println("close cID=", s.getData().cID)

	// Remove from user registry
	d := s.getData()
	h.usersMu.Lock()
	delete(h.users, d.accountUUID)
	h.usersMu.Unlock()

	// Leave room
	h.shareToRoom(d, pt.DisconnectS2C{ID: d.cID})

	// Remove all subscriptions
	h.unsetRoom(s)
}

func (h *Handler) OnPing(c *gws.Conn, payload []byte) {
	// minnaengine doesn't send pings
	// but respond anyway
	_ = c.WritePong(nil)
}
func (h *Handler) OnPong(c *gws.Conn, payload []byte) {}
