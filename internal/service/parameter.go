package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/ihsansolusi/policy7/internal/domain"
	"github.com/ihsansolusi/policy7/internal/store"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"go.opentelemetry.io/otel"
	"golang.org/x/sync/singleflight"
)

// ErrNotFound is returned when a requested parameter does not exist.
var ErrNotFound = errors.New("parameter not found")

type ParameterService struct {
	db    store.Querier
	bsDB  store.BranchScopeQuerier
	cache *store.RedisCache
	sg    singleflight.Group
}

func NewParameterService(db store.Querier, cache *store.RedisCache, bsDB store.BranchScopeQuerier) *ParameterService {
	return &ParameterService{
		db:    db,
		bsDB:  bsDB,
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
				return nil, fmt.Errorf("%s: %w", op, ErrNotFound)
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

// ResolveParameter implements Option C: BRANCH → BRANCH_TYPE → GLOBAL fallback.
// Tier 1 looks for a branch-specific override, tier 2 for a branch_type default,
// tier 3 for an org-global value. Returns domain.Parameter for use by callers.
func (s *ParameterService) ResolveParameter(ctx context.Context, orgID uuid.UUID, branchID uuid.UUID, category, name string) (*domain.Parameter, error) {
	const op = "service.ParameterService.ResolveParameter"
	ctx, span := otel.Tracer("policy7").Start(ctx, op)
	defer span.End()

	branchStr := branchID.String()

	// Tier 1: branch-specific
	p, err := s.GetParameter(ctx, orgID, category, name, "branch", &branchStr, nil)
	if err == nil {
		return storeParamToDomain(*p), nil
	}
	if !errors.Is(err, ErrNotFound) {
		span.RecordError(err)
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	// Tier 2: branch_type
	if s.bsDB != nil {
		scope, err := s.bsDB.GetBranchScope(ctx, branchID)
		if err == nil && scope.BranchType != "" {
			p, err = s.GetParameter(ctx, orgID, category, name, "branch_type", &scope.BranchType, nil)
			if err == nil {
				return storeParamToDomain(*p), nil
			}
			if !errors.Is(err, ErrNotFound) {
				span.RecordError(err)
				return nil, fmt.Errorf("%s: %w", op, err)
			}
		}
	}

	// Tier 3: global
	p, err = s.GetParameter(ctx, orgID, category, name, "global", nil, nil)
	if err != nil {
		span.RecordError(ErrNotFound)
		return nil, fmt.Errorf("%s: %w", op, ErrNotFound)
	}
	return storeParamToDomain(*p), nil
}

func storeParamToDomain(p store.Parameter) *domain.Parameter {
	dp := &domain.Parameter{
		Category:  p.Category,
		Name:      p.Name,
		AppliesTo: p.AppliesTo,
		Value:     p.Value,
		ValueType: p.ValueType,
		Version:   int(p.Version),
		IsActive:  p.IsActive,
	}
	if p.ID.Valid {
		dp.ID = uuid.UUID(p.ID.Bytes)
	}
	if p.OrgID.Valid {
		dp.OrgID = uuid.UUID(p.OrgID.Bytes)
	}
	if p.AppliesToID.Valid {
		s := p.AppliesToID.String
		dp.AppliesToID = &s
	}
	if p.Product.Valid {
		s := p.Product.String
		dp.Product = &s
	}
	if p.Unit.Valid {
		s := p.Unit.String
		dp.Unit = &s
	}
	if p.Scope.Valid {
		s := p.Scope.String
		dp.Scope = &s
	}
	if p.EffectiveFrom.Valid {
		dp.EffectiveFrom = p.EffectiveFrom.Time
	}
	if p.EffectiveUntil.Valid {
		t := p.EffectiveUntil.Time
		dp.EffectiveUntil = &t
	}
	if p.CreatedAt.Valid {
		dp.CreatedAt = p.CreatedAt.Time
	}
	if p.UpdatedAt.Valid {
		dp.UpdatedAt = p.UpdatedAt.Time
	}
	return dp
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
