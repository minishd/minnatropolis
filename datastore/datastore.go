package datastore

import (
	"errors"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/minishd/minnatropolis/queries"
)

var (
	ErrNotUnique   = errors.New("not unique")
	ErrFailsCheck  = errors.New("fails check")
	ErrNotFound    = errors.New("not found")
	ErrUnknownFkey = errors.New("foreign-key violation")
)

// Database abstraction to decouple
// DB code from rest of app code
type DataStore struct {
	q    *queries.Queries
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *DataStore {
	return &DataStore{
		q:    queries.New(pool),
		pool: pool,
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
		case pgerrcode.ForeignKeyViolation:
			return ErrUnknownFkey
		}
	}
	return nil
}
