package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/ihsansolusi/policy7/internal/service"
	"github.com/ihsansolusi/policy7/internal/store"
	"github.com/rs/zerolog"
)

// mockQuerier implements store.Querier for testing
type mockQuerier struct {
	store.Querier
}

func (m *mockQuerier) GetParameter(ctx context.Context, arg store.GetParameterParams) (store.Parameter, error) {
	var value []byte
	switch arg.Category {
	case "transaction_limit":
		// Mirror the real stored shape: both caps live in the value.
		value = []byte(`{"scope":"per_transaction","currency":"IDR","transaction_limit":100000000,"authorization_limit":25000000}`)
	case "authorization_limit":
		// Real authorization_limit params use the "limit_amount" key.
		value = []byte(`{"currency":"IDR","limit_amount":50000000}`)
	default:
		value = []byte(`{"limit": 100}`)
	}
	return store.Parameter{
		Category: arg.Category,
		Name:     arg.Name,
		Value:    value,
	}, nil
}

// postParam issues an authenticated POST to the given path and returns the
// decoded `data` envelope on a 200, failing the test otherwise.
func postParam(t *testing.T, r http.Handler, path string, body map[string]interface{}) map[string]interface{} {
	t.Helper()
	raw, _ := json.Marshal(body)
	req, _ := http.NewRequest(http.MethodPost, path, bytes.NewBuffer(raw))
	req.Header.Set("X-Org-ID", uuid.New().String())
	req.Header.Set("X-Service-Key", "test-service-key")
	req.Header.Set("X-Service-ID", "test")
	req.Header.Set("X-API-Key", "test-key")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("POST %s: expected status 200, got %d body=%s", path, w.Code, w.Body.String())
	}

	var res struct {
		Success bool                   `json:"success"`
		Data    map[string]interface{} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatalf("POST %s: failed to decode body %s: %v", path, w.Body.String(), err)
	}
	return res.Data
}

// The mock transaction_limit value is {transaction_limit:100000000, authorization_limit:25000000}.
func TestValidateTransactionLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := &mockQuerier{}
	svc := service.NewParameterService(db, nil, nil)
	adminSvc := service.NewAdminParameterService(db, nil, nil)

	r := gin.Default()
	SetupRoutes(r, svc, adminSvc, nil, zerolog.Nop(), nil, nil)

	cases := []struct {
		name           string
		amount         float64
		wantDecision   string
		wantCanInput   bool
		wantAutoAuth   bool
		wantValid      bool
		wantRemaining  bool
		remainingValue float64
	}{
		{"auto_authorized", 15000000, "AUTO_AUTHORIZED", true, true, true, true, 85000000},
		{"boundary_auth_limit", 25000000, "AUTO_AUTHORIZED", true, true, true, true, 75000000},
		{"requires_authorization", 75000000, "REQUIRES_AUTHORIZATION", true, false, true, true, 25000000},
		{"boundary_txn_limit", 100000000, "REQUIRES_AUTHORIZATION", true, false, true, true, 0},
		{"rejected", 150000000, "REJECTED", false, false, false, false, 0},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			data := postParam(t, r, "/v1/params/transaction_limit/validate", map[string]interface{}{
				"name":    "teller_transfer_max",
				"amount":  tc.amount,
				"role_id": "teller",
				"product": "transfer",
			})

			if got := data["decision"].(string); got != tc.wantDecision {
				t.Errorf("decision = %q, want %q", got, tc.wantDecision)
			}
			if got := data["can_input"].(bool); got != tc.wantCanInput {
				t.Errorf("can_input = %v, want %v", got, tc.wantCanInput)
			}
			if got := data["auto_authorized"].(bool); got != tc.wantAutoAuth {
				t.Errorf("auto_authorized = %v, want %v", got, tc.wantAutoAuth)
			}
			if got := data["is_valid"].(bool); got != tc.wantValid {
				t.Errorf("is_valid = %v, want %v", got, tc.wantValid)
			}
			// Regression guard: the old bug made limit always 0.
			if got := data["limit"].(float64); got != 100000000 {
				t.Errorf("limit = %v, want 100000000", got)
			}
			txn := data["transaction_limit"].(map[string]interface{})
			if got := txn["max"].(float64); got != 100000000 {
				t.Errorf("transaction_limit.max = %v, want 100000000", got)
			}
			_, hasRemaining := txn["remaining"]
			if hasRemaining != tc.wantRemaining {
				t.Errorf("transaction_limit.remaining present = %v, want %v", hasRemaining, tc.wantRemaining)
			}
			if tc.wantRemaining {
				if got := txn["remaining"].(float64); got != tc.remainingValue {
					t.Errorf("transaction_limit.remaining = %v, want %v", got, tc.remainingValue)
				}
			}
			auth := data["authorization_limit"].(map[string]interface{})
			if got := auth["max"].(float64); got != 25000000 {
				t.Errorf("authorization_limit.max = %v, want 25000000", got)
			}
		})
	}
}

func TestMain(m *testing.M) {
	os.Setenv("SERVICE_KEY", "test-service-key")
	os.Exit(m.Run())
}
