package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type ContractHandler struct{}

func NewContractHandler() *ContractHandler {
	return &ContractHandler{}
}

func (h *ContractHandler) Categories(c *gin.Context) {
	writeSuccess(c, http.StatusOK, gin.H{
		"categories": []gin.H{
			{"code": "transaction_limit", "requires": []string{"org_id", "role_id|role_code", "product"}},
			{"code": "approval_threshold", "requires": []string{"org_id", "role_id|role_code", "product"}},
			{"code": "operational_hours", "requires": []string{"org_id", "role_id|role_code"}},
			{"code": "product_access", "requires": []string{"org_id", "role_id|role_code", "product"}},
			{"code": "rate", "requires": []string{"org_id", "product"}},
			{"code": "fee", "requires": []string{"org_id", "product"}},
			{"code": "regulatory_threshold", "requires": []string{"org_id"}, "conditional": []string{"branch_id for branch-scoped"}},
		},
	})
}

func (h *ContractHandler) CallerContext(c *gin.Context) {
	writeSuccess(c, http.StatusOK, gin.H{
		"fields": []gin.H{
			{"name": "org_id", "required": true, "source": "X-Org-ID/JWT"},
			{"name": "branch_id", "required": false, "source": "query/body/JWT", "note": "required for branch-scoped"},
			{"name": "user_id", "required": false, "source": "X-User-ID/JWT", "note": "required for admin mutation"},
			{"name": "role_id", "required": false, "source": "query/body/JWT"},
			{"name": "role_code", "required": false, "source": "query/body/JWT"},
			{"name": "product", "required": false, "source": "query/body", "note": "required for product-scoped category"},
		},
		"validation": []string{
			"missing org_id -> INVALID_CALLER_CONTEXT",
			"branch-scoped without branch_id -> INVALID_CALLER_CONTEXT",
			"admin mutation without user_id -> INVALID_CALLER_CONTEXT",
			"tenant mismatch -> TENANT_SCOPE_VIOLATION",
		},
	})
}

func (h *ContractHandler) Errors(c *gin.Context) {
	writeSuccess(c, http.StatusOK, gin.H{
		"codes": []gin.H{
			{"code": "INVALID_CALLER_CONTEXT", "http_status": 400, "retryable": false},
			{"code": "INVALID_CATEGORY", "http_status": 422, "retryable": false},
			{"code": "INVALID_PARAMETER_SHAPE", "http_status": 422, "retryable": false},
			{"code": "TENANT_SCOPE_VIOLATION", "http_status": 403, "retryable": false},
			{"code": "PARAMETER_NOT_FOUND", "http_status": 404, "retryable": false},
			{"code": "CATEGORY_NOT_CONFIGURED", "http_status": 404, "retryable": false},
			{"code": "POLICY_BACKEND_UNAVAILABLE", "http_status": 503, "retryable": true},
		},
	})
}
