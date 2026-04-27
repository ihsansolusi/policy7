package api

import (
	"encoding/json"
	"net/http"

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
		c.JSON(http.StatusBadRequest, gin.H{"error": "X-Org-ID header is required"})
		return
	}

	orgID, err := uuid.Parse(orgIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid org ID format"})
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
		// Log error here in a real app
		c.JSON(http.StatusNotFound, gin.H{"error": "parameter not found"})
		return
	}

	c.JSON(http.StatusOK, param)
}

func (h *ParameterHandler) GetEffectiveParameter(c *gin.Context) {
	category := c.Param("category")
	name := c.Param("name")

	orgIDStr := c.GetHeader("X-Org-ID")
	if orgIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "X-Org-ID header is required"})
		return
	}

	orgID, err := uuid.Parse(orgIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid org ID format"})
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
		c.JSON(http.StatusNotFound, gin.H{"error": "effective parameter not found"})
		return
	}

	c.JSON(http.StatusOK, param)
}

func (h *ParameterHandler) ValidateTransactionLimit(c *gin.Context) {
	orgIDStr := c.GetHeader("X-Org-ID")
	if orgIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "X-Org-ID header is required"})
		return
	}
	orgID, err := uuid.Parse(orgIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid org ID"})
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
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
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
		c.JSON(http.StatusNotFound, gin.H{"error": "limit parameter not found"})
		return
	}

	var limitData struct {
		Limit float64 `json:"limit"`
	}
	if err := json.Unmarshal(param.Value, &limitData); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to parse limit parameter value"})
		return
	}

	isValid := req.Amount <= limitData.Limit
	c.JSON(http.StatusOK, gin.H{
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
		c.JSON(http.StatusBadRequest, gin.H{"error": "name query parameter is required"})
		return
	}

	orgIDStr := c.GetHeader("X-Org-ID")
	if orgIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "X-Org-ID header is required"})
		return
	}
	orgID, err := uuid.Parse(orgIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid org ID format"})
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
		c.JSON(http.StatusNotFound, gin.H{"error": "effective parameter not found"})
		return
	}

	c.JSON(http.StatusOK, param)
}

func (h *ParameterHandler) GetRates(c *gin.Context) {
	product := c.Param("product")

	orgIDStr := c.GetHeader("X-Org-ID")
	if orgIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "X-Org-ID header is required"})
		return
	}
	orgID, err := uuid.Parse(orgIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid org ID format"})
		return
	}

	name := c.Query("name")
	if name == "" {
		name = "interest_rate"
	}

	param, err := h.svc.GetParameter(c.Request.Context(), orgID, "rates", name, "global", nil, &product)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "rate parameter not found"})
		return
	}
	c.JSON(http.StatusOK, param)
}

func (h *ParameterHandler) GetFees(c *gin.Context) {
	product := c.Param("product")

	orgIDStr := c.GetHeader("X-Org-ID")
	if orgIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "X-Org-ID header is required"})
		return
	}
	orgID, err := uuid.Parse(orgIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid org ID format"})
		return
	}

	name := c.Query("name")
	if name == "" {
		name = "admin_fee"
	}

	param, err := h.svc.GetParameter(c.Request.Context(), orgID, "fees", name, "global", nil, &product)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "fee parameter not found"})
		return
	}
	c.JSON(http.StatusOK, param)
}

func (h *ParameterHandler) GetRegulatory(c *gin.Context) {
	regType := c.Param("type")

	orgIDStr := c.GetHeader("X-Org-ID")
	if orgIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "X-Org-ID header is required"})
		return
	}
	orgID, err := uuid.Parse(orgIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid org ID format"})
		return
	}

	param, err := h.svc.GetParameter(c.Request.Context(), orgID, "regulatory", regType, "global", nil, nil)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "regulatory parameter not found"})
		return
	}
	c.JSON(http.StatusOK, param)
}

func (h *ParameterHandler) CheckRegulatory(c *gin.Context) {
	regType := c.Param("type")

	orgIDStr := c.GetHeader("X-Org-ID")
	if orgIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "X-Org-ID header is required"})
		return
	}
	orgID, err := uuid.Parse(orgIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid org ID format"})
		return
	}

	var req struct {
		Amount float64 `json:"amount"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	param, err := h.svc.GetParameter(c.Request.Context(), orgID, "regulatory", regType, "global", nil, nil)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "regulatory threshold not found"})
		return
	}

	var thresholdData struct {
		Threshold float64 `json:"threshold"`
	}
	if err := json.Unmarshal(param.Value, &thresholdData); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to parse threshold parameter value"})
		return
	}

	isExceeded := req.Amount > thresholdData.Threshold
	c.JSON(http.StatusOK, gin.H{
		"is_exceeded": isExceeded,
		"amount":      req.Amount,
		"threshold":   thresholdData.Threshold,
	})
}

func (h *ParameterHandler) CheckAuthorizationLimit(c *gin.Context) {
	orgIDStr := c.GetHeader("X-Org-ID")
	if orgIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "X-Org-ID header is required"})
		return
	}
	orgID, err := uuid.Parse(orgIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid org ID format"})
		return
	}

	var req struct {
		RoleID string  `json:"role_id"`
		Amount float64 `json:"amount"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	param, err := h.svc.GetParameter(c.Request.Context(), orgID, "authorization_limit", "max_amount", "role", &req.RoleID, nil)
	if err != nil {
		param, err = h.svc.GetParameter(c.Request.Context(), orgID, "authorization_limit", "max_amount", "global", nil, nil)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "authorization limit not found"})
			return
		}
	}

	var limitData struct {
		Limit float64 `json:"limit"`
	}
	if err := json.Unmarshal(param.Value, &limitData); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to parse limit parameter value"})
		return
	}

	isAuthorized := req.Amount <= limitData.Limit
	c.JSON(http.StatusOK, gin.H{
		"is_authorized":  isAuthorized,
		"amount":         req.Amount,
		"limit":          limitData.Limit,
		"parameter_used": param.ID,
	})
}
