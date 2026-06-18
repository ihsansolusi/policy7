package api

import (
	"encoding/json"

	"github.com/gin-gonic/gin"
	"github.com/ihsansolusi/lib7-service-go/audit7client"
)

// policyAuditEntry describes one policy7 mutation to forward to the central
// audit7 service as a MODIFICATION event.
type policyAuditEntry struct {
	Action       string          // create_category / update_category / delete_category / create_parameter / ...
	ResourceType string          // parameter_category | parameter
	ResourceID   string          // category code or parameter id
	ResourceName string          // human-friendly label (category name, parameter name)
	OrgID        string          // resolved org (actor envelope)
	UserID       string          // resolved actor user id
	WfInstanceID string          // workflow instance id → correlation_id
	Data         json.RawMessage // the wf data payload — carries the request context the BFF injected
	Before       any             // prior state snapshot (nil for create)
	After        any             // new state snapshot (nil for delete)
}

// wfAuditContext is the request transport context the bos7 BFF injects into the
// workflow `data` payload (workflow7's ActorEnvelope headers only carry identity
// + branch, not ip/ua/session). Mirrors bos7 initiator withAuditContext.
type wfAuditContext struct {
	IPAddress  string `json:"ip_address"`
	UserAgent  string `json:"user_agent"`
	SessionID  string `json:"session_id"`
	BranchID   string `json:"branch_id"`
	BranchCode string `json:"branch_code"`
}

func parseWfAuditContext(data json.RawMessage) wfAuditContext {
	var ctx wfAuditContext
	if len(data) > 0 {
		_ = json.Unmarshal(data, &ctx)
	}
	return ctx
}

// forwardPolicyAudit ships a policy mutation to audit7 (fire-and-forget). A nil
// client (AUDIT7_URL unset) makes this a no-op. Actor identity comes from the
// workflow7-signed ActorEnvelope headers (X-Actor-*); ip/ua/session/branch come
// from the request context the bos7 BFF injected into the wf data (falling back
// to the envelope branch headers). correlation_id is the workflow instance id so
// the audit row links back to the approval flow. Channel is BFF because the
// mutation originated from the bos7-enterprise policy-management UI via workflow7.
func forwardPolicyAudit(c *gin.Context, client *audit7client.Client, e policyAuditEntry) {
	if client == nil {
		return
	}
	ctx := parseWfAuditContext(e.Data)

	branchID := ctx.BranchID
	if branchID == "" {
		branchID = c.GetHeader("X-Actor-BranchID")
	}
	branchCode := ctx.BranchCode
	if branchCode == "" {
		branchCode = c.GetHeader("X-Actor-BranchCode")
	}
	display := c.GetHeader("X-Actor-Username")
	if display == "" {
		display = e.UserID
	}

	client.SendAsync(c.Request.Context(), audit7client.Event{
		OrgID:         e.OrgID,
		BranchID:      branchID,
		BranchCode:    branchCode,
		SessionID:     ctx.SessionID,
		CorrelationID: e.WfInstanceID,
		IPAddress:     ctx.IPAddress,
		UserAgent:     ctx.UserAgent,
		Actor: audit7client.Actor{
			Type:    "user",
			ID:      e.UserID,
			Display: display,
		},
		EventCategory:  "MODIFICATION",
		Action:         e.Action,
		Resource:       audit7client.Resource{Type: e.ResourceType, ID: e.ResourceID, Name: e.ResourceName},
		Result:         "SUCCESS",
		Severity:       "INFO",
		Channel:        "BFF",
		SourceApp:      "policy7",
		Module:         "policy-management",
		BeforeSnapshot: e.Before,
		AfterSnapshot:  e.After,
	})
}
