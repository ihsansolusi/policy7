package api

import (
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
