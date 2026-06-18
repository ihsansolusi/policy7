package api

import (
	"crypto/subtle"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/ihsansolusi/lib7-service-go/audit7client"
	"github.com/ihsansolusi/lib7-service-go/metrics"
	"github.com/ihsansolusi/lib7-service-go/middleware"
	"github.com/ihsansolusi/lib7-service-go/token"
	"github.com/ihsansolusi/policy7/internal/service"
	"github.com/rs/zerolog"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
)

// serviceKeyValidator returns a constant-time matcher against SERVICE_KEY env.
// Empty env or X_SERVICE_KEY_DISABLED=true disables the bypass (all requests must use bearer token).
func serviceKeyValidator() func(string) bool {
	if os.Getenv("X_SERVICE_KEY_DISABLED") == "true" {
		return func(string) bool { return false }
	}
	configured := os.Getenv("SERVICE_KEY")
	if configured == "" {
		return func(string) bool { return false }
	}
	configuredBytes := []byte(configured)
	return func(key string) bool {
		return subtle.ConstantTimeCompare([]byte(key), configuredBytes) == 1
	}
}

// SetupRoutes configures the global middleware stack and all application routes.
//
// Middleware order: RequestID → Recovery → RequestSizeLimit → RequestLogger → Tracing → Metrics
// tracer and metricsReg may be nil (middleware is skipped when nil — useful in tests).
func SetupRoutes(
	r *gin.Engine,
	svc *service.ParameterService,
	adminSvc *service.AdminParameterService,
	tokenMaker token.Maker,
	logger zerolog.Logger,
	tracer trace.Tracer,
	metricsReg *metrics.Registry,
) {
	// Normalize nil tracer to noop so handler constructors always get a valid tracer.
	if tracer == nil {
		tracer = noop.NewTracerProvider().Tracer("policy7")
	}

	// Global middleware stack (service7-template spec §3.1 order)
	r.Use(middleware.RequestID())
	r.Use(middleware.Recovery(logger))
	r.Use(middleware.RequestSizeLimit(1 << 20)) // 1 MB max body
	r.Use(middleware.RequestLogger(logger))
	r.Use(middleware.Tracing(tracer))
	if metricsReg != nil {
		r.Use(middleware.Metrics(metricsReg))
	}
	// No SecurityHeaders/CORS — policy7 is an internal-only service.

	// Health check (no auth)
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status": "up",
		})
	})

	// audit7 forwarder for policy mutations (system of record). nil when
	// AUDIT7_URL is unset/placeholder → forwarding is a safe no-op. A nats://
	// URL publishes durably to audit7's JetStream ingest; http:// is legacy.
	audit7Client := audit7client.New(os.Getenv("AUDIT7_URL"), logger)

	handler := NewParameterHandler(svc, tracer, logger)
	adminHandler := NewAdminHandler(adminSvc, tracer, logger, audit7Client)
	categoryHandler := NewCategoryHandler(adminSvc, tracer, logger, audit7Client)
	contractHandler := NewContractHandler(tracer, logger)

	// Auth middleware applied to all /v1 and /admin/v1 endpoints:
	// bearer JWT (auth7-issued) OR X-Service-Key (BFF/M2M bypass).
	authMW := middleware.Auth(tokenMaker, serviceKeyValidator())

	v1 := r.Group("/v1")
	v1.Use(authMW)
	v1.Use(middleware.RequireDelegatedOrM2M())
	{
		// Basic REST API for parameters
		v1.GET("/params/:category/:name", handler.GetParameter)
		v1.GET("/params/:category/:name/effective", handler.GetEffectiveParameter)
		v1.POST("/params/transaction_limit/validate", handler.ValidateTransactionLimit)
		v1.GET("/params/approval-thresholds", handler.GetApprovalThresholds)
		v1.GET("/params/operational-hours", handler.GetOperationalHours)
		v1.GET("/params/product-access", handler.GetProductAccess)

		// Plan 04 Category Endpoints
		v1.GET("/params/rates/:product", handler.GetRates)
		v1.GET("/params/fees/:product", handler.GetFees)
		v1.GET("/params/regulatory/:type", handler.GetRegulatory)
		v1.POST("/params/regulatory/:type/check", handler.CheckRegulatory)
		v1.POST("/params/authorization_limit/check", handler.CheckAuthorizationLimit)
		v1.GET("/contracts/categories", contractHandler.Categories)
		v1.GET("/contracts/caller-context", contractHandler.CallerContext)
		v1.GET("/contracts/errors", contractHandler.Errors)
	}

	adminV1 := r.Group("/admin/v1")
	adminV1.Use(authMW)
	adminV1.Use(middleware.RequireDelegatedOrM2M())
	{
		adminV1.GET("/params", adminHandler.List)
		adminV1.GET("/params/:id", adminHandler.GetByID)
		adminV1.POST("/params", adminHandler.Create)
		adminV1.POST("/params/bulk-import", adminHandler.BulkImport)
		adminV1.POST("/params/query", adminHandler.ParamsQuery)
		adminV1.PUT("/params/:id", adminHandler.Update)
		adminV1.DELETE("/params/:id", adminHandler.Delete)
		adminV1.GET("/params/:id/history", adminHandler.GetHistory)

		// Wave C — category metadata (value_schema + x-ui/x-rules).
		// GET endpoints drive the dynamic value-form renderer; CRUD lets
		// admins manage categories directly (workflow approval out of scope).
		adminV1.GET("/categories", categoryHandler.List)
		adminV1.GET("/categories/:code", categoryHandler.GetByCode)
		adminV1.POST("/categories", categoryHandler.Create)
		adminV1.PUT("/categories/:code", categoryHandler.Update)
		adminV1.DELETE("/categories/:code", categoryHandler.Delete)

		// Workflow7 approval callbacks — restricted to M2M callers (workflow7)
		// and require a valid audit signature. Middleware applied at GROUP level.
		paramsWf := adminV1.Group("/params")
		paramsWf.Use(middleware.RequireM2M())
		paramsWf.Use(middleware.VerifyAuditSignatureFromEnv())
		{
			paramsWf.POST("/wf-create", adminHandler.WfCreate)
			paramsWf.PUT("/:id/wf-update", adminHandler.WfUpdate)
			paramsWf.POST("/:id/wf-delete", adminHandler.WfDelete)
		}

		// Workflow7 approval callbacks for parameter categories — same M2M +
		// audit-signature group middleware as the param callbacks (#576).
		categoriesWf := adminV1.Group("/categories")
		categoriesWf.Use(middleware.RequireM2M())
		categoriesWf.Use(middleware.VerifyAuditSignatureFromEnv())
		{
			categoriesWf.POST("/wf-create", categoryHandler.WfCreate)
			categoriesWf.PUT("/:code/wf-update", categoryHandler.WfUpdate)
			categoriesWf.POST("/:code/wf-delete", categoryHandler.WfDelete)
		}
	}
}
