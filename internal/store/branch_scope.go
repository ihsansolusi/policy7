package store

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/ihsansolusi/policy7/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// BranchScopeQuerier is implemented by *Queries for branch_scope table access.
type BranchScopeQuerier interface {
	GetBranchScope(ctx context.Context, branchID uuid.UUID) (*domain.BranchScope, error)
	UpsertBranchScope(ctx context.Context, arg UpsertBranchScopeParams) error
}

// UpsertBranchScopeParams holds values for an upsert into branch_scope.
type UpsertBranchScopeParams struct {
	BranchID       uuid.UUID
	OrgID          string
	BranchType     string
	ParentBranchID *uuid.UUID
	UpdatedAt      time.Time
}

const getBranchScopeSQL = `
SELECT branch_id, org_id, branch_type, parent_branch_id, updated_at, synced_at
FROM branch_scope
WHERE branch_id = $1
`

func (q *Queries) GetBranchScope(ctx context.Context, branchID uuid.UUID) (*domain.BranchScope, error) {
	row := q.db.QueryRow(ctx, getBranchScopeSQL, branchID)
	var (
		bID      uuid.UUID
		orgID    string
		btType   pgtype.Text
		parentID pgtype.UUID
		updAt    time.Time
		syncAt   time.Time
	)
	if err := row.Scan(&bID, &orgID, &btType, &parentID, &updAt, &syncAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, pgx.ErrNoRows
		}
		return nil, err
	}
	s := &domain.BranchScope{
		BranchID:   bID,
		OrgID:      orgID,
		BranchType: btType.String,
		UpdatedAt:  updAt,
		SyncedAt:   syncAt,
	}
	if parentID.Valid {
		id := uuid.UUID(parentID.Bytes)
		s.ParentBranchID = &id
	}
	return s, nil
}

const upsertBranchScopeSQL = `
INSERT INTO branch_scope (branch_id, org_id, branch_type, parent_branch_id, updated_at, synced_at)
VALUES ($1, $2, $3, $4, $5, NOW())
ON CONFLICT (branch_id) DO UPDATE SET
    org_id           = EXCLUDED.org_id,
    branch_type      = EXCLUDED.branch_type,
    parent_branch_id = EXCLUDED.parent_branch_id,
    updated_at       = EXCLUDED.updated_at,
    synced_at        = NOW()
WHERE branch_scope.updated_at < EXCLUDED.updated_at
`

func (q *Queries) UpsertBranchScope(ctx context.Context, arg UpsertBranchScopeParams) error {
	var pgParentID pgtype.UUID
	if arg.ParentBranchID != nil {
		_ = pgParentID.Scan(arg.ParentBranchID.String())
	}
	_, err := q.db.Exec(ctx, upsertBranchScopeSQL,
		arg.BranchID,
		arg.OrgID,
		arg.BranchType,
		pgParentID,
		arg.UpdatedAt,
	)
	return err
}

// Compile-time check that *Queries satisfies BranchScopeQuerier.
var _ BranchScopeQuerier = (*Queries)(nil)
