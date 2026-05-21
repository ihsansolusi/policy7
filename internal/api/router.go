package api

import (
	"crypto/subtle"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/ihsansolusi/lib7-service-go/middleware"
	"github.com/ihsansolusi/lib7-service-go/token"
	"github.com/ihsansolusi/policy7/internal/service"
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

// SetupRoutes configures all the routes for the application
func SetupRoutes(r *gin.Engine, svc *service.ParameterService, adminSvc *service.AdminParameterService, tokenMaker token.Maker) {
	// Health check (no auth)
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status": "up",
		})
	})

	handler := NewParameterHandler(svc)
	adminHandler := NewAdminHandler(adminSvc)
	contractHandler := NewContractHandler()

	// Auth middleware applied to all /v1 and /admin/v1 endpoints:
	// bearer JWT (auth7-issued) OR X-Service-Key (BFF/M2M bypass).
	authMW := middleware.Auth(tokenMaker, serviceKeyValidator())

	v1 := r.Group("/v1")
	v1.Use(authMW)
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
	{
		adminV1.GET("/params", adminHandler.List)
		adminV1.GET("/params/:id", adminHandler.GetByID)
		adminV1.POST("/params", adminHandler.Create)
		adminV1.POST("/params/bulk-import", adminHandler.BulkImport)
		adminV1.PUT("/params/:id", adminHandler.Update)
		adminV1.DELETE("/params/:id", adminHandler.Delete)
		adminV1.GET("/params/:id/history", adminHandler.GetHistory)
	}
}
