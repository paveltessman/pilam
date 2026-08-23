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
