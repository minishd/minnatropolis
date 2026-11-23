package webtypes

import (
	"encoding/json/v2"

	"github.com/minishd/minnatropolis/datastore"
)

// Shared API type for user roles
type Role string

// Maps directly to datastore types for now as well ...
const (
	RoleDefault              = Role(datastore.RoleDefault)
	RoleMusicContributor     = Role(datastore.RoleMusicContributor)
	RoleBookCoverContributor = Role(datastore.RoleBookCoverContributor)
	RoleDeveloper            = Role(datastore.RoleDeveloper)
	RoleSiteDeveloper        = Role(datastore.RoleSiteDeveloper)
	RoleModerator            = Role(datastore.RoleModerator)
	RoleAdministrator        = Role(datastore.RoleAdministrator)
)

func (r *Role) unmarshal(s string) error {
	// Make sure it's a valid variant
	switch Role(s) {
	case RoleDefault, RoleMusicContributor, RoleBookCoverContributor,
		RoleDeveloper, RoleSiteDeveloper, RoleModerator, RoleAdministrator:
		*r = Role(s)
		return nil
	default:
		return ErrInvalidEnum
	}
}

func (r *Role) UnmarshalText(text []byte) error {
	return r.unmarshal(string(text))
}

func (r Role) MarshalJSON() ([]byte, error) {
	return json.Marshal(string(r))
}

func (r Role) ToDS() datastore.Role {
	// For now also
	return datastore.Role(r)
}

func WRole(r datastore.Role) Role {
	// For also now ...
	return Role(r)
}
