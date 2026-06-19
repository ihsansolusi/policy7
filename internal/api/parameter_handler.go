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

	// transaction_limit params store both caps in the value:
	// {"transaction_limit": <n>, "authorization_limit": <n>, "currency": ..., "scope": ...}
	var limitData struct {
		TransactionLimit   float64 `json:"transaction_limit"`
		AuthorizationLimit float64 `json:"authorization_limit"`
	}
	if err := json.Unmarshal(param.Value, &limitData); err != nil {
		span.RecordError(err)
		logger.Error().Err(err).Str("op", op).Msg("failed to parse limit")
		writeError(c, http.StatusUnprocessableEntity, "INVALID_PARAMETER_SHAPE", "failed to parse limit parameter value", false, nil)
		return
	}

	txnMax := limitData.TransactionLimit
	authMax := limitData.AuthorizationLimit

	// Two-limit decision per docs/specs/02-api-detail.md §3.6:
	//   amount > transaction_limit            → REJECTED
	//   amount <= authorization_limit         → AUTO_AUTHORIZED
	//   otherwise (auth < amount <= txn)      → REQUIRES_AUTHORIZATION
	var decision, reason, nextStep, message string
	var canInput, autoAuthorized bool
	switch {
	case req.Amount > txnMax:
		decision = "REJECTED"
		reason = "Amount exceeds transaction limit"
	case req.Amount <= authMax:
		decision = "AUTO_AUTHORIZED"
		canInput = true
		autoAuthorized = true
		message = "Transaction automatically authorized"
	default:
		decision = "REQUIRES_AUTHORIZATION"
		canInput = true
		reason = "Amount exceeds authorization limit"
		nextStep = "Request supervisor authorization"
	}

	txnLimit := gin.H{"max": txnMax}
	if canInput {
		txnLimit["remaining"] = txnMax - req.Amount
	}

	resp := gin.H{
		"amount":              req.Amount,
		"decision":            decision,
		"can_input":           canInput,
		"auto_authorized":     autoAuthorized,
		"transaction_limit":   txnLimit,
		"authorization_limit": gin.H{"max": authMax},
		"parameter_used":      param.ID,
		// Backward-compatible fields for the legacy single-limit callers.
		"is_valid": canInput,
		"limit":    txnMax,
	}
	if reason != "" {
		resp["reason"] = reason
	}
	if nextStep != "" {
		resp["next_step"] = nextStep
	}
	if message != "" {
		resp["message"] = message
	}

	span.SetStatus(codes.Ok, "")
	writeSuccess(c, http.StatusOK, resp)
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
		RoleID       string  `json:"role_id"`
		ApproverRole string  `json:"approver_role"`
		Name         string  `json:"name"`
		Amount       float64 `json:"amount"`
		Product      *string `json:"product"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, http.StatusBadRequest, "INVALID_REQUEST", err.Error(), false, nil)
		return
	}

	// Accept either role_id or approver_role (spec §3.7 uses approver_role).
	role := strings.TrimSpace(req.RoleID)
	if role == "" {
		role = strings.TrimSpace(req.ApproverRole)
	}
	if role == "" {
		writeError(c, http.StatusBadRequest, "INVALID_CALLER_CONTEXT", "role_id or approver_role is required", false, gin.H{"field": "role_id"})
		return
	}

	// The authorization_limit param name is configurable per request; default
	// matches the seeded data model (name "auto_authorize_max", value key "limit_amount").
	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = "auto_authorize_max"
	}

	param, err := h.svc.GetParameter(ctx, orgID, "authorization_limit", name, "role", &role, req.Product)
	if err != nil {
		param, err = h.svc.GetParameter(ctx, orgID, "authorization_limit", name, "global", nil, req.Product)
		if err != nil {
			span.RecordError(err)
			logger.Error().Err(err).Str("op", op).Msg("failed")
			writeError(c, http.StatusNotFound, "PARAMETER_NOT_FOUND", "authorization limit not found", false, nil)
			return
		}
	}

	// authorization_limit params store the cap under "limit_amount".
	var limitData struct {
		LimitAmount float64 `json:"limit_amount"`
	}
	if err := json.Unmarshal(param.Value, &limitData); err != nil {
		span.RecordError(err)
		logger.Error().Err(err).Str("op", op).Msg("failed to parse limit")
		writeError(c, http.StatusUnprocessableEntity, "INVALID_PARAMETER_SHAPE", "failed to parse limit parameter value", false, nil)
		return
	}

	limit := limitData.LimitAmount
	allowed := req.Amount <= limit

	resp := gin.H{
		"allowed":        allowed,
		"approver_limit": limit,
		"requested":      req.Amount,
		"parameter_used": param.ID,
		// Backward-compatible fields for the legacy callers.
		"is_authorized": allowed,
		"amount":        req.Amount,
		"limit":         limit,
	}
	if allowed {
		resp["remaining"] = limit - req.Amount
		resp["message"] = "Approver can authorize this amount"
	} else {
		resp["exceeded_by"] = req.Amount - limit
		resp["escalation_required"] = true
		resp["message"] = "Amount exceeds approver authorization limit"
	}

	span.SetStatus(codes.Ok, "")
	writeSuccess(c, http.StatusOK, resp)
}
