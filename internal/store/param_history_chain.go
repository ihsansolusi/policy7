package store

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"
)

// This query is intentionally hand-written rather than generated into
// query.sql.go: the committed sqlc output has drifted from the migrations
// (some NOT NULL columns are still emitted as nullable pgtype wrappers), so a
// full `sqlc generate` rewrites ~250 unrelated lines. Keeping it here adds the
// #587 full-chain query surgically. When the sqlc drift is reconciled, move
// this into query.sql and regenerate.

const getParameterHistoryByIdentity = `
WITH target AS (
    SELECT category, name, applies_to, applies_to_id, product
    FROM parameters WHERE parameters.id = $1 AND parameters.org_id = $2
)
SELECT h.id, h.parameter_id, h.org_id, h.previous_value, h.new_value, h.change_type,
       h.previous_version, h.new_version, h.change_reason, h.change_metadata,
       h.changed_by, h.changed_at
FROM parameter_history h
JOIN parameters p ON p.id = h.parameter_id
CROSS JOIN target t
WHERE h.org_id = $2
  AND p.category = t.category
  AND p.name = t.name
  AND p.applies_to = t.applies_to
  AND COALESCE(p.applies_to_id, '') = COALESCE(t.applies_to_id, '')
  AND COALESCE(p.product, '') = COALESCE(t.product, '')
ORDER BY h.new_version ASC, h.changed_at ASC`

// GetParameterHistoryByIdentityParams identifies any one version row; the query
// resolves its identity tuple and returns the whole chain.
type GetParameterHistoryByIdentityParams struct {
	ID    pgtype.UUID `json:"id"`
	OrgID pgtype.UUID `json:"org_id"`
}

// GetParameterHistoryByIdentity returns every parameter_history row for the
// identity tuple that parameter ID belongs to (across all version row ids),
// ordered oldest→newest (#587).
func (q *Queries) GetParameterHistoryByIdentity(ctx context.Context, arg GetParameterHistoryByIdentityParams) ([]ParameterHistory, error) {
	rows, err := q.db.Query(ctx, getParameterHistoryByIdentity, arg.ID, arg.OrgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []ParameterHistory{}
	for rows.Next() {
		var i ParameterHistory
		if err := rows.Scan(
			&i.ID,
			&i.ParameterID,
			&i.OrgID,
			&i.PreviousValue,
			&i.NewValue,
			&i.ChangeType,
			&i.PreviousVersion,
			&i.NewVersion,
			&i.ChangeReason,
			&i.ChangeMetadata,
			&i.ChangedBy,
			&i.ChangedAt,
		); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}
