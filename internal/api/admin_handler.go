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
	"github.com/ihsansolusi/lib7-service-go/logging"
	"github.com/ihsansolusi/lib7-service-go/middleware"
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
}

func NewAdminHandler(svc *service.AdminParameterService, tracer trace.Tracer, logger zerolog.Logger) *AdminHandler {
	return &AdminHandler{svc: svc, tracer: tracer, logger: logger}
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

func (h *AdminHandler) Create(c *gin.Context) {
	const op = "rest.AdminHandler.Create"
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

	var req struct {
		Category     string          `json:"category"`
		Name         string          `json:"name"`
		AppliesTo    string          `json:"applies_to"`
		AppliesToID  *string         `json:"applies_to_id"`
		Product      *string         `json:"product"`
		Value        json.RawMessage `json:"value"`
		ValueType    string          `json:"value_type"`
		Unit         *string         `json:"unit"`
		Scope        *string         `json:"scope"`
		ChangeReason string          `json:"change_reason"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, http.StatusBadRequest, "INVALID_REQUEST", err.Error(), false, nil)
		return
	}
	if err := validateCategoryContext(req.Category, req.AppliesTo, req.AppliesToID, req.Product); err != nil {
		writeError(c, http.StatusUnprocessableEntity, "INVALID_CALLER_CONTEXT", err.Error(), false, nil)
		return
	}

	var pgOrgID, pgUserID pgtype.UUID
	_ = pgOrgID.Scan(orgID.String())
	_ = pgUserID.Scan(userID.String())

	var appliesToID pgtype.Text
	if req.AppliesToID != nil {
		appliesToID = pgtype.Text{String: *req.AppliesToID, Valid: true}
	}
	var product pgtype.Text
	if req.Product != nil {
		product = pgtype.Text{String: *req.Product, Valid: true}
	}
	var unit pgtype.Text
	if req.Unit != nil {
		unit = pgtype.Text{String: *req.Unit, Valid: true}
	}
	var scope pgtype.Text
	if req.Scope != nil {
		scope = pgtype.Text{String: *req.Scope, Valid: true}
	}

	param, err := h.svc.Create(ctx, store.CreateParameterParams{
		OrgID:          pgOrgID,
		Category:       req.Category,
		Name:           req.Name,
		AppliesTo:      req.AppliesTo,
		AppliesToID:    appliesToID,
		Product:        product,
		Value:          req.Value,
		ValueType:      req.ValueType,
		Unit:           unit,
		Scope:          scope,
		EffectiveFrom:  pgtype.Timestamptz{Time: time.Now(), Valid: true},
		EffectiveUntil: pgtype.Timestamptz{Valid: false},
		IsActive:       true,
		Version:        1,
		CreatedBy:      pgUserID,
	}, req.ChangeReason)

	if err != nil {
		span.RecordError(err)
		logger.Error().Err(err).Str("op", op).Msg("failed")
		writeError(c, http.StatusInternalServerError, "POLICY_BACKEND_UNAVAILABLE", err.Error(), true, nil)
		return
	}

	span.SetStatus(codes.Ok, "")
	writeSuccess(c, http.StatusCreated, gin.H{
		"parameter": param,
		"audit": gin.H{
			"history_change_type": "create",
			"event_type":          "policy7.params.created",
		},
	})
}

func (h *AdminHandler) Update(c *gin.Context) {
	const op = "rest.AdminHandler.Update"
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

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		writeError(c, http.StatusBadRequest, "INVALID_CALLER_CONTEXT", "invalid parameter ID", false, gin.H{"field": "id"})
		return
	}

	var req struct {
		Value        json.RawMessage `json:"value"`
		ChangeReason string          `json:"change_reason"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, http.StatusBadRequest, "INVALID_REQUEST", err.Error(), false, nil)
		return
	}
	if len(req.Value) == 0 {
		writeError(c, http.StatusUnprocessableEntity, "INVALID_PARAMETER_SHAPE", "value is required", false, gin.H{"field": "value"})
		return
	}

	param, err := h.svc.Update(ctx, id, orgID, userID, req.Value, req.ChangeReason)
	if err != nil {
		span.RecordError(err)
		logger.Error().Err(err).Str("op", op).Msg("failed")
		if strings.Contains(err.Error(), "inactive parameter") {
			writeError(c, http.StatusConflict, "POLICY_CONFLICT", err.Error(), false, nil)
			return
		}
		writeError(c, http.StatusInternalServerError, "POLICY_BACKEND_UNAVAILABLE", err.Error(), true, nil)
		return
	}

	span.SetStatus(codes.Ok, "")
	writeSuccess(c, http.StatusOK, gin.H{
		"parameter": param,
		"audit": gin.H{
			"history_change_type": "update",
			"event_type":          "policy7.params.updated",
		},
	})
}

func (h *AdminHandler) Delete(c *gin.Context) {
	const op = "rest.AdminHandler.Delete"
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

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		writeError(c, http.StatusBadRequest, "INVALID_CALLER_CONTEXT", "invalid parameter ID", false, gin.H{"field": "id"})
		return
	}

	var req struct {
		ChangeReason string `json:"change_reason"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, http.StatusBadRequest, "INVALID_REQUEST", err.Error(), false, nil)
		return
	}
	if strings.TrimSpace(req.ChangeReason) == "" {
		writeError(c, http.StatusUnprocessableEntity, "INVALID_CALLER_CONTEXT", "change_reason is required", false, gin.H{"field": "change_reason"})
		return
	}

	err = h.svc.Delete(ctx, id, orgID, userID, req.ChangeReason)
	if err != nil {
		span.RecordError(err)
		logger.Error().Err(err).Str("op", op).Msg("failed")
		writeError(c, http.StatusInternalServerError, "POLICY_BACKEND_UNAVAILABLE", err.Error(), true, nil)
		return
	}

	span.SetStatus(codes.Ok, "")
	writeNoContent(c)
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

// ParamsQuery handles POST /admin/v1/params/query — DataTable cursor-based pagination.
func (h *AdminHandler) ParamsQuery(c *gin.Context) {
	const op = "rest.AdminHandler.ParamsQuery"
	logger := logging.WithTrace(c.Request.Context(), h.logger)
	ctx, span := h.tracer.Start(c.Request.Context(), op)
	defer span.End()

	orgID, ok := getOrgID(c)
	if !ok {
		return
	}

	var req DataTableRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		span.RecordError(err)
		writeError(c, http.StatusBadRequest, "INVALID_REQUEST", err.Error(), false, nil)
		return
	}

	payload := middleware.MustGetPayload(c)
	result, err := h.svc.ListParamsCursor(ctx, service.CursorParams{
		OrgID:      orgID,
		BranchID:   payload.BranchID,
		ReqType:    req.ReqType,
		PageSize:   req.PageSize,
		TopData:    req.TopData,
		BottomData: req.BottomData,
		Search:     req.SearchText,
	})
	if err != nil {
		span.RecordError(err)
		logger.Error().Err(err).Str("op", op).Msg("failed")
		writeError(c, http.StatusInternalServerError, "POLICY_BACKEND_UNAVAILABLE", err.Error(), true, nil)
		return
	}

	span.SetStatus(codes.Ok, "")
	c.JSON(http.StatusOK, DataTableResponse{
		Data:      result.Data,
		AllowNext: result.AllowNext,
		AllowPrev: result.AllowPrev,
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

func validateCategoryContext(category, appliesTo string, appliesToID, product *string) error {
	supported := map[string]bool{
		"transaction_limit":    true,
		"approval_threshold":   true,
		"operational_hours":    true,
		"product_access":       true,
		"rate":                 true,
		"fee":                  true,
		"regulatory_threshold": true,
	}
	if !supported[category] {
		return errors.New("unsupported category")
	}
	if appliesTo == "branch" && (appliesToID == nil || strings.TrimSpace(*appliesToID) == "") {
		return errors.New("applies_to_id is required for branch scope")
	}
	productScoped := category == "transaction_limit" || category == "product_access" || category == "rate" || category == "fee"
	if productScoped && (product == nil || strings.TrimSpace(*product) == "") {
		return errors.New("product is required for this category")
	}
	return nil
}
