package api

import (
	"context"
	"crypto/rand"

	"github.com/alexedwards/argon2id"
	"github.com/google/uuid"
	"github.com/minishd/minnatropolis/api/weberrors"
	"github.com/minishd/minnatropolis/datastore"
)

// Shared state for /auth API endpoint handlers
type authHandlers struct {
	ds *datastore.DataStore
}

// Helper function that generates a session token for a user
// and returns its string value
func (h *authHandlers) issueSessionToken(ctx context.Context, forUser uuid.UUID) (token string, err error) {
	// Generate token string
	// then try to add it to DB
	newToken := rand.Text()
	err = h.ds.InsertSessionToken(ctx, forUser, newToken)
	if err != nil {
		return
	}
	// Successful
	return
}

type whoamiRes struct {
	Username string
}

func (h *authHandlers) handleWhoami(ctx context.Context) (res whoamiRes, err error) {
	// ...

	return
}

type registerReq struct {
	Username string `validate:"required"`
	Password string `validate:"required"`
}
type registerRes struct {
	InitialToken string
}

func (h *authHandlers) handleRegister(ctx context.Context, req registerReq) (res registerRes, err error) {
	// Hash password
	pwHash, err := argon2id.CreateHash(req.Password, argon2id.DefaultParams)
	if err != nil {
		return
	}

	// Try to create in DB
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
	res.InitialToken = token

	return
}

type loginReq struct {
	Username string `validate:"required"`
	Password string `validate:"required"`
}
type loginRes struct {
	Token string
}

func (h *authHandlers) handleLogin(ctx context.Context, req loginReq) (res loginRes, err error) {
	// Look up user in database
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
	res.Token = token

	return
}
