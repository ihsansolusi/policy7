package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/ihsansolusi/policy7/internal/service"
	"github.com/ihsansolusi/policy7/internal/store"
	"github.com/rs/zerolog"
	"go.opentelemetry.io/otel/trace/noop"
)

// historyMock proves the history endpoint uses the full-chain (identity-tuple)
// query, not the per-id one: the per-id method returns nothing, the identity
// method returns the whole v1→v3 chain.
type historyMock struct{ store.Querier }

func (m *historyMock) GetParameterHistory(ctx context.Context, arg store.GetParameterHistoryParams) ([]store.ParameterHistory, error) {
	return []store.ParameterHistory{}, nil // per-id: fragmented (should NOT be used)
}

func (m *historyMock) GetParameterHistoryByIdentity(ctx context.Context, arg store.GetParameterHistoryByIdentityParams) ([]store.ParameterHistory, error) {
	return []store.ParameterHistory{
		{ChangeType: "create", NewVersion: 1},
		{ChangeType: "update", NewVersion: 2},
		{ChangeType: "update", NewVersion: 3},
	}, nil
}

func TestGetHistoryFullChain(t *testing.T) {
	gin.SetMode(gin.TestMode)
	adminSvc := service.NewAdminParameterService(&historyMock{}, nil, nil)
	h := NewAdminHandler(adminSvc, noop.NewTracerProvider().Tracer(""), zerolog.Nop(), nil)
	r := gin.New()
	r.GET("/admin/v1/params/:id/history", h.GetHistory)

	req, _ := http.NewRequest(http.MethodGet, "/admin/v1/params/"+uuid.New().String()+"/history", nil)
	req.Header.Set("X-Org-ID", uuid.New().String())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var res struct {
		Data []struct {
			ChangeType string `json:"change_type"`
			NewVersion int32  `json:"new_version"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatalf("decode: %v (%s)", err, w.Body.String())
	}
	if len(res.Data) != 3 {
		t.Fatalf("want full chain of 3, got %d (per-id query used instead of identity?)", len(res.Data))
	}
	for i, want := range []int32{1, 2, 3} {
		if res.Data[i].NewVersion != want {
			t.Errorf("row %d new_version = %d, want %d (chain must be oldest→newest)", i, res.Data[i].NewVersion, want)
		}
	}
}
