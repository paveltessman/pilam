-- name: CreateAuditEntry :exec
INSERT INTO audit_entry (
    id, at, actor_id, entity, entity_id,
    action, field_key, old_value, new_value, request_id
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10);

-- name: ListAuditEntries :many
-- The trail of the rows named, newest first. One entity per call, because the
-- index is over (entity, entity_id, at DESC).
SELECT * FROM audit_entry
WHERE entity = $1 AND entity_id = ANY(sqlc.arg(entity_ids)::uuid[])
ORDER BY at DESC, id DESC
LIMIT sqlc.arg(row_limit)::int;
