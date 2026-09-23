package chat

// Functions meant to be called by HTTP API
// can go here

import (
	"github.com/google/uuid"
	"github.com/minishd/minnatropolis/datastore"
)

// Run f on each of an account's chat connections
func (h *Handler) forEachConnOf(accountUUID uuid.UUID, f func(d *clientData)) {
	h.usersMu.RLock()
	defer h.usersMu.RUnlock()

	for u := range h.users {
		d := u.getData()
		if d.accountUUID == accountUUID {
			f(d)
		}
	}
}

// Update a user's blocklist
func (h *Handler) UpdateBlockList(accountUUID uuid.UUID, blocked []*datastore.User) {
	// Make new blocklist
	blocklistNew := make(map[uuid.UUID]struct{}, len(blocked))
	for _, them := range blocked {
		blocklistNew[them.ID] = struct{}{}
	}

	h.forEachConnOf(accountUUID, func(d *clientData) {
		d.mu.Lock()
		d.blocklist = blocklistNew
		d.mu.Unlock()
	})
}

// Update a user's party. nil if they left
func (h *Handler) UpdateParty(accountUUID uuid.UUID, partyID *uuid.UUID) {
	h.forEachConnOf(accountUUID, func(d *clientData) {
		d.mu.Lock()
		d.partyID = partyID
		d.mu.Unlock()
	})
}
