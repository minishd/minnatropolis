package room

import (
	"slices"
	"sync"

	pt "github.com/minishd/minnatropolis/api/room/protocol"
	"github.com/minishd/minnatropolis/api/room/user"
)

// A pub/sub room.
//
// It keeps track of who's in a room/map
// so we can send packets to them as needed
type room struct {
	sync.RWMutex
	members []*user.User
}

// Whether or not a room exists.
// Also, note that singleplayer rooms still
// exist here. We just don't send packets
// when we're in one
func (h *Handler) hasRoom(roomID int32) bool {
	_, ok := h.rooms[roomID]
	return ok
}

// Remove a user from a room.
// (Does nothing if they were already not
// in the room associated with their room ID)
//
// This doesn't actually hide the user from other
// players, it just makes it so that they don't
// receive any more game events from their current
// room.
func (h *Handler) unsetRoom(m *user.User) {
	d := m.GetData()

	// Remove them from their room
	room := h.rooms[d.RoomID]
	room.Lock()
	room.members = slices.DeleteFunc(room.members, func(rm *user.User) bool {
		return rm.GetData().CID == d.CID
	})
	room.Unlock()
}

// Set what pub/sub room a player is in,
// removing them from their old one if necessary.
//
// Like [Handler.unsetRoom] it doesn't send any packets
// to other players, it just decides what map a player
// will receive packets for
func (h *Handler) setRoom(m *user.User, roomID int32) {
	// Remove them from old room
	h.unsetRoom(m)

	// Put them in new room
	room := h.rooms[roomID]
	room.Lock()
	room.members = append(room.members, m)
	room.Unlock()

	// Set their room ID
	m.GetData().RoomID = roomID
}

// Whether or not we should skip packets in a room.
func (h *Handler) arePacketsSkippedMap(us *user.ClientData) bool {
	return h.filters.IsMapSingleplayer(us.RoomID)
}

func hasBlocked(source *user.ClientData, subject *user.ClientData) (ok bool) {
	_, ok = source.Blocklist[subject.AccountUUID]
	return
}

// Whether or not we should skip packets about a player.
func (h *Handler) arePacketsSkippedPlayer(us *user.ClientData, them *user.ClientData) bool {
	// Is it ourselves? We already know what we sent
	if us.CID == them.CID {
		return true
	}

	// Lock blocklists so we can check safely
	us.BlocklistMu.RLock()
	them.BlocklistMu.RLock()
	defer us.BlocklistMu.RUnlock()
	defer them.BlocklistMu.RUnlock()

	// Did we block them, or they block us?
	return hasBlocked(us, them) || hasBlocked(them, us)
}

// Send a message to everyone else in the room.
func (h *Handler) shareToRoom(d *user.ClientData, msgs ...any) {
	// Skip if it's a room where we don't
	// want to network players (singleplayer)
	if h.arePacketsSkippedMap(d) {
		return
	}

	// Send to room members
	room := h.rooms[d.RoomID]
	room.RLock()
	for _, m := range room.members {
		if h.arePacketsSkippedPlayer(d, m.GetData()) {
			continue
		}

		// Add to message queue
		m.Send(msgs...)
	}
	room.RUnlock()
}

// Change from one room to another.
// This function handles hiding the player from
// the map they were in previously and showing
// them in the new map, so it is appropriate
// for actual game map transitions
func (h *Handler) changeRoom(u *user.User, newID int32) {
	d := u.GetData()

	// If the two rooms are different,
	// we need to handle leaving the other room
	if newID != d.RoomID {
		// Tell other players we left
		h.shareToRoom(d, pt.DisconnectS2C{ID: d.CID})
	}

	// Introduce to new room
	// (it should be ok to send immediately here?)
	roomMsgs := []any{pt.RoomInfoS2C{RoomID: newID}}
	if h.coun != nil {
		// If game is Collective Unconscious,
		// also set the current event variable.
		roomMsgs = append(roomMsgs, h.coun.GetEventPacket())
	}
	u.SendImmediate(roomMsgs...)
	h.setRoom(u, newID)

	// Tell us that everyone is here,
	// if it is not a singleplayer map
	if !h.arePacketsSkippedMap(d) {
		var introMsgs []any
		room := h.rooms[newID]
		room.RLock()
		for _, m := range room.members {
			md := m.GetData()
			if h.arePacketsSkippedPlayer(d, md) {
				continue
			}
			introMsgs = append(introMsgs, md.GetIntroMessages()...)
		}
		u.Send(introMsgs...)
		room.RUnlock()
	}

	// Tell everyone else we're here
	h.shareToRoom(d, d.GetIntroMessages()...)
}
