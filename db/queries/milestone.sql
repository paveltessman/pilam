-- name: GetMilestoneType :one
SELECT * FROM milestone_type
WHERE id = $1;

-- name: ListMilestoneTypes :many
SELECT * FROM milestone_type
ORDER BY name;

-- name: CreateMilestoneType :exec
INSERT INTO milestone_type (id, name, description, active)
VALUES ($1, $2, $3, $4);

-- name: UpdateMilestoneType :execrows
UPDATE milestone_type
SET name  = $2,
    description = $3,
    active      = $4,
    updated_at  = now()
WHERE id = $1;

-- name: GetMilestoneTemplate :one
SELECT * FROM milestone_template
WHERE id = $1;

-- name: ListMilestoneTemplates :many
SELECT * FROM milestone_template
ORDER BY name;

-- name: CreateMilestoneTemplate :exec
INSERT INTO milestone_template (id, name, description, is_default, active)
VALUES ($1, $2, $3, $4, $5);

-- name: UpdateMilestoneTemplate :execrows
UPDATE milestone_template
SET name        = $2,
    description = $3,
    is_default  = $4,
    active      = $5,
    updated_at  = now()
WHERE id = $1;

-- name: ClearMilestoneTemplateDefault :exec
-- Drops the default flag of every template but the one named. The write that
-- names a new default runs this first, because the index holds one true row.
UPDATE milestone_template
SET is_default = false,
    updated_at = now()
WHERE is_default AND id <> $1;

-- name: ListMilestoneTemplateItems :many
SELECT * FROM milestone_template_item
WHERE template_id = $1
ORDER BY position;

-- name: CreateMilestoneTemplateItem :exec
INSERT INTO milestone_template_item (id, template_id, type_id, offset_days, position)
VALUES ($1, $2, $3, $4, $5);

-- name: UpdateMilestoneTemplateItem :execrows
UPDATE milestone_template_item
SET type_id     = $2,
    offset_days = $3,
    position    = $4,
    updated_at  = now()
WHERE id = $1;

-- name: DeleteMilestoneTemplateItem :execrows
DELETE FROM milestone_template_item
WHERE id = $1;

-- name: SetMilestoneTemplateItemPosition :execrows
UPDATE milestone_template_item
SET position   = $2,
    updated_at = now()
WHERE id = $1;

-- name: GetMilestone :one
SELECT * FROM milestone
WHERE id = $1;

-- name: ListMilestonesByModel :many
-- The calendar of one model, active and inactive, in plan date order. Two
-- milestones on the same day break the tie by identifier, so the order of a
-- read never changes between two calls.
SELECT * FROM milestone
WHERE model_id = $1
ORDER BY plan_date, id;

-- name: CreateMilestone :exec
INSERT INTO milestone (
    id, model_id, type_id, baseline_date, plan_date, fact_date, note, active
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8);

-- name: UpdateMilestone :execrows
UPDATE milestone
SET baseline_date = $2,
    plan_date     = $3,
    fact_date     = $4,
    note          = $5,
    active        = $6,
    updated_at    = now()
WHERE id = $1;

-- name: GetModelTargetDate :one
-- The day every date of a model is reckoned from. The model holds no date of
-- its own, so this reads the drop that holds it.
SELECT drop.target_date
FROM model
JOIN drop ON drop.id = model.drop_id
WHERE model.id = $1;

-- name: ListMilestonesBySeason :many
-- The milestone list: the active steps of the active models of one season, with
-- the names the screen reads beside them.
SELECT sqlc.embed(milestone),
       milestone_type.name AS type_name,
       model.article,
       model.drop_id,
       drop.name           AS drop_name
FROM milestone
JOIN model         ON model.id         = milestone.model_id
JOIN drop          ON drop.id          = model.drop_id
JOIN milestone_type ON milestone_type.id = milestone.type_id
WHERE milestone.active
  AND model.active
  AND drop.season_id = sqlc.arg(season_id)::uuid
  AND (sqlc.narg(drop_id)::uuid IS NULL OR model.drop_id     = sqlc.narg(drop_id)::uuid)
  AND (sqlc.narg(type_id)::uuid IS NULL OR milestone.type_id = sqlc.narg(type_id)::uuid)
ORDER BY milestone.plan_date, milestone.id;

-- name: CountModelsWithoutCalendar :many
-- The models of one season that hold no calendar, counted per drop.
SELECT model.drop_id, drop.name AS drop_name, count(*) AS models
FROM model
JOIN drop ON drop.id = model.drop_id
WHERE model.active
  AND drop.season_id = sqlc.arg(season_id)::uuid
  AND (sqlc.narg(drop_id)::uuid IS NULL OR model.drop_id = sqlc.narg(drop_id)::uuid)
  AND NOT EXISTS (
      SELECT 1 FROM milestone
      WHERE milestone.model_id = model.id AND milestone.active
  )
GROUP BY model.drop_id, drop.name
ORDER BY drop.name;
