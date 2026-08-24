package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

// visitorLimiter tracks a single client IP's token bucket plus when it was
// last seen, so idle entries can be swept from memory.
type visitorLimiter struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// ipRateLimiter is a simple in-memory, per-IP token-bucket limiter. Good
// enough for this single-instance demo deployment — no Redis needed.
type ipRateLimiter struct {
	mu       sync.Mutex
	visitors map[string]*visitorLimiter
	rps      rate.Limit
	burst    int
}

func newIPRateLimiter(rps rate.Limit, burst int) *ipRateLimiter {
	l := &ipRateLimiter{
		visitors: make(map[string]*visitorLimiter),
		rps:      rps,
		burst:    burst,
	}
	go l.cleanupLoop()
	return l
}

func (l *ipRateLimiter) getLimiter(ip string) *rate.Limiter {
	l.mu.Lock()
	defer l.mu.Unlock()

	v, exists := l.visitors[ip]
	if !exists {
		lim := rate.NewLimiter(l.rps, l.burst)
		l.visitors[ip] = &visitorLimiter{limiter: lim, lastSeen: time.Now()}
		return lim
	}
	v.lastSeen = time.Now()
	return v.limiter
}

// cleanupLoop periodically evicts visitors that haven't been seen in a
// while so the map doesn't grow unbounded over a long-running process.
func (l *ipRateLimiter) cleanupLoop() {
	for {
		time.Sleep(5 * time.Minute)
		l.mu.Lock()
		for ip, v := range l.visitors {
			if time.Since(v.lastSeen) > 15*time.Minute {
				delete(l.visitors, ip)
			}
		}
		l.mu.Unlock()
	}
}

// RateLimit returns a gin middleware allowing `rps` requests per second per
// client IP, with a `burst` allowance for short spikes. Intended for public,
// unauthenticated endpoints prone to abuse (OTP/login). Each call creates
// its OWN independent limiter instance/map — callers should hold one
// long-lived variable per distinct endpoint category (item E: "use
// endpoint-specific limits rather than one global number"), never share
// one RateLimit(...) value across unrelated routes.
func RateLimit(rps float64, burst int) gin.HandlerFunc {
	return NamedRateLimit("unlabeled", rps, burst)
}

// NamedRateLimit is RateLimit with a label attached to the audit event a
// violation generates (Phase 7 item E: "record rate-limit violations for
// security monitoring"). Every endpoint-specific limiter added in
// internal/routes uses this so a security review can distinguish "OTP
// abuse" from "export abuse" from "webhook abuse" in the audit log rather
// than one undifferentiated 429 count.
func NamedRateLimit(label string, rps float64, burst int) gin.HandlerFunc {
	limiter := newIPRateLimiter(rate.Limit(rps), burst)
	return func(c *gin.Context) {
		ip := c.ClientIP()
		if !limiter.getLimiter(ip).Allow() {
			// Aggregated rather than one synchronous DB insert per
			// rejected request — see rate_limit_audit.go for why that
			// was a DoS amplification vector.
			recordRateLimitViolation(c, label, ip, c.FullPath())
			c.JSON(http.StatusTooManyRequests, gin.H{
				"message": "Too many requests — please slow down and try again shortly",
			})
			c.Abort()
			return
		}
		c.Next()
	}
}
