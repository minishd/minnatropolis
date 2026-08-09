-- name: CreateUser :one
INSERT INTO users (username, pw_hash_type, pw_hash)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetUserByUsername :one
SELECT * FROM users
WHERE username = $1;

-- name: CreateSessionToken :one
INSERT INTO session_tokens (for_user, token)
VALUES ($1, $2)
RETURNING *;

-- name: LookupSessionToken :one
SELECT * FROM session_tokens
WHERE token = $1;
