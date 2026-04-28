package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// ValidServiceKeys mocks a database or config lookup for service authentication
var ValidServiceKeys = map[string]string{
	"auth7":            "key-auth7-dev",
	"core7-enterprise": "key-core7-dev",
	"workflow7":        "key-work7-dev",
	"notif7":           "key-notif7-dev",
	"test":             "test-key", // For testing
}

func ServiceAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		serviceID := c.GetHeader("X-Service-ID")
		apiKey := c.GetHeader("X-API-Key")

		if serviceID == "" || apiKey == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing service credentials"})
			return
		}

		validKey, exists := ValidServiceKeys[serviceID]
		if !exists || validKey != apiKey {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid service credentials"})
			return
		}

		c.Set("service_id", serviceID)
		c.Next()
	}
}
