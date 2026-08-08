package datastore

import (
	"context"
	"errors"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/minishd/minnatropolis/queries"
)

var (
	ErrMissingTypeForDB  = errors.New("no corresponding type (app->db)")
	ErrMissingTypeForApp = errors.New("no corresponding type (db->app)")

	ErrNotUnique  = errors.New("not unique")
	ErrFailsCheck = errors.New("fails check")
)

type DataStore struct {
	q *queries.Queries
}

func New(q *queries.Queries) *DataStore {
	return &DataStore{q}
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
