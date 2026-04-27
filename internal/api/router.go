package api

import (
	"github.com/gin-gonic/gin"
	"github.com/ihsansolusi/policy7/internal/service"
)

// SetupRoutes configures all the routes for the application
func SetupRoutes(r *gin.Engine, svc *service.ParameterService, adminSvc *service.AdminParameterService) {
	// Health check
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status": "up",
		})
	})

	handler := NewParameterHandler(svc)
	adminHandler := NewAdminHandler(adminSvc)

	v1 := r.Group("/v1")
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
	}

	adminV1 := r.Group("/admin/v1")
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
