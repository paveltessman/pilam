-- Milestone templates: the ordered list of types a calendar is built from.
--
-- The constraints carry the decisions of the feature. One type appears once per
-- template, an offset is zero or negative, and at most one template is the
-- default.

-- +goose Up
CREATE TABLE milestone_template (
    id          uuid        PRIMARY KEY,
    name        text        NOT NULL,
    description text        NOT NULL,
    is_default  boolean     NOT NULL DEFAULT false,
    active      boolean     NOT NULL DEFAULT true,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX milestone_template_name_key ON milestone_template (lower(name));

-- One template is the default for a new model. A partial index leaves every
-- other row free to hold false.
CREATE UNIQUE INDEX milestone_template_default_key ON milestone_template (is_default)
    WHERE is_default;

CREATE TABLE milestone_template_item (
    id          uuid        PRIMARY KEY,
    template_id uuid        NOT NULL REFERENCES milestone_template (id) ON DELETE CASCADE,
    type_id     uuid        NOT NULL REFERENCES milestone_type (id),
    offset_days integer     NOT NULL CHECK (offset_days <= 0),
    position    integer     NOT NULL CHECK (position >= 0),
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now(),

    -- One type appears once per template.
    CONSTRAINT milestone_template_item_type_key UNIQUE (template_id, type_id),

    -- Positions of one template are distinct.
    -- Deferred, because a reorder swaps two of them inside one transaction and
    -- the rows hold the same number for a moment on the way.
    CONSTRAINT milestone_template_item_order_key UNIQUE (template_id, position) DEFERRABLE INITIALLY DEFERRED
);

-- +goose Down
DROP TABLE milestone_template_item;
DROP TABLE milestone_template;
