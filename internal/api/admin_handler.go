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
	"github.com/ihsansolusi/policy7/internal/service"
	"github.com/ihsansolusi/policy7/internal/store"
	"github.com/jackc/pgx/v5/pgtype"
)

type AdminHandler struct {
	svc *service.AdminParameterService
}

func NewAdminHandler(svc *service.AdminParameterService) *AdminHandler {
	return &AdminHandler{svc: svc}
}

func (h *AdminHandler) List(c *gin.Context) {
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
		params, err = h.svc.List(c.Request.Context(), orgID, limit, offset)
	} else {
		params, err = h.svc.ListFiltered(c.Request.Context(), orgID, category, product, appliesTo, limit, offset)
	}
	if err != nil {
		writeError(c, http.StatusInternalServerError, "POLICY_BACKEND_UNAVAILABLE", "failed to list parameters", true, nil)
		return
	}

	writeSuccess(c, http.StatusOK, params)
}

func (h *AdminHandler) GetByID(c *gin.Context) {
	orgID, ok := getOrgID(c)
	if !ok {
		return
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		writeError(c, http.StatusBadRequest, "INVALID_CALLER_CONTEXT", "invalid parameter ID format", false, gin.H{"field": "id"})
		return
	}

	param, err := h.svc.GetByID(c.Request.Context(), id, orgID)
	if err != nil {
		writeError(c, http.StatusNotFound, "PARAMETER_NOT_FOUND", err.Error(), false, nil)
		return
	}

	writeSuccess(c, http.StatusOK, param)
}

func (h *AdminHandler) Create(c *gin.Context) {
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

	param, err := h.svc.Create(c.Request.Context(), store.CreateParameterParams{
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
		writeError(c, http.StatusInternalServerError, "POLICY_BACKEND_UNAVAILABLE", err.Error(), true, nil)
		return
	}

	writeSuccess(c, http.StatusCreated, gin.H{
		"parameter": param,
		"audit": gin.H{
			"history_change_type": "create",
			"event_type":          "policy7.params.created",
		},
	})
}

func (h *AdminHandler) Update(c *gin.Context) {
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

	param, err := h.svc.Update(c.Request.Context(), id, orgID, userID, req.Value, req.ChangeReason)
	if err != nil {
		if strings.Contains(err.Error(), "inactive parameter") {
			writeError(c, http.StatusConflict, "POLICY_CONFLICT", err.Error(), false, nil)
			return
		}
		writeError(c, http.StatusInternalServerError, "POLICY_BACKEND_UNAVAILABLE", err.Error(), true, nil)
		return
	}

	writeSuccess(c, http.StatusOK, gin.H{
		"parameter": param,
		"audit": gin.H{
			"history_change_type": "update",
			"event_type":          "policy7.params.updated",
		},
	})
}

func (h *AdminHandler) Delete(c *gin.Context) {
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

	err = h.svc.Delete(c.Request.Context(), id, orgID, userID, req.ChangeReason)
	if err != nil {
		writeError(c, http.StatusInternalServerError, "POLICY_BACKEND_UNAVAILABLE", err.Error(), true, nil)
		return
	}

	writeNoContent(c)
}

func (h *AdminHandler) GetHistory(c *gin.Context) {
	orgID, ok := getOrgID(c)
	if !ok {
		return
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		writeError(c, http.StatusBadRequest, "INVALID_CALLER_CONTEXT", "invalid parameter ID format", false, gin.H{"field": "id"})
		return
	}

	histories, err := h.svc.GetHistory(c.Request.Context(), id, orgID)
	if err != nil {
		writeError(c, http.StatusInternalServerError, "POLICY_BACKEND_UNAVAILABLE", err.Error(), true, nil)
		return
	}

	writeSuccess(c, http.StatusOK, histories)
}

func (h *AdminHandler) BulkImport(c *gin.Context) {
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

	count, err := h.svc.BulkImport(c.Request.Context(), orgID, userID, paramsToCreate)
	if err != nil {
		writeError(c, http.StatusInternalServerError, "POLICY_BACKEND_UNAVAILABLE", err.Error(), true, nil)
		return
	}

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
