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
    expires_at      TIMESTAMPTZ,
    reason          TEXT              NOT NULL DEFAULT ''
);

-- why: finding users with a specific punishment type
CREATE INDEX idx_punishments__punishment_type
ON punishments (punishment_type);


--// relations between users and their punishments

CREATE TABLE punishment_users (
    id           UUID        PRIMARY KEY DEFAULT uuidv7 (),
    created_at   TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    related_user UUID        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    punishment   UUID        NOT NULL REFERENCES punishments (id) ON DELETE CASCADE,
    ended_at     TIMESTAMPTZ
);

-- why: make sure there is only one not-ended relation between
-- a user and a punishment
CREATE UNIQUE INDEX idxuniq_punishment_users__related_user__single_unended
ON punishment_users (related_user)
WHERE ended_at IS NULL;

-- why: finding a user's latest active punishment relation
CREATE INDEX idx_punishment_users__user__latest_unended
ON punishment_users (related_user, created_at DESC)
WHERE ended_at IS NULL;

-- why: finding all punishment relations for a user
CREATE INDEX idx_punishment_users__user
ON punishment_users (related_user);

-- why: finding out what users a punishment affects
-- .. not used yet
CREATE INDEX idx_punishment_users__punishment
ON punishment_users (punishment);


--// ips that have punishments associated with them

CREATE TABLE punishment_ips (
    id         UUID        PRIMARY KEY DEFAULT uuidv7 (),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    ip         INET        NOT NULL,
    punishment UUID        NOT NULL REFERENCES punishments (id) ON DELETE CASCADE,
    ended_at   TIMESTAMPTZ
);

-- why: make sure an IP can only have one active punishment at a time
CREATE UNIQUE INDEX idxuniq_punishment_ips__ip__single_unended
ON punishment_ips (ip)
WHERE ended_at IS NULL;

-- why: looking up the latest active punishment for an IP
-- .. used when registering accounts
CREATE INDEX idx_punishment_ips__ip__latest_unended
ON punishment_ips (ip, created_at DESC)
WHERE ended_at IS NULL;

-- why: looking up all punishments for an ip
CREATE INDEX idx_punishment_ips__ip
ON punishment_ips (ip);

-- why: finding out what IPs a punishment affects
-- .. not used yet
CREATE INDEX idx_punishment_ips__punishment
ON punishment_ips (punishment);


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
ADD COLUMN last_visit_at      TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
ADD COLUMN last_seen_ip       INET,
ADD COLUMN mod_note           TEXT        NOT NULL DEFAULT '',
ADD COLUMN role               role_t      NOT NULL DEFAULT 'default'::role_t;

-- why: finding all users with a last seen IP
-- .. for applying IP punishments
CREATE INDEX idx_users__last_seen_ip
ON users (last_seen_ip);

-- why: searching for users by their username
CREATE INDEX idx_users__username__prefix
ON users (username text_pattern_ops);

-- why: searching for users by their role
CREATE INDEX idx_users__role
ON users (role);


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


--// views

CREATE VIEW users_full AS
SELECT u.id, u.created_at, u.username, u.pw_hash_type, u.pw_hash, u.last_visit_at, u.last_seen_ip, u.mod_note, u.role,
    p.id AS p_id, p.created_at AS p_created_at, p.punishment_type AS p_punishment_type, p.is_ip_enforced AS p_is_ip_enforced,
    p.expires_at AS p_expires_at, p.reason AS p_reason
FROM users u
LEFT JOIN LATERAL (
	SELECT p.*
	FROM punishment_users pu
	JOIN punishments p ON p.id = pu.punishment
	WHERE pu.related_user = u.id
	  AND pu.ended_at IS NULL
	  AND (p.expires_at IS NULL OR CURRENT_TIMESTAMP < p.expires_at)
	ORDER BY pu.created_at DESC
	LIMIT 1
) p ON TRUE;


--/// ...
-- +goose Down
DROP VIEW users_full;

DROP TABLE audit_log;
DROP TYPE log_type_t;

DROP INDEX idx_users__username__prefix;
ALTER TABLE users
DROP COLUMN role,
DROP COLUMN mod_note,
DROP COLUMN last_seen_ip,
DROP COLUMN last_visit_at;
DROP TYPE role_t;

DROP TABLE punishment_ips;
DROP TABLE punishment_users;
DROP TABLE punishments;
DROP TYPE punishment_type_t;
