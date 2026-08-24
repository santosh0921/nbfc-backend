package middleware

import "github.com/gin-gonic/gin"

// SecurityHeaders (Phase 7 item K) sets a conservative baseline of
// response headers on every request. This is a pure JSON API (no HTML
// templates are ever rendered by this backend — the admin panel and
// customer app are separate Flutter clients calling it over HTTPS), so
// the CSP is deliberately restrictive (default-src 'none') rather than
// tuned for a page that loads scripts/styles/images — there is no such
// page here to break.
func SecurityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("Referrer-Policy", "no-referrer")
		c.Header("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")
		c.Header("Permissions-Policy", "geolocation=(), microphone=(), camera=()")
		// Strict-Transport-Security only makes sense once the app is
		// actually served over HTTPS (true for any real deployment behind
		// a TLS-terminating load balancer/PaaS, false for local
		// http://localhost dev) — sent unconditionally since a browser
		// simply ignores it on a plain-HTTP connection rather than
		// erroring, so there is no dev-mode harm in always sending it.
		c.Header("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		c.Next()
	}
}
