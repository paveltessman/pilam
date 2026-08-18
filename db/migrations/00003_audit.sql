-- The audit trail. One row per field change.

-- +goose Up
CREATE TABLE audit_entry (
    id         uuid        PRIMARY KEY,
    at         timestamptz NOT NULL,
    actor_id   uuid        NOT NULL REFERENCES app_user (id),
    entity     text        NOT NULL,
    entity_id  uuid        NOT NULL,
    action     text        NOT NULL,
    field_key  text        NOT NULL DEFAULT '',
    old_value  text,
    new_value  text,
    request_id text        NOT NULL DEFAULT ''
);

-- The history of one entity, newest first.
CREATE INDEX audit_entry_entity_idx ON audit_entry (entity, entity_id, at DESC);

-- +goose Down
DROP TABLE audit_entry;
