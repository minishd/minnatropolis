package chat

// Event handlers for chat websocket

import (
	"log"
	"net/http"
	"strings"
	"sync"
	"unicode"

	"github.com/google/uuid"
	"github.com/lxzan/gws"
	"github.com/minishd/minnatropolis/api/room"
	"github.com/minishd/minnatropolis/api/web"
	"github.com/minishd/minnatropolis/datastore"
	"golang.org/x/time/rate"
)

// Shared handler for chat websocket events
// Implements [gws.Event]
type Handler struct {
	ds *datastore.DataStore

	rh *room.Handler // for map chat

	// Keyed by connection because one account can have multiple tabs open
	users   map[*User]struct{}
	usersMu sync.RWMutex
}

func NewHandler(ds *datastore.DataStore, rh *room.Handler) *Handler {
	return &Handler{
		ds: ds,
		rh: rh,

		users: make(map[*User]struct{}),
	}
}

func (h *Handler) Authorize(r *http.Request, session gws.SessionStorage) bool {
	// Look up token
	st, err := web.GetAuth(h.ds, r)
	if err != nil {
		log.Println("session token lookup failed:", err)
		return false
	}
	if st == nil {
		// Guests are name-less,
		// so they can't chat
		return false
	}

	// Look up blocklist
	ctx := r.Context()
	blocked, err := h.ds.GetBlockedUsers(ctx, st.ForUser.ID)
	if err != nil {
		log.Println("blocklist lookup failed:", err)
		return false
	}
	blocklist := make(map[uuid.UUID]struct{}, len(blocked))
	for _, user := range blocked {
		blocklist[user.ID] = struct{}{}
	}

	// Look up party
	party, err := h.ds.GetUserParty(ctx, st.ForUser.ID)
	if err != nil {
		log.Println("party lookup failed:", err)
		return false
	}
	var partyID *uuid.UUID
	if party != nil {
		partyID = &party.ID
	}

	// Set up data
	session.Store(kClientData, &clientData{
		accountUUID: st.ForUser.ID,
		name:        st.ForUser.Username,
		limiter:     rate.NewLimiter(rate.Every(sendRateLimitEvery), sendRateLimitBurst),
		blocklist:   blocklist,
		partyID:     partyID,
	})

	// Authorize connection
	return true
}

func (h *Handler) OnOpen(c *gws.Conn) {
	u := NewUser(c)

	// Add to user registry
	h.usersMu.Lock()
	h.users[u] = struct{}{}
	h.usersMu.Unlock()
}

func (h *Handler) OnMessage(c *gws.Conn, msg *gws.Message) {
	defer msg.Close()

	u := NewUser(c)
	d := u.getData()

	if msg.Opcode != gws.OpcodeText {
		return
	}

	// Don't let them spam
	if !d.limiter.Allow() {
		u.SendError(web.ErrTooManyRequests)
		return
	}

	// Parse & validate message
	req, err := web.ParseJSON[sendMessageC2S](msg)
	if err != nil {
		u.SendError(err)
		return
	}

	// No empty messages or control characters
	content := strings.TrimSpace(req.Content)
	if content == "" || strings.ContainsFunc(content, unicode.IsControl) {
		u.SendError(web.ErrBodyInvalid)
		return
	}

	h.processMessage(u, req.Channel, content)
}

func (h *Handler) OnClose(c *gws.Conn, err error) {
	u := NewUser(c)

	// Remove from user registry
	h.usersMu.Lock()
	delete(h.users, u)
	h.usersMu.Unlock()
}

func (h *Handler) OnPing(c *gws.Conn, payload []byte) {
	_ = c.WritePong(payload)
}
func (h *Handler) OnPong(c *gws.Conn, payload []byte) {}
