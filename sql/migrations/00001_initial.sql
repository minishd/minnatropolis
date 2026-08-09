--/// initial migration

-- +goose Up
--/// ...

CREATE EXTENSION citext;

--// users

-- future-proofing for if hash algo. is ever changed
CREATE TYPE pw_hash_type_t AS ENUM (
    'argon2id'
);

CREATE TABLE users (
    id           UUID           PRIMARY KEY DEFAULT uuidv7 (),
    created_at   TIMESTAMPTZ    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    -- usernames must be 1 to 16 chars in length
    -- and only contain letters, numbers, and underscores
    username     CITEXT         UNIQUE NOT NULL
        CHECK ((length(username) BETWEEN 1 AND 16) AND username ~* '^\w*$'),
    pw_hash_type pw_hash_type_t NOT NULL DEFAULT 'argon2id'::pw_hash_type_t,
    pw_hash      TEXT           NOT NULL
);

-- why: looking up user by username
CREATE INDEX idx_users__username
ON users (username);

--// session tokens

CREATE TABLE session_tokens (
    id           UUID           PRIMARY KEY DEFAULT uuidv7 (),
    created_at   TIMESTAMPTZ    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    for_user     UUID           NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    token        TEXT           UNIQUE NOT NULL
);

-- why: looking up session tokens for api requests
CREATE INDEX idx_session_tokens__token
ON session_tokens (token);

-- +goose Down
--/// ...

DROP TABLE session_tokens;
DROP TABLE users;
DROP TYPE pw_hash_type_t;
DROP EXTENSION citext;
