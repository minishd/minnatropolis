package datastore

import (
	"context"
	"errors"

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

func (ds *DataStore) CreateSessionToken(ctx context.Context, forUser uuid.UUID, token string) (*SessionToken, error) {
	st, err := ds.q.CreateSessionToken(ctx, queries.CreateSessionTokenParams{
		ForUser: forUser,
		Token:   token,
	})
	// Token collisions not accounted for..

	if err != nil {
		return nil, err
	}
	return dbSessionTokenToApp(st), nil
}
