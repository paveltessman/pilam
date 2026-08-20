-- Catalog: season, drop, model, and model photo.
--
-- The chain is season -> drop -> model -> model_photo.

-- +goose Up
CREATE TABLE season (
    id         uuid        PRIMARY KEY,
    name       text        NOT NULL,
    start_date date        NOT NULL,
    active     boolean     NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX season_name_key ON season (lower(name));

CREATE TABLE drop (
    id          uuid        PRIMARY KEY,
    season_id   uuid        NOT NULL REFERENCES season (id),
    name        text        NOT NULL,
    target_date date        NOT NULL,
    active      boolean     NOT NULL DEFAULT true,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now()
);

-- One name per season. Two seasons can each hold a drop of the same name.
CREATE UNIQUE INDEX drop_season_name_key ON drop (season_id, lower(name));

CREATE TABLE model (
    id         uuid        PRIMARY KEY,
    drop_id    uuid        NOT NULL REFERENCES drop (id),
    article    text        NOT NULL,
    active     boolean     NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

-- The models of one drop, and the join that the season filter goes through.
CREATE INDEX model_drop_idx ON model (drop_id);

CREATE TABLE model_photo (
    id         uuid        PRIMARY KEY,
    model_id   uuid        NOT NULL REFERENCES model (id) ON DELETE CASCADE,
    media_key  text        NOT NULL,
    position   integer     NOT NULL CHECK (position >= 0),
    created_at timestamptz NOT NULL DEFAULT now(),

    -- Positions of one model are distinct.
    -- Deferred, because a reorder swaps two of them inside one transaction and
    -- the rows hold the same number for a moment on the way.
    CONSTRAINT model_photo_order_key UNIQUE (model_id, position) DEFERRABLE INITIALLY DEFERRED
);

-- +goose Down
DROP TABLE model_photo;
DROP TABLE model;
DROP TABLE drop;
DROP TABLE season;
