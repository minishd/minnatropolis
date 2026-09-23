package chat

// Type definitions for chat users
// and associated functions

import (
	"encoding/json"
	"errors"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/lxzan/gws"
	"github.com/minishd/minnatropolis/api/web"
	"golang.org/x/time/rate"
)

// Per-connection chat rate limit
const (
	sendRateLimitEvery = time.Second
	sendRateLimitBurst = 5
)

// Data associated with a chat client
type clientData struct {
	accountUUID uuid.UUID
	name        string

	limiter *rate.Limiter

	// The HTTP API can change these while they're connected
	mu        sync.RWMutex
	blocklist map[uuid.UUID]struct{}
	partyID   *uuid.UUID // nil if not in a party
}

// Whether or not this user has blocked someone
func (d *clientData) hasBlocked(accountUUID uuid.UUID) (ok bool) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	_, ok = d.blocklist[accountUUID]
	return
}

// Get the party this user is in, if any
func (d *clientData) getParty() (partyID uuid.UUID, ok bool) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	if d.partyID == nil {
		return
	}
	return *d.partyID, true
}

// Did either of them block the other?
func isBlockedBetween(a, b *clientData) bool {
	return a.hasBlocked(b.accountUUID) || b.hasBlocked(a.accountUUID)
}

// Wrapper around a [gws.Conn].
type User gws.Conn

func NewUser(c *gws.Conn) *User { return (*User)(c) }

// Get underlying [gws.Conn].
func (u *User) Conn() *gws.Conn { return (*gws.Conn)(u) }

// Key that client data is stored under in
// session storage k/v
const kClientData = "cd"

// Get [clientData] associated with a connection.
func (u *User) getData() *clientData {
	cd, _ := u.Conn().Session().Load(kClientData)
	return cd.(*clientData)
}

// Send an already-serialized message.
func (u *User) sendRaw(data []byte) {
	u.Conn().WriteAsync(gws.OpcodeText, data, nil)
}

// Serialize and send a message.
func (u *User) Send(msg any) {
	data, err := json.Marshal(msg)
	if err != nil {
		panic(err)
	}
	u.sendRaw(data)
}

// Send an error back to the user
func (u *User) SendError(err error) {
	werr, ok := errors.AsType[*web.Error](err)
	if !ok {
		log.Println("chat raised error:", err)
		werr = web.ErrServerInternal
	}

	u.Send(errorS2C{Type: typeError, Error: werr.Note})
}
