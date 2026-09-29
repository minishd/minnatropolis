package room

// Functions meant to be called by HTTP API
// can go here

import (
	"github.com/google/uuid"
	pt "github.com/minishd/minnatropolis/api/room/protocol"
	"github.com/minishd/minnatropolis/api/room/user"
	"github.com/minishd/minnatropolis/datastore"
)

// If a user is in `first` but not `second`,
// send the specified packets to them
func (h *Handler) sendToListDifferences(
	us *user.User,
	first map[uuid.UUID]struct{},
	second map[uuid.UUID]struct{},
	makePackets func(*user.ClientData) []any,
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
		usData := us.Data()
		themData := them.Data()
		if usData.RoomID != themData.RoomID {
			continue
		}
		// Send packets to the two players
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
	d := us.Data()
	d.BlocklistMu.Lock()
	defer d.BlocklistMu.Unlock()

	// Make new blocklist
	blocklistNew := make(map[uuid.UUID]struct{}, len(blocked))
	for _, them := range blocked {
		blocklistNew[them.ID] = struct{}{}
	}

	// Lock users
	h.usersMu.RLock()
	defer h.usersMu.RUnlock()

	// Handle disconnections
	h.sendToListDifferences(us, blocklistNew, d.Blocklist, func(cd *user.ClientData) []any {
		return []any{pt.DisconnectS2C{ID: cd.CID}}
	})

	// Handle connections
	h.sendToListDifferences(us, d.Blocklist, blocklistNew, func(cd *user.ClientData) []any {
		return cd.GetIntroMessages()
	})

	// Set blocklist
	d.Blocklist = blocklistNew
}

// Get the account IDs of everyone in the same room
// as a user, including them. Used for map chat
func (h *Handler) GetRoommates(accountUUID uuid.UUID) (roommates map[uuid.UUID]struct{}, ok bool) {
	// Find the user
	h.usersMu.RLock()
	us, ok := h.users[accountUUID]
	h.usersMu.RUnlock()
	if !ok {
		// Seems not in-game
		return
	}

	d := us.Data()
	roommates = map[uuid.UUID]struct{}{d.AccountUUID: {}}
	if h.arePacketsSkippedMap(d) {
		// Singleplayer map, so just them
		return
	}

	room := h.rooms[d.RoomID]
	room.RLock()
	for _, m := range room.members {
		roommates[m.Data().AccountUUID] = struct{}{}
	}
	room.RUnlock()

	return
}
