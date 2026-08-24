package middleware

import (
	"log"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/santosh0921/nbfc-backend/internal/observability"
)

// RequestIDFrom returns the correlation id assigned by RequestID()
// (internal/middleware/request_id.go), or "" if that middleware is not
// installed — which is the case in a few narrow unit tests that build
// their own engine.
//
// NOTE: Phase 15's report listed request correlation IDs as MISSING.
// That was incorrect — RequestID() already existed and was already wired
// in cmd/api/main.go. Only the metrics layer below was actually missing,
// and this file adds just that rather than duplicating correlation.
func RequestIDFrom(c *gin.Context) string {
	if v, ok := c.Get("request_id"); ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// Observability (Phase 16 items T/U) records per-request metrics and
// emits one structured log line per request.
//
// The log line deliberately carries only method, path template, status,
// duration, and request id. It does NOT log the query string, request
// body, headers, or any identifier that could carry PII or a credential,
// per item U's prohibition. The path TEMPLATE (c.FullPath(), e.g.
// "/admin/loans/:id") is used rather than the raw URL so an id in the
// path is never written to logs, and so metrics group by route instead
// of exploding into one series per id.
func Observability() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		elapsed := time.Since(start)

		route := c.FullPath()
		if route == "" {
			// Unmatched route — group all 404s together rather than
			// creating a metric series per probed path, which would let
			// a scanner inflate this process's memory.
			route = "unmatched"
		}

		status := c.Writer.Status()
		observability.Inc(observability.MetricHTTPRequests)
		observability.ObserveLatency("http."+route, elapsed)

		switch {
		case status >= 500:
			observability.Inc(observability.MetricHTTPErrors5xx)
		case status == 401 || status == 403:
			observability.Inc(observability.MetricAuthFailures)
			observability.Inc(observability.MetricHTTPErrors4xx)
		case status == 429:
			observability.Inc(observability.MetricRateLimitHits)
			observability.Inc(observability.MetricHTTPErrors4xx)
		case status >= 400:
			observability.Inc(observability.MetricHTTPErrors4xx)
		}

		// Structured single-line log (item U). One line keeps it
		// greppable and cheap; a log pipeline can parse the key=value
		// pairs without a JSON encoder in the request hot path.
		log.Printf("level=info ts=%s request_id=%s method=%s route=%s status=%d duration_ms=%.2f",
			start.UTC().Format(time.RFC3339), RequestIDFrom(c),
			c.Request.Method, route, status, float64(elapsed.Nanoseconds())/1e6)
	}
}
