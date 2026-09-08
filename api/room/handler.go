package room

// Event handlers for room websocket

import (
	"bytes"
	"crypto/sha1"
	"encoding/binary"
	"log"
	"math/rand/v2"
	"net/http"
	"strconv"
	"sync"
	"sync/atomic"

	"github.com/google/uuid"
	"github.com/lxzan/gws"
	"github.com/minishd/minnatropolis/api/room/filters"
	pt "github.com/minishd/minnatropolis/api/room/protocol"
	"github.com/minishd/minnatropolis/datastore"
)

// Shared handler for room websocket events
// Implements [gws.Event]
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
