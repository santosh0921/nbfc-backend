package integrity

import (
	"net/http"
	"os"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/observability"
)

// AdminIntegrityCheckHandler runs every financial integrity check and
// returns the report.
//
// Returns HTTP 200 with passed=false rather than an error status when a
// check FAILS: the request itself succeeded, and a monitoring system
// should distinguish "the check ran and found a problem" from "the check
// could not run". The `passed` field and the severity counts are what an
// alert should key on, not the status code.
func AdminIntegrityCheckHandler(c *gin.Context) {
	c.JSON(http.StatusOK, RunAll())
}

// AdminMetricsHandler exposes the in-process metrics snapshot.
//
// Admin-authenticated deliberately. Metrics reveal traffic shape, error
// rates, and route names — useful to an attacker profiling the system —
// so this is not public, unlike /health and /liveness which expose only
// a liveness boolean.
func AdminMetricsHandler(c *gin.Context) {
	// Sample database pool state at read time rather than continuously.
	// These are the numbers an operator needs when diagnosing "requests
	// are queueing" — item T names DB connections explicitly — and
	// sampling on demand avoids a background goroutine whose only job is
	// to poll.
	if database.DB != nil {
		if sqlDB, err := database.DB.DB(); err == nil {
			s := sqlDB.Stats()
			observability.SetGauge("db.connections.open", float64(s.OpenConnections))
			observability.SetGauge("db.connections.in_use", float64(s.InUse))
			observability.SetGauge("db.connections.idle", float64(s.Idle))
			observability.SetGauge("db.connections.max_open", float64(s.MaxOpenConnections))
			// A rising wait count/duration is the signal that the pool is
			// the bottleneck rather than the database itself.
			observability.SetGauge("db.wait_count", float64(s.WaitCount))
			observability.SetGauge("db.wait_duration_ms", float64(s.WaitDuration.Milliseconds()))
		}
	}
	c.JSON(http.StatusOK, observability.Collect())
}

// AdminReconciliationHandler returns the per-record disbursement/ledger
// reconciliation (Phase 16.1 Part 11). `?rows=true` includes every row;
// the default returns totals only.
func AdminReconciliationHandler(c *gin.Context) {
	c.JSON(http.StatusOK, ReconcileDisbursementLedger(c.Query("rows") == "true"))
}

// AdminBackupStatusHandler exposes backup monitoring (Part 7).
// Thresholds come from BACKUP_MAX_AGE_HOURS / BACKUP_MIN_BYTES, and the
// monitored directory from BACKUP_DESTINATION — the same variable the
// backup script writes to, so monitoring cannot drift from the job.
func AdminBackupStatusHandler(c *gin.Context) {
	dir := os.Getenv("BACKUP_DESTINATION")
	maxAge, _ := strconv.ParseFloat(os.Getenv("BACKUP_MAX_AGE_HOURS"), 64)
	minBytes, _ := strconv.ParseInt(os.Getenv("BACKUP_MIN_BYTES"), 10, 64)
	c.JSON(http.StatusOK, BackupStatusFor(dir, maxAge, minBytes))
}

// AdminSyncStatusHandler exposes backup-database synchronization
// freshness (Phase 18 Step 11/12) — a separate mechanism from the
// file-based backup status above.
func AdminSyncStatusHandler(c *gin.Context) {
	dir := os.Getenv("SYNC_MANIFEST_DIR")
	maxAge, _ := strconv.ParseFloat(os.Getenv("SYNC_MAX_AGE_MINUTES"), 64)
	c.JSON(http.StatusOK, SyncStatusFor(dir, maxAge))
}
