package room

// Event handlers for room websocket

import (
	"bytes"
	"crypto/sha1"
	"encoding/binary"
	"log"
	"net/http"
	"strconv"
	"sync"
	"sync/atomic"

	"github.com/google/uuid"
	"github.com/lxzan/gws"
	"github.com/minishd/minnatropolis/api/room/filters"
	pt "github.com/minishd/minnatropolis/api/room/protocol"
	"github.com/minishd/minnatropolis/api/room/unconscious"
	"github.com/minishd/minnatropolis/api/room/user"
	"github.com/minishd/minnatropolis/api/web"
	"github.com/minishd/minnatropolis/datastore"
)

// Shared handler for room websocket events
// Implements [gws.Event]
type Handler struct {
	guardPSK []byte

	ds      *datastore.DataStore
	filters *filters.Filters
	coun    *unconscious.Unconscious

	rooms   map[int32]*room
	users   map[uuid.UUID]*user.User
	usersMu sync.RWMutex

	// Increments by 1 for each
	// connection opened
	cIDCounter atomic.Int32
}

func NewHandler(
	ds *datastore.DataStore, guardPSK []byte,
	filters *filters.Filters, coun *unconscious.Unconscious,
) *Handler {
	rooms := make(map[int32]*room)
	for roomID := range filters.GetMaps() {
		rooms[roomID] = &room{}
	}

	users := make(map[uuid.UUID]*user.User)

	return &Handler{
		guardPSK: guardPSK,

		ds:      ds,
		filters: filters,
		coun:    coun,

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

	// Player fields
	username := "" // set it empty, guests are name-less.
	accountUUID := uuid.New()
	loggedIn := false
	blocklist := make(map[uuid.UUID]struct{})

	// Look up token, if present
	st, err := web.GetAuth(h.ds, r)
	if err != nil {
		log.Println("session token lookup failed:", err)
		// we'll let them through still,
		// but they will be a guest
	}

	// Get player fields
	if st != nil {
		username = st.ForUser.Username
		accountUUID = st.ForUser.ID
		loggedIn = true

		// Also look up blocklist
		ctx := r.Context()
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
	_, connected := h.users[accountUUID]
	h.usersMu.RUnlock()
	if connected {
		return false
	}

	// Set up data
	user.InitData(session, h.cIDCounter.Add(1), username, roomID, accountUUID, loggedIn, blocklist)

	// Authorize connection
	return true
}

func (h *Handler) OnOpen(c *gws.Conn) {
	s := user.New(c)
	log.Println("open cID=", s.Data().CID)

	// Add to user registry
	// ..
	// We don't do it in [Handler.Authorize] because
	// it's called before there's a [gws.Conn]
	d := s.Data()
	h.usersMu.Lock()

	// Double-check that they didn't connect
	// in between [Handler.Authorize]'s check
	// and now..
	if _, ok := h.users[d.AccountUUID]; ok {
		// They did, so close this connection
		h.usersMu.Unlock()
		log.Println("early close cID=", s.Data().CID)
		s.Conn().WriteClose(1000, nil)
		return
	}

	// Seems ok so add them
	go s.SendLoop()
	h.users[d.AccountUUID] = s
	h.usersMu.Unlock()

	// Set up initial packets
	// ..
	initial := []any{
		pt.SyncPlayerDataS2C{
			HostID:     d.CID,
			Key:        d.GuardKey,
			UUID:       d.AccountUUID,
			Rank:       d.Rank,
			IsLoggedIn: d.LoggedIn,
			Badge:      d.Badge,
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

	// If game is Collective Unconscious,
	// we also want to set the time and weather.
	if h.coun != nil {
		initial = append(initial, h.coun.GetTimePacket())
		initial = append(initial, h.coun.GetWeatherPacket())
	}

	// Send initial packet immediately..
	s.SendImmediate(initial...)

	// Add to room
	h.changeRoom(s, d.RoomID)
}

func (h *Handler) OnMessage(c *gws.Conn, msg *gws.Message) {
	defer msg.Close()

	s := user.New(c)
	d := s.Data()

	m := msg.Bytes()
	if len(m) < 8 {
		// Missing guard data
		return
	}

	// Verify HMAC
	// ..
	hash := sha1.New()
	hash.Write(h.guardPSK)
	hash.Write(d.GuardKeyBytes)
	hash.Write(m[4:])
	if !bytes.Equal(hash.Sum(nil)[:4], m[:4]) {
		// Invalid HMAC
		return
	}

	// Verify counter
	// ..
	count := binary.BigEndian.Uint32(m[4:8])
	if count <= d.GuardCount {
		// The sent count should only increase
		log.Println("declined count")
		return
	}
	d.GuardCount = count

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
	s := user.New(c)
	log.Println("close cID=", s.Data().CID)

	// Remove all subscriptions
	h.unsetRoom(s)

	// Remove from user registry
	// Only if it's this connection because early closed ones were never added
	d := s.Data()
	h.usersMu.Lock()
	if h.users[d.AccountUUID] == s {
		delete(h.users, d.AccountUUID)
	}
	h.usersMu.Unlock()

	// Close message loop
	// It's important to do this after de-registering
	// the user so that nothing sends to a closed channel
	d.OnClose()

	// Leave room
	h.shareToRoom(d, pt.DisconnectS2C{ID: d.CID})
}

func (h *Handler) OnPing(c *gws.Conn, payload []byte) {
	// minnaengine doesn't send pings
	// but respond anyway
	_ = c.WritePong(nil)
}
func (h *Handler) OnPong(c *gws.Conn, payload []byte) {}
