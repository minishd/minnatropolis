package datastore

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/minishd/minnatropolis/datastore/scan2"
)

type Party struct {
	ID        uuid.UUID `scan2:"0"`
	CreatedAt time.Time `scan2:"1"`
	Name      string    `scan2:"2"`
}

const sqlGetUserParty = `
SELECT p.*
FROM party_members pm
JOIN parties p ON p.id = pm.party
WHERE pm.member_user = $1;`

func getUserParty(ctx context.Context, tx scan2.Querier, userID uuid.UUID) (*Party, error) {
	party, err := scan2.ScanOne2[Party](ctx, tx, sqlGetUserParty, userID)

	if err != nil {
		// Are they not in a party?
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}

		return nil, err
	}

	return party, nil
}

func (ds *DataStore) GetUserParty(ctx context.Context, user uuid.UUID) (*Party, error) {
	return getUserParty(ctx, ds.pool, user)
}

const getPartyMembers = `
SELECT u.id, u.created_at, u.username, u.pw_hash_type, u.pw_hash, u.last_visit_at, u.last_seen_ip, u.mod_note, u.role, p.id, p.created_at, p.punishment_type, p.is_ip_enforced, p.is_disabled, p.expires_at, p.reason
FROM party_members pm
     JOIN users u       ON u.id = pm.member_user
LEFT JOIN punishments p ON p.id = u.current_punishment
                       AND p.is_disabled IS FALSE
                       AND (p.expires_at IS NULL OR CURRENT_TIMESTAMP < p.expires_at)
WHERE pm.party = $1
ORDER BY pm.created_at`

func (ds *DataStore) GetPartyMembers(ctx context.Context, party uuid.UUID) ([]*User, error) {
	users, err := scan2.ScanMany2[User](ctx, ds.pool, getPartyMembers, party)
	if err != nil {
		return nil, err
	}

	return users, nil
}

const sqlCreateParty = `
INSERT INTO parties (name)
VALUES ($1)
RETURNING id, created_at, name`

func createParty(ctx context.Context, tx pgx.Tx, name string) (*Party, error) {
	party, err := scan2.ScanOne2[Party](ctx, tx, sqlCreateParty, name)

	if err != nil {
		// Make sure we didn't get an error from an
		// invalid party name
		if err := checkPgError(err); err != nil {
			return nil, err
		}

		return nil, err
	}

	// Ok, return party
	return party, nil
}

const sqlInsertPartyMember = `
INSERT INTO party_members (party, member_user)
VALUES ($1, $2)`

func insertPartyMember(ctx context.Context, tx pgx.Tx, partyID, userID uuid.UUID) error {
	_, err := tx.Exec(ctx, sqlInsertPartyMember, partyID, userID)

	if err != nil {
		// If a user tries to join more than one party,
		// it causes a unique constraint violation
		// If a user tries to join a nonexistent party,
		// it raises unknown foreign key error
		if err := checkPgError(err); err != nil {
			return err
		}

		return err
	}

	// Ok, no error was raised
	return nil
}

// Creates a party with the user in it
func (ds *DataStore) CreatePartyWithUser(ctx context.Context, forUser uuid.UUID, name string) (*Party, error) {
	tx, err := ds.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	// Create underlying party
	party, err := createParty(ctx, tx, name)
	if err != nil {
		return nil, err
	}

	// Add creator to party
	err = insertPartyMember(ctx, tx, party.ID, forUser)
	if err != nil {
		return nil, err
	}

	// Commit transaction
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	// Return the created party
	return party, nil
}

// Adds a user to a party, then returns the party
func (ds *DataStore) AddUserToParty(ctx context.Context, party, user uuid.UUID) (*Party, error) {
	tx, err := ds.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	// Create party member row
	if err := insertPartyMember(ctx, tx, party, user); err != nil {
		return nil, err
	}

	// Get the party
	joined, err := getUserParty(ctx, tx, user)
	if err != nil {
		return nil, err
	}

	// Commit the transaction
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	// Return the party they joined
	return joined, nil
}

const sqlDeletePartyMember = `
DELETE FROM party_members
WHERE member_user = $1
RETURNING party`

func deletePartyMember(ctx context.Context, tx pgx.Tx, memberUserID uuid.UUID) (*uuid.UUID, error) {
	row := tx.QueryRow(ctx, sqlDeletePartyMember, memberUserID)

	// Read what party they were in
	var partyID uuid.UUID
	err := row.Scan(&partyID)
	if err != nil {
		// Were they not in a party?
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}

		return nil, err
	}

	// Return party ID
	return &partyID, nil
}

const sqlDeletePartyIfEmpty = `
DELETE FROM parties p
WHERE p.id = $1
  AND NOT EXISTS (
    SELECT 1 FROM party_members pm
    WHERE pm.party = p.id
  )`

func deletePartyIfEmpty(ctx context.Context, tx pgx.Tx, partyID uuid.UUID) error {
	_, err := tx.Exec(ctx, sqlDeletePartyIfEmpty, partyID)
	return err
}

// Removes a user from their party, deleting it if it's empty
func (ds *DataStore) RemoveUserFromParty(ctx context.Context, user uuid.UUID) error {
	tx, err := ds.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	// Try to delete the party member row
	partyID_, err := deletePartyMember(ctx, tx, user)
	if err != nil {
		return err
	}
	partyID := *partyID_

	// If the party is now empty, delete it
	if err := deletePartyIfEmpty(ctx, tx, partyID); err != nil {
		return err
	}

	// Commit the transaction
	return tx.Commit(ctx)
}
