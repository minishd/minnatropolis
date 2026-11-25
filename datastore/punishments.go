package datastore

import (
	"context"
	"net/netip"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/minishd/minnatropolis/datastore/scan2"
)

type PunishmentType string

const (
	PunishmentMute      PunishmentType = "mute"
	PunishmentBan       PunishmentType = "ban"
	PunishmentBanSilent PunishmentType = "ban_silent"
)

func (e *PunishmentType) Scan(src any) error {
	s := scanStringType[PunishmentType](src)
	switch s {
	case PunishmentMute, PunishmentBan, PunishmentBanSilent: // valid variants
		*e = s
		return nil
	default:
		panic("unknown punishment type from db")
	}
}

type Punishment struct {
	ID             uuid.UUID      `scan2:"0"`
	CreatedAt      time.Time      `scan2:"1"`
	PunishmentType PunishmentType `scan2:"2"`
	IsIPEnforced   bool           `scan2:"3"`
	IsDisabled     bool           `scan2:"4"`
	ExpiresAt      *time.Time     `scan2:"5"`
	Reason         string         `scan2:"6"`
}

func (p *Punishment) IsHidden() bool {
	return p.PunishmentType == PunishmentBanSilent
}

func (p *Punishment) DoesBlockRegistration() bool {
	return p.PunishmentType == PunishmentBan
}

func (p *Punishment) DoesMakePlayerInvisible() bool {
	switch p.PunishmentType {
	case PunishmentBan, PunishmentBanSilent:
		return true
	case PunishmentMute:
		return false
	default:
		panic("unhandled punishment type")
	}
}

const sqlGetActivePunishmentForIP = `
SELECT p.id, p.created_at, p.punishment_type, p.is_ip_enforced, p.is_disabled, p.expires_at, p.reason FROM punishments p
JOIN punishment_ips pi ON pi.punishment = p.id
WHERE pi.ip = $1
  AND pi.ended_at IS NULL
  AND (p.expires_at IS NULL OR CURRENT_TIMESTAMP < p.expires_at)`

func (ds *DataStore) GetActivePunishmentForIP(ip netip.Addr) (*Punishment, error) {
	ctx := context.Background()
	return scan2.ScanOne2[Punishment](ctx, ds.pool, sqlGetActivePunishmentForIP, ip)
}

type PunishmentIP struct {
	ID         uuid.UUID  `scan2:"0"`
	CreatedAt  time.Time  `scan2:"1"`
	IP         netip.Addr `scan2:"2"`
	Punishment uuid.UUID  `scan2:"3"`
	EndedAt    *time.Time `scan2:"4"`
}

const sqlSetUserIPPunishmentGetActiveRelation = `
SELECT pi.*
FROM users u
JOIN punishment_ips pi ON pi.ip = u.last_seen_ip
JOIN punishments p     ON p.id = pi.punishment
WHERE pi.ip = $1
  AND p.expires_at > CURRENT_TIMESTAMP`

const sqlSetUserIPPunishmentEndRelation = `
UPDATE punishment_ips
SET ended_at = CURRENT_TIMESTAMP
WHERE id = $1`

const sqlSetUserIPPunishmentMakeRelation = `
INSERT INTO punishment_ips (ip, punishment)
VALUES ($1, $2)`

func setUserIPCurrentPunishment(ctx context.Context, tx pgx.Tx, userID uuid.UUID, punishID uuid.UUID) error {
	// Make sure the IP doesn't already have an active punishment
	rel, err := scan2.ScanOne2[PunishmentIP](ctx, tx, sqlSetUserIPPunishmentGetActiveRelation, userID)
	if err != nil {
		return err
	}

	// If there is an active punishment, we need to
	// end it before we apply the new one
	if rel != nil {
		if err := mustOneFound(tx.Exec(ctx, sqlSetUserIPPunishmentEndRelation, rel.ID)); err != nil {
			return err
		}
	}

	// Next, create the relation between the punishment and the IP
	if _, err := tx.Exec(ctx, sqlSetUserIPPunishmentMakeRelation, rel.IP, punishID); err != nil {
		return err
	}

	return nil
}

type PunishmentUser struct {
	ID          uuid.UUID  `scan2:"0"`
	CreatedAt   time.Time  `scan2:"1"`
	RelatedUser uuid.UUID  `scan2:"2"`
	Punishment  uuid.UUID  `scan2:"3"`
	EndedAt     *time.Time `scan2:"4"`
}

const sqlSetUserPunishmentGetActiveRelation = `
SELECT pu.*
FROM punishment_users pu
JOIN punishments p ON p.id = pu.punishment
WHERE pu.related_user = $1
  AND p.expires_at > CURRENT_TIMESTAMP`

const sqlSetUserPunishmentEndRelation = `
UPDATE punishment_users
SET ended_at = CURRENT_TIMESTAMP
WHERE id = $1`

const sqlSetUserPunishmentMakeRelation = `
INSERT INTO punishment_users (related_user, punishment)
VALUES ($1, $2)`

func setUserCurrentPunishment(ctx context.Context, tx pgx.Tx, userID uuid.UUID, punishID uuid.UUID) error {
	// Make sure they don't have a relation to a punishment that
	// has already expired.
	rel, err := scan2.ScanOne2[PunishmentUser](ctx, tx, sqlSetUserPunishmentGetActiveRelation, userID)
	if err != nil {
		return err
	}

	// If there is a non-ended relation, we need to end it first
	// (users can only have one active punishment)
	if rel != nil {
		if err := mustOneFound(tx.Exec(ctx, sqlSetUserPunishmentEndRelation, rel.ID)); err != nil {
			return err
		}
	}

	// Next, create the rel
	// ation between the punishment and the user
	if _, err := tx.Exec(ctx, sqlSetUserPunishmentMakeRelation, userID, punishID); err != nil {
		return err
	}

	return nil
}

const sqlPunishUserMakePunishment = `
INSERT INTO punishments (punishment_type, is_ip_enforced, expires_at, reason)
VALUES ($1, $2, $3, $4)
RETURNING *`

const sqlPunishUserFindUsersWithIP = `
SELECT uf_other.*
FROM users u
JOIN users_full uf_other ON u.last_seen_ip = uf_other.last_seen_ip
WHERE u.id = $1`

type ApplyPunishmentError struct {
	User  *User
	Error error
}

type ApplyPunishmentErrors struct {
	Errors []ApplyPunishmentError
}

func (aperrs ApplyPunishmentErrors) Error() string {
	var sb strings.Builder

	sb.WriteString("couldn't apply punishments: ")
	for i, aperr := range aperrs.Errors {
		sb.WriteString(aperr.User.ID.String())
		sb.WriteString("(")
		sb.WriteString(aperr.Error.Error())
		sb.WriteString(")")
		if i+1 != len(aperrs.Errors) {
			sb.WriteString("; ")
		}
	}

	return sb.String()
}

func (ds *DataStore) PunishUser(ptype PunishmentType, userID uuid.UUID, ipEnforced bool,
	expiresAt *time.Time, reason string) (p *Punishment, err error) {
	ctx := context.Background()

	// Begin transaction
	tx, err := ds.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	// TODO: Check if user is of sufficient rank
	// TODO: Disconnect the user (to be done in admin handler)

	// First, create the punishment
	punish, err := scan2.ScanOne2[Punishment](ctx, tx, sqlPunishUserMakePunishment, ptype, ipEnforced, expiresAt, reason)
	if err != nil {
		return nil, err
	}

	// If it is IP enforced, we need to punish the IP itself
	// as well as all users sharing it.
	// If it is not, we just punish the one user.
	if !ipEnforced {
		// Apply the punishment to the user.
		if err := setUserCurrentPunishment(ctx, tx, userID, punish.ID); err != nil {
			return nil, err
		}
	} else {
		// Apply it to the IP
		if err := setUserIPCurrentPunishment(ctx, tx, userID, punish.ID); err != nil {
			return nil, err
		}

		// Get users sharing the IP
		users, err := scan2.ScanMany2[User](ctx, tx, sqlPunishUserFindUsersWithIP, userID)
		if err != nil {
			return nil, err
		}

		// Apply punishment to all of them
		// This could be batched but I would need to batch each
		// individual step of the punishment application process
		// It is insignificant unless a user has a very large amount
		// of alt accounts anyway
		var aperrs ApplyPunishmentErrors
		for _, user := range users {
			if err := setUserCurrentPunishment(ctx, tx, user.ID, punish.ID); err != nil {
				aperr := ApplyPunishmentError{user, err}
				aperrs.Errors = append(aperrs.Errors, aperr)
			}
		}
		if len(aperrs.Errors) != 0 {
			return nil, aperrs
		}
	}

	// We did everything we wanted to so commit
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	return punish, nil
}
