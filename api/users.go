package api

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/minishd/minnatropolis/api/web"
	"github.com/minishd/minnatropolis/datastore"
)

type usersHandlers struct {
	ds *datastore.DataStore
}

// Returns all accounts the user has blocked
// (Needs authentication)
func (h *usersHandlers) handleBlockList(w http.ResponseWriter, r *http.Request, session *datastore.SessionToken) (err error) {
	type blocklistEntry struct {
		ID       uuid.UUID
		Username string
	}
	type blocklistRes struct {
		Blocked []blocklistEntry
	}

	// Look up user's block list
	ctx := r.Context()
	users, err := h.ds.GetBlockedUsers(ctx, session.ForUser.ID)
	if err != nil {
		return
	}

	// Convert to blocklist entries
	blocked := []blocklistEntry{}
	for _, user := range users {
		blocked = append(blocked, blocklistEntry{
			ID:       user.ID,
			Username: user.Username,
		})
	}

	web.SendResOK(w, blocklistRes{blocked})
	return
}

// Adds an account to user's block list
// (Needs authentication)
func (h *usersHandlers) handleBlockListAdd(w http.ResponseWriter, r *http.Request, session *datastore.SessionToken) (err error) {
	type blocklistAddReq struct {
		UserID uuid.UUID
	}
	type blocklistAddRes struct {
		Success bool
	}

	// Parse request
	req, err := web.ParseReq[blocklistAddReq](r)
	if err != nil {
		return
	}

	// Try to add blocked user
	ctx := r.Context()
	err = h.ds.InsertBlockRelation(ctx, session.ForUser.ID, req.UserID)
	switch err {
	case datastore.ErrNotUnique:
		err = web.ErrAlreadyBlocked
	case datastore.ErrUnknownFkey:
		err = web.ErrNoSuchUser
	}
	if err != nil {
		return
	}

	// No error, they should be blocked now
	web.SendResOK(w, blocklistAddRes{true})
	return
}

// Removes an account from user's block list
// (Needs authentication)
func (h *usersHandlers) handleBlockListRemove(w http.ResponseWriter, r *http.Request, session *datastore.SessionToken) (err error) {
	type blocklistRemoveReq struct {
		UserID uuid.UUID
	}
	type blocklistRemoveRes struct {
		Success bool
	}

	// Parse request
	req, err := web.ParseReq[blocklistRemoveReq](r)
	if err != nil {
		return
	}

	// Try to remove blocked user
	ctx := r.Context()
	err = h.ds.DeleteBlockRelation(ctx, session.ForUser.ID, req.UserID)
	if err == datastore.ErrNotFound {
		err = web.ErrNotBlocked
		return
	}
	if err != nil {
		return
	}

	// No error, so they should be unblocked now
	web.SendResOK(w, blocklistRemoveRes{true})
	return
}
