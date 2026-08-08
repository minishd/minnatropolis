-- name: CreateUser :one
INSERT INTO users (username, pw_hash_type, pw_hash)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetUserByUsername :one
SELECT * FROM users
WHERE username = $1;
