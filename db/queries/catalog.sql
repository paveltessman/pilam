-- name: GetSeason :one
SELECT * FROM season
WHERE id = $1;

-- name: ListSeasons :many
SELECT * FROM season
ORDER BY start_date, name;

-- name: CreateSeason :exec
INSERT INTO season (id, name, start_date, active)
VALUES ($1, $2, $3, $4);

-- name: UpdateSeason :execrows
UPDATE season
SET name       = $2,
    start_date = $3,
    active     = $4,
    updated_at = now()
WHERE id = $1;

-- name: GetDrop :one
SELECT * FROM drop
WHERE id = $1;

-- name: ListDropsBySeason :many
SELECT * FROM drop
WHERE season_id = $1
ORDER BY target_date, name;

-- name: CreateDrop :exec
INSERT INTO drop (id, season_id, name, target_date, active)
VALUES ($1, $2, $3, $4, $5);

-- name: UpdateDrop :execrows
UPDATE drop
SET name        = $2,
    target_date = $3,
    active      = $4,
    updated_at  = now()
WHERE id = $1;

-- name: GetModel :one
SELECT * FROM model
WHERE id = $1;

-- name: ListModels :many
-- The season case joins drop, because the model holds no season column.
SELECT model.* FROM model
JOIN drop ON drop.id = model.drop_id
WHERE (sqlc.narg(season_id)::uuid    IS NULL OR drop.season_id = sqlc.narg(season_id)::uuid)
  AND (sqlc.narg(drop_id)::uuid      IS NULL OR model.drop_id  = sqlc.narg(drop_id)::uuid)
  AND (sqlc.narg(active)::boolean    IS NULL OR model.active   = sqlc.narg(active)::boolean)
ORDER BY model.article, model.id;

-- name: CreateModel :exec
INSERT INTO model (id, drop_id, article, active)
VALUES ($1, $2, $3, $4);

-- name: UpdateModel :execrows
UPDATE model
SET article    = $2,
    active     = $3,
    updated_at = now()
WHERE id = $1;

-- name: ListPhotosByModel :many
SELECT * FROM model_photo
WHERE model_id = $1
ORDER BY position;

-- name: CreatePhoto :exec
INSERT INTO model_photo (id, model_id, media_key, position)
VALUES ($1, $2, $3, $4);

-- name: DeletePhoto :execrows
DELETE FROM model_photo
WHERE id = $1;

-- name: SetPhotoPosition :execrows
UPDATE model_photo
SET position = $2
WHERE id = $1;

-- name: ListThumbnails :many
-- The cover of each model named: the photo at position 0. A model with no
-- photo has no row here, so a list reads every cover in one query.
SELECT * FROM model_photo
WHERE model_id = ANY(@model_ids::uuid[])
  AND position = 0;
