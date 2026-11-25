package datastore

import (
	"errors"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/minishd/minnatropolis/datastore/scan2"
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
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *DataStore {
	return &DataStore{
		pool: pool,
	}
}

// Register types that we will be taking
// from the database
func init() {
	scan2.Register2[SessionToken]()
	scan2.Register2[User]()
	scan2.Register2[Punishment]()
	scan2.Register2[Party]()
	scan2.Register2[NewUser]()
	scan2.Register2[PunishmentUser]()
	scan2.Register2[PunishmentIP]()
}

// Make sure an Exec() call affected exactly
// one row
func mustOneFound(ct pgconn.CommandTag, err error) error {
	if err != nil {
		return err
	}
	if ct.RowsAffected() != 1 {
		return ErrNotFound
	}
	return nil
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

// Helper function for our custom types' scanners
func scanStringType[T ~string](src any) T {
	switch s := src.(type) {
	case string:
		return T(s)
	case []byte:
		return T(s)
	default:
		panic("unsupported scan type")
	}
}

// Shared sort-direction type for search queries
type SortDirection int

const (
	SortAscending = iota
	SortDescending
)

func (sd SortDirection) SQL() string {
	switch sd {
	case SortAscending:
		return "ASC"
	case SortDescending:
		return "DESC"
	default:
		panic("invalid sort direction into sql")
	}
}
