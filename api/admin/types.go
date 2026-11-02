package admin

import (
	"github.com/minishd/minnatropolis/api/web"
)

type usersSort string

const (
	usersSortUsername     usersSort = "username"
	usersSortEmail        usersSort = "email"
	usersSortRegisterDate usersSort = "register_date"
	usersSortLastBanDate  usersSort = "last_ban_date"
	usersSortLastSeenDate usersSort = "last_seen_date"
)

func (us *usersSort) unmarshal(s string) error {
	// Make sure it's a valid variant
	switch usersSort(s) {
	case usersSortUsername, usersSortEmail, usersSortRegisterDate,
		usersSortLastBanDate, usersSortLastSeenDate:
		*us = usersSort(s)
		return nil
	default:
		return web.ErrInvalidEnum
	}
}

func (us *usersSort) UnmarshalText(b []byte) error {
	return us.unmarshal(string(b))
}

type sortOrder string

const (
	sortOrderAsc  sortOrder = "asc"
	sortOrderDesc sortOrder = "desc"
)

func (so *sortOrder) unmarshal(s string) error {
	// Make sure it's a valid variant
	switch sortOrder(s) {
	case sortOrderAsc, sortOrderDesc:
		*so = sortOrder(s)
		return nil
	default:
		return web.ErrInvalidEnum
	}
}

func (so *sortOrder) UnmarshalText(b []byte) error {
	return so.unmarshal(string(b))
}
