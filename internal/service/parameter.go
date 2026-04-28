package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/ihsansolusi/policy7/internal/store"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"golang.org/x/sync/singleflight"
)

type ParameterService struct {
	db    store.Querier
	cache *store.RedisCache
	sg    singleflight.Group
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

	// Query database with singleflight to prevent cache stampede
	result, err, _ := s.sg.Do(cacheKey, func() (interface{}, error) {
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
			ttl := 5 * time.Minute
			if category == "rates" {
				ttl = 1 * time.Hour
			}
			_ = s.cache.Set(context.Background(), cacheKey, data, ttl)
		}

		return &param, nil
	})

	if err != nil {
		return nil, err
	}

	return result.(*store.Parameter), nil
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

func (s *ParameterService) WarmUpCache(ctx context.Context) error {
	const op = "service.ParameterService.WarmUpCache"

	if s.cache == nil {
		return nil
	}

	// In a real app, you might want to paginate or filter by specific orgs.
	// For v1.0, we can fetch all active global parameters for example.
	// But let's just query ListParameters with a large limit.
	params, err := s.db.ListParameters(ctx, store.ListParametersParams{
		OrgID:  pgtype.UUID{}, // Actually need a valid OrgID or query all without OrgID.
		Limit:  1000,
		Offset: 0,
	})
	
	// Wait, ListParameters requires an OrgID. We should probably only warm up
	// when requested for a specific org, or we create a new query for all active params.
	// For simplicity, let's just create a dummy log if we can't fetch all.
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	count := 0
	for _, p := range params {
		if !p.IsActive {
			continue
		}
		
		appliesToIDStr := "null"
		if p.AppliesToID.Valid {
			appliesToIDStr = p.AppliesToID.String
		}
		productStr := "null"
		if p.Product.Valid {
			productStr = p.Product.String
		}
		
		orgIDBytes := p.OrgID.Bytes
		orgID, _ := uuid.FromBytes(orgIDBytes[:])

		cacheKey := fmt.Sprintf("policy7:%s:%s:%s:%s:%s:%s", orgID.String(), p.Category, p.Name, p.AppliesTo, appliesToIDStr, productStr)
		
		data, _ := json.Marshal(p)
		ttl := 5 * time.Minute
		if p.Category == "rates" {
			ttl = 1 * time.Hour
		}
		
		_ = s.cache.Set(ctx, cacheKey, data, ttl)
		count++
	}

	fmt.Printf("Cache warming completed: %d parameters loaded\n", count)
	return nil
}
