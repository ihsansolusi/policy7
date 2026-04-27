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
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
)

type mockAdminQuerier struct {
	store.Querier
}

func (m *mockAdminQuerier) CreateParameter(ctx context.Context, arg store.CreateParameterParams) (store.Parameter, error) {
	return store.Parameter{
		ID:       pgtype.UUID{Bytes: uuid.New(), Valid: true},
		OrgID:    arg.OrgID,
		Category: arg.Category,
		Name:     arg.Name,
		Value:    arg.Value,
		Version:  1,
	}, nil
}

func (m *mockAdminQuerier) CreateParameterHistory(ctx context.Context, arg store.CreateParameterHistoryParams) (store.ParameterHistory, error) {
	return store.ParameterHistory{}, nil
}

func TestAdminCreateParameter(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := &mockAdminQuerier{}
	adminSvc := service.NewAdminParameterService(db, nil)

	r := gin.Default()
	adminHandler := NewAdminHandler(adminSvc)
	r.POST("/admin/v1/params", adminHandler.Create)

	reqBody := map[string]interface{}{
		"category":      "transaction_limit",
		"name":          "teller_transfer_max",
		"applies_to":    "global",
		"value":         json.RawMessage(`{"limit": 1000}`),
		"value_type":    "json",
		"change_reason": "initial setup",
	}
	body, _ := json.Marshal(reqBody)

	req, _ := http.NewRequest(http.MethodPost, "/admin/v1/params", bytes.NewBuffer(body))
	req.Header.Set("X-Org-ID", uuid.New().String())
	req.Header.Set("X-User-ID", uuid.New().String())
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
}
