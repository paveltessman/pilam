-- name: GetUser :one
SELECT * FROM app_user
WHERE id = $1;

-- name: GetUserByEmail :one
SELECT * FROM app_user
WHERE lower(email) = lower(sqlc.arg(email)::text);

-- name: CreateUser :one
INSERT INTO app_user (id, email, first_name, last_name, passwd_hash, role)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;
