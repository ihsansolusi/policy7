-- name: GetParameter :one
SELECT *
FROM parameters
WHERE org_id = $1
  AND category = $2
  AND name = $3
  AND applies_to = $4
  AND (applies_to_id = $5 OR ($5::varchar IS NULL AND applies_to_id IS NULL))
  AND (product = $6 OR ($6::varchar IS NULL AND product IS NULL))
  AND is_active = TRUE
  AND effective_from <= NOW()
  AND (effective_until IS NULL OR effective_until > NOW())
ORDER BY version DESC
LIMIT 1;

-- name: CreateParameter :one
INSERT INTO parameters (
  org_id, category, name, applies_to, applies_to_id, product, 
  value, value_type, unit, scope, effective_from, effective_until, 
  version, is_active, created_by
) VALUES (
  $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15
)
RETURNING *;

-- name: GetParameterByID :one
SELECT * FROM parameters WHERE id = $1 AND org_id = $2;

-- name: ListParameters :many
SELECT * FROM parameters
WHERE org_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: DeactivateParameter :exec
UPDATE parameters
SET is_active = FALSE, updated_at = NOW()
WHERE id = $1 AND org_id = $2;

-- name: CreateParameterHistory :one
INSERT INTO parameter_history (
    parameter_id, org_id, previous_value, new_value,
    change_type, previous_version, new_version, change_reason,
    change_metadata, changed_by
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10
)
RETURNING *;

-- name: GetParameterHistory :many
SELECT * FROM parameter_history
WHERE parameter_id = $1 AND org_id = $2
ORDER BY changed_at DESC;
