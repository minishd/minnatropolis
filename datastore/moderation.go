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
		return "u.username"
	case UserSearchSortRegisterDate:
		return "u.created_at"
	case UserSearchSortLastBanDate:
		return "p.created_at"
	case UserSearchSortLastSeenDate:
		return "u.last_visit_at"
	default:
		panic("invalid user search sort type to sql")
	}
}

type UserSearchSort struct {
	By        UserSearchSortType
	Direction SortDirection
}

const sqlSearchUsers = `
SELECT u.id, u.created_at, u.username, u.pw_hash_type, u.pw_hash, u.last_visit_at, u.last_seen_ip, u.mod_note, u.role, p.id, p.created_at, p.punishment_type, p.is_ip_enforced, p.is_disabled, p.expires_at, p.reason
FROM users u
LEFT JOIN punishments p ON p.id = u.current_punishment
                       AND p.is_disabled IS FALSE
                       AND (p.expires_at IS NULL OR CURRENT_TIMESTAMP < p.expires_at)
{shares_ip_join}
{where}
ORDER BY {sort_column} {sort_direction}
LIMIT {limit}
OFFSET {offset}`

const sqlSearchUsersSharesIPJoin = `
LEFT JOIN users u2 ON u2.id = @shares_ip_with`

const sqlSearchUsersFilterUsername = `
u.username ILIKE
	replace( replace( replace( @username, '\', '\\' ), '%', '\%' ), '_', '\_' ) || '%'`

const sqlSearchUsersFilterUserID = `
u.id = @user_id`

const sqlSearchUsersFilterHasPunishment = `
p.punishment_type = @punishment_type`

const sqlSearchUsersFilterSharesIPWith = `
u.last_seen_ip = u2.last_seen_ip`

const sqlSearchUsersFilterHasRole = `
u.role = @role`

func (ds *DataStore) SearchUsers(ctx context.Context, filters UserSearchFilters, sort UserSearchSort, offset, limit uint64) ([]*User, error) {
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
