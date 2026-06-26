package api

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/ihsansolusi/lib7-service-go/middleware"
	"github.com/ihsansolusi/lib7-service-go/token"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

func ctxWithPayload(p *token.Payload) *gin.Context {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	if p != nil {
		c.Set(middleware.PayloadKey, p)
	}
	return c
}

func TestCallerIdentity(t *testing.T) {
	tests := []struct {
		name    string
		payload *token.Payload
		want    string
	}{
		{"no payload", nil, "unknown"},
		{"m2m client_credentials", &token.Payload{ClientID: "workflow7"}, "workflow7"},
		{"delegated bff (act.sub)", &token.Payload{ActorID: "core7-service-enterprise"}, "core7-service-enterprise"},
		{"system service-key", &token.Payload{UserID: uuid.Nil, Roles: []string{"system"}}, "system"},
		{"direct user token", &token.Payload{UserID: uuid.New(), Roles: []string{"teller"}}, "user"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := callerIdentity(ctxWithPayload(tt.payload)); got != tt.want {
				t.Fatalf("callerIdentity = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestTrackUsageNilCounterIsNoOp(t *testing.T) {
	c := ctxWithPayload(&token.Payload{ClientID: "workflow7"})
	// Must not panic when the metrics registry was not wired (e.g. tests).
	trackUsage(nil)(c)
}

func TestTrackUsageIncrementsByCaller(t *testing.T) {
	counter := prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "test_usage_total", Help: "t"},
		[]string{"route", "caller"},
	)
	c := ctxWithPayload(&token.Payload{ClientID: "core7-service-financing"})
	trackUsage(counter)(c)

	// route is empty (no matched gin route in a bare test context); caller is the label we care about.
	m := &dto.Metric{}
	if err := counter.WithLabelValues("", "core7-service-financing").Write(m); err != nil {
		t.Fatalf("write metric: %v", err)
	}
	if got := m.GetCounter().GetValue(); got != 1 {
		t.Fatalf("counter = %v, want 1", got)
	}
}
