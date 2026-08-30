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

	rooms map[int32]*room

	// Increments by 1 for each
	// connection opened
	cIDCounter atomic.Int32
}

func NewHandler(ds *datastore.DataStore, guardPSK []byte, filters *filters.Filters) *Handler {
	rooms := make(map[int32]*room)
	for roomID := range filters.GetMaps() {
		rooms[roomID] = &room{}
	}

	return &Handler{
		guardPSK: guardPSK,

		ds:      ds,
		filters: filters,

		rooms: rooms,
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

	// Look up token
	ctx := r.Context()
	st, _ := h.ds.LookupSessionToken(ctx, token)

	// Get player fields
	username := "" // set it empty, guests are name-less.
	accountUUID := uuid.NewString()
	loggedIn := false

	if st != nil {
		username = st.ForUser.Username
		accountUUID = st.ForUser.ID.String()
		loggedIn = true
	}

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

// Send a message to everyone else in the room.
func (h *Handler) shareToRoom(d *clientData, msgs ...any) {
	// Serialize and create [gws.Broadcaster]
	msgBytes := pt.Serialize(msgs...)
	bc := gws.NewBroadcaster(gws.OpcodeBinary, msgBytes)

	// Send to room members
	room := h.rooms[d.roomID]
	room.RLock()
	for _, m := range room.members {
		// Skip if we're excluding this user
		if m.getData().cID == d.cID {
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

	// Tell us that everyone is here
	var introMsgs []any
	room := h.rooms[newID]
	room.RLock()
	for _, o := range room.members {
		if o.getData().cID == d.cID {
			continue
		}
		introMsgs = append(introMsgs, o.GetIntroMessages()...)
	}
	u.Send(introMsgs...)
	room.RUnlock()

	// Tell everyone else we're here
	h.shareToRoom(d, u.GetIntroMessages()...)
}

func (h *Handler) OnOpen(c *gws.Conn) {
	s := NewUser(c)
	log.Println("open cID=", s.getData().cID)

	// Set up initial packets
	// ..
	d := s.getData()
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
	picNames := h.filters.GetPictureNames()
	picPrefixes := h.filters.GetPicturePrefixes()
	battleAnimIDs := h.filters.GetBattleAnimIDs()
	if len(picNames) != 0 {
		initial = append(initial, pt.PictureSyncListS2C{
			Type: pt.PictureListName,
			List: picNames,
		})
	}
	if len(picPrefixes) != 0 {
		initial = append(initial, pt.PictureSyncListS2C{
			Type: pt.PictureListPrefix,
			List: picPrefixes,
		})
	}
	if len(battleAnimIDs) != 0 {
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
		h.shareToRoom(d, pt.FlashS2C{ID: d.cID, R: m.R, G: m.G, B: m.B, Power: m.Power, Frames: m.Frames})

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

	// Leave room
	d := s.getData()
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
