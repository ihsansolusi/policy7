package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/ihsansolusi/policy7/internal/service"
	"github.com/ihsansolusi/policy7/internal/store"
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

	params, err := h.svc.List(c.Request.Context(), orgID, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": params})
}

func (h *AdminHandler) GetByID(c *gin.Context) {
	orgID, ok := getOrgID(c)
	if !ok {
		return
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid parameter ID format"})
		return
	}

	param, err := h.svc.GetByID(c.Request.Context(), id, orgID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, param)
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
		Category       string          `json:"category"`
		Name           string          `json:"name"`
		AppliesTo      string          `json:"applies_to"`
		AppliesToID    *string         `json:"applies_to_id"`
		Product        *string         `json:"product"`
		Value          json.RawMessage `json:"value"`
		ValueType      string          `json:"value_type"`
		Unit           *string         `json:"unit"`
		Scope          *string         `json:"scope"`
		ChangeReason   string          `json:"change_reason"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
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
		OrgID:         pgOrgID,
		Category:      req.Category,
		Name:          req.Name,
		AppliesTo:     req.AppliesTo,
		AppliesToID:   appliesToID,
		Product:       product,
		Value:         req.Value,
		ValueType:     req.ValueType,
		Unit:          unit,
		Scope:         scope,
		IsActive:      true,
		Version:       1,
		CreatedBy:     pgUserID,
	}, req.ChangeReason)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, param)
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
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid parameter ID"})
		return
	}

	var req struct {
		Value        json.RawMessage `json:"value"`
		ChangeReason string          `json:"change_reason"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	param, err := h.svc.Update(c.Request.Context(), id, orgID, userID, req.Value, req.ChangeReason)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, param)
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
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid parameter ID"})
		return
	}

	var req struct {
		ChangeReason string `json:"change_reason"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err = h.svc.Delete(c.Request.Context(), id, orgID, userID, req.ChangeReason)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusNoContent, nil)
}

func (h *AdminHandler) GetHistory(c *gin.Context) {
	orgID, ok := getOrgID(c)
	if !ok {
		return
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid parameter ID format"})
		return
	}

	histories, err := h.svc.GetHistory(c.Request.Context(), id, orgID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": histories})
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
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":       "bulk import completed",
		"success_count": count,
		"total_count":   len(req),
	})
}

func getOrgID(c *gin.Context) (uuid.UUID, bool) {
	orgIDStr := c.GetHeader("X-Org-ID")
	if orgIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "X-Org-ID header is required"})
		return uuid.Nil, false
	}
	orgID, err := uuid.Parse(orgIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid org ID format"})
		return uuid.Nil, false
	}
	return orgID, true
}

func getUserID(c *gin.Context) (uuid.UUID, bool) {
	userIDStr := c.GetHeader("X-User-ID")
	if userIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "X-User-ID header is required"})
		return uuid.Nil, false
	}
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user ID format"})
		return uuid.Nil, false
	}
	return userID, true
}
