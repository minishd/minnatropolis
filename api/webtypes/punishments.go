package webtypes

import (
	"encoding/json/v2"

	"github.com/minishd/minnatropolis/datastore"
)

// Shared read/write API type for punishment types
type PunishmentType string

// Maps directly to datastore types for now
const (
	PunishmentMute      = PunishmentType(datastore.PunishmentMute)
	PunishmentBan       = PunishmentType(datastore.PunishmentBan)
	PunishmentBanSilent = PunishmentType(datastore.PunishmentBanSilent)
)

func (pt *PunishmentType) unmarshal(s string) error {
	// Make sure it's a valid variant
	switch PunishmentType(s) {
	case PunishmentMute, PunishmentBan, PunishmentBanSilent:
		*pt = PunishmentType(s)
		return nil
	default:
		return ErrInvalidEnum
	}
}

func (pt *PunishmentType) UnmarshalText(text []byte) error {
	return pt.unmarshal(string(text))
}

func (pt *PunishmentType) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}

	return pt.unmarshal(s)
}

func (pt PunishmentType) MarshalJSON() ([]byte, error) {
	return json.Marshal(string(pt))
}

func (pt PunishmentType) ToDS() datastore.PunishmentType {
	// For now
	return datastore.PunishmentType(pt)
}

func WPunishmentType(pt datastore.PunishmentType) PunishmentType {
	// For now.....
	return PunishmentType(pt)
}
