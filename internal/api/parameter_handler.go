package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/ihsansolusi/policy7/internal/service"
)

type ParameterHandler struct {
	svc *service.ParameterService
}

func NewParameterHandler(svc *service.ParameterService) *ParameterHandler {
	return &ParameterHandler{svc: svc}
}

// GetParameter handles the GET /v1/params/:category/:name endpoint
func (h *ParameterHandler) GetParameter(c *gin.Context) {
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

	param, err := h.svc.GetParameter(c.Request.Context(), orgID, category, name, appliesTo, appliesToID, product)
	if err != nil {
		writeError(c, http.StatusNotFound, "PARAMETER_NOT_FOUND", "parameter not found", false, nil)
		return
	}

	writeSuccess(c, http.StatusOK, param)
}

func (h *ParameterHandler) GetEffectiveParameter(c *gin.Context) {
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

	param, err := h.svc.GetEffectiveParameter(c.Request.Context(), orgID, category, name, product, resCtx)
	if err != nil {
		writeError(c, http.StatusNotFound, "PARAMETER_NOT_FOUND", "effective parameter not found", false, nil)
		return
	}

	writeSuccess(c, http.StatusOK, param)
}

func (h *ParameterHandler) ValidateTransactionLimit(c *gin.Context) {
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

	param, err := h.svc.GetEffectiveParameter(c.Request.Context(), orgID, "transaction_limit", req.Name, req.Product, resCtx)
	if err != nil {
		writeError(c, http.StatusNotFound, "PARAMETER_NOT_FOUND", "limit parameter not found", false, nil)
		return
	}

	var limitData struct {
		Limit float64 `json:"limit"`
	}
	if err := json.Unmarshal(param.Value, &limitData); err != nil {
		writeError(c, http.StatusUnprocessableEntity, "INVALID_PARAMETER_SHAPE", "failed to parse limit parameter value", false, nil)
		return
	}

	isValid := req.Amount <= limitData.Limit
	writeSuccess(c, http.StatusOK, gin.H{
		"is_valid":       isValid,
		"amount":         req.Amount,
		"limit":          limitData.Limit,
		"parameter_used": param.ID,
	})
}

func (h *ParameterHandler) GetApprovalThresholds(c *gin.Context) {
	h.handleCategoryRequest(c, "approval_threshold")
}

func (h *ParameterHandler) GetOperationalHours(c *gin.Context) {
	h.handleCategoryRequest(c, "operational_hours")
}

func (h *ParameterHandler) GetProductAccess(c *gin.Context) {
	h.handleCategoryRequest(c, "product_access")
}

func (h *ParameterHandler) handleCategoryRequest(c *gin.Context, category string) {
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

	param, err := h.svc.GetEffectiveParameter(c.Request.Context(), orgID, category, name, product, resCtx)
	if err != nil {
		writeError(c, http.StatusNotFound, "PARAMETER_NOT_FOUND", "effective parameter not found", false, nil)
		return
	}

	writeSuccess(c, http.StatusOK, param)
}

func (h *ParameterHandler) GetRates(c *gin.Context) {
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

	param, err := h.svc.GetParameter(c.Request.Context(), orgID, "rates", name, "global", nil, &product)
	if err != nil {
		writeError(c, http.StatusNotFound, "PARAMETER_NOT_FOUND", "rate parameter not found", false, nil)
		return
	}
	writeSuccess(c, http.StatusOK, param)
}

func (h *ParameterHandler) GetFees(c *gin.Context) {
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

	param, err := h.svc.GetParameter(c.Request.Context(), orgID, "fees", name, "global", nil, &product)
	if err != nil {
		writeError(c, http.StatusNotFound, "PARAMETER_NOT_FOUND", "fee parameter not found", false, nil)
		return
	}
	writeSuccess(c, http.StatusOK, param)
}

func (h *ParameterHandler) GetRegulatory(c *gin.Context) {
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

	param, err := h.svc.GetParameter(c.Request.Context(), orgID, "regulatory", regType, "global", nil, nil)
	if err != nil {
		writeError(c, http.StatusNotFound, "PARAMETER_NOT_FOUND", "regulatory parameter not found", false, nil)
		return
	}
	writeSuccess(c, http.StatusOK, param)
}

func (h *ParameterHandler) CheckRegulatory(c *gin.Context) {
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

	param, err := h.svc.GetParameter(c.Request.Context(), orgID, "regulatory", regType, "global", nil, nil)
	if err != nil {
		writeError(c, http.StatusNotFound, "PARAMETER_NOT_FOUND", "regulatory threshold not found", false, nil)
		return
	}

	var thresholdData struct {
		Threshold float64 `json:"threshold"`
	}
	if err := json.Unmarshal(param.Value, &thresholdData); err != nil {
		writeError(c, http.StatusUnprocessableEntity, "INVALID_PARAMETER_SHAPE", "failed to parse threshold parameter value", false, nil)
		return
	}

	isExceeded := req.Amount > thresholdData.Threshold
	writeSuccess(c, http.StatusOK, gin.H{
		"is_exceeded": isExceeded,
		"amount":      req.Amount,
		"threshold":   thresholdData.Threshold,
	})
}

func (h *ParameterHandler) CheckAuthorizationLimit(c *gin.Context) {
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

	param, err := h.svc.GetParameter(c.Request.Context(), orgID, "authorization_limit", "max_amount", "role", &req.RoleID, nil)
	if err != nil {
		param, err = h.svc.GetParameter(c.Request.Context(), orgID, "authorization_limit", "max_amount", "global", nil, nil)
		if err != nil {
			writeError(c, http.StatusNotFound, "PARAMETER_NOT_FOUND", "authorization limit not found", false, nil)
			return
		}
	}

	var limitData struct {
		Limit float64 `json:"limit"`
	}
	if err := json.Unmarshal(param.Value, &limitData); err != nil {
		writeError(c, http.StatusUnprocessableEntity, "INVALID_PARAMETER_SHAPE", "failed to parse limit parameter value", false, nil)
		return
	}

	isAuthorized := req.Amount <= limitData.Limit
	writeSuccess(c, http.StatusOK, gin.H{
		"is_authorized":  isAuthorized,
		"amount":         req.Amount,
		"limit":          limitData.Limit,
		"parameter_used": param.ID,
	})
}
