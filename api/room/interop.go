package room

// Functions meant to be called by HTTP API
// can go here

import (
	"github.com/google/uuid"
	pt "github.com/minishd/minnatropolis/api/room/protocol"
	"github.com/minishd/minnatropolis/datastore"
)

// If a user is in `first` but not `second`,
// send the specified packets to them
func (h *Handler) sendToListDifferences(
	us *User,
	first map[uuid.UUID]struct{},
	second map[uuid.UUID]struct{},
	makePackets func(*clientData) []any,
) {
	for themID, _ := range first {
		// Skip if in other list
		// We only want the differences between
		// the two lists
		if _, ok := second[themID]; ok {
			continue
		}
		// Get them
		them, ok := h.users[themID]
		if !ok {
			// Not online
			continue
		}
		// Skip if not in same room
		usData := us.getData()
		themData := them.getData()
		if usData.roomID != themData.roomID {
			continue
		}
		// Hide players from eachother
		us.Send(makePackets(themData)...)
		them.Send(makePackets(usData)...)
	}
}

// Update a user's blocklist, showing & hiding players
// as needed
func (h *Handler) UpdateBlockList(accountUUID uuid.UUID, blocked []*datastore.User) {
	// Find the user
	us, ok := h.users[accountUUID]
	if !ok {
		// Seems not online
		return
	}

	// Lock blocklist
	// We don't want another update to come in
	// as we're dispatching connect/disconnect packets
	// That could cause invalid states
	d := us.getData()
	d.blocklistMu.Lock()
	defer d.blocklistMu.Unlock()

	// Make new blocklist
	blocklistNew := make(map[uuid.UUID]struct{}, len(blocked))
	for _, them := range blocked {
		blocklistNew[them.ID] = struct{}{}
	}

	// Lock users
	h.usersMu.RLock()
	defer h.usersMu.RUnlock()

	// Handle disconnections
	h.sendToListDifferences(us, blocklistNew, d.blocklist, func(cd *clientData) []any {
		return []any{pt.DisconnectS2C{ID: cd.cID}}
	})

	// Handle connections
	h.sendToListDifferences(us, d.blocklist, blocklistNew, func(cd *clientData) []any {
		return cd.getIntroMessages()
	})

	// Set blocklist
	d.blocklist = blocklistNew
}
