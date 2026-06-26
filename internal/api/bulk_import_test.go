package api

import (
	"bytes"
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

// TestBulkImportPerRowResults covers #588: each row reports {row,status,error,code},
// and a bad row does not sink the others (best-effort).
func TestBulkImportPerRowResults(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := &mockAdminQuerier{categories: map[string]store.ParameterCategory{
		"transaction_limit": activeCategory("transaction_limit", twoLimitSchema),
	}}
	adminSvc := service.NewAdminParameterService(db, nil, nil)
	h := NewAdminHandler(adminSvc, noop.NewTracerProvider().Tracer(""), zerolog.Nop(), nil)
	r := gin.New()
	r.POST("/admin/v1/params/bulk-import", h.BulkImport)

	rows := []map[string]interface{}{
		{ // 0 — OK
			"category": "transaction_limit", "name": "teller_transfer_max", "applies_to": "global",
			"value": json.RawMessage(`{"transaction_limit":100,"authorization_limit":50,"currency":"IDR"}`), "value_type": "json",
		},
		{ // 1 — unknown category → INVALID_CATEGORY
			"category": "does_not_exist", "name": "x", "applies_to": "global",
			"value": json.RawMessage(`{"a":1}`), "value_type": "json",
		},
		{ // 2 — branch scope without applies_to_id → INVALID_CALLER_CONTEXT (scope shape)
			"category": "transaction_limit", "name": "y", "applies_to": "branch",
			"value": json.RawMessage(`{"transaction_limit":1,"authorization_limit":1,"currency":"IDR"}`), "value_type": "json",
		},
	}
	raw, _ := json.Marshal(rows)
	req, _ := http.NewRequest(http.MethodPost, "/admin/v1/params/bulk-import", bytes.NewBuffer(raw))
	req.Header.Set("X-Org-ID", uuid.New().String())
	req.Header.Set("X-User-ID", uuid.New().String())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var res struct {
		Data struct {
			Summary struct {
				SuccessCount int `json:"success_count"`
				FailedCount  int `json:"failed_count"`
				TotalCount   int `json:"total_count"`
			} `json:"summary"`
			Results []struct {
				Row    int    `json:"row"`
				Status string `json:"status"`
				Code   string `json:"code"`
			} `json:"results"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatalf("decode: %v (%s)", err, w.Body.String())
	}
	d := res.Data
	if d.Summary.SuccessCount != 1 || d.Summary.FailedCount != 2 || d.Summary.TotalCount != 3 {
		t.Fatalf("summary = %+v, want 1/2/3", d.Summary)
	}
	if len(d.Results) != 3 {
		t.Fatalf("want 3 results, got %d", len(d.Results))
	}
	if d.Results[0].Status != "created" {
		t.Errorf("row 0 = %q, want created", d.Results[0].Status)
	}
	if d.Results[1].Status != "failed" || d.Results[1].Code != "INVALID_CATEGORY" {
		t.Errorf("row 1 = %+v, want failed/INVALID_CATEGORY", d.Results[1])
	}
	if d.Results[2].Status != "failed" || d.Results[2].Code != "INVALID_CALLER_CONTEXT" {
		t.Errorf("row 2 = %+v, want failed/INVALID_CALLER_CONTEXT", d.Results[2])
	}
}
