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
	return ds.q.InsertSessionToken(ctx, queries.InsertSessionTokenParams{
		ForUser:   forUser,
		Token:     token,
		ExpiresAt: expiresAt,
	})
	// Token collisions not accounted for
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

func getBlockedUsers(ctx context.Context, q *queries.Queries, originUser uuid.UUID) ([]*User, error) {
	users, err := q.GetUserBlockList(ctx, originUser)
	if err != nil {
		return nil, err
	}

	var appUsers []*User
	for _, user := range users {
		appUsers = append(appUsers, dbUserToApp(user))
	}
	return appUsers, nil
}

func (ds *DataStore) GetBlockedUsers(ctx context.Context, originUser uuid.UUID) ([]*User, error) {
	return getBlockedUsers(ctx, ds.q, originUser)
}

func (ds *DataStore) InsertBlockRelation(ctx context.Context, originUser, blockedUser uuid.UUID) ([]*User, error) {
	tx, err := ds.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	qtx := ds.q.WithTx(tx)

	err = qtx.InsertBlockRelation(ctx, queries.InsertBlockRelationParams{
		OriginUser:  originUser,
		BlockedUser: blockedUser,
	})
	// We require each relation to be unique,
	// so check for unique violation error.
	// Also bad user IDs.
	if err := checkPgError(err); err != nil {
		return nil, err
	}
	if err != nil {
		return nil, err
	}

	users, err := getBlockedUsers(ctx, qtx, originUser)
	if err != nil {
		return nil, err
	}

	err = tx.Commit(ctx)
	if err != nil {
		return nil, err
	}

	return users, nil
}

func (ds *DataStore) DeleteBlockRelation(ctx context.Context, originUser, blockedUser uuid.UUID) ([]*User, error) {
	tx, err := ds.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	qtx := ds.q.WithTx(tx)

	rows, err := qtx.DeleteBlockRelation(ctx, queries.DeleteBlockRelationParams{
		OriginUser:  originUser,
		BlockedUser: blockedUser,
	})
	if err != nil {
		return nil, err
	}
	if rows == 0 {
		return nil, ErrNotFound
	}

	users, err := getBlockedUsers(ctx, qtx, originUser)
	if err != nil {
		return nil, err
	}

	err = tx.Commit(ctx)
	if err != nil {
		return nil, err
	}

	return users, nil
}

func (ds *DataStore) GetUserParty(ctx context.Context, user uuid.UUID) (*Party, error) {
	party, err := ds.q.GetUserParty(ctx, user)

	// Are they not in a party?
	if err == pgx.ErrNoRows {
		return nil, nil
	}

	if err != nil {
		return nil, err
	}
	return dbPartyToApp(party), nil
}

func (ds *DataStore) GetPartyMembers(ctx context.Context, party uuid.UUID) ([]*User, error) {
	users, err := ds.q.GetPartyMembers(ctx, party)
	if err != nil {
		return nil, err
	}

	var appUsers []*User
	for _, user := range users {
		appUsers = append(appUsers, dbUserToApp(user))
	}
	return appUsers, nil
}

// Creates a party with the user in it
func (ds *DataStore) CreateParty(ctx context.Context, forUser uuid.UUID, name string) (*Party, error) {
	tx, err := ds.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	qtx := ds.q.WithTx(tx)

	party, err := qtx.CreateParty(ctx, name)
	if err := checkPgError(err); err != nil {
		return nil, err
	}
	if err != nil {
		return nil, err
	}

	err = qtx.InsertPartyMember(ctx, queries.InsertPartyMemberParams{
		Party:      party.ID,
		MemberUser: forUser,
	})
	// Users can only be in one party
	if err := checkPgError(err); err != nil {
		return nil, err
	}
	if err != nil {
		return nil, err
	}

	err = tx.Commit(ctx)
	if err != nil {
		return nil, err
	}

	return dbPartyToApp(party), nil
}

// Adds a user to a party, then returns the party
func (ds *DataStore) InsertPartyMember(ctx context.Context, party, user uuid.UUID) (*Party, error) {
	tx, err := ds.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	qtx := ds.q.WithTx(tx)

	err = qtx.InsertPartyMember(ctx, queries.InsertPartyMemberParams{
		Party:      party,
		MemberUser: user,
	})
	// Already in a party, or no such party
	if err := checkPgError(err); err != nil {
		return nil, err
	}
	if err != nil {
		return nil, err
	}

	joined, err := qtx.GetUserParty(ctx, user)
	if err != nil {
		return nil, err
	}

	err = tx.Commit(ctx)
	if err != nil {
		return nil, err
	}

	return dbPartyToApp(joined), nil
}

// Removes a user from their party, deleting it if it's empty
func (ds *DataStore) DeletePartyMember(ctx context.Context, user uuid.UUID) error {
	tx, err := ds.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	qtx := ds.q.WithTx(tx)

	party, err := qtx.DeletePartyMember(ctx, user)
	// Were they not in a party?
	if err == pgx.ErrNoRows {
		return ErrNotFound
	}
	if err != nil {
		return err
	}

	err = qtx.DeletePartyIfEmpty(ctx, party)
	if err != nil {
		return err
	}

	return tx.Commit(ctx)
}
