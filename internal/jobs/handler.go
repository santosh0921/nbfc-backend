package jobs

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/models"
)

func actor(c *gin.Context) string {
	if v, ok := c.Get("admin_username"); ok {
		if s, ok := v.(string); ok && s != "" {
			return s
		}
	}
	return "unknown"
}

// AdminListJobsHandler returns the job inventory (Part 7) — what exists,
// what it does, how often it should run, and whether it is safe to retry.
func AdminListJobsHandler(c *gin.Context) {
	type view struct {
		Name              string     `json:"name"`
		Purpose           string     `json:"purpose"`
		SuggestedSchedule string     `json:"suggestedSchedule"`
		Idempotent        bool       `json:"idempotent"`
		FinancialImpact   bool       `json:"financialImpact"`
		OncePerPeriod     bool       `json:"oncePerPeriod"`
		LastSuccessAt     *time.Time `json:"lastSuccessAt"`
	}
	out := make([]view, 0, len(registry))
	for _, d := range List() {
		v := view{
			Name: d.Name, Purpose: d.Purpose,
			SuggestedSchedule: d.SuggestedSchedule,
			Idempotent:        d.Idempotent, FinancialImpact: d.FinancialImpact,
			OncePerPeriod:     d.RunKeyFor != nil,
		}
		if last := LastSuccessful(d.Name); last != nil {
			t := last.StartedAt
			v.LastSuccessAt = &t
		}
		out = append(out, v)
	}
	c.JSON(http.StatusOK, gin.H{"jobs": out})
}

// AdminRunJobHandler is the endpoint an EXTERNAL scheduler calls.
//
// Status codes are chosen for schedulers, not humans:
//   202 — ran successfully
//   409 — another instance holds the lock, or already ran this period.
//         A scheduler must treat this as SUCCESS, not a failure to retry:
//         it means the work is being done, just not by this call.
//   500 — the job actually failed and needs attention.
func AdminRunJobHandler(c *gin.Context) {
	name := c.Param("name")
	source := c.DefaultQuery("source", "SCHEDULER")

	exec, err := Run(name, actor(c), source)
	switch {
	case errors.Is(err, ErrUnknownJob):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	case errors.Is(err, ErrAlreadyRunning), errors.Is(err, ErrAlreadyRanForPeriod):
		c.JSON(http.StatusConflict, gin.H{
			"status":    "skipped",
			"reason":    err.Error(),
			"guidance":  "This is not a failure. Another instance is running the job, or it already ran for this period. A scheduler should treat 409 as success.",
			"execution": exec,
		})
	case err != nil:
		c.JSON(http.StatusInternalServerError, gin.H{
			"status": "failed", "error": err.Error(), "execution": exec,
		})
	default:
		c.JSON(http.StatusAccepted, gin.H{"status": "succeeded", "execution": exec})
	}
}

// AdminJobExecutionsHandler returns recent executions, so a failed or
// never-run job is visible rather than silent.
func AdminJobExecutionsHandler(c *gin.Context) {
	limit := 100
	if v, err := strconv.Atoi(c.Query("limit")); err == nil && v > 0 && v <= 500 {
		limit = v
	}
	q := database.DB.Order("started_at DESC").Limit(limit)
	if name := c.Query("job"); name != "" {
		q = q.Where("job_name = ?", name)
	}
	if status := c.Query("status"); status != "" {
		q = q.Where("status = ?", status)
	}
	var rows []models.JobExecution
	q.Find(&rows)
	c.JSON(http.StatusOK, gin.H{"executions": rows})
}

// AdminStaleJobsHandler is the monitoring endpoint: it answers "is
// scheduled work actually happening?" A non-empty result means it is not.
func AdminStaleJobsHandler(c *gin.Context) {
	hours := 26 // a daily job is stale if it has not succeeded in ~a day
	if v, err := strconv.Atoi(c.Query("hours")); err == nil && v > 0 {
		hours = v
	}
	stale := StaleJobs(time.Duration(hours) * time.Hour)
	c.JSON(http.StatusOK, gin.H{
		"thresholdHours": hours,
		"staleJobs":      stale,
		"healthy":        len(stale) == 0,
		"note":           "A job listed here has not completed successfully within the threshold. If scheduling is not yet configured, every job will appear here — that is the expected state until an external scheduler is wired up.",
	})
}
