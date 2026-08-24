-- The calendar of one model: one milestone per step of the critical path.
--
-- The constraints carry the decisions of the feature. A model holds one
-- milestone per type, and a note is 500 characters at most.
--
-- The three dates stay apart. The baseline is what the calendar promised, the
-- plan is what the team expects now, and the fact is the day the step was done.

-- +goose Up
CREATE TABLE milestone (
    id            uuid        PRIMARY KEY,
    model_id      uuid        NOT NULL REFERENCES model (id) ON DELETE CASCADE,
    type_id       uuid        NOT NULL REFERENCES milestone_type (id),
    baseline_date date        NOT NULL,
    plan_date     date        NOT NULL,
    fact_date     date,
    note          text        NOT NULL DEFAULT '' CHECK (char_length(note) <= 500),
    active        boolean     NOT NULL DEFAULT true,
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now(),

    -- A model holds one milestone per type.
    CONSTRAINT milestone_model_type_key UNIQUE (model_id, type_id)
);

-- The calendar section of the model screen: the milestones of one model, in
-- plan date order.
CREATE INDEX milestone_model_plan_idx ON milestone (model_id, plan_date);

-- +goose Down
DROP TABLE milestone;
