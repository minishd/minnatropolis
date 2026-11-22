package datastore

import (
	"context"
	"errors"
	"net/netip"
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
  AND p.is_disabled IS FALSE
  AND (p.expires_at IS NULL OR CURRENT_TIMESTAMP < p.expires_at)`

func (ds *DataStore) GetActivePunishmentForIP(ctx context.Context, ip netip.Addr) (*Punishment, error) {
	punishment, err := scan2.ScanOne2[Punishment](ctx, ds.pool, sqlGetActivePunishmentForIP, ip)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}

		return nil, err
	}

	return punishment, nil
}
