// Package health implements the Phase 7 item S health endpoints —
// /health, /readiness, /liveness. None of these expose credentials or
// internal infrastructure details (no connection strings, no hostnames,
// no stack traces) — only booleans and short status labels.
package health

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/santosh0921/nbfc-backend/internal/database"
)

// LivenessHandler — "is the process alive at all." Never touches the
// database or any external dependency: if this endpoint can't respond,
// nothing meaningful about health checking could help anyway (the
// process itself is the thing that's down). An orchestrator should use
// this to decide whether to kill/restart the container.
func LivenessHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "alive"})
}

// ReadinessHandler — "is the process ready to serve real traffic."
// Checks the database connection with a short, bounded timeout so a slow
// dependency degrades this endpoint's response time rather than hanging
// it indefinitely. An orchestrator should use this to decide whether to
// route traffic to this instance (not whether to restart it — that's
// liveness).
func ReadinessHandler(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
	defer cancel()

	checks := gin.H{}
	ready := true

	if database.DB == nil {
		checks["database"] = "not_initialized"
		ready = false
	} else {
		sqlDB, err := database.DB.DB()
		if err != nil {
			checks["database"] = "error"
			ready = false
		} else if err := sqlDB.PingContext(ctx); err != nil {
			checks["database"] = "unreachable"
			ready = false
		} else {
			checks["database"] = "ok"
		}
	}

	status := http.StatusOK
	if !ready {
		status = http.StatusServiceUnavailable
	}
	c.JSON(status, gin.H{"status": readyLabel(ready), "checks": checks})
}

// HealthHandler is a combined, slightly more verbose view for humans/
// dashboards — functionally readiness plus a timestamp, kept as a
// separate route per item S's explicit /health + /readiness + /liveness
// trio rather than collapsing them into one.
func HealthHandler(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
	defer cancel()

	dbStatus := "ok"
	ready := true
	if database.DB == nil {
		dbStatus, ready = "not_initialized", false
	} else if sqlDB, err := database.DB.DB(); err != nil {
		dbStatus, ready = "error", false
	} else if err := sqlDB.PingContext(ctx); err != nil {
		dbStatus, ready = "unreachable", false
	}

	status := http.StatusOK
	if !ready {
		status = http.StatusServiceUnavailable
	}
	c.JSON(status, gin.H{
		"status":    readyLabel(ready),
		"timestamp": time.Now().UTC(),
		"dependencies": gin.H{
			"database": dbStatus,
		},
	})
}

func readyLabel(ready bool) string {
	if ready {
		return "ok"
	}
	return "degraded"
}
