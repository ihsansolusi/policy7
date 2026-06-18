package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/ihsansolusi/lib7-service-go/logging"
	"github.com/ihsansolusi/policy7/internal/store"
	"github.com/jackc/pgx/v5/pgtype"
	"go.opentelemetry.io/otel/codes"
)

// WfCallbackResponse is the standard reply workflow7 expects from a callback.
// ReachedMaxRetry signals workflow7 to stop retrying when true.
type WfCallbackResponse struct {
	Success         bool   `json:"success"`
	ID              string `json:"id,omitempty"`
	ReachedMaxRetry bool   `json:"reached_max_retry"`
	Message         string `json:"message,omitempty"`
}

// wfEnvelope is the generic workflow7 callback envelope: an instance id plus the
// opaque payload captured at submit time. Data is unmarshalled into the
// per-operation shape below.
type wfEnvelope struct {
	WfInstanceID string          `json:"wf_instance_id"`
	Data         json.RawMessage `json:"data"`
}

// wfCreateData is the create payload carried inside the envelope. Mirrors the
// direct admin POST /admin/v1/params body so the BFF can reuse the same shape.
type wfCreateData struct {
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

// wfUpdateData is the update payload: a new value plus the change reason.
type wfUpdateData struct {
	Value        json.RawMessage `json:"value"`
	ChangeReason string          `json:"change_reason"`
}

// wfDeleteData carries the change reason for a soft-delete.
type wfDeleteData struct {
	ChangeReason string `json:"change_reason"`
}

// WfCreate handles POST /admin/v1/params/wf-create — workflow7 callback that
// persists a parameter approved through the approval flow. Reuses the same
// service logic as the direct admin API (create new active version + write
// parameter_history with the supplied change_reason).
func (h *AdminHandler) WfCreate(c *gin.Context) {
	const op = "rest.AdminHandler.WfCreate"
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

	var env wfEnvelope
	if err := c.ShouldBindJSON(&env); err != nil {
		span.RecordError(err)
		writeError(c, http.StatusBadRequest, "INVALID_REQUEST", err.Error(), false, nil)
		return
	}
	var req wfCreateData
	if err := json.Unmarshal(env.Data, &req); err != nil {
		span.RecordError(err)
		writeError(c, http.StatusBadRequest, "INVALID_REQUEST", err.Error(), false, gin.H{"field": "data"})
		return
	}
	if err := validateScopeContext(req.AppliesTo, req.AppliesToID); err != nil {
		writeError(c, http.StatusUnprocessableEntity, "INVALID_CALLER_CONTEXT", err.Error(), false, nil)
		return
	}
	if strings.TrimSpace(req.ChangeReason) == "" {
		writeError(c, http.StatusUnprocessableEntity, "INVALID_CALLER_CONTEXT", "change_reason is required", false, gin.H{"field": "change_reason"})
		return
	}

	var pgOrgID, pgUserID pgtype.UUID
	_ = pgOrgID.Scan(orgID.String())
	_ = pgUserID.Scan(userID.String())

	var appliesToID, product, unit, scope pgtype.Text
	if req.AppliesToID != nil {
		appliesToID = pgtype.Text{String: *req.AppliesToID, Valid: true}
	}
	if req.Product != nil {
		product = pgtype.Text{String: *req.Product, Valid: true}
	}
	if req.Unit != nil {
		unit = pgtype.Text{String: *req.Unit, Valid: true}
	}
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
		if writeSchemaError(c, err) {
			return
		}
		span.RecordError(err)
		logger.Error().Err(err).Str("op", op).Str("wf_instance_id", env.WfInstanceID).Msg("wf create parameter failed")
		writeError(c, http.StatusInternalServerError, "POLICY_BACKEND_UNAVAILABLE", err.Error(), true, nil)
		return
	}

	forwardPolicyAudit(c, h.audit7, policyAuditEntry{
		Action: "create_parameter", ResourceType: "parameter",
		ResourceID: pgUUIDString(param.ID), ResourceName: req.Category + "/" + req.Name,
		OrgID: orgID.String(), UserID: userID.String(), WfInstanceID: env.WfInstanceID,
		Data: env.Data,
		After: json.RawMessage(env.Data),
	})

	span.SetStatus(codes.Ok, "")
	c.JSON(http.StatusOK, WfCallbackResponse{Success: true, ID: pgUUIDString(param.ID)})
}

// WfUpdate handles PUT /admin/v1/params/:id/wf-update — workflow7 callback that
// applies an approved value change (version++ + deactivate prior + history).
func (h *AdminHandler) WfUpdate(c *gin.Context) {
	const op = "rest.AdminHandler.WfUpdate"
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

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		writeError(c, http.StatusBadRequest, "INVALID_CALLER_CONTEXT", "invalid parameter ID", false, gin.H{"field": "id"})
		return
	}

	var env wfEnvelope
	if err := c.ShouldBindJSON(&env); err != nil {
		span.RecordError(err)
		writeError(c, http.StatusBadRequest, "INVALID_REQUEST", err.Error(), false, nil)
		return
	}
	var req wfUpdateData
	if err := json.Unmarshal(env.Data, &req); err != nil {
		span.RecordError(err)
		writeError(c, http.StatusBadRequest, "INVALID_REQUEST", err.Error(), false, gin.H{"field": "data"})
		return
	}
	if len(req.Value) == 0 {
		writeError(c, http.StatusUnprocessableEntity, "INVALID_PARAMETER_SHAPE", "value is required", false, gin.H{"field": "value"})
		return
	}
	if strings.TrimSpace(req.ChangeReason) == "" {
		writeError(c, http.StatusUnprocessableEntity, "INVALID_CALLER_CONTEXT", "change_reason is required", false, gin.H{"field": "change_reason"})
		return
	}

	param, err := h.svc.Update(ctx, id, orgID, userID, req.Value, req.ChangeReason)
	if err != nil {
		if writeSchemaError(c, err) {
			return
		}
		span.RecordError(err)
		logger.Error().Err(err).Str("op", op).Str("id", id.String()).Str("wf_instance_id", env.WfInstanceID).Msg("wf update parameter failed")
		if strings.Contains(err.Error(), "inactive parameter") {
			writeError(c, http.StatusConflict, "POLICY_CONFLICT", err.Error(), false, nil)
			return
		}
		writeError(c, http.StatusInternalServerError, "POLICY_BACKEND_UNAVAILABLE", err.Error(), true, nil)
		return
	}

	forwardPolicyAudit(c, h.audit7, policyAuditEntry{
		Action: "update_parameter", ResourceType: "parameter",
		ResourceID: id.String(),
		OrgID:      orgID.String(), UserID: userID.String(), WfInstanceID: env.WfInstanceID,
		Data:  env.Data,
		After: json.RawMessage(env.Data),
	})

	span.SetStatus(codes.Ok, "")
	c.JSON(http.StatusOK, WfCallbackResponse{Success: true, ID: pgUUIDString(param.ID)})
}

// WfDelete handles POST /admin/v1/params/:id/wf-delete — workflow7 callback that
// soft-deletes (deactivate + effective_until) an approved deletion + history.
func (h *AdminHandler) WfDelete(c *gin.Context) {
	const op = "rest.AdminHandler.WfDelete"
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

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		writeError(c, http.StatusBadRequest, "INVALID_CALLER_CONTEXT", "invalid parameter ID", false, gin.H{"field": "id"})
		return
	}

	var env wfEnvelope
	if err := c.ShouldBindJSON(&env); err != nil {
		span.RecordError(err)
		writeError(c, http.StatusBadRequest, "INVALID_REQUEST", err.Error(), false, nil)
		return
	}
	var req wfDeleteData
	if err := json.Unmarshal(env.Data, &req); err != nil {
		span.RecordError(err)
		writeError(c, http.StatusBadRequest, "INVALID_REQUEST", err.Error(), false, gin.H{"field": "data"})
		return
	}
	if strings.TrimSpace(req.ChangeReason) == "" {
		writeError(c, http.StatusUnprocessableEntity, "INVALID_CALLER_CONTEXT", "change_reason is required", false, gin.H{"field": "change_reason"})
		return
	}

	if err := h.svc.Delete(ctx, id, orgID, userID, req.ChangeReason); err != nil {
		span.RecordError(err)
		logger.Error().Err(err).Str("op", op).Str("id", id.String()).Str("wf_instance_id", env.WfInstanceID).Msg("wf delete parameter failed")
		writeError(c, http.StatusInternalServerError, "POLICY_BACKEND_UNAVAILABLE", err.Error(), true, nil)
		return
	}

	forwardPolicyAudit(c, h.audit7, policyAuditEntry{
		Action: "delete_parameter", ResourceType: "parameter",
		ResourceID: id.String(),
		OrgID:      orgID.String(), UserID: userID.String(), WfInstanceID: env.WfInstanceID,
		Data:   env.Data,
		Before: json.RawMessage(env.Data),
	})

	span.SetStatus(codes.Ok, "")
	c.JSON(http.StatusOK, WfCallbackResponse{Success: true, ID: id.String()})
}

// getActorOrgID resolves the org from the audit-signed actor envelope
// (X-Actor-OrgID), falling back to the direct-admin X-Org-ID header. workflow7
// service-task callbacks send X-Actor-OrgID (lib7 ActorEnvelope convention),
// not X-Org-ID — mirror getActorUserID so the wf-* handlers accept both.
func getActorOrgID(c *gin.Context) (uuid.UUID, bool) {
	raw := c.GetHeader("X-Actor-OrgID")
	if raw == "" {
		raw = c.GetHeader("X-Org-ID")
	}
	if raw == "" {
		writeError(c, http.StatusBadRequest, "INVALID_CALLER_CONTEXT", "X-Actor-OrgID header is required", false, gin.H{"field": "org_id"})
		return uuid.Nil, false
	}
	orgID, err := uuid.Parse(raw)
	if err != nil {
		writeError(c, http.StatusBadRequest, "INVALID_CALLER_CONTEXT", "invalid org ID format", false, gin.H{"field": "org_id"})
		return uuid.Nil, false
	}
	return orgID, true
}

// getActorUserID resolves the acting user from the audit-signed actor envelope
// (X-Actor-UserID), falling back to the direct-admin X-User-ID header.
func getActorUserID(c *gin.Context) (uuid.UUID, bool) {
	raw := c.GetHeader("X-Actor-UserID")
	if raw == "" {
		raw = c.GetHeader("X-User-ID")
	}
	if raw == "" {
		writeError(c, http.StatusBadRequest, "INVALID_CALLER_CONTEXT", "X-Actor-UserID header is required", false, gin.H{"field": "actor_user_id"})
		return uuid.Nil, false
	}
	userID, err := uuid.Parse(raw)
	if err != nil {
		writeError(c, http.StatusBadRequest, "INVALID_CALLER_CONTEXT", "invalid actor user ID format", false, gin.H{"field": "actor_user_id"})
		return uuid.Nil, false
	}
	return userID, true
}

// pgUUIDString renders a pgtype.UUID as its canonical string form.
func pgUUIDString(u pgtype.UUID) string {
	if !u.Valid {
		return ""
	}
	id, err := uuid.FromBytes(u.Bytes[:])
	if err != nil {
		return ""
	}
	return id.String()
}
