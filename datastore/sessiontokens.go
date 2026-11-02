package datastore

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/minishd/minnatropolis/queries"
)

type SessionToken struct {
	ID        uuid.UUID
	CreatedAt time.Time
	ForUser   *User
	Token     string
}

func dbSessionTokenToApp(st queries.SessionToken, u queries.User, p *queries.Punishment) *SessionToken {
	return &SessionToken{
		ID:        st.ID,
		CreatedAt: st.CreatedAt,
		ForUser:   dbUserToApp(u, p),
		Token:     st.Token,
	}
}

func (ds *DataStore) InsertSessionToken(ctx context.Context, forUser uuid.UUID, token string, expiresAt time.Time) error {
	return ds.q.InsertSessionToken(ctx, queries.InsertSessionTokenParams{
		ForUser:   forUser,
		Token:     token,
		ExpiresAt: expiresAt,
	})
	// Token collisions not accounted for
}

func (ds *DataStore) LookupSessionToken(ctx context.Context, token string) (*SessionToken, error) {
	row, err := ds.q.LookupSessionToken(ctx, token)

	// Is there no active session token with that value?
	if err == pgx.ErrNoRows {
		return nil, nil
	}

	if err != nil {
		return nil, err
	}
	return dbSessionTokenToApp(row.SessionToken, row.User, row.Punishment), nil
}

func (ds *DataStore) DeleteSessionToken(ctx context.Context, id uuid.UUID) error {
	rows, err := ds.q.DeleteSessionToken(ctx, id)
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

func (ds *DataStore) ClearOtherSessionTokensForUser(ctx context.Context, forUser, exceptFor uuid.UUID) error {
	return ds.q.ClearOtherSessionTokensForUser(ctx, queries.ClearOtherSessionTokensForUserParams{
		ForUser: forUser,
		ID:      exceptFor,
	})
}

func (ds *DataStore) UpdateSessionTokenExpiry(ctx context.Context, id uuid.UUID, expiresAt time.Time) error {
	return ds.q.UpdateSessionTokenExpiry(ctx, queries.UpdateSessionTokenExpiryParams{
		ID:        id,
		ExpiresAt: expiresAt,
	})
}
