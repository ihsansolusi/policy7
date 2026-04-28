package client

import (
	"context"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/ihsansolusi/policy7/internal/api"
	"github.com/ihsansolusi/policy7/internal/api/middleware"
	"github.com/ihsansolusi/policy7/internal/service"
	"github.com/ihsansolusi/policy7/internal/store"
	"github.com/jackc/pgx/v5/pgtype"
)

type mockQuerier struct {
	store.Querier
}

func (m *mockQuerier) GetParameter(ctx context.Context, arg store.GetParameterParams) (store.Parameter, error) {
	return store.Parameter{
		ID:       pgtype.UUID{Bytes: uuid.New(), Valid: true},
		OrgID:    arg.OrgID,
		Category: arg.Category,
		Name:     arg.Name,
		Value:    []byte(`{"limit": 100}`),
		Version:  1,
	}, nil
}

func TestClient_ValidateTransactionLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := &mockQuerier{}
	svc := service.NewParameterService(db, nil)

	r := gin.Default()
	r.Use(middleware.ServiceAuth())
	handler := api.NewParameterHandler(svc)
	r.POST("/v1/params/transaction_limit/validate", handler.ValidateTransactionLimit)

	server := httptest.NewServer(r)
	defer server.Close()

	client := NewClient(server.URL, "test", "test-key")

	req := ValidationRequest{
		Name:   "teller_transfer_max",
		Amount: 50,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	resp, err := client.ValidateTransactionLimit(ctx, uuid.New().String(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !resp.IsValid {
		t.Errorf("expected is_valid to be true")
	}
}
