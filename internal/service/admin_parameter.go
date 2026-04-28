package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/ihsansolusi/policy7/internal/store"
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

func (s *AdminParameterService) Create(ctx context.Context, arg store.CreateParameterParams, changeReason string) (*store.Parameter, error) {
	param, err := s.db.CreateParameter(ctx, arg)
	if err != nil {
		return nil, fmt.Errorf("failed to create parameter: %w", err)
	}

	_, err = s.db.CreateParameterHistory(ctx, store.CreateParameterHistoryParams{
		ParameterID:   param.ID,
		OrgID:         param.OrgID,
		NewValue:      param.Value,
		ChangeType:    "create",
		NewVersion:    param.Version,
		ChangeReason:  pgtype.Text{String: changeReason, Valid: true},
		ChangedBy:     arg.CreatedBy,
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
		ParameterID:   param.ID,
		OrgID:         param.OrgID,
		PreviousValue: param.Value,
		NewValue:      param.Value,
		ChangeType:    "delete",
		PreviousVersion: pgtype.Int4{Int32: param.Version, Valid: true},
		NewVersion:    param.Version,
		ChangeReason:  pgtype.Text{String: reason, Valid: true},
		ChangedBy:     pgUserID,
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
		PreviousVersion: pgtype.Int4{Int32: oldParam.Version, Valid: true},
		NewVersion:      newParam.Version,
		ChangeReason:    pgtype.Text{String: reason, Valid: true},
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

func (s *AdminParameterService) GetHistory(ctx context.Context, id uuid.UUID, orgID uuid.UUID) ([]store.ParameterHistory, error) {
	var pgID, pgOrgID pgtype.UUID
	_ = pgID.Scan(id.String())
	_ = pgOrgID.Scan(orgID.String())

	histories, err := s.db.GetParameterHistory(ctx, store.GetParameterHistoryParams{
		ParameterID: pgID,
		OrgID:       pgOrgID,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get history: %w", err)
	}
	return histories, nil
}

func (s *AdminParameterService) BulkImport(ctx context.Context, orgID, userID uuid.UUID, params []store.CreateParameterParams) (int, error) {
	successCount := 0
	for _, p := range params {
		var pgOrgID, pgUserID pgtype.UUID
		_ = pgOrgID.Scan(orgID.String())
		_ = pgUserID.Scan(userID.String())

		p.OrgID = pgOrgID
		p.CreatedBy = pgUserID
		p.IsActive = true
		p.Version = 1

		_, err := s.Create(ctx, p, "bulk import")
		if err == nil {
			successCount++
		}
	}
	return successCount, nil
}

