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
