--/// initial migration

-- +goose Up
CREATE TABLE examples (
  id           BIGSERIAL PRIMARY KEY,
  name         text      NOT NULL,
  display_name text
);

-- +goose Down
DROP TABLE examples;
