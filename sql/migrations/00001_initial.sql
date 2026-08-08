--/// initial migration

-- +goose Up
--// ...

CREATE EXTENSION citext;

CREATE TYPE pw_hash_type_t AS ENUM (
    'argon2id'
);

CREATE TABLE users (
  id           UUID           PRIMARY KEY DEFAULT uuidv7 (),
  created_at   TIMESTAMPTZ    NOT NULL DEFAULT CURRENT_TIMESTAMP,
  username     CITEXT         NOT NULL UNIQUE CHECK (length(username) >= 1),
  pw_hash_type pw_hash_type_t NOT NULL DEFAULT 'argon2id'::pw_hash_type_t,
  pw_hash      TEXT           NOT NULL
);

-- why: looking up user by username
CREATE INDEX idx_users__username
ON users (username);


-- +goose Down
--// ...

DROP TABLE users;
DROP TYPE pw_hash_type_t;
DROP EXTENSION citext;
