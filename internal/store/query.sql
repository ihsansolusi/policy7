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

-- name: ListParametersFiltered :many
-- List parameters with optional category/product/applies_to filters.
-- Pass NULL to skip a filter. Used by admin UI list pages.
SELECT * FROM parameters
WHERE org_id = $1
  AND (sqlc.narg('category')::TEXT IS NULL OR category = sqlc.narg('category')::TEXT)
  AND (sqlc.narg('product')::TEXT IS NULL OR product = sqlc.narg('product')::TEXT)
  AND (sqlc.narg('applies_to')::TEXT IS NULL OR applies_to::TEXT = sqlc.narg('applies_to')::TEXT)
  AND is_active = TRUE
ORDER BY name ASC, created_at DESC
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

-- name: GetParameterHistoryByIdentity :many
-- Full version chain (#587): each version is a distinct parameters row (own id),
-- so history keyed by parameter_id is fragmented. Resolve the identity tuple of
-- parameter $1, then gather history across ALL rows sharing that tuple
-- (org_id, category, name, applies_to, COALESCE applies_to_id, COALESCE product),
-- ordered oldest->newest.
WITH target AS (
    SELECT category, name, applies_to, applies_to_id, product
    FROM parameters WHERE parameters.id = $1 AND parameters.org_id = $2
)
SELECT h.* FROM parameter_history h
JOIN parameters p ON p.id = h.parameter_id
CROSS JOIN target t
WHERE h.org_id = $2
  AND p.category = t.category
  AND p.name = t.name
  AND p.applies_to = t.applies_to
  AND COALESCE(p.applies_to_id, '') = COALESCE(t.applies_to_id, '')
  AND COALESCE(p.product, '') = COALESCE(t.product, '')
ORDER BY h.new_version ASC, h.changed_at ASC;

-- name: ListParameterCategories :many
-- List all category metadata for an org (active + inactive), ordered for display.
SELECT * FROM parameter_categories
WHERE org_id = $1
ORDER BY display_order ASC, code ASC;

-- name: GetParameterCategoryByCode :one
SELECT * FROM parameter_categories
WHERE org_id = $1 AND code = $2;

-- name: CreateParameterCategory :one
INSERT INTO parameter_categories (
    org_id, code, name, description, value_schema, default_value,
    display_order, icon, color, is_active, created_by
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11
)
RETURNING *;

-- name: UpdateParameterCategory :one
UPDATE parameter_categories
SET name = $3,
    description = $4,
    value_schema = $5,
    default_value = $6,
    display_order = $7,
    icon = $8,
    color = $9,
    is_active = $10,
    updated_by = $11,
    updated_at = NOW()
WHERE org_id = $1 AND code = $2
RETURNING *;

-- name: DeactivateParameterCategory :exec
UPDATE parameter_categories
SET is_active = FALSE, updated_by = $3, updated_at = NOW()
WHERE org_id = $1 AND code = $2;
