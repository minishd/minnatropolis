package datastore

import (
	"context"
	"net/netip"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/minishd/minnatropolis/queries"
)

type PwHashType string

const (
	PwHashArgon2id PwHashType = PwHashType(queries.PwHashTypeTArgon2id)
)

type Role string

const (
	RoleDefault              Role = Role(queries.RoleTDefault)
	RoleMusicContributor     Role = Role(queries.RoleTMusicContributor)
	RoleBookCoverContributor Role = Role(queries.RoleTBookCoverContributor)
	RoleDeveloper            Role = Role(queries.RoleTDeveloper)
	RoleSiteDeveloper        Role = Role(queries.RoleTSiteDeveloper)
	RoleModerator            Role = Role(queries.RoleTModerator)
	RoleAdministrator        Role = Role(queries.RoleTAdministrator)
)

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

func dbPwHashTypeToApp(dbPHT queries.PwHashTypeT) PwHashType {
	pht := PwHashType(dbPHT)

	switch pht {
	case PwHashArgon2id:
		break
	default:
		panic("unknown pw hash type from db")
	}

	return pht
}
func appPwHashTypeToDB(pht PwHashType) queries.PwHashTypeT {
	return queries.PwHashTypeT(pht)
}

func dbRoleToApp(dbRole queries.RoleT) Role {
	role := Role(dbRole)

	switch role {
	case RoleDefault, RoleMusicContributor, RoleBookCoverContributor,
		RoleDeveloper, RoleSiteDeveloper,
		RoleModerator, RoleAdministrator:
		break
	default:
		panic("unknown role from db")
	}

	return role
}

type User struct {
	ID                uuid.UUID
	CreatedAt         time.Time
	Username          string
	PwHashType        PwHashType
	PwHash            string
	LastVisitAt       time.Time
	LastSeenIP        *netip.Addr
	ModNote           string
	CurrentPunishment *Punishment
	Role              Role
}

func (u *User) HasPunishment(pt PunishmentType) bool {
	if punish := u.CurrentPunishment; punish != nil && punish.PunishmentType == pt {
		return true
	}
	return false
}

func dbUserToApp(u queries.User, p *queries.Punishment) *User {
	var punishment *Punishment
	if p != nil {
		punishment = dbPunishmentToApp(*p)
	}

	return &User{
		ID:                u.ID,
		CreatedAt:         u.CreatedAt,
		Username:          u.Username,
		PwHashType:        dbPwHashTypeToApp(u.PwHashType),
		PwHash:            u.PwHash,
		LastVisitAt:       u.LastVisitAt,
		LastSeenIP:        u.LastSeenIp,
		ModNote:           u.ModNote,
		CurrentPunishment: punishment,
		Role:              dbRoleToApp(u.Role),
	}
}

func (ds *DataStore) CreateUser(ctx context.Context, username, pwHash string, pwHashType PwHashType,
	currentPunishID *uuid.UUID) (*User, error) {

	user, err := ds.q.CreateUser(ctx, queries.CreateUserParams{
		Username:          username,
		PwHashType:        appPwHashTypeToDB(pwHashType),
		PwHash:            pwHash,
		CurrentPunishment: currentPunishID,
	})

	if err := checkPgError(err); err != nil {
		return nil, err
	}
	if err != nil {
		return nil, err
	}
	return dbUserToApp(user, nil), nil
}

func (ds *DataStore) GetUserByUsername(ctx context.Context, username string) (*User, error) {
	row, err := ds.q.GetUserByUsername(ctx, username)

	// Did we not find an account?
	if err == pgx.ErrNoRows {
		return nil, nil
	}

	if err != nil {
		return nil, err
	}
	return dbUserToApp(row.User, row.Punishment), nil
}

func (ds *DataStore) UpdateUserLastVisit(ctx context.Context, id uuid.UUID, visitTime time.Time) error {
	return ds.q.UpdateUserLastVisit(ctx, queries.UpdateUserLastVisitParams{
		ID:          id,
		LastVisitAt: visitTime,
	})
	// I thought that I checked for updates that
	// didn't affect any rows but I guess not
}

func getBlockedUsers(ctx context.Context, q *queries.Queries, originUser uuid.UUID) ([]*User, error) {
	rows, err := q.GetUserBlockList(ctx, originUser)
	if err != nil {
		return nil, err
	}

	appUsers := make([]*User, 0, len(rows))
	for _, row := range rows {
		appUsers = append(appUsers, dbUserToApp(row.User, row.Punishment))
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
