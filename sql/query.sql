-- name: CreateUser :one
INSERT INTO users (username, pw_hash_type, pw_hash)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetUserByUsername :one
SELECT * FROM users
WHERE username = $1;

-- name: InsertSessionToken :exec
INSERT INTO session_tokens (for_user, token, expires_at)
VALUES ($1, $2, $3);

-- name: DeleteSessionToken :exec
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

-- name: LookupSessionTokenWithUser :one
SELECT st.*,
    u.id AS user_id,
    u.created_at AS user_created_at,
    u.username AS user_username,
    u.pw_hash_type AS user_pw_hash_type,
    u.pw_hash AS user_pw_hash
FROM session_tokens st
JOIN users u ON st.for_user = u.id
WHERE token = $1
  AND expires_at > CURRENT_TIMESTAMP;
