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
// unauthenticated endpoints prone to abuse (OTP/login).
func RateLimit(rps float64, burst int) gin.HandlerFunc {
	limiter := newIPRateLimiter(rate.Limit(rps), burst)
	return func(c *gin.Context) {
		ip := c.ClientIP()
		if !limiter.getLimiter(ip).Allow() {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"message": "Too many requests — please slow down and try again shortly",
			})
			c.Abort()
			return
		}
		c.Next()
	}
}
