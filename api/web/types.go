package web

// This file contains models to be used by request or response types
// Some types may map 1-to-1, but we still want to keep them separate
// from datastore types...
// The ones here will usually implement MarshalJSON and such

import (
	"encoding/json"
	"fmt"

	"github.com/minishd/minnatropolis/datastore"
)

type UnmarshalError struct{ reason string }

var (
	ErrInvalidEnum = &UnmarshalError{"enum value is invalid"}
)

func (ue *UnmarshalError) Error() string {
	return fmt.Sprintf("unmarshal error (%s)", ue.reason)
}

// Shared read/write API type for punishment types
type PunishmentType string

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
