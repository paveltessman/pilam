-- name: CreateAuditEntry :exec
INSERT INTO audit_entry (
    id, at, actor_id, entity, entity_id,
    action, field_key, old_value, new_value, request_id
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10);
