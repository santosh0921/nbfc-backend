package middleware

import "net/http"
import "github.com/gin-gonic/gin"

// MaxBodySize (Phase 7 item L: "maximum payload sizes") wraps the request
// body in the standard library's http.MaxBytesReader so a client can
// never force the server to read an unbounded body into memory before
// any handler-level validation (e.g. document upload's own explicit size
// check, internal/documents/handler.go) even runs — the read itself
// starts failing once the limit is exceeded. maxBytes comfortably covers
// the largest legitimate payload in this app today (a base64-encoded
// 10MB document upload, ~13.3MB encoded) with headroom, while still
// bounding worst-case per-request memory use.
func MaxBodySize(maxBytes int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes)
		c.Next()
	}
}
