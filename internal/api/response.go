package api

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type errorBody struct {
	Code       string      `json:"code"`
	Message    string      `json:"message"`
	HTTPStatus int         `json:"http_status"`
	Retryable  bool        `json:"retryable"`
	Details    interface{} `json:"details,omitempty"`
	TraceID    string      `json:"trace_id"`
}

func writeError(c *gin.Context, status int, code, message string, retryable bool, details interface{}) {
	traceID := c.GetHeader("X-Trace-ID")
	if traceID == "" {
		traceID = uuid.NewString()
	}
	c.JSON(status, gin.H{
		"success": false,
		"error": errorBody{
			Code:       code,
			Message:    message,
			HTTPStatus: status,
			Retryable:  retryable,
			Details:    details,
			TraceID:    traceID,
		},
	})
}

func writeSuccess(c *gin.Context, status int, data interface{}) {
	c.JSON(status, gin.H{
		"success": true,
		"data":    data,
	})
}
