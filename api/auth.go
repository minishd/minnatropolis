package api

import (
	"context"
	"crypto/rand"
	"net/http"
	"strings"

	"github.com/alexedwards/argon2id"
	"github.com/google/uuid"
	"github.com/minishd/minnatropolis/api/weberrors"
	"github.com/minishd/minnatropolis/datastore"
)

// Shared state for /auth API endpoint handlers
type authHandlers struct {
	ds *datastore.DataStore
}

// Session handler wrapper for authentication
func (h *authHandlers) requireAuth(next sessionHandler) handleError {
	return func(w http.ResponseWriter, r *http.Request) (err error) {
		header := r.Header.Get("Authorization")
		after, found := strings.CutPrefix(header, "Bearer ")
		if !found {
			return weberrors.ErrUnauthorized
		}

		sessionToken, err := h.ds.LookupSessionToken(r.Context(), after)
		if err != nil {
			return
		}
		if sessionToken == nil {
			return weberrors.ErrUnauthorized
		}

		return next(w, r, sessionToken)
	}
}

// Helper function that generates a session token for a user
// and returns its string value
func (h *authHandlers) issueSessionToken(ctx context.Context, forUser uuid.UUID) (token string, err error) {
	// Generate token string
	// then try to add it to DB
	token = rand.Text()
	err = h.ds.InsertSessionToken(ctx, forUser, token)
	if err != nil {
		return
	}
	// Successful
	return
}

// Route that returns the requester's username
func (h *authHandlers) handleWhoami(w http.ResponseWriter, r *http.Request, session *datastore.SessionToken) (err error) {
	type whoamiRes struct {
		Username string
	}

	sendRes(w, whoamiRes{session.ForUser.Username})
	return
}

// Handles account registration
func (h *authHandlers) handleRegister(w http.ResponseWriter, r *http.Request) (err error) {
	type registerReq struct {
		Username string `validate:"required"`
		Password string `validate:"required"`
	}
	type registerRes struct {
		InitialToken string
	}

	// Parse request
	req, err := parseReq[registerReq](r)
	if err != nil {
		return
	}

	// Hash password
	pwHash, err := argon2id.CreateHash(req.Password, argon2id.DefaultParams)
	if err != nil {
		return
	}

	// Try to create in DB
	ctx := r.Context()
	user, err := h.ds.CreateUser(ctx, req.Username, pwHash, datastore.PhtArgon2id)
	if err == datastore.ErrNotUnique {
		err = weberrors.ErrUsernameTaken
		return
	}
	if err == datastore.ErrFailsCheck {
		err = weberrors.ErrUsernameInvalid
		return
	}
	if err != nil {
		return
	}

	// Return a session token
	// for the new account
	token, err := h.issueSessionToken(ctx, user.ID)
	if err != nil {
		return
	}
	sendRes(w, registerRes{token})

	return
}

// Handles logging into accounts
func (h *authHandlers) handleLogin(w http.ResponseWriter, r *http.Request) (err error) {
	type loginReq struct {
		Username string `validate:"required"`
		Password string `validate:"required"`
	}
	type loginRes struct {
		Token string
	}

	// Parse request
	req, err := parseReq[loginReq](r)
	if err != nil {
		return
	}

	// Look up user in database
	ctx := r.Context()
	user, err := h.ds.GetUserByUsername(ctx, req.Username)
	if err != nil {
		return
	}
	if user == nil {
		err = weberrors.ErrInvalidCredentials
		return
	}

	// Check if password matches
	match, err := argon2id.ComparePasswordAndHash(req.Password, user.PwHash)
	if err != nil {
		return
	}
	if !match {
		err = weberrors.ErrInvalidCredentials
		return
	}

	// It does match so give them a session token
	token, err := h.issueSessionToken(ctx, user.ID)
	if err != nil {
		return
	}
	sendRes(w, loginRes{token})

	return
}
