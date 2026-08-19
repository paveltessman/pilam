-- name: CreateAuditEntry :exec
INSERT INTO audit_entry (
    id, at, actor_id, entity, entity_id,
    action, field_key, old_value, new_value, request_id
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10);

-- name: ListAuditEntries :many
SELECT * FROM audit_entry
WHERE entity = $1 AND entity_id = $2
ORDER BY at DESC, id DESC
LIMIT sqlc.arg(row_limit)::int;
