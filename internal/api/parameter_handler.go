package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/ihsansolusi/lib7-service-go/logging"
	"github.com/ihsansolusi/policy7/internal/service"
	"github.com/rs/zerolog"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

type ParameterHandler struct {
	svc    *service.ParameterService
	tracer trace.Tracer
	logger zerolog.Logger
}

func NewParameterHandler(svc *service.ParameterService, tracer trace.Tracer, logger zerolog.Logger) *ParameterHandler {
	return &ParameterHandler{svc: svc, tracer: tracer, logger: logger}
}

func (h *ParameterHandler) GetParameter(c *gin.Context) {
	const op = "rest.ParameterHandler.GetParameter"
	logger := logging.WithTrace(c.Request.Context(), h.logger)
	ctx, span := h.tracer.Start(c.Request.Context(), op)
	defer span.End()

	category := c.Param("category")
	name := c.Param("name")

	orgIDStr := c.GetHeader("X-Org-ID")
	if orgIDStr == "" {
		writeError(c, http.StatusBadRequest, "INVALID_CALLER_CONTEXT", "X-Org-ID header is required", false, gin.H{"field": "org_id"})
		return
	}

	orgID, err := uuid.Parse(orgIDStr)
	if err != nil {
		writeError(c, http.StatusBadRequest, "INVALID_CALLER_CONTEXT", "invalid org ID format", false, gin.H{"field": "org_id"})
		return
	}

	appliesTo := c.Query("applies_to")
	if appliesTo == "" {
		appliesTo = "global"
	}

	var appliesToID *string
	if id := c.Query("applies_to_id"); id != "" {
		appliesToID = &id
	}

	var product *string
	if p := c.Query("product"); p != "" {
		product = &p
	}

	param, err := h.svc.GetParameter(ctx, orgID, category, name, appliesTo, appliesToID, product)
	if err != nil {
		span.RecordError(err)
		logger.Error().Err(err).Str("op", op).Msg("failed")
		writeError(c, http.StatusNotFound, "PARAMETER_NOT_FOUND", "parameter not found", false, nil)
		return
	}

	span.SetStatus(codes.Ok, "")
	writeSuccess(c, http.StatusOK, param)
}

func (h *ParameterHandler) GetEffectiveParameter(c *gin.Context) {
	const op = "rest.ParameterHandler.GetEffectiveParameter"
	logger := logging.WithTrace(c.Request.Context(), h.logger)
	ctx, span := h.tracer.Start(c.Request.Context(), op)
	defer span.End()

	category := c.Param("category")
	name := c.Param("name")

	orgIDStr := c.GetHeader("X-Org-ID")
	if orgIDStr == "" {
		writeError(c, http.StatusBadRequest, "INVALID_CALLER_CONTEXT", "X-Org-ID header is required", false, gin.H{"field": "org_id"})
		return
	}

	orgID, err := uuid.Parse(orgIDStr)
	if err != nil {
		writeError(c, http.StatusBadRequest, "INVALID_CALLER_CONTEXT", "invalid org ID format", false, gin.H{"field": "org_id"})
		return
	}

	var product *string
	if p := c.Query("product"); p != "" {
		product = &p
	}

	var userID, roleID, branchID *string
	if u := c.Query("user_id"); u != "" {
		userID = &u
	}
	if r := c.Query("role_id"); r != "" {
		roleID = &r
	}
	if b := c.Query("branch_id"); b != "" {
		branchID = &b
	}

	resCtx := service.ResolutionContext{
		UserID:   userID,
		RoleID:   roleID,
		BranchID: branchID,
		Global:   true,
	}

	param, err := h.svc.GetEffectiveParameter(ctx, orgID, category, name, product, resCtx)
	if err != nil {
		span.RecordError(err)
		logger.Error().Err(err).Str("op", op).Msg("failed")
		writeError(c, http.StatusNotFound, "PARAMETER_NOT_FOUND", "effective parameter not found", false, nil)
		return
	}

	span.SetStatus(codes.Ok, "")
	writeSuccess(c, http.StatusOK, param)
}

func (h *ParameterHandler) ValidateTransactionLimit(c *gin.Context) {
	const op = "rest.ParameterHandler.ValidateTransactionLimit"
	logger := logging.WithTrace(c.Request.Context(), h.logger)
	ctx, span := h.tracer.Start(c.Request.Context(), op)
	defer span.End()

	orgIDStr := c.GetHeader("X-Org-ID")
	if orgIDStr == "" {
		writeError(c, http.StatusBadRequest, "INVALID_CALLER_CONTEXT", "X-Org-ID header is required", false, gin.H{"field": "org_id"})
		return
	}
	orgID, err := uuid.Parse(orgIDStr)
	if err != nil {
		writeError(c, http.StatusBadRequest, "INVALID_CALLER_CONTEXT", "invalid org ID", false, gin.H{"field": "org_id"})
		return
	}

	var req struct {
		Name     string  `json:"name"`
		Amount   float64 `json:"amount"`
		UserID   *string `json:"user_id"`
		RoleID   *string `json:"role_id"`
		BranchID *string `json:"branch_id"`
		Product  *string `json:"product"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, http.StatusBadRequest, "INVALID_REQUEST", err.Error(), false, nil)
		return
	}
	if strings.TrimSpace(req.Name) == "" {
		writeError(c, http.StatusBadRequest, "INVALID_CALLER_CONTEXT", "name is required", false, gin.H{"field": "name"})
		return
	}
	if req.RoleID == nil || strings.TrimSpace(*req.RoleID) == "" {
		writeError(c, http.StatusBadRequest, "INVALID_CALLER_CONTEXT", "role_id or role_code is required for transaction_limit", false, gin.H{"field": "role_id"})
		return
	}
	if req.Product == nil || strings.TrimSpace(*req.Product) == "" {
		writeError(c, http.StatusBadRequest, "INVALID_CALLER_CONTEXT", "product is required for transaction_limit", false, gin.H{"field": "product"})
		return
	}

	resCtx := service.ResolutionContext{
		UserID:   req.UserID,
		RoleID:   req.RoleID,
		BranchID: req.BranchID,
		Global:   true,
	}

	param, err := h.svc.GetEffectiveParameter(ctx, orgID, "transaction_limit", req.Name, req.Product, resCtx)
	if err != nil {
		span.RecordError(err)
		logger.Error().Err(err).Str("op", op).Msg("failed")
		writeError(c, http.StatusNotFound, "PARAMETER_NOT_FOUND", "limit parameter not found", false, nil)
		return
	}

	var limitData struct {
		Limit float64 `json:"limit"`
	}
	if err := json.Unmarshal(param.Value, &limitData); err != nil {
		span.RecordError(err)
		logger.Error().Err(err).Str("op", op).Msg("failed to parse limit")
		writeError(c, http.StatusUnprocessableEntity, "INVALID_PARAMETER_SHAPE", "failed to parse limit parameter value", false, nil)
		return
	}

	isValid := req.Amount <= limitData.Limit
	span.SetStatus(codes.Ok, "")
	writeSuccess(c, http.StatusOK, gin.H{
		"is_valid":       isValid,
		"amount":         req.Amount,
		"limit":          limitData.Limit,
		"parameter_used": param.ID,
	})
}

func (h *ParameterHandler) GetApprovalThresholds(c *gin.Context) {
	const op = "rest.ParameterHandler.GetApprovalThresholds"
	ctx, span := h.tracer.Start(c.Request.Context(), op)
	defer span.End()
	h.handleCategoryRequest(c, ctx, "approval_threshold")
}

func (h *ParameterHandler) GetOperationalHours(c *gin.Context) {
	const op = "rest.ParameterHandler.GetOperationalHours"
	ctx, span := h.tracer.Start(c.Request.Context(), op)
	defer span.End()
	h.handleCategoryRequest(c, ctx, "operational_hours")
}

func (h *ParameterHandler) GetProductAccess(c *gin.Context) {
	const op = "rest.ParameterHandler.GetProductAccess"
	ctx, span := h.tracer.Start(c.Request.Context(), op)
	defer span.End()
	h.handleCategoryRequest(c, ctx, "product_access")
}

// handleCategoryRequest is a shared helper for category-specific GET endpoints.
// The caller creates the root span; this function records errors and sets status on it.
func (h *ParameterHandler) handleCategoryRequest(c *gin.Context, ctx context.Context, category string) {
	const op = "rest.ParameterHandler.handleCategoryRequest"
	span := trace.SpanFromContext(ctx)
	logger := logging.WithTrace(ctx, h.logger)

	name := c.Query("name")
	if name == "" {
		writeError(c, http.StatusBadRequest, "INVALID_CALLER_CONTEXT", "name query parameter is required", false, gin.H{"field": "name"})
		return
	}

	orgIDStr := c.GetHeader("X-Org-ID")
	if orgIDStr == "" {
		writeError(c, http.StatusBadRequest, "INVALID_CALLER_CONTEXT", "X-Org-ID header is required", false, gin.H{"field": "org_id"})
		return
	}
	orgID, err := uuid.Parse(orgIDStr)
	if err != nil {
		writeError(c, http.StatusBadRequest, "INVALID_CALLER_CONTEXT", "invalid org ID format", false, gin.H{"field": "org_id"})
		return
	}

	var product *string
	if p := c.Query("product"); p != "" {
		product = &p
	}

	var userID, roleID, branchID *string
	if u := c.Query("user_id"); u != "" {
		userID = &u
	}
	if r := c.Query("role_id"); r != "" {
		roleID = &r
	}
	if b := c.Query("branch_id"); b != "" {
		branchID = &b
	}

	if roleID == nil || strings.TrimSpace(*roleID) == "" {
		writeError(c, http.StatusBadRequest, "INVALID_CALLER_CONTEXT", "role_id or role_code is required for this category", false, gin.H{"field": "role_id"})
		return
	}
	if (category == "product_access" || category == "approval_threshold") && (product == nil || strings.TrimSpace(*product) == "") {
		writeError(c, http.StatusBadRequest, "INVALID_CALLER_CONTEXT", "product is required for this category", false, gin.H{"field": "product"})
		return
	}

	resCtx := service.ResolutionContext{
		UserID:   userID,
		RoleID:   roleID,
		BranchID: branchID,
		Global:   true,
	}

	param, err := h.svc.GetEffectiveParameter(ctx, orgID, category, name, product, resCtx)
	if err != nil {
		span.RecordError(err)
		logger.Error().Err(err).Str("op", op).Msg("failed")
		writeError(c, http.StatusNotFound, "PARAMETER_NOT_FOUND", "effective parameter not found", false, nil)
		return
	}

	span.SetStatus(codes.Ok, "")
	writeSuccess(c, http.StatusOK, param)
}

func (h *ParameterHandler) GetRates(c *gin.Context) {
	const op = "rest.ParameterHandler.GetRates"
	logger := logging.WithTrace(c.Request.Context(), h.logger)
	ctx, span := h.tracer.Start(c.Request.Context(), op)
	defer span.End()

	product := c.Param("product")

	orgIDStr := c.GetHeader("X-Org-ID")
	if orgIDStr == "" {
		writeError(c, http.StatusBadRequest, "INVALID_CALLER_CONTEXT", "X-Org-ID header is required", false, gin.H{"field": "org_id"})
		return
	}
	orgID, err := uuid.Parse(orgIDStr)
	if err != nil {
		writeError(c, http.StatusBadRequest, "INVALID_CALLER_CONTEXT", "invalid org ID format", false, gin.H{"field": "org_id"})
		return
	}

	name := c.Query("name")
	if name == "" {
		name = "interest_rate"
	}

	param, err := h.svc.GetParameter(ctx, orgID, "rates", name, "global", nil, &product)
	if err != nil {
		span.RecordError(err)
		logger.Error().Err(err).Str("op", op).Msg("failed")
		writeError(c, http.StatusNotFound, "PARAMETER_NOT_FOUND", "rate parameter not found", false, nil)
		return
	}
	span.SetStatus(codes.Ok, "")
	writeSuccess(c, http.StatusOK, param)
}

func (h *ParameterHandler) GetFees(c *gin.Context) {
	const op = "rest.ParameterHandler.GetFees"
	logger := logging.WithTrace(c.Request.Context(), h.logger)
	ctx, span := h.tracer.Start(c.Request.Context(), op)
	defer span.End()

	product := c.Param("product")

	orgIDStr := c.GetHeader("X-Org-ID")
	if orgIDStr == "" {
		writeError(c, http.StatusBadRequest, "INVALID_CALLER_CONTEXT", "X-Org-ID header is required", false, gin.H{"field": "org_id"})
		return
	}
	orgID, err := uuid.Parse(orgIDStr)
	if err != nil {
		writeError(c, http.StatusBadRequest, "INVALID_CALLER_CONTEXT", "invalid org ID format", false, gin.H{"field": "org_id"})
		return
	}

	name := c.Query("name")
	if name == "" {
		name = "admin_fee"
	}

	param, err := h.svc.GetParameter(ctx, orgID, "fees", name, "global", nil, &product)
	if err != nil {
		span.RecordError(err)
		logger.Error().Err(err).Str("op", op).Msg("failed")
		writeError(c, http.StatusNotFound, "PARAMETER_NOT_FOUND", "fee parameter not found", false, nil)
		return
	}
	span.SetStatus(codes.Ok, "")
	writeSuccess(c, http.StatusOK, param)
}

func (h *ParameterHandler) GetRegulatory(c *gin.Context) {
	const op = "rest.ParameterHandler.GetRegulatory"
	logger := logging.WithTrace(c.Request.Context(), h.logger)
	ctx, span := h.tracer.Start(c.Request.Context(), op)
	defer span.End()

	regType := c.Param("type")

	orgIDStr := c.GetHeader("X-Org-ID")
	if orgIDStr == "" {
		writeError(c, http.StatusBadRequest, "INVALID_CALLER_CONTEXT", "X-Org-ID header is required", false, gin.H{"field": "org_id"})
		return
	}
	orgID, err := uuid.Parse(orgIDStr)
	if err != nil {
		writeError(c, http.StatusBadRequest, "INVALID_CALLER_CONTEXT", "invalid org ID format", false, gin.H{"field": "org_id"})
		return
	}

	param, err := h.svc.GetParameter(ctx, orgID, "regulatory", regType, "global", nil, nil)
	if err != nil {
		span.RecordError(err)
		logger.Error().Err(err).Str("op", op).Msg("failed")
		writeError(c, http.StatusNotFound, "PARAMETER_NOT_FOUND", "regulatory parameter not found", false, nil)
		return
	}
	span.SetStatus(codes.Ok, "")
	writeSuccess(c, http.StatusOK, param)
}

func (h *ParameterHandler) CheckRegulatory(c *gin.Context) {
	const op = "rest.ParameterHandler.CheckRegulatory"
	logger := logging.WithTrace(c.Request.Context(), h.logger)
	ctx, span := h.tracer.Start(c.Request.Context(), op)
	defer span.End()

	regType := c.Param("type")

	orgIDStr := c.GetHeader("X-Org-ID")
	if orgIDStr == "" {
		writeError(c, http.StatusBadRequest, "INVALID_CALLER_CONTEXT", "X-Org-ID header is required", false, gin.H{"field": "org_id"})
		return
	}
	orgID, err := uuid.Parse(orgIDStr)
	if err != nil {
		writeError(c, http.StatusBadRequest, "INVALID_CALLER_CONTEXT", "invalid org ID format", false, gin.H{"field": "org_id"})
		return
	}

	var req struct {
		Amount float64 `json:"amount"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, http.StatusBadRequest, "INVALID_REQUEST", err.Error(), false, nil)
		return
	}

	param, err := h.svc.GetParameter(ctx, orgID, "regulatory", regType, "global", nil, nil)
	if err != nil {
		span.RecordError(err)
		logger.Error().Err(err).Str("op", op).Msg("failed")
		writeError(c, http.StatusNotFound, "PARAMETER_NOT_FOUND", "regulatory threshold not found", false, nil)
		return
	}

	var thresholdData struct {
		Threshold float64 `json:"threshold"`
	}
	if err := json.Unmarshal(param.Value, &thresholdData); err != nil {
		span.RecordError(err)
		logger.Error().Err(err).Str("op", op).Msg("failed to parse threshold")
		writeError(c, http.StatusUnprocessableEntity, "INVALID_PARAMETER_SHAPE", "failed to parse threshold parameter value", false, nil)
		return
	}

	isExceeded := req.Amount > thresholdData.Threshold
	span.SetStatus(codes.Ok, "")
	writeSuccess(c, http.StatusOK, gin.H{
		"is_exceeded": isExceeded,
		"amount":      req.Amount,
		"threshold":   thresholdData.Threshold,
	})
}

func (h *ParameterHandler) CheckAuthorizationLimit(c *gin.Context) {
	const op = "rest.ParameterHandler.CheckAuthorizationLimit"
	logger := logging.WithTrace(c.Request.Context(), h.logger)
	ctx, span := h.tracer.Start(c.Request.Context(), op)
	defer span.End()

	orgIDStr := c.GetHeader("X-Org-ID")
	if orgIDStr == "" {
		writeError(c, http.StatusBadRequest, "INVALID_CALLER_CONTEXT", "X-Org-ID header is required", false, gin.H{"field": "org_id"})
		return
	}
	orgID, err := uuid.Parse(orgIDStr)
	if err != nil {
		writeError(c, http.StatusBadRequest, "INVALID_CALLER_CONTEXT", "invalid org ID format", false, gin.H{"field": "org_id"})
		return
	}

	var req struct {
		RoleID string  `json:"role_id"`
		Amount float64 `json:"amount"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, http.StatusBadRequest, "INVALID_REQUEST", err.Error(), false, nil)
		return
	}
	if strings.TrimSpace(req.RoleID) == "" {
		writeError(c, http.StatusBadRequest, "INVALID_CALLER_CONTEXT", "role_id is required", false, gin.H{"field": "role_id"})
		return
	}

	param, err := h.svc.GetParameter(ctx, orgID, "authorization_limit", "max_amount", "role", &req.RoleID, nil)
	if err != nil {
		param, err = h.svc.GetParameter(ctx, orgID, "authorization_limit", "max_amount", "global", nil, nil)
		if err != nil {
			span.RecordError(err)
			logger.Error().Err(err).Str("op", op).Msg("failed")
			writeError(c, http.StatusNotFound, "PARAMETER_NOT_FOUND", "authorization limit not found", false, nil)
			return
		}
	}

	var limitData struct {
		Limit float64 `json:"limit"`
	}
	if err := json.Unmarshal(param.Value, &limitData); err != nil {
		span.RecordError(err)
		logger.Error().Err(err).Str("op", op).Msg("failed to parse limit")
		writeError(c, http.StatusUnprocessableEntity, "INVALID_PARAMETER_SHAPE", "failed to parse limit parameter value", false, nil)
		return
	}

	isAuthorized := req.Amount <= limitData.Limit
	span.SetStatus(codes.Ok, "")
	writeSuccess(c, http.StatusOK, gin.H{
		"is_authorized":  isAuthorized,
		"amount":         req.Amount,
		"limit":          limitData.Limit,
		"parameter_used": param.ID,
	})
}
