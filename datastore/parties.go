package datastore

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/minishd/minnatropolis/queries"
)

type Party struct {
	ID        uuid.UUID
	CreatedAt time.Time
	Name      string
}

func dbPartyToApp(in queries.Party) *Party {
	return &Party{
		ID:        in.ID,
		CreatedAt: in.CreatedAt,
		Name:      in.Name,
	}
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
	rows, err := ds.q.GetPartyMembers(ctx, party)
	if err != nil {
		return nil, err
	}

	appUsers := make([]*User, 0, len(rows))
	for _, row := range rows {
		appUsers = append(appUsers, dbUserToApp(row.User, row.Punishment))
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
