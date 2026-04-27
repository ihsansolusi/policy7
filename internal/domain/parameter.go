package domain

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type Parameter struct {
	ID             uuid.UUID       `json:"id"`
	OrgID          uuid.UUID       `json:"org_id"`
	Category       string          `json:"category"`
	Name           string          `json:"name"`
	AppliesTo      string          `json:"applies_to"`
	AppliesToID    *string         `json:"applies_to_id,omitempty"`
	Product        *string         `json:"product,omitempty"`
	Value          json.RawMessage `json:"value"`
	ValueType      string          `json:"value_type"`
	Unit           *string         `json:"unit,omitempty"`
	Scope          *string         `json:"scope,omitempty"`
	EffectiveFrom  time.Time       `json:"effective_from"`
	EffectiveUntil *time.Time      `json:"effective_until,omitempty"`
	Version        int             `json:"version"`
	IsActive       bool            `json:"is_active"`
	CreatedAt      time.Time       `json:"created_at"`
	UpdatedAt      time.Time       `json:"updated_at"`
}
