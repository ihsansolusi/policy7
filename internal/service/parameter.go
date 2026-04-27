package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/ihsansolusi/policy7/internal/store"
)

type ParameterService struct {
	db    store.Querier
	cache *store.RedisCache
}

func NewParameterService(db store.Querier, cache *store.RedisCache) *ParameterService {
	return &ParameterService{
		db:    db,
		cache: cache,
	}
}

func (s *ParameterService) GetParameter(ctx context.Context, orgID uuid.UUID, category, name, appliesTo string, appliesToID, product *string) (*store.Parameter, error) {
	const op = "service.ParameterService.GetParameter"

	// Cache key
	appliesToIDStr := "null"
	if appliesToID != nil {
		appliesToIDStr = *appliesToID
	}
	productStr := "null"
	if product != nil {
		productStr = *product
	}
	cacheKey := fmt.Sprintf("policy7:%s:%s:%s:%s:%s:%s", orgID, category, name, appliesTo, appliesToIDStr, productStr)

	// Try cache first
	if s.cache != nil {
		cached, err := s.cache.Get(ctx, cacheKey)
		if err == nil && cached != nil {
			var param store.Parameter
			if err := json.Unmarshal(cached, &param); err == nil {
				return &param, nil
			}
		}
	}

	// Prepare params
	var pgOrgID pgtype.UUID
	_ = pgOrgID.Scan(orgID.String())

	var pgAppliesToID pgtype.Text
	if appliesToID != nil {
		pgAppliesToID = pgtype.Text{String: *appliesToID, Valid: true}
	}

	var pgProduct pgtype.Text
	if product != nil {
		pgProduct = pgtype.Text{String: *product, Valid: true}
	}

	// Query Database
	param, err := s.db.GetParameter(ctx, store.GetParameterParams{
		OrgID:       pgOrgID,
		Category:    category,
		Name:        name,
		AppliesTo:   appliesTo,
		AppliesToID: pgAppliesToID,
		Product:     pgProduct,
	})

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("%s: parameter not found", op)
		}
		return nil, fmt.Errorf("%s: failed to get parameter: %w", op, err)
	}

	// Cache result
	if s.cache != nil {
		data, _ := json.Marshal(param)
		_ = s.cache.Set(ctx, cacheKey, data, 5*time.Minute)
	}

	return &param, nil
}

type ResolutionContext struct {
	UserID   *string
	RoleID   *string
	BranchID *string
	Global   bool
}

func (s *ParameterService) GetEffectiveParameter(ctx context.Context, orgID uuid.UUID, category, name string, product *string, resCtx ResolutionContext) (*store.Parameter, error) {
	if resCtx.UserID != nil {
		p, err := s.GetParameter(ctx, orgID, category, name, "user", resCtx.UserID, product)
		if err == nil {
			return p, nil
		}
	}

	if resCtx.RoleID != nil {
		p, err := s.GetParameter(ctx, orgID, category, name, "role", resCtx.RoleID, product)
		if err == nil {
			return p, nil
		}
	}

	if resCtx.BranchID != nil {
		p, err := s.GetParameter(ctx, orgID, category, name, "branch", resCtx.BranchID, product)
		if err == nil {
			return p, nil
		}
	}

	if resCtx.Global {
		p, err := s.GetParameter(ctx, orgID, category, name, "global", nil, product)
		if err == nil {
			return p, nil
		}
	}

	return nil, fmt.Errorf("no effective parameter found for %s:%s", category, name)
}
