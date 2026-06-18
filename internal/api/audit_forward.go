package api

import (
	"github.com/gin-gonic/gin"
	"github.com/ihsansolusi/lib7-service-go/audit7client"
)

// policyAuditEntry describes one policy7 mutation to forward to the central
// audit7 service as a MODIFICATION event.
type policyAuditEntry struct {
	Action       string // create_category / update_category / delete_category / create_parameter / ...
	ResourceType string // parameter_category | parameter
	ResourceID   string // category code or parameter id
	ResourceName string // human-friendly label (category name, parameter name)
	OrgID        string // resolved org (actor envelope)
	UserID       string // resolved actor user id
	WfInstanceID string // workflow instance id → correlation_id
	Before       any    // prior state snapshot (nil for create)
	After        any    // new state snapshot (nil for delete)
}

// forwardPolicyAudit ships a policy mutation to audit7 (fire-and-forget). A nil
// client (AUDIT7_URL unset) makes this a no-op. Actor display + branch come from
// the lib7 ActorEnvelope headers workflow7 signs onto the wf-* callback; the
// correlation_id is the workflow instance id so the audit row links back to the
// approval flow. Channel is BFF because the mutation originated from the
// bos7-enterprise policy-management UI via workflow7.
func forwardPolicyAudit(c *gin.Context, client *audit7client.Client, e policyAuditEntry) {
	if client == nil {
		return
	}
	display := c.GetHeader("X-Actor-Username")
	if display == "" {
		display = e.UserID
	}
	client.SendAsync(c.Request.Context(), audit7client.Event{
		OrgID:         e.OrgID,
		BranchID:      c.GetHeader("X-Actor-BranchID"),
		BranchCode:    c.GetHeader("X-Actor-BranchCode"),
		CorrelationID: e.WfInstanceID,
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
