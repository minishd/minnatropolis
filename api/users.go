package api

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/minishd/minnatropolis/api/room"
	"github.com/minishd/minnatropolis/api/web"
	"github.com/minishd/minnatropolis/datastore"
)

type usersHandlers struct {
	ds *datastore.DataStore
	rh *room.Handler
}

type blocklistEntry struct {
	ID       uuid.UUID
	Username string
}
type blocklistRes struct {
	Blocked []blocklistEntry
}

func usersToBlockList(users []*datastore.User) []blocklistEntry {
	blocked := []blocklistEntry{}
	for _, user := range users {
		blocked = append(blocked, blocklistEntry{
			ID:       user.ID,
			Username: user.Username,
		})
	}
	return blocked
}

// Returns all accounts the user has blocked
// (Needs authentication)
func (h *usersHandlers) handleBlockList(w http.ResponseWriter, r *http.Request, session *datastore.SessionToken) (err error) {
	// Look up user's block list
	ctx := r.Context()
	users, err := h.ds.GetBlockedUsers(ctx, session.ForUser.ID)
	if err != nil {
		return
	}

	// Convert to blocklist entries, return..
	blocked := usersToBlockList(users)
	web.SendResOK(w, blocklistRes{blocked})
	return
}

// Adds an account to user's block list
// (Needs authentication)
func (h *usersHandlers) handleBlockListAdd(w http.ResponseWriter, r *http.Request, session *datastore.SessionToken) (err error) {
	type blocklistAddReq struct {
		UserID uuid.UUID
	}
	type blocklistAddRes = blocklistRes

	// Parse request
	req, err := web.ParseReq[blocklistAddReq](r)
	if err != nil {
		return
	}

	// Try to add blocked user
	ctx := r.Context()
	users, err := h.ds.InsertBlockRelation(ctx, session.ForUser.ID, req.UserID)
	switch err {
	case datastore.ErrNotUnique:
		err = web.ErrAlreadyBlocked
	case datastore.ErrUnknownFkey:
		err = web.ErrNoSuchUser
	}
	if err != nil {
		return
	}

	// Update multiplayer
	h.rh.UpdateBlockList(session.ForUser.ID, users)

	// No error, they should be blocked now
	blocked := usersToBlockList(users)
	web.SendResOK(w, blocklistAddRes{blocked})
	return
}

// Removes an account from user's block list
// (Needs authentication)
func (h *usersHandlers) handleBlockListRemove(w http.ResponseWriter, r *http.Request, session *datastore.SessionToken) (err error) {
	type blocklistRemoveReq struct {
		UserID uuid.UUID
	}
	type blocklistRemoveRes = blocklistRes

	// Parse request
	req, err := web.ParseReq[blocklistRemoveReq](r)
	if err != nil {
		return
	}

	// Try to remove blocked user
	ctx := r.Context()
	users, err := h.ds.DeleteBlockRelation(ctx, session.ForUser.ID, req.UserID)
	if err == datastore.ErrNotFound {
		err = web.ErrNotBlocked
		return
	}
	if err != nil {
		return
	}

	// Update multiplayer
	h.rh.UpdateBlockList(session.ForUser.ID, users)

	// No error, so they should be unblocked now
	blocked := usersToBlockList(users)
	web.SendResOK(w, blocklistRemoveRes{blocked})
	return
}
