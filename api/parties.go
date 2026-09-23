package api

import (
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/minishd/minnatropolis/api/chat"
	"github.com/minishd/minnatropolis/api/web"
	"github.com/minishd/minnatropolis/datastore"
)

// Shared state for /parties API endpoint handlers
type partiesHandlers struct {
	ds *datastore.DataStore
	ch *chat.Handler
}

type partyMember struct {
	ID       uuid.UUID
	Username string
}
type partyInfo struct {
	ID        uuid.UUID
	Name      string
	CreatedAt time.Time
	Members   []partyMember
}
type partyRes struct {
	// nil if the user isn't in a party
	Party *partyInfo
}

// Helper for looking up a party's members & sending it back
func (h *partiesHandlers) sendPartyOK(w http.ResponseWriter, r *http.Request, party *datastore.Party) (err error) {
	ctx := r.Context()
	users, err := h.ds.GetPartyMembers(ctx, party.ID)
	if err != nil {
		return
	}

	members := []partyMember{}
	for _, user := range users {
		members = append(members, partyMember{
			ID:       user.ID,
			Username: user.Username,
		})
	}

	web.SendResOK(w, partyRes{&partyInfo{
		ID:        party.ID,
		Name:      party.Name,
		CreatedAt: party.CreatedAt,
		Members:   members,
	}})
	return
}

// Returns the party the user is in
// (Needs authentication)
func (h *partiesHandlers) handleMe(w http.ResponseWriter, r *http.Request, session *datastore.SessionToken) (err error) {
	ctx := r.Context()
	party, err := h.ds.GetUserParty(ctx, session.ForUser.ID)
	if err != nil {
		return
	}
	if party == nil {
		// Not in a party
		web.SendResOK(w, partyRes{nil})
		return
	}

	return h.sendPartyOK(w, r, party)
}

// Creates a party with the user in it
// (Needs authentication)
func (h *partiesHandlers) handleCreate(w http.ResponseWriter, r *http.Request, session *datastore.SessionToken) (err error) {
	type createReq struct {
		// Further validation of name is implemented
		// as a DB constraint
		Name string `validate:"required"`
	}

	// Parse request
	req, err := web.ParseReq[createReq](r)
	if err != nil {
		return
	}

	// Try to create in DB
	ctx := r.Context()
	name := strings.TrimSpace(req.Name)
	party, err := h.ds.CreateParty(ctx, session.ForUser.ID, name)
	switch err {
	case datastore.ErrNotUnique:
		err = web.ErrAlreadyInParty
	case datastore.ErrFailsCheck:
		err = web.ErrPartyNameInvalid
	}
	if err != nil {
		return
	}

	// Update chat
	h.ch.UpdateParty(session.ForUser.ID, &party.ID)

	return h.sendPartyOK(w, r, party)
}

// Adds the user to a party by its ID
// (Needs authentication)
func (h *partiesHandlers) handleJoin(w http.ResponseWriter, r *http.Request, session *datastore.SessionToken) (err error) {
	type joinReq struct {
		PartyID uuid.UUID
	}

	// Parse request
	req, err := web.ParseReq[joinReq](r)
	if err != nil {
		return
	}

	// Try to join
	ctx := r.Context()
	party, err := h.ds.InsertPartyMember(ctx, req.PartyID, session.ForUser.ID)
	switch err {
	case datastore.ErrNotUnique:
		err = web.ErrAlreadyInParty
	case datastore.ErrUnknownFkey:
		err = web.ErrNoSuchParty
	}
	if err != nil {
		return
	}

	// Update chat
	h.ch.UpdateParty(session.ForUser.ID, &party.ID)

	return h.sendPartyOK(w, r, party)
}

// Removes the user from their party
// (Needs authentication)
func (h *partiesHandlers) handleLeave(w http.ResponseWriter, r *http.Request, session *datastore.SessionToken) (err error) {
	ctx := r.Context()
	err = h.ds.DeletePartyMember(ctx, session.ForUser.ID)
	if err == datastore.ErrNotFound {
		err = web.ErrNotInParty
		return
	}
	if err != nil {
		return
	}

	// Update chat
	h.ch.UpdateParty(session.ForUser.ID, nil)

	web.SendResOK(w, partyRes{nil})
	return
}
