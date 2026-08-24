package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// RequestID assigns a correlation ID to every request (reusing an
// incoming X-Request-Id if the caller already set one), exposes it on the
// response header, and stores it in the gin context so audit.RecordDetailed
// can tie an audit row back to the exact request that produced it.
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.GetHeader("X-Request-Id")
		if id == "" {
			id = uuid.NewString()
		}
		c.Set("request_id", id)
		c.Header("X-Request-Id", id)
		c.Next()
	}
}
