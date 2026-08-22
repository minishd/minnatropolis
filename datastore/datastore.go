package datastore

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/minishd/minnatropolis/queries"
)

var (
	ErrNotUnique  = errors.New("not unique")
	ErrFailsCheck = errors.New("fails check")
)

// Database abstraction to decouple
// DB code from rest of app code
type DataStore struct {
	q *queries.Queries
}

func New(pool *pgxpool.Pool) *DataStore {
	return &DataStore{
		q: queries.New(pool),
	}
}

// Convert Postgres error to one we defined
// so other code doesn't deal with [pgx]
func checkPgError(err error) error {
	if pe, ok := errors.AsType[*pgconn.PgError](err); ok {
		switch pe.Code {
		case pgerrcode.UniqueViolation:
			return ErrNotUnique
		case pgerrcode.CheckViolation:
			return ErrFailsCheck
		}
	}
	return nil
}

func (ds *DataStore) CreateUser(ctx context.Context, username, pwHash string, pwHashType PwHashType) (*User, error) {
	user, err := ds.q.CreateUser(ctx, queries.CreateUserParams{
		Username:   username,
		PwHashType: appPwHashTypeToDB(pwHashType),
		PwHash:     pwHash,
	})

	if err := checkPgError(err); err != nil {
		return nil, err
	}
	if err != nil {
		return nil, err
	}
	return dbUserToApp(user), nil
}

func (ds *DataStore) GetUserByUsername(ctx context.Context, username string) (*User, error) {
	user, err := ds.q.GetUserByUsername(ctx, username)

	// Did we not find an account?
	if err == pgx.ErrNoRows {
		return nil, nil
	}

	if err != nil {
		return nil, err
	}
	return dbUserToApp(user), nil
}

func (ds *DataStore) InsertSessionToken(ctx context.Context, forUser uuid.UUID, token string, expiresAt time.Time) error {
	err := ds.q.InsertSessionToken(ctx, queries.InsertSessionTokenParams{
		ForUser:   forUser,
		Token:     token,
		ExpiresAt: expiresAt,
	})
	// Token collisions not accounted for..
	if err != nil {
		return err
	}
	return nil
}

func (ds *DataStore) LookupSessionToken(ctx context.Context, token string) (*SessionToken, error) {
	st, err := ds.q.LookupSessionTokenWithUser(ctx, token)

	// Is there no active session token with that value?
	if err == pgx.ErrNoRows {
		return nil, nil
	}

	if err != nil {
		return nil, err
	}
	return dbSessionTokenWithUserToApp(st), nil
}

func (ds *DataStore) DeleteSessionToken(ctx context.Context, id uuid.UUID) error {
	return ds.q.DeleteSessionToken(ctx, id)
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
