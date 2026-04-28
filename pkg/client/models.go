package client

import (
	"encoding/json"

	"github.com/google/uuid"
)

type ValidationRequest struct {
	Name     string  `json:"name"`
	Amount   float64 `json:"amount"`
	UserID   *string `json:"user_id,omitempty"`
	RoleID   *string `json:"role_id,omitempty"`
	BranchID *string `json:"branch_id,omitempty"`
	Product  *string `json:"product,omitempty"`
}

type ValidationResponse struct {
	IsValid       bool      `json:"is_valid"`
	Amount        float64   `json:"amount"`
	Limit         float64   `json:"limit"`
	ParameterUsed uuid.UUID `json:"parameter_used"`
}

type Parameter struct {
	ID        uuid.UUID       `json:"id"`
	OrgID     uuid.UUID       `json:"org_id"`
	Category  string          `json:"category"`
	Name      string          `json:"name"`
	Value     json.RawMessage `json:"value"`
	Version   int             `json:"version"`
}

type RegulatoryRequest struct {
	Amount float64 `json:"amount"`
}

type RegulatoryResponse struct {
	IsExceeded bool    `json:"is_exceeded"`
	Amount     float64 `json:"amount"`
	Threshold  float64 `json:"threshold"`
}

type AuthorizationRequest struct {
	RoleID string  `json:"role_id"`
	Amount float64 `json:"amount"`
}

type AuthorizationResponse struct {
	IsAuthorized  bool      `json:"is_authorized"`
	Amount        float64   `json:"amount"`
	Limit         float64   `json:"limit"`
	ParameterUsed uuid.UUID `json:"parameter_used"`
}
