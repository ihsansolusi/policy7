package domain

import (
	"time"

	"github.com/google/uuid"
)

type BranchScope struct {
	BranchID       uuid.UUID  `json:"branch_id"`
	OrgID          string     `json:"org_id"`
	BranchType     string     `json:"branch_type"`
	ParentBranchID *uuid.UUID `json:"parent_branch_id,omitempty"`
	UpdatedAt      time.Time  `json:"updated_at"`
	SyncedAt       time.Time  `json:"synced_at"`
}
