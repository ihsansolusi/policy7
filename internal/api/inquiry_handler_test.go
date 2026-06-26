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
)

// inqMock implements just the Querier methods the inquiry endpoints touch.
type inqMock struct{ store.Querier }

func (m *inqMock) GetParameter(ctx context.Context, arg store.GetParameterParams) (store.Parameter, error) {
	if arg.Name == "missing" {
		return store.Parameter{}, pgx.ErrNoRows
	}
	return store.Parameter{
		Category:  arg.Category,
		Name:      arg.Name,
		AppliesTo: arg.AppliesTo, // echo the matched scope
		Value:     []byte(`{"rate":4.5}`),
		ValueType: "json",
		Version:   1,
		IsActive:  true,
	}, nil
}

func (m *inqMock) ListParametersFiltered(ctx context.Context, arg store.ListParametersFilteredParams) ([]store.Parameter, error) {
	return []store.Parameter{
		{Category: arg.Category.String, Name: "deposito_3m", AppliesTo: "product", Value: []byte(`{"rate":3.25}`), ValueType: "json", Version: 1, IsActive: true},
		{Category: arg.Category.String, Name: "deposito_12m", AppliesTo: "product", Value: []byte(`{"rate":4.5}`), ValueType: "json", Version: 1, IsActive: true},
	}, nil
}

func (m *inqMock) ListParameterCategories(ctx context.Context, orgID pgtype.UUID) ([]store.ParameterCategory, error) {
	return []store.ParameterCategory{
		{Code: "rate", Name: "Rates", ValueSchema: json.RawMessage(`{"type":"object","properties":{"rate":{"type":"number"}}}`), IsActive: true, DisplayOrder: 1},
	}, nil
}

func newInquiryRouter() http.Handler {
	gin.SetMode(gin.TestMode)
	db := &inqMock{}
	svc := service.NewParameterService(db, nil, nil)
	adminSvc := service.NewAdminParameterService(db, nil, nil)
	r := gin.New()
	SetupRoutes(r, svc, adminSvc, nil, zerolog.Nop(), nil, nil)
	return r
}

func authReq(method, path string, body any) *http.Request {
	var rdr *bytes.Buffer = bytes.NewBuffer(nil)
	if body != nil {
		raw, _ := json.Marshal(body)
		rdr = bytes.NewBuffer(raw)
	}
	req, _ := http.NewRequest(method, path, rdr)
	req.Header.Set("X-Org-ID", uuid.New().String())
	req.Header.Set("X-Service-Key", "test-service-key")
	return req
}

func TestResolveBatch(t *testing.T) {
	r := newInquiryRouter()
	role := "teller"
	body := map[string]any{
		"context": map[string]any{"role_id": role, "product": "transfer"},
		"keys": []map[string]string{
			{"category": "rate", "name": "deposito_12m"},
			{"category": "rate", "name": "missing"},
		},
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, authReq(http.MethodPost, "/v1/params/resolve", body))
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var res struct {
		Data struct {
			Results []struct {
				Category string `json:"category"`
				Name     string `json:"name"`
				Found    bool   `json:"found"`
			} `json:"results"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatalf("decode: %v (%s)", err, w.Body.String())
	}
	if len(res.Data.Results) != 2 {
		t.Fatalf("want 2 results, got %d", len(res.Data.Results))
	}
	if !res.Data.Results[0].Found {
		t.Errorf("key 0 (deposito_12m) should be found")
	}
	if res.Data.Results[1].Found {
		t.Errorf("key 1 (missing) should be not found")
	}
}

func TestResolveBatchEmptyKeys(t *testing.T) {
	r := newInquiryRouter()
	w := httptest.NewRecorder()
	r.ServeHTTP(w, authReq(http.MethodPost, "/v1/params/resolve", map[string]any{"keys": []any{}}))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("empty keys should be 400, got %d", w.Code)
	}
}

func TestSnapshot(t *testing.T) {
	r := newInquiryRouter()
	w := httptest.NewRecorder()
	r.ServeHTTP(w, authReq(http.MethodGet, "/v1/params?category=rate", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var res struct {
		Data struct {
			Category string            `json:"category"`
			Count    int               `json:"count"`
			Params   []json.RawMessage `json:"params"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatalf("decode: %v (%s)", err, w.Body.String())
	}
	if res.Data.Category != "rate" || res.Data.Count != 2 || len(res.Data.Params) != 2 {
		t.Fatalf("unexpected snapshot: %+v", res.Data)
	}
}

func TestSnapshotRequiresCategory(t *testing.T) {
	r := newInquiryRouter()
	w := httptest.NewRecorder()
	r.ServeHTTP(w, authReq(http.MethodGet, "/v1/params", nil))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("missing category should be 400, got %d", w.Code)
	}
}

// keep pgtype import used (mirrors store param construction elsewhere).
var _ = pgtype.Text{}

func TestV1CategoriesDiscovery(t *testing.T) {
	r := newInquiryRouter()
	w := httptest.NewRecorder()
	r.ServeHTTP(w, authReq(http.MethodGet, "/v1/categories", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var res struct {
		Data []struct {
			Code        string          `json:"code"`
			ValueSchema json.RawMessage `json:"value_schema"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatalf("decode: %v (%s)", err, w.Body.String())
	}
	if len(res.Data) != 1 || res.Data[0].Code != "rate" {
		t.Fatalf("unexpected categories: %+v", res.Data)
	}
	if len(res.Data[0].ValueSchema) == 0 || string(res.Data[0].ValueSchema) == "null" {
		t.Errorf("discovery must surface value_schema; got %s", res.Data[0].ValueSchema)
	}
}
