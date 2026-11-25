package datastore

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/minishd/minnatropolis/datastore/scan2"
)

type SessionToken struct {
	ID        uuid.UUID `scan2:"0"`
	CreatedAt time.Time `scan2:"1"`
	ForUser   *User     `scan2:"4"`
	Token     string    `scan2:"2"`
	ExpiresAt time.Time `scan2:"3"`
}

const sqlCreateSessionToken = `
INSERT INTO session_tokens (for_user, token, expires_at)
VALUES ($1, $2, $3)`

func (ds *DataStore) CreateSessionToken(forUser uuid.UUID, token string, expiresAt time.Time) error {
	ctx := context.Background()
	_, err := ds.pool.Exec(ctx, sqlCreateSessionToken, forUser, token, expiresAt)
	return err
	// Token collisions not accounted for
}

const sqlLookupSessionToken = `
SELECT st.id, st.created_at, st.token, st.expires_at, uf.*
FROM session_tokens st
JOIN users_full uf ON uf.id = st.for_user
WHERE st.token = $1
  AND st.expires_at > CURRENT_TIMESTAMP`

func (ds *DataStore) LookupSessionToken(token string) (*SessionToken, error) {
	ctx := context.Background()
	return scan2.ScanOne2[SessionToken](ctx, ds.pool, sqlLookupSessionToken, token)
}

const sqlDeleteSessionToken = `
DELETE FROM session_tokens
WHERE id = $1`

func (ds *DataStore) DeleteSessionToken(id uuid.UUID) error {
	ctx := context.Background()
	result, err := ds.pool.Exec(ctx, sqlDeleteSessionToken, id)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

const sqlClearOtherSessionTokensForUser = `
DELETE FROM session_tokens
WHERE for_user = $1
  AND id != $2`

func (ds *DataStore) ClearOtherSessionTokensForUser(forUser, exceptFor uuid.UUID) error {
	ctx := context.Background()
	_, err := ds.pool.Exec(ctx, sqlClearOtherSessionTokensForUser, forUser, exceptFor)
	return err
}

const sqlUpdateSessionTokenExpiry = `
UPDATE session_tokens
SET expires_at = $2
WHERE id = $1`

func (ds *DataStore) UpdateSessionTokenExpiry(id uuid.UUID, expiresAt time.Time) error {
	ctx := context.Background()
	_, err := ds.pool.Exec(ctx, sqlUpdateSessionTokenExpiry, id, expiresAt)
	return err
}
