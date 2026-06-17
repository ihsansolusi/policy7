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
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel/trace/noop"
)

const txnLimitSchemaJSON = `{"type":"object","required":["transaction_limit","authorization_limit","currency"],` +
	`"properties":{"transaction_limit":{"type":"number","minimum":0},` +
	`"authorization_limit":{"type":"number","minimum":0},` +
	`"currency":{"type":"string","enum":["IDR"]}},` +
	`"x-rules":[{"op":"lte","left":"authorization_limit","right":"transaction_limit"}]}`

// schemaQuerier returns the transaction_limit value_schema for any category
// lookup, so create/update flows exercise the validation backstop.
type schemaQuerier struct {
	store.Querier
}

func (m *schemaQuerier) GetParameterCategoryByCode(ctx context.Context, arg store.GetParameterCategoryByCodeParams) (store.ParameterCategory, error) {
	return store.ParameterCategory{
		Code:        arg.Code,
		ValueSchema: json.RawMessage(txnLimitSchemaJSON),
		IsActive:    true,
	}, nil
}

func (m *schemaQuerier) CreateParameter(ctx context.Context, arg store.CreateParameterParams) (store.Parameter, error) {
	return store.Parameter{ID: pgtype.UUID{Bytes: uuid.New(), Valid: true}, Category: arg.Category, Value: arg.Value, Version: 1}, nil
}

func (m *schemaQuerier) CreateParameterHistory(ctx context.Context, arg store.CreateParameterHistoryParams) (store.ParameterHistory, error) {
	return store.ParameterHistory{}, nil
}

func (m *schemaQuerier) ListParameterCategories(ctx context.Context, orgID pgtype.UUID) ([]store.ParameterCategory, error) {
	return []store.ParameterCategory{{
		Code:         "transaction_limit",
		Name:         "Transaction Limits",
		ValueSchema:  json.RawMessage(txnLimitSchemaJSON),
		DisplayOrder: pgtype.Int4{Int32: 1, Valid: true},
		IsActive:     true,
	}}, nil
}

func newParamCreateRequest(t *testing.T, value string) *http.Request {
	t.Helper()
	body, _ := json.Marshal(map[string]interface{}{
		"category":      "transaction_limit",
		"name":          "teller_transfer_max",
		"applies_to":    "global",
		"product":       "transfer",
		"value":         json.RawMessage(value),
		"value_type":    "json",
		"change_reason": "wave c test",
	})
	req, _ := http.NewRequest(http.MethodPost, "/admin/v1/params", bytes.NewBuffer(body))
	req.Header.Set("X-Org-ID", uuid.New().String())
	req.Header.Set("X-User-ID", uuid.New().String())
	req.Header.Set("Content-Type", "application/json")
	return req
}

func setupAdminRouter(db store.Querier) *gin.Engine {
	gin.SetMode(gin.TestMode)
	adminSvc := service.NewAdminParameterService(db, nil, nil)
	h := NewAdminHandler(adminSvc, noop.NewTracerProvider().Tracer(""), zerolog.Nop())
	r := gin.New()
	r.POST("/admin/v1/params", h.Create)
	return r
}

func TestCreateParameter_ValidValuePasses(t *testing.T) {
	r := setupAdminRouter(&schemaQuerier{})
	req := newParamCreateRequest(t, `{"transaction_limit":100000000,"authorization_limit":25000000,"currency":"IDR"}`)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestCreateParameter_XRuleViolationRejected(t *testing.T) {
	r := setupAdminRouter(&schemaQuerier{})
	// authorization_limit > transaction_limit violates the lte x-rule.
	req := newParamCreateRequest(t, `{"transaction_limit":25000000,"authorization_limit":99000000,"currency":"IDR"}`)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)

	var resp map[string]interface{}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, false, resp["success"])
	errObj, _ := resp["error"].(map[string]interface{})
	assert.Equal(t, "INVALID_PARAMETER_VALUE", errObj["code"])
}

func TestCreateParameter_MissingRequiredRejected(t *testing.T) {
	r := setupAdminRouter(&schemaQuerier{})
	req := newParamCreateRequest(t, `{"transaction_limit":100}`)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
}

func TestListCategories(t *testing.T) {
	gin.SetMode(gin.TestMode)
	adminSvc := service.NewAdminParameterService(&schemaQuerier{}, nil, nil)
	h := NewCategoryHandler(adminSvc, noop.NewTracerProvider().Tracer(""), zerolog.Nop())
	r := gin.New()
	r.GET("/admin/v1/categories", h.List)

	req, _ := http.NewRequest(http.MethodGet, "/admin/v1/categories", nil)
	req.Header.Set("X-Org-ID", uuid.New().String())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp struct {
		Success bool               `json:"success"`
		Data    []categoryResponse `json:"data"`
	}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.True(t, resp.Success)
	assert.Len(t, resp.Data, 1)
	assert.Equal(t, "transaction_limit", resp.Data[0].Code)
	// value_schema must be surfaced verbatim for the FE adapter.
	assert.Contains(t, string(resp.Data[0].ValueSchema), "x-rules")
}
