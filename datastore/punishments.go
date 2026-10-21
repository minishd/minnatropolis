package datastore

import (
	"context"
	"net/netip"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/minishd/minnatropolis/queries"
)

type PunishmentType string

const (
	PunishmentMute      = PunishmentType(queries.PunishmentTypeTMute)
	PunishmentBan       = PunishmentType(queries.PunishmentTypeTBan)
	PunishmentBanSilent = PunishmentType(queries.PunishmentTypeTBanSilent)
)

func dbPunishmentTypeToApp(dbPT queries.PunishmentTypeT) PunishmentType {
	pt := PunishmentType(dbPT)

	switch pt {
	case PunishmentMute, PunishmentBan, PunishmentBanSilent:
		break
	default:
		panic("unknown punishment type from db")
	}

	return pt
}

type Punishment struct {
	ID             uuid.UUID
	CreatedAt      time.Time
	PunishmentType PunishmentType
	ExpiresAt      *time.Time
	Reason         string
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

func dbPunishmentToApp(in queries.Punishment) *Punishment {
	return &Punishment{
		ID:             in.ID,
		CreatedAt:      in.CreatedAt,
		PunishmentType: dbPunishmentTypeToApp(in.PunishmentType),
		ExpiresAt:      in.ExpiresAt,
		Reason:         in.Reason,
	}
}

func (ds *DataStore) GetActivePunishmentForIP(ctx context.Context, ip netip.Addr) (*Punishment, error) {
	punishment, err := ds.q.GetActivePunishmentForIP(ctx, ip)

	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return dbPunishmentToApp(punishment), nil
}
