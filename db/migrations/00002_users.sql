-- The account table. `user` is a reserved word in Postgres, so the table is
-- `app_user`. The Go type stays auth.User.

-- +goose Up
CREATE TABLE app_user (
    id                   uuid        PRIMARY KEY,
    email                text        NOT NULL,
    first_name           text        NOT NULL,
    last_name            text        NOT NULL,
    passwd_hash          text        NOT NULL,
    role                 text        NOT NULL CHECK (role IN ('member', 'root')),
    active               boolean     NOT NULL DEFAULT true,
    session_epoch        integer     NOT NULL DEFAULT 1,
    passwd_expired       boolean     NOT NULL DEFAULT true,
    created_at           timestamptz NOT NULL DEFAULT now(),
    updated_at           timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX app_user_email_key ON app_user (lower(email));

-- +goose Down
-- The index belongs to the table and goes with it.
DROP TABLE app_user;
