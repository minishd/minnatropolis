package datastore

import (
	"context"
	"net/netip"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/minishd/minnatropolis/datastore/scan2"
)

type PwHashType string

const (
	PwHashArgon2id PwHashType = "argon2id"
)

func (e *PwHashType) Scan(src any) error {
	s := scanStringType[PwHashType](src)
	switch s {
	case PwHashArgon2id: // valid variants
		*e = s
		return nil
	default:
		panic("unknown pw hash type from db")
	}
}

type Role string

const (
	RoleDefault              Role = "default"
	RoleMusicContributor     Role = "music_contributor"
	RoleBookCoverContributor Role = "book_cover_contributor"
	RoleDeveloper            Role = "developer"
	RoleSiteDeveloper        Role = "site_developer"
	RoleModerator            Role = "moderator"
	RoleAdministrator        Role = "administrator"
)

func (e *Role) Scan(src any) error {
	s := scanStringType[Role](src)
	switch s {
	case RoleDefault, RoleMusicContributor, RoleBookCoverContributor,
		RoleDeveloper, RoleSiteDeveloper,
		RoleModerator, RoleAdministrator: // valid variants
		*e = s
		return nil
	default:
		panic("unknown role from db")
	}
}

type PermissionLevel = int

const (
	PermissionLevelNone = iota
	PermissionLevelModerator
	PermissionLevelAdministrator
)

// Get the permission level of a role.
// Permission level is used to check if somebody has
// "at least" a role with a certain amount of privilege.
func (r Role) PermissionLevel() int {
	switch r {
	case RoleModerator:
		return PermissionLevelModerator
	case RoleAdministrator:
		return PermissionLevelAdministrator
	default:
		return PermissionLevelNone
	}
}

type User struct {
	ID                uuid.UUID   `scan2:"0"`
	CreatedAt         time.Time   `scan2:"1"`
	Username          string      `scan2:"2"`
	PwHashType        PwHashType  `scan2:"3"`
	PwHash            string      `scan2:"4"`
	LastVisitAt       time.Time   `scan2:"5"`
	LastSeenIP        *netip.Addr `scan2:"6"`
	ModNote           string      `scan2:"7"`
	CurrentPunishment *Punishment `scan2:"9"`
	Role              Role        `scan2:"8"`
}

func (u *User) HasPunishment(pt PunishmentType) bool {
	if punish := u.CurrentPunishment; punish != nil && punish.PunishmentType == pt {
		return true
	}
	return false
}

const sqlCreateUser = `
INSERT INTO users (username, pw_hash_type, pw_hash, current_punishment)
VALUES ($1, $2, $3, $4)
RETURNING id, created_at, username, pw_hash_type, pw_hash, last_visit_at, last_seen_ip, mod_note, role`

type NewUser struct {
	ID          uuid.UUID   `scan2:"0"`
	CreatedAt   time.Time   `scan2:"1"`
	Username    string      `scan2:"2"`
	PwHashType  PwHashType  `scan2:"3"`
	PwHash      string      `scan2:"4"`
	LastVisitAt time.Time   `scan2:"5"`
	LastSeenIP  *netip.Addr `scan2:"6"`
	ModNote     string      `scan2:"7"`
	Role        Role        `scan2:"8"`
}

func (ds *DataStore) CreateUser(username, pwHash string, pwHashType PwHashType,
	currentPunishID *uuid.UUID) (*NewUser, error) {
	// There isn't an easy way to fetch punishment data in an INSERT clause
	// that I know of, so we return a partial user
	ctx := context.Background()
	newUser, err := scan2.ScanOne2[NewUser](ctx, ds.pool, sqlCreateUser, username, pwHashType, pwHash, currentPunishID)

	if err != nil {
		// If the username is already taken,
		// if the username is not valid..
		if err := checkPgError(err); err != nil {
			return nil, err
		}

		return nil, err
	}

	// Return the partial user
	return newUser, nil
}

const sqlGetUserByUsername = `
SELECT uf.*
FROM users_full uf
WHERE u.username = $1`

func (ds *DataStore) GetUserByUsername(username string) (*User, error) {
	ctx := context.Background()
	return scan2.ScanOne2[User](ctx, ds.pool, sqlGetUserByUsername, username)
}

const sqlUpdateUserLastVisit = `
UPDATE users
SET last_visit_at = $2,
    last_seen_ip  = $3
WHERE id = $1`

func (ds *DataStore) UpdateUserLastVisit(id uuid.UUID, visitTime time.Time, visitIP netip.Addr) error {
	ctx := context.Background()
	_, err := ds.pool.Exec(ctx, sqlUpdateUserLastVisit, id, visitTime, visitIP)
	return err
}

const sqlGetUserBlockList = `
SELECT uf.*
FROM block_relations br
JOIN users_full uf ON uf.id = br.blocked_user
WHERE br.origin_user = $1`

func getUserBlockList(ctx context.Context, tx scan2.Querier, originUser uuid.UUID) ([]*User, error) {
	users, err := scan2.ScanMany2[User](ctx, tx, sqlGetUserBlockList, originUser)
	if err != nil {
		return nil, err
	}

	return users, nil
}

func (ds *DataStore) GetBlockedUsers(originUser uuid.UUID) ([]*User, error) {
	ctx := context.Background()
	return getUserBlockList(ctx, ds.pool, originUser)
}

const sqlInsertBlockRelation = `
INSERT INTO block_relations (origin_user, blocked_user)
VALUES ($1, $2)`

func insertBlockRelation(ctx context.Context, tx pgx.Tx, originUser, blockedUser uuid.UUID) error {
	// Insert the relation
	if _, err := tx.Exec(ctx, sqlInsertBlockRelation, originUser, blockedUser); err != nil {
		// We require each relation to be unique,
		// so check for unique violation error.
		// Also bad user IDs.
		if err := checkPgError(err); err != nil {
			return err
		}

		return err
	}

	// There was no problem
	return nil
}

func (ds *DataStore) AddBlockRelation(originUser, blockedUser uuid.UUID) ([]*User, error) {
	ctx := context.Background()

	tx, err := ds.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	// Create the block relation
	if err := insertBlockRelation(ctx, tx, originUser, blockedUser); err != nil {
		return nil, err
	}

	// Get user's new block list
	users, err := getUserBlockList(ctx, tx, originUser)
	if err != nil {
		return nil, err
	}

	// Commit the transaction
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	// Return the list of blocked users
	return users, nil
}

const sqlDeleteBlockRelation = `
DELETE FROM block_relations
WHERE origin_user = $1
  AND blocked_user = $2`

func deleteBlockRelation(ctx context.Context, tx pgx.Tx, originUser, blockedUser uuid.UUID) error {
	return mustOneFound(tx.Exec(ctx, sqlDeleteBlockRelation, originUser, blockedUser))
}

func (ds *DataStore) RemoveBlockRelation(originUser, blockedUser uuid.UUID) ([]*User, error) {
	ctx := context.Background()

	tx, err := ds.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	// Delete the block relation
	if err := deleteBlockRelation(ctx, tx, originUser, blockedUser); err != nil {
		return nil, err
	}

	// Get the new block list
	users, err := getUserBlockList(ctx, tx, originUser)
	if err != nil {
		return nil, err
	}

	// Commit the transaction
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	// Return the new block list
	return users, nil
}
