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
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel/trace/noop"
)

// mockAdminQuerier serves category metadata from an in-memory map so tests can
// exercise the data-driven category gate (existence + active + value_schema)
// without a database. A code absent from the map reports pgx.ErrNoRows.
type mockAdminQuerier struct {
	store.Querier
	categories map[string]store.ParameterCategory
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

func (m *mockAdminQuerier) GetParameterCategoryByCode(ctx context.Context, arg store.GetParameterCategoryByCodeParams) (store.ParameterCategory, error) {
	if cat, ok := m.categories[arg.Code]; ok {
		return cat, nil
	}
	return store.ParameterCategory{}, pgx.ErrNoRows
}

// activeCategory builds an active category row with the given code + value_schema.
func activeCategory(code, schema string) store.ParameterCategory {
	c := store.ParameterCategory{Code: code, IsActive: true}
	if schema != "" {
		c.ValueSchema = json.RawMessage(schema)
	}
	return c
}

// twoLimitSchema mirrors the seeded transaction_limit value_schema essentials.
const twoLimitSchema = `{"type":"object","required":["transaction_limit","authorization_limit","currency"],` +
	`"properties":{"transaction_limit":{"type":"number","minimum":0},"authorization_limit":{"type":"number","minimum":0},` +
	`"currency":{"type":"string","enum":["IDR"]}},` +
	`"x-rules":[{"op":"lte","left":"authorization_limit","right":"transaction_limit"}]}`

const authLimitSchema = `{"type":"object","required":["authorization_limit","currency"],` +
	`"properties":{"authorization_limit":{"type":"number","minimum":0},"currency":{"type":"string","enum":["IDR"]}}}`

// doCreate posts a create request through the admin Create handler and returns
// the recorder. The querier's category map controls the data-driven gate.
func doCreate(t *testing.T, db store.Querier, body map[string]interface{}) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	adminSvc := service.NewAdminParameterService(db, nil, nil)
	r := gin.New()
	h := NewAdminHandler(adminSvc, noop.NewTracerProvider().Tracer(""), zerolog.Nop())
	r.POST("/admin/v1/params", h.Create)

	raw, _ := json.Marshal(body)
	req, _ := http.NewRequest(http.MethodPost, "/admin/v1/params", bytes.NewBuffer(raw))
	req.Header.Set("X-Org-ID", uuid.New().String())
	req.Header.Set("X-User-ID", uuid.New().String())
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// TestAdminCreate_TransactionLimitRoleScopedNoProduct is the core regression for
// core7-devroot#579: a role-scoped transaction_limit with the two-limit value
// and NO product must succeed (the old hardcoded product-rule rejected it).
func TestAdminCreate_TransactionLimitRoleScopedNoProduct(t *testing.T) {
	db := &mockAdminQuerier{categories: map[string]store.ParameterCategory{
		"transaction_limit": activeCategory("transaction_limit", twoLimitSchema),
	}}
	w := doCreate(t, db, map[string]interface{}{
		"category":   "transaction_limit",
		"name":       "teller_transfer_max",
		"applies_to": "role",
		"value":      json.RawMessage(`{"transaction_limit":100000000,"authorization_limit":25000000,"currency":"IDR"}`),
		"value_type": "json",
	})
	assert.Equal(t, http.StatusCreated, w.Code, w.Body.String())
}

// TestAdminCreate_AuthorizationLimitNoProduct: authorization_limit was missing
// from the old allowlist entirely (→ "unsupported category"). It must now be
// creatable, role-scoped, with no product.
func TestAdminCreate_AuthorizationLimitNoProduct(t *testing.T) {
	db := &mockAdminQuerier{categories: map[string]store.ParameterCategory{
		"authorization_limit": activeCategory("authorization_limit", authLimitSchema),
	}}
	w := doCreate(t, db, map[string]interface{}{
		"category":   "authorization_limit",
		"name":       "teller_authorization_limit",
		"applies_to": "role",
		"value":      json.RawMessage(`{"authorization_limit":25000000,"currency":"IDR"}`),
		"value_type": "json",
	})
	assert.Equal(t, http.StatusCreated, w.Code, w.Body.String())
}

// TestAdminCreate_BrandNewCategoryUsable: an admin-added category (not in any
// hardcoded list) with a value_schema is immediately usable for parameters.
func TestAdminCreate_BrandNewCategoryUsable(t *testing.T) {
	const schema = `{"type":"object","required":["max_score"],"properties":{"max_score":{"type":"number","minimum":0}}}`
	db := &mockAdminQuerier{categories: map[string]store.ParameterCategory{
		"loan_scoring": activeCategory("loan_scoring", schema),
	}}
	w := doCreate(t, db, map[string]interface{}{
		"category":   "loan_scoring",
		"name":       "kpr_max_score",
		"applies_to": "global",
		"value":      json.RawMessage(`{"max_score":800}`),
		"value_type": "json",
	})
	assert.Equal(t, http.StatusCreated, w.Code, w.Body.String())
}

// TestAdminCreate_UnknownCategoryRejected: a category with no row in
// parameter_categories is invalid (422 INVALID_CATEGORY) — proving validity is
// data-driven, not a hardcoded allowlist.
func TestAdminCreate_UnknownCategoryRejected(t *testing.T) {
	db := &mockAdminQuerier{categories: map[string]store.ParameterCategory{}}
	w := doCreate(t, db, map[string]interface{}{
		"category":   "does_not_exist",
		"name":       "whatever",
		"applies_to": "global",
		"value":      json.RawMessage(`{"x":1}`),
		"value_type": "json",
	})
	assert.Equal(t, http.StatusUnprocessableEntity, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "INVALID_CATEGORY")
}

// TestAdminCreate_InactiveCategoryRejected: an inactive category is not valid.
func TestAdminCreate_InactiveCategoryRejected(t *testing.T) {
	inactive := activeCategory("retired_cat", "")
	inactive.IsActive = false
	db := &mockAdminQuerier{categories: map[string]store.ParameterCategory{
		"retired_cat": inactive,
	}}
	w := doCreate(t, db, map[string]interface{}{
		"category":   "retired_cat",
		"name":       "whatever",
		"applies_to": "global",
		"value":      json.RawMessage(`{"x":1}`),
		"value_type": "json",
	})
	assert.Equal(t, http.StatusUnprocessableEntity, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "INVALID_CATEGORY")
}

// TestAdminCreate_ValueSchemaRequiredEnforced: value_schema `required` still
// rejects a value missing a mandatory field (422 INVALID_PARAMETER_VALUE). Field
// requirements come from the schema, not category-specific code.
func TestAdminCreate_ValueSchemaRequiredEnforced(t *testing.T) {
	db := &mockAdminQuerier{categories: map[string]store.ParameterCategory{
		"transaction_limit": activeCategory("transaction_limit", twoLimitSchema),
	}}
	w := doCreate(t, db, map[string]interface{}{
		"category":   "transaction_limit",
		"name":       "teller_transfer_max",
		"applies_to": "role",
		"value":      json.RawMessage(`{"transaction_limit":100000000}`), // missing authorization_limit + currency
		"value_type": "json",
	})
	assert.Equal(t, http.StatusUnprocessableEntity, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "INVALID_PARAMETER_VALUE")
}

// TestAdminCreate_BranchScopeRequiresAppliesToID: the one scope rule that
// remains in the handler (branch scope needs applies_to_id).
func TestAdminCreate_BranchScopeRequiresAppliesToID(t *testing.T) {
	db := &mockAdminQuerier{categories: map[string]store.ParameterCategory{
		"transaction_limit": activeCategory("transaction_limit", ""),
	}}
	w := doCreate(t, db, map[string]interface{}{
		"category":   "transaction_limit",
		"name":       "branch_cap",
		"applies_to": "branch", // no applies_to_id
		"value":      json.RawMessage(`{"transaction_limit":1}`),
		"value_type": "json",
	})
	assert.Equal(t, http.StatusUnprocessableEntity, w.Code, w.Body.String())
}
