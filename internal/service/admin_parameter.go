package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/ihsansolusi/policy7/internal/domain"
	"github.com/ihsansolusi/policy7/internal/store"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type AdminParameterService struct {
	db        store.Querier
	cache     *store.RedisCache
	publisher *NATSClient
}

func NewAdminParameterService(db store.Querier, cache *store.RedisCache, publisher *NATSClient) *AdminParameterService {
	return &AdminParameterService{
		db:        db,
		cache:     cache,
		publisher: publisher,
	}
}

// validateCategoryOnCreate is the data-driven category gate for parameter
// creation. Category validity is sourced entirely from parameter_categories —
// there is no hardcoded allowlist — so a category is valid iff an active row
// exists for the org. A missing/inactive category yields a *domain.CategoryError
// (HTTP 422). When the category exists, its value_schema (Wave C,
// PLAN-WC-XUI-CONVENTION) is enforced against the value, yielding a
// *domain.SchemaValidationError (HTTP 422) on violation. Any field requirement
// (e.g. product) comes from the schema's `required`, not from category-specific
// code here.
func (s *AdminParameterService) validateCategoryOnCreate(ctx context.Context, orgID pgtype.UUID, category string, value []byte) error {
	cat, err := s.db.GetParameterCategoryByCode(ctx, store.GetParameterCategoryByCodeParams{
		OrgID: orgID,
		Code:  category,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return &domain.CategoryError{Code: category, Reason: "category does not exist for this org"}
		}
		return fmt.Errorf("failed to load category for validation: %w", err)
	}
	if !cat.IsActive {
		return &domain.CategoryError{Code: category, Reason: "category is not active"}
	}
	return domain.ValidateValue(cat.ValueSchema, value)
}

// validateValueOnUpdate enforces the value_schema for an existing parameter's
// category. Unlike create, it does NOT require the category to exist in
// parameter_categories: legacy parameters may reference categories with no
// metadata row, and those must remain editable. Validation is therefore opt-in
// — a missing category row (or one without a value_schema) accepts the value
// unchanged. A schema violation yields a *domain.SchemaValidationError (422).
func (s *AdminParameterService) validateValueOnUpdate(ctx context.Context, orgID pgtype.UUID, category string, value []byte) error {
	cat, err := s.db.GetParameterCategoryByCode(ctx, store.GetParameterCategoryByCodeParams{
		OrgID: orgID,
		Code:  category,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil // no category metadata → nothing to validate against
		}
		return fmt.Errorf("failed to load category for validation: %w", err)
	}
	return domain.ValidateValue(cat.ValueSchema, value)
}

func (s *AdminParameterService) Create(ctx context.Context, arg store.CreateParameterParams, changeReason string) (*store.Parameter, error) {
	if err := s.validateCategoryOnCreate(ctx, arg.OrgID, arg.Category, arg.Value); err != nil {
		return nil, err
	}

	param, err := s.db.CreateParameter(ctx, arg)
	if err != nil {
		return nil, fmt.Errorf("failed to create parameter: %w", err)
	}

	_, err = s.db.CreateParameterHistory(ctx, store.CreateParameterHistoryParams{
		ParameterID:     param.ID,
		OrgID:           param.OrgID,
		NewValue:        param.Value,
		ChangeType:      "create",
		PreviousVersion: 0,
		NewVersion:      param.Version,
		ChangeReason:    changeReason,
		ChangeMetadata:  []byte("{}"),
		ChangedBy:       arg.CreatedBy,
	})
	if err != nil {
		fmt.Printf("failed to create history: %v\n", err)
	}

	// Publish Event
	if s.publisher != nil {
		orgIDBytes := param.OrgID.Bytes
		orgID, _ := uuid.FromBytes(orgIDBytes[:])
		_ = s.publisher.PublishParameterEvent(ctx, "policy7.params.created", orgID.String(), param)
	}

	return &param, nil
}

func (s *AdminParameterService) GetByID(ctx context.Context, id uuid.UUID, orgID uuid.UUID) (*store.Parameter, error) {
	var pgID, pgOrgID pgtype.UUID
	_ = pgID.Scan(id.String())
	_ = pgOrgID.Scan(orgID.String())

	param, err := s.db.GetParameterByID(ctx, store.GetParameterByIDParams{
		ID:    pgID,
		OrgID: pgOrgID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("parameter not found")
		}
		return nil, fmt.Errorf("failed to get parameter: %w", err)
	}
	return &param, nil
}

func (s *AdminParameterService) List(ctx context.Context, orgID uuid.UUID, limit, offset int32) ([]store.Parameter, error) {
	var pgOrgID pgtype.UUID
	_ = pgOrgID.Scan(orgID.String())

	params, err := s.db.ListParameters(ctx, store.ListParametersParams{
		OrgID:  pgOrgID,
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list parameters: %w", err)
	}
	return params, nil
}

// ListFiltered lists parameters with optional category / product / applies_to
// filters. Empty string for a filter means "skip that filter" (matches all).
// Powers the admin UI list pages (fees, rates, regulatory, etc.).
func (s *AdminParameterService) ListFiltered(ctx context.Context, orgID uuid.UUID, category, product, appliesTo string, limit, offset int32) ([]store.Parameter, error) {
	var pgOrgID pgtype.UUID
	_ = pgOrgID.Scan(orgID.String())

	toPgText := func(v string) pgtype.Text {
		if v == "" {
			return pgtype.Text{Valid: false}
		}
		return pgtype.Text{String: v, Valid: true}
	}

	params, err := s.db.ListParametersFiltered(ctx, store.ListParametersFilteredParams{
		OrgID:     pgOrgID,
		Limit:     limit,
		Offset:    offset,
		Category:  toPgText(category),
		Product:   toPgText(product),
		AppliesTo: toPgText(appliesTo),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list parameters (filtered): %w", err)
	}
	return params, nil
}

func (s *AdminParameterService) Delete(ctx context.Context, id uuid.UUID, orgID uuid.UUID, userID uuid.UUID, reason string) error {
	var pgID, pgOrgID, pgUserID pgtype.UUID
	_ = pgID.Scan(id.String())
	_ = pgOrgID.Scan(orgID.String())
	_ = pgUserID.Scan(userID.String())

	param, err := s.db.GetParameterByID(ctx, store.GetParameterByIDParams{ID: pgID, OrgID: pgOrgID})
	if err != nil {
		return err
	}

	err = s.db.DeactivateParameter(ctx, store.DeactivateParameterParams{
		ID:    pgID,
		OrgID: pgOrgID,
	})
	if err != nil {
		return fmt.Errorf("failed to deactivate parameter: %w", err)
	}

	_, err = s.db.CreateParameterHistory(ctx, store.CreateParameterHistoryParams{
		ParameterID:     param.ID,
		OrgID:           param.OrgID,
		PreviousValue:   param.Value,
		NewValue:        param.Value,
		ChangeType:      "delete",
		PreviousVersion: param.Version,
		NewVersion:      param.Version,
		ChangeReason:    reason,
		ChangeMetadata:  []byte("{}"),
		ChangedBy:       pgUserID,
	})
	if err != nil {
		fmt.Printf("failed to create history: %v\n", err)
	}

	if s.cache != nil {
		pattern := fmt.Sprintf("policy7:%s:%s:*", orgID.String(), param.Category)
		_ = s.cache.DelPattern(ctx, pattern)
	}

	// Publish Event
	if s.publisher != nil {
		orgIDBytes := param.OrgID.Bytes
		orgID, _ := uuid.FromBytes(orgIDBytes[:])
		_ = s.publisher.PublishParameterEvent(ctx, "policy7.params.deleted", orgID.String(), param)
	}

	return nil
}

func (s *AdminParameterService) Update(ctx context.Context, id uuid.UUID, orgID uuid.UUID, userID uuid.UUID, newValue []byte, reason string) (*store.Parameter, error) {
	var pgID, pgOrgID, pgUserID pgtype.UUID
	_ = pgID.Scan(id.String())
	_ = pgOrgID.Scan(orgID.String())
	_ = pgUserID.Scan(userID.String())

	oldParam, err := s.db.GetParameterByID(ctx, store.GetParameterByIDParams{ID: pgID, OrgID: pgOrgID})
	if err != nil {
		return nil, err
	}

	if !oldParam.IsActive {
		return nil, fmt.Errorf("cannot update inactive parameter")
	}

	if err := s.validateValueOnUpdate(ctx, oldParam.OrgID, oldParam.Category, newValue); err != nil {
		return nil, err
	}

	err = s.db.DeactivateParameter(ctx, store.DeactivateParameterParams{
		ID:    pgID,
		OrgID: pgOrgID,
	})
	if err != nil {
		return nil, err
	}

	newParam, err := s.db.CreateParameter(ctx, store.CreateParameterParams{
		OrgID:          oldParam.OrgID,
		Category:       oldParam.Category,
		Name:           oldParam.Name,
		AppliesTo:      oldParam.AppliesTo,
		AppliesToID:    oldParam.AppliesToID,
		Product:        oldParam.Product,
		Value:          newValue,
		ValueType:      oldParam.ValueType,
		Unit:           oldParam.Unit,
		Scope:          oldParam.Scope,
		EffectiveFrom:  oldParam.EffectiveFrom,
		EffectiveUntil: oldParam.EffectiveUntil,
		Version:        oldParam.Version + 1,
		IsActive:       true,
		CreatedBy:      pgUserID,
	})
	if err != nil {
		return nil, err
	}

	_, err = s.db.CreateParameterHistory(ctx, store.CreateParameterHistoryParams{
		ParameterID:     newParam.ID,
		OrgID:           newParam.OrgID,
		PreviousValue:   oldParam.Value,
		NewValue:        newParam.Value,
		ChangeType:      "update",
		PreviousVersion: oldParam.Version,
		NewVersion:      newParam.Version,
		ChangeReason:    reason,
		ChangeMetadata:  []byte("{}"),
		ChangedBy:       pgUserID,
	})

	if s.cache != nil {
		pattern := fmt.Sprintf("policy7:%s:%s:*", orgID.String(), oldParam.Category)
		_ = s.cache.DelPattern(ctx, pattern)
	}

	// Publish Event
	if s.publisher != nil {
		orgIDBytes := newParam.OrgID.Bytes
		orgID, _ := uuid.FromBytes(orgIDBytes[:])
		_ = s.publisher.PublishParameterEvent(ctx, "policy7.params.updated", orgID.String(), newParam)
	}

	return &newParam, nil
}

// GetHistory returns the FULL version chain for the parameter identity that `id`
// belongs to (#587). Each version is a separate parameters row (own id), so the
// history is gathered across all rows sharing the identity tuple
// (org_id, category, name, applies_to, applies_to_id, product), ordered oldest→newest.
func (s *AdminParameterService) GetHistory(ctx context.Context, id uuid.UUID, orgID uuid.UUID) ([]store.ParameterHistory, error) {
	var pgID, pgOrgID pgtype.UUID
	_ = pgID.Scan(id.String())
	_ = pgOrgID.Scan(orgID.String())

	histories, err := s.db.GetParameterHistoryByIdentity(ctx, store.GetParameterHistoryByIdentityParams{
		ID:    pgID,
		OrgID: pgOrgID,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get history: %w", err)
	}
	return histories, nil
}

// CursorParams carries request fields for DataTable cursor-based pagination.
type CursorParams struct {
	OrgID      uuid.UUID
	BranchID   uuid.UUID
	ReqType    string
	PageSize   int
	TopData    string
	BottomData string
	Search     string
}

// CursorResult is the DataTable cursor pagination response.
type CursorResult struct {
	Data      []store.Parameter
	AllowNext bool
	AllowPrev bool
}

func (s *AdminParameterService) ListParamsCursor(ctx context.Context, params CursorParams) (*CursorResult, error) {
	const op = "service.AdminParameterService.ListParamsCursor"
	_ = op
	return &CursorResult{Data: []store.Parameter{}, AllowNext: false, AllowPrev: false}, nil
}

// ListCategories returns all category metadata (active + inactive) for an org,
// ordered for display. Powers the Wave C category-driven admin UI.
func (s *AdminParameterService) ListCategories(ctx context.Context, orgID uuid.UUID) ([]store.ParameterCategory, error) {
	var pgOrgID pgtype.UUID
	_ = pgOrgID.Scan(orgID.String())

	cats, err := s.db.ListParameterCategories(ctx, pgOrgID)
	if err != nil {
		return nil, fmt.Errorf("failed to list categories: %w", err)
	}
	return cats, nil
}

// GetCategory returns a single category by code, scoped to org.
func (s *AdminParameterService) GetCategory(ctx context.Context, orgID uuid.UUID, code string) (*store.ParameterCategory, error) {
	var pgOrgID pgtype.UUID
	_ = pgOrgID.Scan(orgID.String())

	cat, err := s.db.GetParameterCategoryByCode(ctx, store.GetParameterCategoryByCodeParams{
		OrgID: pgOrgID,
		Code:  code,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("category not found")
		}
		return nil, fmt.Errorf("failed to get category: %w", err)
	}
	return &cat, nil
}

// CreateCategory inserts a new category. The value_schema (if any) is validated
// as well-formed JSON before persisting; a duplicate code returns an error the
// handler maps to 409.
func (s *AdminParameterService) CreateCategory(ctx context.Context, arg store.CreateParameterCategoryParams) (*store.ParameterCategory, error) {
	if err := validateSchemaWellFormed(arg.ValueSchema); err != nil {
		return nil, err
	}
	cat, err := s.db.CreateParameterCategory(ctx, arg)
	if err != nil {
		return nil, fmt.Errorf("failed to create category: %w", err)
	}
	return &cat, nil
}

// UpdateCategory updates an existing category by code.
func (s *AdminParameterService) UpdateCategory(ctx context.Context, arg store.UpdateParameterCategoryParams) (*store.ParameterCategory, error) {
	if err := validateSchemaWellFormed(arg.ValueSchema); err != nil {
		return nil, err
	}
	cat, err := s.db.UpdateParameterCategory(ctx, arg)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("category not found")
		}
		return nil, fmt.Errorf("failed to update category: %w", err)
	}
	return &cat, nil
}

// DeleteCategory soft-deletes a category (is_active = FALSE), consistent with
// the parameter soft-delete model. Re-enable via UpdateCategory(is_active=true).
func (s *AdminParameterService) DeleteCategory(ctx context.Context, orgID, userID uuid.UUID, code string) error {
	var pgOrgID, pgUserID pgtype.UUID
	_ = pgOrgID.Scan(orgID.String())
	_ = pgUserID.Scan(userID.String())

	if err := s.db.DeactivateParameterCategory(ctx, store.DeactivateParameterCategoryParams{
		OrgID:     pgOrgID,
		Code:      code,
		UpdatedBy: pgUserID,
	}); err != nil {
		return fmt.Errorf("failed to delete category: %w", err)
	}
	return nil
}

// validateSchemaWellFormed rejects a value_schema that is present but not a
// valid JSON object. An empty/absent schema is allowed (free-form category).
func validateSchemaWellFormed(schema []byte) error {
	if len(schema) == 0 || string(schema) == "null" {
		return nil
	}
	var probe map[string]interface{}
	if err := json.Unmarshal(schema, &probe); err != nil {
		return &domain.SchemaValidationError{Errors: []domain.FieldError{
			{Field: "value_schema", Message: "must be a valid JSON Schema object"},
		}}
	}
	return nil
}
