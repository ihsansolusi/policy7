package api

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/ihsansolusi/lib7-service-go/logging"
	"github.com/ihsansolusi/policy7/internal/service"
	"go.opentelemetry.io/otel/codes"
)

// maxResolveKeys bounds a single batch-resolve request.
const maxResolveKeys = 100

// snapshotLimit caps how many parameters a category snapshot returns.
const snapshotLimit = 1000

// ResolveBatch resolves many parameters in one call (Grup 2 — generic inquiry).
// One decision often needs several parameters; this avoids N round-trips. The
// resolution context (branch/role/user/product) applies to every key, and each
// key is resolved with the same fallback as GET …/effective
// (user→role→branch→global). The matched scope is the returned parameter's
// applies_to. Missing keys come back found=false (the call still returns 200).
//
//	POST /v1/params/resolve
//	{ "context": {"branch_id","role_id","user_id","product"},
//	  "keys": [ {"category","name"}, … ] }
func (h *ParameterHandler) ResolveBatch(c *gin.Context) {
	const op = "rest.ParameterHandler.ResolveBatch"
	ctx, span := h.tracer.Start(c.Request.Context(), op)
	defer span.End()

	orgID, ok := requireOrgID(c)
	if !ok {
		return
	}

	var req struct {
		Context struct {
			UserID   *string `json:"user_id"`
			RoleID   *string `json:"role_id"`
			BranchID *string `json:"branch_id"`
			Product  *string `json:"product"`
		} `json:"context"`
		Keys []struct {
			Category string `json:"category"`
			Name     string `json:"name"`
		} `json:"keys"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, http.StatusBadRequest, "INVALID_REQUEST", err.Error(), false, nil)
		return
	}
	if len(req.Keys) == 0 {
		writeError(c, http.StatusBadRequest, "INVALID_REQUEST", "keys must not be empty", false, gin.H{"field": "keys"})
		return
	}
	if len(req.Keys) > maxResolveKeys {
		writeError(c, http.StatusBadRequest, "TOO_MANY_KEYS", "too many keys in one request", false, gin.H{"max": maxResolveKeys})
		return
	}

	resCtx := service.ResolutionContext{
		UserID:   req.Context.UserID,
		RoleID:   req.Context.RoleID,
		BranchID: req.Context.BranchID,
		Global:   true,
	}

	results := make([]gin.H, 0, len(req.Keys))
	for _, k := range req.Keys {
		if strings.TrimSpace(k.Category) == "" || strings.TrimSpace(k.Name) == "" {
			results = append(results, gin.H{"category": k.Category, "name": k.Name, "found": false, "error": "category and name are required"})
			continue
		}
		param, err := h.svc.GetEffectiveParameter(ctx, orgID, k.Category, k.Name, req.Context.Product, resCtx)
		if err != nil {
			results = append(results, gin.H{"category": k.Category, "name": k.Name, "found": false, "parameter": nil})
			continue
		}
		// param.AppliesTo carries the tier that matched (the "matched scope").
		results = append(results, gin.H{"category": k.Category, "name": k.Name, "found": true, "parameter": param})
	}

	span.SetStatus(codes.Ok, "")
	writeSuccess(c, http.StatusOK, gin.H{"results": results})
}

// Snapshot returns all active parameters in a category for the org (Grup 2 —
// generic inquiry), optionally filtered by product. Backs cache-warm / "give me
// all rates" reads, replacing the hardcoded per-category list endpoints.
//
//	GET /v1/params?category={code}&product={code}
func (h *ParameterHandler) Snapshot(c *gin.Context) {
	const op = "rest.ParameterHandler.Snapshot"
	logger := logging.WithTrace(c.Request.Context(), h.logger)
	ctx, span := h.tracer.Start(c.Request.Context(), op)
	defer span.End()

	orgID, ok := requireOrgID(c)
	if !ok {
		return
	}

	category := strings.TrimSpace(c.Query("category"))
	if category == "" {
		writeError(c, http.StatusBadRequest, "INVALID_CALLER_CONTEXT", "category query parameter is required", false, gin.H{"field": "category"})
		return
	}

	var product *string
	if p := c.Query("product"); p != "" {
		product = &p
	}

	params, err := h.svc.SnapshotByCategory(ctx, orgID, category, product, snapshotLimit)
	if err != nil {
		span.RecordError(err)
		logger.Error().Err(err).Str("op", op).Msg("failed")
		writeError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to list parameters", true, nil)
		return
	}

	span.SetStatus(codes.Ok, "")
	writeSuccess(c, http.StatusOK, gin.H{"category": category, "count": len(params), "params": params})
}

// requireOrgID extracts and validates the X-Org-ID header, writing the standard
// error response and returning ok=false when absent/invalid.
func requireOrgID(c *gin.Context) (uuid.UUID, bool) {
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
