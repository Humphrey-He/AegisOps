package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const TraceIDKey = "traceId"

func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		traceID := c.GetHeader("X-Request-Id")
		if traceID == "" {
			traceID = uuid.NewString()
		}
		c.Set(TraceIDKey, traceID)
		c.Header("X-Request-Id", traceID)
		c.Next()
	}
}

func TraceID(c *gin.Context) string {
	if traceID, ok := c.Get(TraceIDKey); ok {
		if value, ok := traceID.(string); ok {
			return value
		}
	}
	return ""
}
