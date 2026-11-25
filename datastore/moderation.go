package datastore

import (
	"context"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/minishd/minnatropolis/datastore/scan2"
)

type UserSearchFilters struct {
	Username      string          // optional "starts with" filter
	UserID        uuid.UUID       // optional "equals" filter
	HasPunishment *PunishmentType // optional "equals" filter
	SharesIPWith  uuid.UUID       // optional "equals" filter
	HasRole       *Role           // optional "equals" filter
}

type UserSearchSortType int

const (
	UserSearchSortUsername UserSearchSortType = iota + 1
	UserSearchSortRegisterDate
	UserSearchSortLastBanDate
	UserSearchSortLastSeenDate
)

func (st UserSearchSortType) SQL() string {
	switch st {
	case UserSearchSortUsername:
		return "uf.username"
	case UserSearchSortRegisterDate:
		return "uf.created_at"
	case UserSearchSortLastBanDate:
		return "uf.p_created_at"
	case UserSearchSortLastSeenDate:
		return "uf.p_last_visit_at"
	default:
		panic("invalid user search sort type to sql")
	}
}

type UserSearchSort struct {
	By        UserSearchSortType
	Direction SortDirection
}

const sqlSearchUsers = `
SELECT uf.*
FROM users_full uf
{shares_ip_join}
{where}
ORDER BY {sort_column} {sort_direction}
LIMIT {limit}
OFFSET {offset}`

const sqlSearchUsersSharesIPJoin = `
LEFT JOIN users u2 ON u2.id = @shares_ip_with`

const sqlSearchUsersFilterUsername = `
uf.username ILIKE
	replace( replace( replace( @username, '\', '\\' ), '%', '\%' ), '_', '\_' ) || '%'`

const sqlSearchUsersFilterUserID = `
uf.id = @user_id`

const sqlSearchUsersFilterHasPunishment = `
uf.p_punishment_type = @punishment_type`

const sqlSearchUsersFilterSharesIPWith = `
uf.last_seen_ip = u2.last_seen_ip`

const sqlSearchUsersFilterHasRole = `
uf.role = @role`

func (ds *DataStore) SearchUsers(filters UserSearchFilters, sort UserSearchSort, offset, limit uint64) ([]*User, error) {
	ctx := context.Background()

	// Named args for filters
	args := pgx.StrictNamedArgs{}

	// WHERE clauses we'll use for filtering
	var where []string
	// The shares-IP LEFT JOIN, if we are
	// using that filter
	var sharesIPJoin string

	// Add clauses and args for the filters we need
	if filters.Username != "" {
		where = append(where, sqlSearchUsersFilterUsername)
		args["username"] = filters.Username
	}
	if filters.UserID != uuid.Nil {
		where = append(where, sqlSearchUsersFilterUserID)
		args["user_id"] = filters.UserID
	}
	if filters.HasPunishment != nil {
		where = append(where, sqlSearchUsersFilterHasPunishment)
		args["punishment_type"] = filters.HasPunishment
	}
	if filters.SharesIPWith != uuid.Nil {
		where = append(where, sqlSearchUsersFilterSharesIPWith)
		sharesIPJoin = sqlSearchUsersSharesIPJoin
		args["shares_ip_with"] = filters.SharesIPWith
	}
	if filters.HasRole != nil {
		where = append(where, sqlSearchUsersFilterHasRole)
		args["role"] = filters.HasRole
	}

	// Construct WHERE clause
	var whereClause string
	if len(where) != 0 {
		whereClause = "WHERE " + strings.Join(where, " AND ")
	}

	// Construct full query
	query := sqlSearchUsers
	query = strings.Replace(query, "{shares_ip_join}", sharesIPJoin, 1)
	query = strings.Replace(query, "{where}", whereClause, 1)
	query = strings.Replace(query, "{sort_column}", sort.By.SQL(), 1)
	query = strings.Replace(query, "{sort_direction}", sort.Direction.SQL(), 1)
	query = strings.Replace(query, "{limit}", strconv.FormatUint(limit, 10), 1)
	query = strings.Replace(query, "{offset}", strconv.FormatUint(offset, 10), 1)

	// Query users
	users, err := scan2.ScanMany2[User](ctx, ds.pool, query, args)
	if err != nil {
		return nil, err
	}

	// Return users
	return users, nil
}
