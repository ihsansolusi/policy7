package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/ihsansolusi/lib7-service-go/audit7client"
	"github.com/ihsansolusi/lib7-service-go/logging"
	"github.com/ihsansolusi/policy7/internal/service"
	"github.com/ihsansolusi/policy7/internal/store"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/rs/zerolog"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// DataTableRequest is the cursor-based pagination request body for admin list endpoints.
type DataTableRequest struct {
	ReqType    string `json:"req_type"`
	PageSize   int    `json:"page_size"`
	TopData    string `json:"top_data"`
	BottomData string `json:"bottom_data"`
	SearchText string `json:"search_text"`
	SortColumn string `json:"sort_column"`
}

// DataTableResponse wraps cursor-paginated results for the admin DataTable UI.
type DataTableResponse struct {
	Data      any  `json:"data"`
	AllowNext bool `json:"allow_next"`
	AllowPrev bool `json:"allow_prev"`
}

type AdminHandler struct {
	svc    *service.AdminParameterService
	tracer trace.Tracer
	logger zerolog.Logger
	audit7 *audit7client.Client
}

func NewAdminHandler(svc *service.AdminParameterService, tracer trace.Tracer, logger zerolog.Logger, audit7 *audit7client.Client) *AdminHandler {
	return &AdminHandler{svc: svc, tracer: tracer, logger: logger, audit7: audit7}
}

func (h *AdminHandler) List(c *gin.Context) {
	const op = "rest.AdminHandler.List"
	logger := logging.WithTrace(c.Request.Context(), h.logger)
	ctx, span := h.tracer.Start(c.Request.Context(), op)
	defer span.End()

	orgID, ok := getOrgID(c)
	if !ok {
		return
	}

	limit := int32(10)
	if l := c.Query("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = int32(parsed)
		}
	}

	offset := int32(0)
	if o := c.Query("offset"); o != "" {
		if parsed, err := strconv.Atoi(o); err == nil && parsed >= 0 {
			offset = int32(parsed)
		}
	}

	// Optional filters: empty string means "skip that filter".
	category := c.Query("category")
	product := c.Query("product")
	appliesTo := c.Query("applies_to")

	var (
		params []store.Parameter
		err    error
	)
	if category == "" && product == "" && appliesTo == "" {
		params, err = h.svc.List(ctx, orgID, limit, offset)
	} else {
		params, err = h.svc.ListFiltered(ctx, orgID, category, product, appliesTo, limit, offset)
	}
	if err != nil {
		span.RecordError(err)
		logger.Error().Err(err).Str("op", op).Msg("failed")
		writeError(c, http.StatusInternalServerError, "POLICY_BACKEND_UNAVAILABLE", "failed to list parameters", true, nil)
		return
	}

	span.SetStatus(codes.Ok, "")
	writeSuccess(c, http.StatusOK, params)
}

func (h *AdminHandler) GetByID(c *gin.Context) {
	const op = "rest.AdminHandler.GetByID"
	logger := logging.WithTrace(c.Request.Context(), h.logger)
	ctx, span := h.tracer.Start(c.Request.Context(), op)
	defer span.End()

	orgID, ok := getOrgID(c)
	if !ok {
		return
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		writeError(c, http.StatusBadRequest, "INVALID_CALLER_CONTEXT", "invalid parameter ID format", false, gin.H{"field": "id"})
		return
	}

	param, err := h.svc.GetByID(ctx, id, orgID)
	if err != nil {
		span.RecordError(err)
		logger.Error().Err(err).Str("op", op).Msg("failed")
		writeError(c, http.StatusNotFound, "PARAMETER_NOT_FOUND", err.Error(), false, nil)
		return
	}

	span.SetStatus(codes.Ok, "")
	writeSuccess(c, http.StatusOK, param)
}

func (h *AdminHandler) GetHistory(c *gin.Context) {
	const op = "rest.AdminHandler.GetHistory"
	logger := logging.WithTrace(c.Request.Context(), h.logger)
	ctx, span := h.tracer.Start(c.Request.Context(), op)
	defer span.End()

	orgID, ok := getOrgID(c)
	if !ok {
		return
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		writeError(c, http.StatusBadRequest, "INVALID_CALLER_CONTEXT", "invalid parameter ID format", false, gin.H{"field": "id"})
		return
	}

	histories, err := h.svc.GetHistory(ctx, id, orgID)
	if err != nil {
		span.RecordError(err)
		logger.Error().Err(err).Str("op", op).Msg("failed")
		writeError(c, http.StatusInternalServerError, "POLICY_BACKEND_UNAVAILABLE", err.Error(), true, nil)
		return
	}

	span.SetStatus(codes.Ok, "")
	writeSuccess(c, http.StatusOK, histories)
}

func (h *AdminHandler) BulkImport(c *gin.Context) {
	const op = "rest.AdminHandler.BulkImport"
	logger := logging.WithTrace(c.Request.Context(), h.logger)
	ctx, span := h.tracer.Start(c.Request.Context(), op)
	defer span.End()

	orgID, ok := getOrgID(c)
	if !ok {
		return
	}
	userID, ok := getUserID(c)
	if !ok {
		return
	}

	var req []struct {
		Category    string          `json:"category"`
		Name        string          `json:"name"`
		AppliesTo   string          `json:"applies_to"`
		AppliesToID *string         `json:"applies_to_id"`
		Product     *string         `json:"product"`
		Value       json.RawMessage `json:"value"`
		ValueType   string          `json:"value_type"`
		Unit        *string         `json:"unit"`
		Scope       *string         `json:"scope"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, http.StatusBadRequest, "INVALID_REQUEST", err.Error(), false, nil)
		return
	}

	var paramsToCreate []store.CreateParameterParams
	for _, p := range req {
		var appliesToID pgtype.Text
		if p.AppliesToID != nil {
			appliesToID = pgtype.Text{String: *p.AppliesToID, Valid: true}
		}
		var product pgtype.Text
		if p.Product != nil {
			product = pgtype.Text{String: *p.Product, Valid: true}
		}
		var unit pgtype.Text
		if p.Unit != nil {
			unit = pgtype.Text{String: *p.Unit, Valid: true}
		}
		var scope pgtype.Text
		if p.Scope != nil {
			scope = pgtype.Text{String: *p.Scope, Valid: true}
		}

		paramsToCreate = append(paramsToCreate, store.CreateParameterParams{
			Category:    p.Category,
			Name:        p.Name,
			AppliesTo:   p.AppliesTo,
			AppliesToID: appliesToID,
			Product:     product,
			Value:       p.Value,
			ValueType:   p.ValueType,
			Unit:        unit,
			Scope:       scope,
			// effective_from is NOT NULL; the INSERT lists it explicitly so the
			// column DEFAULT does not apply. Stamp now() like the single-create
			// handler — otherwise every bulk row fails the not-null constraint.
			EffectiveFrom: pgtype.Timestamptz{Time: time.Now(), Valid: true},
		})
	}

	count, err := h.svc.BulkImport(ctx, orgID, userID, paramsToCreate)
	if err != nil {
		span.RecordError(err)
		logger.Error().Err(err).Str("op", op).Msg("failed")
		writeError(c, http.StatusInternalServerError, "POLICY_BACKEND_UNAVAILABLE", err.Error(), true, nil)
		return
	}

	span.SetStatus(codes.Ok, "")
	writeSuccess(c, http.StatusOK, gin.H{
		"message": "bulk import completed",
		"summary": gin.H{
			"success_count": count,
			"total_count":   len(req),
		},
	})
}

func getOrgID(c *gin.Context) (uuid.UUID, bool) {
	orgIDStr := c.GetHeader("X-Org-ID")
	if orgIDStr == "" {
		writeError(c, http.StatusBadRequest, "INVALID_CALLER_CONTEXT", "X-Org-ID header is required", false, gin.H{"field": "org_id"})
		return uuid.Nil, false
	}
	orgID, err := uuid.Parse(orgIDStr)
	if err != nil {
		writeError(c, http.StatusBadRequest, "INVALID_CALLER_CONTEXT", "invalid org ID format", false, gin.H{"field": "org_id"})
		return uuid.Nil, false
	}
	return orgID, true
}

func getUserID(c *gin.Context) (uuid.UUID, bool) {
	userIDStr := c.GetHeader("X-User-ID")
	if userIDStr == "" {
		writeError(c, http.StatusBadRequest, "INVALID_CALLER_CONTEXT", "X-User-ID header is required", false, gin.H{"field": "user_id"})
		return uuid.Nil, false
	}
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		writeError(c, http.StatusBadRequest, "INVALID_CALLER_CONTEXT", "invalid user ID format", false, gin.H{"field": "user_id"})
		return uuid.Nil, false
	}
	return userID, true
}

// validateScopeContext validates the scope shape of a parameter write. It is
// intentionally NOT a category gate: category validity is data-driven and
// enforced by the service against parameter_categories, and any field
// requirement (e.g. product) is declared by the category's value_schema and
// enforced by domain.ValidateValue. The only scope rule that lives here is the
// branch-scope identifier requirement, which is request-shape, not category.
func validateScopeContext(appliesTo string, appliesToID *string) error {
	if appliesTo == "branch" && (appliesToID == nil || strings.TrimSpace(*appliesToID) == "") {
		return errors.New("applies_to_id is required for branch scope")
	}
	return nil
}
