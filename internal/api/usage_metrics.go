package api

import (
	"github.com/gin-gonic/gin"
	"github.com/ihsansolusi/lib7-service-go/middleware"
	"github.com/prometheus/client_golang/prometheus"
)

// trackUsage records a hit to a deprecation-candidate endpoint, labelled by the
// calling service, so we can decide which routes are safe to retire (see
// docs/ROADMAP.md "API usage / deprecation candidates"). A 2026-06 cross-repo
// review found that roughly half of policy7's surface has no in-tree caller
// (the legacy rates/fees paths, the unwired /v1 boundary reads, the direct
// non-workflow admin CRUD, /contracts/*). lib7's http_requests_total{path}
// already tells us *whether* a route is hit; this adds *who* hits it.
//
// Registered as a per-route handler AFTER the group auth middleware, so only
// authenticated callers are counted (auth failures abort earlier). counter is
// nil in tests (metrics registry not wired) → the middleware is a no-op.
func trackUsage(counter *prometheus.CounterVec) gin.HandlerFunc {
	return func(c *gin.Context) {
		if counter == nil {
			c.Next()
			return
		}
		route := c.FullPath()
		caller := callerIdentity(c)
		c.Next()
		counter.WithLabelValues(route, caller).Inc()
	}
}

// callerIdentity derives a stable, low-cardinality label for the upstream
// caller from the verified token payload:
//   - M2M client_credentials (workflow7, core7-service-*) → client_id
//   - delegated BFF call (RFC 8693 token exchange)        → act.sub (ActorID)
//   - X-Service-Key system bypass                         → "system"
//   - direct user token (no act.sub)                      → "user"
//   - no/invalid payload                                  → "unknown"
func callerIdentity(c *gin.Context) string {
	p, ok := middleware.GetPayload(c)
	if !ok || p == nil {
		return "unknown"
	}
	switch {
	case p.ClientID != "":
		return p.ClientID
	case p.ActorID != "":
		return p.ActorID
	case p.IsM2M():
		return "system"
	default:
		return "user"
	}
}
