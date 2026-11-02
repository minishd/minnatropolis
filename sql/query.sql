-- name: CreateUser :one
INSERT INTO users (username, pw_hash_type, pw_hash, current_punishment)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetUserByUsername :one
SELECT sqlc.embed(u), sqlc.nembed(p)
FROM users u
LEFT JOIN punishments p ON p.id = u.current_punishment
                       AND p.is_disabled IS FALSE
                       AND (p.expires_at IS NULL OR CURRENT_TIMESTAMP < p.expires_at)
WHERE u.username = $1;

-- name: UpdateUserLastVisit :exec
UPDATE users
SET last_visit_at = $2
WHERE id = $1;

-- name: InsertSessionToken :exec
INSERT INTO session_tokens (for_user, token, expires_at)
VALUES ($1, $2, $3);

-- name: DeleteSessionToken :execrows
DELETE FROM session_tokens
WHERE id = $1;

-- name: ClearOtherSessionTokensForUser :exec
DELETE FROM session_tokens
WHERE for_user = $1
  AND id != $2;

-- name: UpdateSessionTokenExpiry :exec
UPDATE session_tokens
SET expires_at = $2
WHERE id = $1;

-- name: LookupSessionToken :one
SELECT sqlc.embed(st), sqlc.embed(u), sqlc.nembed(p)
FROM session_tokens st
     JOIN users u       ON u.id = st.for_user
LEFT JOIN punishments p ON p.id = u.current_punishment
                       AND p.is_disabled IS FALSE
                       AND (p.expires_at IS NULL OR CURRENT_TIMESTAMP < p.expires_at)
WHERE st.token = $1
  AND st.expires_at > CURRENT_TIMESTAMP;

-- name: InsertBlockRelation :exec
INSERT INTO block_relations (origin_user, blocked_user)
VALUES ($1, $2);

-- name: DeleteBlockRelation :execrows
DELETE FROM block_relations
WHERE origin_user = $1
  AND blocked_user = $2;

-- name: GetUserBlockList :many
SELECT sqlc.embed(u), sqlc.nembed(p)
FROM block_relations br
     JOIN users u       ON u.id = br.blocked_user
LEFT JOIN punishments p ON p.id = u.current_punishment
                       AND p.is_disabled IS FALSE
                       AND (p.expires_at IS NULL OR CURRENT_TIMESTAMP < p.expires_at)
WHERE br.origin_user = $1;

-- name: CreateParty :one
INSERT INTO parties (name)
VALUES ($1)
RETURNING *;

-- name: GetUserParty :one
SELECT p.*
FROM party_members pm
JOIN parties p ON p.id = pm.party
WHERE pm.member_user = $1;

-- name: GetPartyMembers :many
SELECT sqlc.embed(u), sqlc.nembed(p)
FROM party_members pm
     JOIN users u       ON u.id = pm.member_user
LEFT JOIN punishments p ON p.id = u.current_punishment
                       AND p.is_disabled IS FALSE
                       AND (p.expires_at IS NULL OR CURRENT_TIMESTAMP < p.expires_at)
WHERE pm.party = $1
ORDER BY pm.created_at;

-- name: InsertPartyMember :exec
INSERT INTO party_members (party, member_user)
VALUES ($1, $2);

-- name: DeletePartyMember :one
DELETE FROM party_members
WHERE member_user = $1
RETURNING party;

-- name: DeletePartyIfEmpty :exec
DELETE FROM parties p
WHERE p.id = $1
  AND NOT EXISTS (
    SELECT 1 FROM party_members pm
    WHERE pm.party = p.id
  );

-- name: GetActivePunishmentForIP :one
SELECT p.* FROM punishments p
JOIN punishment_ips pi ON pi.punishment = p.id
WHERE pi.ip = $1
  AND p.is_disabled IS FALSE
  AND (p.expires_at IS NULL OR CURRENT_TIMESTAMP < p.expires_at);
