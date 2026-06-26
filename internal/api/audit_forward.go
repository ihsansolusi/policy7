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

// forwardPolicyAudit ships a policy mutation to audit7 (fire-and-forget) via the
// shared lib7 audit7client.ForwardWfMutation helper. A nil client (AUDIT7_URL
// unset) makes this a no-op. Actor identity comes from the workflow7-signed
// ActorEnvelope headers (X-Actor-*); ip/ua/session/branch are parsed by the
// helper from the wf data the bos7 BFF injected (branch falls back to the
// envelope headers). correlation_id is the workflow instance id; channel is BFF.
func forwardPolicyAudit(c *gin.Context, client *audit7client.Client, e policyAuditEntry) {
	client.ForwardWfMutation(c.Request.Context(), audit7client.WfMutation{
		SourceApp:    "policy7",
		Module:       "policy-management",
		Action:       e.Action,
		ResourceType: e.ResourceType,
		ResourceID:   e.ResourceID,
		ResourceName: e.ResourceName,
		OrgID:        e.OrgID,
		ActorID:      e.UserID,
		ActorDisplay: c.GetHeader("X-Actor-Username"),
		WfInstanceID: e.WfInstanceID,
		BranchID:     c.GetHeader("X-Actor-BranchID"),
		BranchCode:   c.GetHeader("X-Actor-BranchCode"),
		Data:         e.Data,
		Before:       e.Before,
		After:        e.After,
	})
}
