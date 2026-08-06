-- name: CreateExample :one
INSERT INTO examples (
  name, display_name
) VALUES (
  $1, $2
)
RETURNING *;
