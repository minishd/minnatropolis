--/// parties
-- +goose Up

--// parties

CREATE TABLE parties (
    id           UUID           PRIMARY KEY DEFAULT uuidv7 (),
    created_at   TIMESTAMPTZ    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    -- party names must be 1 to 32 chars in length
    name         TEXT           NOT NULL
        CHECK (length(name) BETWEEN 1 AND 32)
);


--// party members

CREATE TABLE party_members (
    id           UUID           PRIMARY KEY DEFAULT uuidv7 (),
    created_at   TIMESTAMPTZ    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    party        UUID           NOT NULL REFERENCES parties (id) ON DELETE CASCADE,
    -- users can only be in one party at a time
    member_user  UUID           UNIQUE NOT NULL REFERENCES users (id) ON DELETE CASCADE
);

-- why: listing a party's members
CREATE INDEX idx_party_members__party
ON party_members (party);


--/// ...
-- +goose Down
DROP TABLE party_members;
DROP TABLE parties;
