package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/ihsansolusi/lib7-service-go/logging"
	"github.com/ihsansolusi/policy7/internal/store"
	"github.com/jackc/pgx/v5/pgtype"
	"go.opentelemetry.io/otel/codes"
)

// wfCategoryEnvelope is the workflow7 callback envelope for category mutations:
// the instance id, the master identity (code + POLICY_CATEGORY type), and the
// opaque payload captured at submit time. Data carries the category fields and
// is unmarshalled into categoryWriteRequest (same shape as the direct admin
// POST/PUT body so the BFF can reuse it).
type wfCategoryEnvelope struct {
	WfInstanceID string          `json:"wf_instance_id"`
	MasterID     string          `json:"master_id"`
	MasterType   string          `json:"master_type"`
	Data         json.RawMessage `json:"data"`
}

// WfCreate handles POST /admin/v1/categories/wf-create — workflow7 callback that
// persists a parameter category approved through the approval flow. Reuses the
// same service logic as the direct admin API (#568): insert the category row
// with its value_schema / default_value document.
func (h *CategoryHandler) WfCreate(c *gin.Context) {
	const op = "rest.CategoryHandler.WfCreate"
	logger := logging.WithTrace(c.Request.Context(), h.logger)
	ctx, span := h.tracer.Start(c.Request.Context(), op)
	defer span.End()

	orgID, ok := getActorOrgID(c)
	if !ok {
		return
	}
	userID, ok := getActorUserID(c)
	if !ok {
		return
	}

	var env wfCategoryEnvelope
	if err := c.ShouldBindJSON(&env); err != nil {
		span.RecordError(err)
		writeError(c, http.StatusBadRequest, "INVALID_REQUEST", err.Error(), false, nil)
		return
	}
	var req categoryWriteRequest
	if err := json.Unmarshal(env.Data, &req); err != nil {
		span.RecordError(err)
		writeError(c, http.StatusBadRequest, "INVALID_REQUEST", err.Error(), false, gin.H{"field": "data"})
		return
	}
	if strings.TrimSpace(req.Code) == "" {
		writeError(c, http.StatusUnprocessableEntity, "INVALID_CALLER_CONTEXT", "code is required", false, gin.H{"field": "code"})
		return
	}
	if strings.TrimSpace(req.Name) == "" {
		writeError(c, http.StatusUnprocessableEntity, "INVALID_CALLER_CONTEXT", "name is required", false, gin.H{"field": "name"})
		return
	}

	var pgOrgID, pgUserID pgtype.UUID
	_ = pgOrgID.Scan(orgID.String())
	_ = pgUserID.Scan(userID.String())

	isActive := true
	if req.IsActive != nil {
		isActive = *req.IsActive
	}

	cat, err := h.svc.CreateCategory(ctx, store.CreateParameterCategoryParams{
		OrgID:        pgOrgID,
		Code:         req.Code,
		Name:         req.Name,
		Description:  optText(req.Description),
		ValueSchema:  req.ValueSchema,
		DefaultValue: req.DefaultValue,
		DisplayOrder: optInt4(req.DisplayOrder),
		Icon:         optText(req.Icon),
		Color:        optText(req.Color),
		IsActive:     isActive,
		CreatedBy:    pgUserID,
	})
	if err != nil {
		if writeSchemaError(c, err) {
			return
		}
		span.RecordError(err)
		logger.Error().Err(err).Str("op", op).Str("wf_instance_id", env.WfInstanceID).Msg("wf create category failed")
		if isDuplicateKey(err) {
			writeError(c, http.StatusConflict, "POLICY_CONFLICT", "category code already exists", false, gin.H{"field": "code"})
			return
		}
		writeError(c, http.StatusInternalServerError, "POLICY_BACKEND_UNAVAILABLE", err.Error(), true, nil)
		return
	}

	forwardPolicyAudit(c, h.audit7, policyAuditEntry{
		Action: "create_category", ResourceType: "parameter_category",
		ResourceID: cat.Code, ResourceName: req.Name,
		OrgID: orgID.String(), UserID: userID.String(), WfInstanceID: env.WfInstanceID,
		After: json.RawMessage(env.Data),
	})

	span.SetStatus(codes.Ok, "")
	c.JSON(http.StatusOK, WfCallbackResponse{Success: true, ID: cat.Code})
}

// WfUpdate handles PUT /admin/v1/categories/:code/wf-update — workflow7 callback
// that applies an approved category change. Loads the current row and patches
// only the supplied fields (same merge semantics as the direct admin PUT).
func (h *CategoryHandler) WfUpdate(c *gin.Context) {
	const op = "rest.CategoryHandler.WfUpdate"
	logger := logging.WithTrace(c.Request.Context(), h.logger)
	ctx, span := h.tracer.Start(c.Request.Context(), op)
	defer span.End()

	orgID, ok := getActorOrgID(c)
	if !ok {
		return
	}
	userID, ok := getActorUserID(c)
	if !ok {
		return
	}

	code := c.Param("code")

	var env wfCategoryEnvelope
	if err := c.ShouldBindJSON(&env); err != nil {
		span.RecordError(err)
		writeError(c, http.StatusBadRequest, "INVALID_REQUEST", err.Error(), false, nil)
		return
	}
	var req categoryWriteRequest
	if err := json.Unmarshal(env.Data, &req); err != nil {
		span.RecordError(err)
		writeError(c, http.StatusBadRequest, "INVALID_REQUEST", err.Error(), false, gin.H{"field": "data"})
		return
	}

	// Load current state so the callback can patch only the supplied fields.
	current, err := h.svc.GetCategory(ctx, orgID, code)
	if err != nil {
		writeError(c, http.StatusNotFound, "CATEGORY_NOT_FOUND", err.Error(), false, gin.H{"code": code})
		return
	}

	merged := mergeCategory(*current, req)

	var pgOrgID, pgUserID pgtype.UUID
	_ = pgOrgID.Scan(orgID.String())
	_ = pgUserID.Scan(userID.String())

	cat, err := h.svc.UpdateCategory(ctx, store.UpdateParameterCategoryParams{
		OrgID:        pgOrgID,
		Code:         code,
		Name:         merged.Name,
		Description:  merged.Description,
		ValueSchema:  merged.ValueSchema,
		DefaultValue: merged.DefaultValue,
		DisplayOrder: merged.DisplayOrder,
		Icon:         merged.Icon,
		Color:        merged.Color,
		IsActive:     merged.IsActive,
		UpdatedBy:    pgUserID,
	})
	if err != nil {
		if writeSchemaError(c, err) {
			return
		}
		span.RecordError(err)
		logger.Error().Err(err).Str("op", op).Str("code", code).Str("wf_instance_id", env.WfInstanceID).Msg("wf update category failed")
		writeError(c, http.StatusInternalServerError, "POLICY_BACKEND_UNAVAILABLE", err.Error(), true, nil)
		return
	}

	forwardPolicyAudit(c, h.audit7, policyAuditEntry{
		Action: "update_category", ResourceType: "parameter_category",
		ResourceID: cat.Code, ResourceName: merged.Name,
		OrgID: orgID.String(), UserID: userID.String(), WfInstanceID: env.WfInstanceID,
		Before: current, After: json.RawMessage(env.Data),
	})

	span.SetStatus(codes.Ok, "")
	c.JSON(http.StatusOK, WfCallbackResponse{Success: true, ID: cat.Code})
}

// WfDelete handles POST /admin/v1/categories/:code/wf-delete — workflow7 callback
// that soft-deletes (is_active = FALSE) an approved category deletion.
func (h *CategoryHandler) WfDelete(c *gin.Context) {
	const op = "rest.CategoryHandler.WfDelete"
	logger := logging.WithTrace(c.Request.Context(), h.logger)
	ctx, span := h.tracer.Start(c.Request.Context(), op)
	defer span.End()

	orgID, ok := getActorOrgID(c)
	if !ok {
		return
	}
	userID, ok := getActorUserID(c)
	if !ok {
		return
	}

	code := c.Param("code")
	// Bind the envelope leniently to recover the wf instance id for the audit
	// correlation; the delete itself only needs org/user/code.
	var env wfCategoryEnvelope
	_ = c.ShouldBindJSON(&env)

	// Capture prior state for the audit before-snapshot (best-effort).
	before, _ := h.svc.GetCategory(ctx, orgID, code)
	if err := h.svc.DeleteCategory(ctx, orgID, userID, code); err != nil {
		span.RecordError(err)
		logger.Error().Err(err).Str("op", op).Str("code", code).Msg("wf delete category failed")
		writeError(c, http.StatusInternalServerError, "POLICY_BACKEND_UNAVAILABLE", err.Error(), true, nil)
		return
	}

	resourceName := code
	if before != nil {
		resourceName = before.Name
	}
	forwardPolicyAudit(c, h.audit7, policyAuditEntry{
		Action: "delete_category", ResourceType: "parameter_category",
		ResourceID: code, ResourceName: resourceName,
		OrgID: orgID.String(), UserID: userID.String(), WfInstanceID: env.WfInstanceID,
		Before: before,
	})

	span.SetStatus(codes.Ok, "")
	c.JSON(http.StatusOK, WfCallbackResponse{Success: true, ID: code})
}
