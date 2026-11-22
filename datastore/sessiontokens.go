package datastore

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
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

func (ds *DataStore) CreateSessionToken(ctx context.Context, forUser uuid.UUID, token string, expiresAt time.Time) error {
	_, err := ds.pool.Exec(ctx, sqlCreateSessionToken, forUser, token, expiresAt)
	return err
	// Token collisions not accounted for
}

const sqlLookupSessionToken = `
SELECT st.id, st.created_at, st.token, st.expires_at, u.id, u.created_at, u.username, u.pw_hash_type, u.pw_hash, u.last_visit_at, u.last_seen_ip, u.mod_note, u.role, p.id, p.created_at, p.punishment_type, p.is_ip_enforced, p.is_disabled, p.expires_at, p.reason
FROM session_tokens st
     JOIN users u       ON u.id = st.for_user
LEFT JOIN punishments p ON p.id = u.current_punishment
                       AND p.is_disabled IS FALSE
                       AND (p.expires_at IS NULL OR CURRENT_TIMESTAMP < p.expires_at)
WHERE st.token = $1
  AND st.expires_at > CURRENT_TIMESTAMP`

func (ds *DataStore) LookupSessionToken(ctx context.Context, token string) (*SessionToken, error) {
	st, err := scan2.ScanOne2[SessionToken](ctx, ds.pool, sqlLookupSessionToken, token)
	if err != nil {
		// Is there no active session token with that value?
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}

		return nil, err
	}

	return st, nil
}

const sqlDeleteSessionToken = `
DELETE FROM session_tokens
WHERE id = $1`

func (ds *DataStore) DeleteSessionToken(ctx context.Context, id uuid.UUID) error {
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

func (ds *DataStore) ClearOtherSessionTokensForUser(ctx context.Context, forUser, exceptFor uuid.UUID) error {
	_, err := ds.pool.Exec(ctx, sqlClearOtherSessionTokensForUser, forUser, exceptFor)
	return err
}

const sqlUpdateSessionTokenExpiry = `
UPDATE session_tokens
SET expires_at = $2
WHERE id = $1`

func (ds *DataStore) UpdateSessionTokenExpiry(ctx context.Context, id uuid.UUID, expiresAt time.Time) error {
	_, err := ds.pool.Exec(ctx, sqlUpdateSessionTokenExpiry, id, expiresAt)
	return err
}
