package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/ihsansolusi/lib7-service-go/audit7client"
	"github.com/ihsansolusi/lib7-service-go/logging"
	"github.com/ihsansolusi/policy7/internal/domain"
	"github.com/ihsansolusi/policy7/internal/service"
	"github.com/ihsansolusi/policy7/internal/store"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/rs/zerolog"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// CategoryHandler serves the Wave C category endpoints: read access for the
// dynamic value-form renderer (value_schema + x-ui), and admin CRUD for
// managing category metadata.
type CategoryHandler struct {
	svc    *service.AdminParameterService
	tracer trace.Tracer
	logger zerolog.Logger
	audit7 *audit7client.Client
}

func NewCategoryHandler(svc *service.AdminParameterService, tracer trace.Tracer, logger zerolog.Logger, audit7 *audit7client.Client) *CategoryHandler {
	return &CategoryHandler{svc: svc, tracer: tracer, logger: logger, audit7: audit7}
}

// categoryResponse is the wire shape for a category. value_schema/default_value
// are surfaced as raw JSON so the frontend adapter receives the full
// JSON-Schema + x-ui/x-rules document untouched.
type categoryResponse struct {
	Code         string          `json:"code"`
	Name         string          `json:"name"`
	Description  string          `json:"description,omitempty"`
	ValueSchema  json.RawMessage `json:"value_schema"`
	DefaultValue json.RawMessage `json:"default_value"`
	DisplayOrder int32           `json:"display_order"`
	Icon         string          `json:"icon,omitempty"`
	Color        string          `json:"color,omitempty"`
	IsActive     bool            `json:"is_active"`
}

func toCategoryResponse(c store.ParameterCategory) categoryResponse {
	resp := categoryResponse{
		Code:         c.Code,
		Name:         c.Name,
		ValueSchema:  c.ValueSchema,
		DefaultValue: c.DefaultValue,
		IsActive:     c.IsActive,
	}
	if c.Description.Valid {
		resp.Description = c.Description.String
	}
	if c.DisplayOrder.Valid {
		resp.DisplayOrder = c.DisplayOrder.Int32
	}
	if c.Icon.Valid {
		resp.Icon = c.Icon.String
	}
	if c.Color.Valid {
		resp.Color = c.Color.String
	}
	if len(resp.ValueSchema) == 0 {
		resp.ValueSchema = json.RawMessage("null")
	}
	if len(resp.DefaultValue) == 0 {
		resp.DefaultValue = json.RawMessage("null")
	}
	return resp
}

// List handles GET /admin/v1/categories.
func (h *CategoryHandler) List(c *gin.Context) {
	const op = "rest.CategoryHandler.List"
	logger := logging.WithTrace(c.Request.Context(), h.logger)
	ctx, span := h.tracer.Start(c.Request.Context(), op)
	defer span.End()

	orgID, ok := getOrgID(c)
	if !ok {
		return
	}

	cats, err := h.svc.ListCategories(ctx, orgID)
	if err != nil {
		span.RecordError(err)
		logger.Error().Err(err).Str("op", op).Msg("failed")
		writeError(c, http.StatusInternalServerError, "POLICY_BACKEND_UNAVAILABLE", "failed to list categories", true, nil)
		return
	}

	out := make([]categoryResponse, 0, len(cats))
	for _, cat := range cats {
		out = append(out, toCategoryResponse(cat))
	}

	span.SetStatus(codes.Ok, "")
	writeSuccess(c, http.StatusOK, out)
}

// GetByCode handles GET /admin/v1/categories/:code.
func (h *CategoryHandler) GetByCode(c *gin.Context) {
	const op = "rest.CategoryHandler.GetByCode"
	logger := logging.WithTrace(c.Request.Context(), h.logger)
	ctx, span := h.tracer.Start(c.Request.Context(), op)
	defer span.End()

	orgID, ok := getOrgID(c)
	if !ok {
		return
	}

	code := c.Param("code")
	cat, err := h.svc.GetCategory(ctx, orgID, code)
	if err != nil {
		span.RecordError(err)
		logger.Error().Err(err).Str("op", op).Str("code", code).Msg("failed")
		writeError(c, http.StatusNotFound, "CATEGORY_NOT_FOUND", err.Error(), false, gin.H{"code": code})
		return
	}

	span.SetStatus(codes.Ok, "")
	writeSuccess(c, http.StatusOK, toCategoryResponse(*cat))
}

// categoryWriteRequest is the create/update body. value_schema/default_value
// are accepted as raw JSON (full JSON-Schema + x-ui/x-rules document).
type categoryWriteRequest struct {
	Code         string          `json:"code"`
	Name         string          `json:"name"`
	Description  *string         `json:"description"`
	ValueSchema  json.RawMessage `json:"value_schema"`
	DefaultValue json.RawMessage `json:"default_value"`
	DisplayOrder *int32          `json:"display_order"`
	Icon         *string         `json:"icon"`
	Color        *string         `json:"color"`
	IsActive     *bool           `json:"is_active"`
}

// mergedCategory holds the resolved write fields after merging a PUT body onto
// the existing row.
type mergedCategory struct {
	Name         string
	Description  pgtype.Text
	ValueSchema  json.RawMessage
	DefaultValue json.RawMessage
	DisplayOrder pgtype.Int4
	Icon         pgtype.Text
	Color        pgtype.Text
	IsActive     bool
}

func mergeCategory(current store.ParameterCategory, req categoryWriteRequest) mergedCategory {
	m := mergedCategory{
		Name:         current.Name,
		Description:  current.Description,
		ValueSchema:  current.ValueSchema,
		DefaultValue: current.DefaultValue,
		DisplayOrder: current.DisplayOrder,
		Icon:         current.Icon,
		Color:        current.Color,
		IsActive:     current.IsActive,
	}
	if strings.TrimSpace(req.Name) != "" {
		m.Name = req.Name
	}
	if req.Description != nil {
		m.Description = optText(req.Description)
	}
	if req.ValueSchema != nil {
		m.ValueSchema = req.ValueSchema
	}
	if req.DefaultValue != nil {
		m.DefaultValue = req.DefaultValue
	}
	if req.DisplayOrder != nil {
		m.DisplayOrder = pgtype.Int4{Int32: *req.DisplayOrder, Valid: true}
	}
	if req.Icon != nil {
		m.Icon = optText(req.Icon)
	}
	if req.Color != nil {
		m.Color = optText(req.Color)
	}
	if req.IsActive != nil {
		m.IsActive = *req.IsActive
	}
	return m
}

// writeSchemaError maps domain validation errors to HTTP 422 and reports whether
// it handled the error. A *domain.SchemaValidationError (value vs value_schema)
// becomes INVALID_PARAMETER_VALUE; a *domain.CategoryError (data-driven category
// gate) becomes INVALID_CATEGORY.
func writeSchemaError(c *gin.Context, err error) bool {
	var schemaErr *domain.SchemaValidationError
	if errors.As(err, &schemaErr) {
		writeError(c, http.StatusUnprocessableEntity, "INVALID_PARAMETER_VALUE",
			"value failed schema validation", false, gin.H{"violations": schemaErr.Errors})
		return true
	}
	var catErr *domain.CategoryError
	if errors.As(err, &catErr) {
		writeError(c, http.StatusUnprocessableEntity, "INVALID_CATEGORY",
			catErr.Error(), false, gin.H{"category": catErr.Code})
		return true
	}
	return false
}

func optText(s *string) pgtype.Text {
	if s == nil {
		return pgtype.Text{Valid: false}
	}
	return pgtype.Text{String: *s, Valid: true}
}

func optInt4(i *int32) pgtype.Int4 {
	if i == nil {
		return pgtype.Int4{Valid: false}
	}
	return pgtype.Int4{Int32: *i, Valid: true}
}

func isDuplicateKey(err error) bool {
	return strings.Contains(strings.ToLower(err.Error()), "duplicate") ||
		strings.Contains(err.Error(), "23505") ||
		strings.Contains(strings.ToLower(err.Error()), "unique constraint")
}
