--/// moderation features
-- +goose Up

--// punishments

CREATE TYPE punishment_type_t AS ENUM (
    'mute',
    'ban',
    'ban_silent'
);

CREATE TABLE punishments (
    id              UUID              PRIMARY KEY DEFAULT uuidv7 (),
    created_at      TIMESTAMPTZ       NOT NULL DEFAULT CURRENT_TIMESTAMP,
    punishment_type punishment_type_t NOT NULL,
    is_ip_enforced  BOOLEAN           NOT NULL,
    is_disabled     BOOLEAN           NOT NULL DEFAULT FALSE,
    expires_at      TIMESTAMPTZ,
    reason          TEXT              NOT NULL DEFAULT ''
);


--// users
--// adds last seen ip, mod note, active punishment, ..

CREATE TYPE role_t AS ENUM (
    'default',
    'music_contributor',
    'book_cover_contributor',
    'developer',
    'site_developer',
    'moderator',
    'administrator'
);

ALTER TABLE users
ADD COLUMN last_seen_ip       INET,
ADD COLUMN mod_note           TEXT   NOT NULL DEFAULT '',
ADD COLUMN current_punishment UUID   REFERENCES punishments (id),
ADD COLUMN role               role_t NOT NULL DEFAULT 'default'::role_t;

-- why: finding all users with a last seen IP
-- .. for applying IP punishments
CREATE INDEX idx_users__last_seen_ip
ON users (last_seen_ip);

-- why: finding all users affected by a punishment
-- .. not yet used
CREATE INDEX idx_users__current_punishment
ON users (current_punishment);


--// ips that have punishments associated with them

CREATE TABLE punishment_ips (
    id         UUID        PRIMARY KEY DEFAULT uuidv7 (),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    ip         INET        UNIQUE NOT NULL,
    punishment UUID        NOT NULL REFERENCES punishments (id) ON DELETE CASCADE
    -- we only allow one punishment per IP
);

-- why: looking up if an IP has any punishments
-- .. used when registering accounts
CREATE INDEX idx_punishment_ips__ip
ON punishment_ips (ip);

-- why: finding out what IPs a punishment affects
-- .. not used yet
CREATE INDEX idx_punishment_ips__punishment
ON punishment_ips (punishment);


--// audit log

-- these should correspond to actual actions moderators can take
-- on the website, not just generic CRUD operations
CREATE TYPE log_type_t AS ENUM (
    'mod_note_changed',
    'punished_user',
    'gave_badge',
    'removed_badge'
);

CREATE TABLE audit_log (
    id         UUID        PRIMARY KEY DEFAULT uuidv7 (),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    log_type   log_type_t  NOT NULL,
    log_detail JSONB       NOT NULL,
    caused_by  UUID        REFERENCES users (id) ON DELETE SET NULL
);

-- why: getting a list of actions by a moderator
CREATE INDEX idx_audit_log__caused_by
ON audit_log (caused_by);

-- why: getting all logs of a certain type
CREATE INDEX idx_audit_log__log_type
ON audit_log (log_type);


--/// ...
-- +goose Down
DROP TABLE audit_log;
DROP TYPE log_type_t;

ALTER TABLE users
DROP COLUMN role,
DROP COLUMN current_punishment,
DROP COLUMN mod_note,
DROP COLUMN last_seen_ip;

DROP TABLE punishment_ips;
DROP TABLE punishments;
DROP TYPE punishment_type_t;
