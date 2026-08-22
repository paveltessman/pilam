-- +goose Up
CREATE TABLE milestone_type (
    id          uuid        PRIMARY KEY,
    short_name  text        NOT NULL,
    description text        NOT NULL,
    active      boolean     NOT NULL DEFAULT true,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX milestone_type_short_name_key ON milestone_type (lower(short_name));

-- +goose Down
DROP TABLE milestone_type;
