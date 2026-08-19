-- name: GetUser :one
SELECT * FROM app_user
WHERE id = $1;

-- name: GetUserByEmail :one
SELECT * FROM app_user
WHERE lower(email) = lower(sqlc.arg(email)::text);

-- name: CreateUser :exec
INSERT INTO app_user (
    id, email, first_name, last_name, passwd_hash,
    role, active, session_epoch, passwd_expired
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9);

-- name: UpdateUser :execrows
UPDATE app_user
SET email          = $2,
    first_name     = $3,
    last_name      = $4,
    passwd_hash    = $5,
    role           = $6,
    active         = $7,
    session_epoch  = $8,
    passwd_expired = $9,
    updated_at     = now()
WHERE id = $1;

-- name: ListUsers :many
SELECT * FROM app_user
ORDER BY first_name;
