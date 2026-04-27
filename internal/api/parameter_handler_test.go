package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/ihsansolusi/policy7/internal/service"
	"github.com/ihsansolusi/policy7/internal/store"
)

// mockQuerier implements store.Querier for testing
type mockQuerier struct {
	store.Querier
}

func (m *mockQuerier) GetParameter(ctx context.Context, arg store.GetParameterParams) (store.Parameter, error) {
	return store.Parameter{
		Category: arg.Category,
		Name:     arg.Name,
		Value:    []byte(`{"limit": 100}`),
	}, nil
}

func TestGetParameter(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Setup service with mock DB and nil cache
	db := &mockQuerier{}
	svc := service.NewParameterService(db, nil)
	adminSvc := service.NewAdminParameterService(db, nil)

	r := gin.Default()
	SetupRoutes(r, svc, adminSvc)

	req, _ := http.NewRequest(http.MethodGet, "/v1/params/transaction_limit/teller_transfer_max", nil)
	// Add required header
	req.Header.Set("X-Org-ID", uuid.New().String())
	
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status code 200, got %v", w.Code)
	}
}

func TestValidateTransactionLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := &mockQuerier{}
	svc := service.NewParameterService(db, nil)
	adminSvc := service.NewAdminParameterService(db, nil)

	r := gin.Default()
	SetupRoutes(r, svc, adminSvc)

	reqBody := map[string]interface{}{
		"name":   "teller_transfer_max",
		"amount": 50,
	}
	body, _ := json.Marshal(reqBody)

	req, _ := http.NewRequest(http.MethodPost, "/v1/params/transaction_limit/validate", bytes.NewBuffer(body))
	req.Header.Set("X-Org-ID", uuid.New().String())

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status code 200, got %v", w.Code)
	}

	var res map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &res)
	
	isValid, ok := res["is_valid"].(bool)
	if !ok || !isValid {
		t.Fatalf("Expected is_valid to be true")
	}
}
