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
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel/trace/noop"
)

// categoryCaptureQuerier records the CreateParameterCategory call so the wf
// callback test can assert the category was persisted with its value_schema.
type categoryCaptureQuerier struct {
	store.Querier
	created *store.CreateParameterCategoryParams
}

func (m *categoryCaptureQuerier) CreateParameterCategory(ctx context.Context, arg store.CreateParameterCategoryParams) (store.ParameterCategory, error) {
	m.created = &arg
	return store.ParameterCategory{
		Code:        arg.Code,
		Name:        arg.Name,
		ValueSchema: arg.ValueSchema,
		IsActive:    arg.IsActive,
	}, nil
}

// TestCategoryWfCreate_PersistsCategory exercises the workflow7 callback path:
// an audit-signed wf-create envelope (master_type POLICY_CATEGORY) carries the
// category fields under data, and the handler persists parameter_categories
// including the value_schema. Middleware (RequireM2M + VerifyAuditSignature) is
// applied at the router group level and verified independently.
func TestCategoryWfCreate_PersistsCategory(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := &categoryCaptureQuerier{}
	adminSvc := service.NewAdminParameterService(db, nil, nil)
	h := NewCategoryHandler(adminSvc, noop.NewTracerProvider().Tracer(""), zerolog.Nop(), nil)
	r := gin.New()
	r.POST("/admin/v1/categories/wf-create", h.WfCreate)

	body, _ := json.Marshal(map[string]interface{}{
		"wf_instance_id": "wf-123",
		"master_id":      "transaction_limit",
		"master_type":    "POLICY_CATEGORY",
		"data": map[string]interface{}{
			"code":         "transaction_limit",
			"name":         "Transaction Limits",
			"value_schema": json.RawMessage(txnLimitSchemaJSON),
		},
	})
	req, _ := http.NewRequest(http.MethodPost, "/admin/v1/categories/wf-create", bytes.NewBuffer(body))
	req.Header.Set("X-Actor-OrgID", uuid.New().String())
	req.Header.Set("X-Actor-UserID", uuid.New().String())
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp WfCallbackResponse
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.True(t, resp.Success)
	assert.Equal(t, "transaction_limit", resp.ID)

	// Category persisted with its value_schema intact.
	assert.NotNil(t, db.created)
	assert.Equal(t, "transaction_limit", db.created.Code)
	assert.Equal(t, "Transaction Limits", db.created.Name)
	assert.Contains(t, string(db.created.ValueSchema), "x-rules")
}

// TestCategoryWfRoutesRegister ensures the category wf-callback routes coexist
// with the direct /categories/:code routes without a gin registration panic.
func TestCategoryWfRoutesRegister(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := &categoryCaptureQuerier{}
	svc := service.NewParameterService(db, nil, nil)
	adminSvc := service.NewAdminParameterService(db, nil, nil)
	r := gin.New()
	assert.NotPanics(t, func() {
		SetupRoutes(r, svc, adminSvc, nil, zerolog.Nop(), nil, nil)
	})
}
