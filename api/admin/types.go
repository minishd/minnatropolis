package admin

import (
	"github.com/minishd/minnatropolis/api/webtypes"
	"github.com/minishd/minnatropolis/datastore"
)

type usersSortBy string

const (
	usersSortUsername     usersSortBy = "username"
	usersSortRegisterDate usersSortBy = "register_date"
	usersSortLastBanDate  usersSortBy = "last_ban_date"
	usersSortLastSeenDate usersSortBy = "last_seen_date"
)

func (sb *usersSortBy) UnmarshalText(b []byte) error {
	// Make sure it's a valid variant
	switch usersSortBy(b) {
	case usersSortUsername, usersSortRegisterDate,
		usersSortLastBanDate, usersSortLastSeenDate:
		*sb = usersSortBy(b)
		return nil
	default:
		return webtypes.ErrInvalidEnum
	}
}

func (sb usersSortBy) ToDS() datastore.UserSearchSortType {
	switch sb {
	case usersSortUsername:
		return datastore.UserSearchSortUsername
	case usersSortRegisterDate:
		return datastore.UserSearchSortRegisterDate
	case usersSortLastBanDate:
		return datastore.UserSearchSortLastBanDate
	case usersSortLastSeenDate:
		return datastore.UserSearchSortLastSeenDate
	default:
		panic("invalid users-sort to datastore")
	}
}
