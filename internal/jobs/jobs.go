// Package jobs provides safe execution of scheduled background work
// (Phase 16.1 Parts 6–10).
//
// ARCHITECTURAL DECISION — NO IN-PROCESS SCHEDULER.
//
// This package contains no time.NewTicker, no time.AfterFunc, and no
// goroutine loop that fires work on a timer. That is deliberate and is
// the central design constraint of this phase: if the application runs
// on N instances, an in-process timer fires N times, and a financial job
// running N times is exactly the failure mode this system must not have.
//
// Instead, scheduling is EXTERNAL (a cloud scheduler, an orchestrator
// CronJob, a systemd timer, Windows Task Scheduler, or managed cron),
// and this package makes each invocation safe regardless of how many
// schedulers or instances fire simultaneously:
//
//	1. DISTRIBUTED LOCK — a PostgreSQL session-level advisory lock keyed
//	   on the job name. The lock lives in the database, so it is shared
//	   across every application instance. Two concurrent invocations
//	   cannot both execute; the loser records SKIPPED_LOCKED and returns.
//	2. EXECUTION RECORD — every attempt writes a models.JobExecution row,
//	   so a job that failed or never ran is visible rather than silent.
//	3. OPTIONAL RUN KEY — a job that must run at most once per logical
//	   period supplies a RunKey; the unique index enforces that in the
//	   database rather than via a racy application check.
package jobs

import (
	"errors"
	"fmt"
	"hash/fnv"
	"time"

	"gorm.io/gorm"

	"github.com/santosh0921/nbfc-backend/internal/audit"
	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/models"
	"github.com/santosh0921/nbfc-backend/internal/observability"
)

var (
	ErrUnknownJob          = errors.New("unknown job")
	ErrAlreadyRunning      = errors.New("job is already running on another instance or in another session")
	ErrAlreadyRanForPeriod = errors.New("job has already run for this period")
)

// Handler performs the job's work and returns how many items it
// processed. Returning an error marks the execution FAILED.
type Handler func() (itemsProcessed int, err error)

// Definition describes one schedulable job.
type Definition struct {
	Name    string
	Purpose string
	// SuggestedSchedule is documentation for whoever configures the
	// external scheduler — this package never acts on it.
	SuggestedSchedule string

	// Idempotent records whether running twice is harmless. Every job
	// registered here must be idempotent; the field exists so the
	// property is stated explicitly and can be asserted in tests.
	Idempotent bool
	// FinancialImpact marks jobs that touch money or financial state.
	// These get more conservative handling and are always audited.
	FinancialImpact bool

	// RunKeyFor returns an idempotency key for a given moment, or ""
	// when the job may run any number of times per period. When set, a
	// second run with the same key is rejected by a unique index.
	RunKeyFor func(t time.Time) string

	Handler Handler
}

var registry = map[string]Definition{}

// Register adds a job definition. Called from RegisterAll below.
func Register(d Definition) {
	registry[d.Name] = d
}

func Get(name string) (Definition, bool) {
	d, ok := registry[name]
	return d, ok
}

// List returns every registered job, for the inventory endpoint.
func List() []Definition {
	out := make([]Definition, 0, len(registry))
	for _, d := range registry {
		out = append(out, d)
	}
	return out
}

// lockKey converts a job name into the bigint an advisory lock needs.
// FNV-1a is deterministic across processes and restarts, which is what
// makes the lock shared rather than per-instance.
func lockKey(name string) int64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte("nbfc.job:" + name))
	// Mask to positive range — advisory lock keys are signed.
	return int64(h.Sum64() & 0x7FFFFFFFFFFFFFFF)
}

// Run executes a job with overlap protection, an execution record, and
// audit logging.
//
// OVERLAP BEHAVIOUR: SKIP. If another invocation holds the lock, this
// one records SKIPPED_LOCKED and returns ErrAlreadyRunning rather than
// queueing or waiting. For daily financial jobs, skipping is correct —
// the next scheduled fire will pick the work up, whereas queueing would
// stack overlapping runs of an already-slow job.
func Run(jobName, triggeredBy, triggerSource string) (*models.JobExecution, error) {
	def, ok := registry[jobName]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrUnknownJob, jobName)
	}
	if triggerSource == "" {
		triggerSource = "MANUAL"
	}

	now := time.Now()
	exec := models.JobExecution{
		JobName: jobName, Status: models.JobStatusRunning,
		TriggeredBy: triggeredBy, TriggerSource: triggerSource,
		StartedAt: now, AttemptNumber: 1,
	}
	if def.RunKeyFor != nil {
		if k := def.RunKeyFor(now); k != "" {
			exec.RunKey = &k
		}
	}

	// A dedicated session is required: advisory locks taken with
	// pg_advisory_lock are held by the SESSION, so the lock and the work
	// must share one connection. GORM's pool would otherwise hand the
	// unlock to a different connection and the lock would leak until
	// that session ended.
	sqlDB, err := database.DB.DB()
	if err != nil {
		return nil, err
	}
	conn, err := sqlDB.Conn(dbCtx())
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	key := lockKey(jobName)
	var got bool
	if err := conn.QueryRowContext(dbCtx(), "SELECT pg_try_advisory_lock($1)", key).Scan(&got); err != nil {
		return nil, err
	}
	if !got {
		// Another instance is running this job. Record the skip so the
		// operator can see contention, then return.
		exec.Status = models.JobStatusSkipped
		// nil, not "": a skip must never occupy the unique run-key slot,
		// and NULL allows unlimited skip rows to be recorded.
		exec.RunKey = nil
		completed := time.Now()
		exec.CompletedAt = &completed
		database.DB.Create(&exec)
		return &exec, ErrAlreadyRunning
	}
	defer func() {
		_, _ = conn.ExecContext(dbCtx(), "SELECT pg_advisory_unlock($1)", key)
	}()

	// Create the RUNNING row. If a RunKey is set and already used, the
	// unique index rejects it — the database, not application logic,
	// enforces once-per-period.
	if err := database.DB.Create(&exec).Error; err != nil {
		if exec.RunKey != nil {
			return nil, fmt.Errorf("%w: %s (run key %s)", ErrAlreadyRanForPeriod, jobName, *exec.RunKey)
		}
		return nil, err
	}

	items, runErr := safeRun(def)

	completed := time.Now()
	exec.CompletedAt = &completed
	exec.DurationMs = completed.Sub(exec.StartedAt).Milliseconds()
	exec.ItemsProcessed = items

	if runErr != nil {
		exec.Status = models.JobStatusFailed
		exec.ErrorMessage = runErr.Error()
		observability.Inc(observability.MetricJobFailures)
	} else {
		exec.Status = models.JobStatusSucceeded
	}
	database.DB.Model(&models.JobExecution{}).Where("id = ?", exec.ID).Updates(map[string]interface{}{
		"status": exec.Status, "completed_at": exec.CompletedAt,
		"duration_ms": exec.DurationMs, "items_processed": exec.ItemsProcessed,
		"error_message": exec.ErrorMessage,
	})

	// Financial jobs are always audited, successful or not.
	if def.FinancialImpact || runErr != nil {
		result := audit.ResultSuccess
		detail := fmt.Sprintf("job=%s items=%d duration_ms=%d", jobName, items, exec.DurationMs)
		if runErr != nil {
			result = audit.ResultFailure
			detail += " error=" + runErr.Error()
		}
		audit.RecordDetailed(nil, audit.Entry{
			ActorType: "system", ActorRef: triggeredBy,
			Action: "job.executed", TargetType: "job", TargetID: jobName,
			Result: result, Details: detail,
		})
	}

	return &exec, runErr
}

// safeRun converts a panic inside a job into an error, so one broken job
// cannot take down the process that is running it.
func safeRun(def Definition) (items int, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("job panicked: %v", r)
		}
	}()
	return def.Handler()
}

// LastSuccessful returns the most recent successful execution of a job,
// or nil. This is what a monitoring check should read to answer "has
// this job run recently enough?"
func LastSuccessful(jobName string) *models.JobExecution {
	var e models.JobExecution
	err := database.DB.Where("job_name = ? AND status = ?", jobName, models.JobStatusSucceeded).
		Order("started_at DESC").First(&e).Error
	if errors.Is(err, gorm.ErrRecordNotFound) || err != nil {
		return nil
	}
	return &e
}

// StaleJob reports a job whose last success is older than maxAge, or
// that has never succeeded — the condition an alert should fire on.
type StaleJob struct {
	JobName           string     `json:"jobName"`
	LastSuccessAt     *time.Time `json:"lastSuccessAt"`
	NeverRan          bool       `json:"neverRan"`
	SuggestedSchedule string     `json:"suggestedSchedule"`
	FinancialImpact   bool       `json:"financialImpact"`
}

// StaleJobs returns jobs that have not succeeded within maxAge. An
// external monitor polls this; a non-empty result means scheduled work
// is not happening.
func StaleJobs(maxAge time.Duration) []StaleJob {
	var out []StaleJob
	cutoff := time.Now().Add(-maxAge)
	for _, d := range registry {
		last := LastSuccessful(d.Name)
		switch {
		case last == nil:
			out = append(out, StaleJob{JobName: d.Name, NeverRan: true,
				SuggestedSchedule: d.SuggestedSchedule, FinancialImpact: d.FinancialImpact})
		case last.StartedAt.Before(cutoff):
			t := last.StartedAt
			out = append(out, StaleJob{JobName: d.Name, LastSuccessAt: &t,
				SuggestedSchedule: d.SuggestedSchedule, FinancialImpact: d.FinancialImpact})
		}
	}
	return out
}

// LockKeyForTest exposes the advisory-lock key derivation so a test can
// simulate a crashed worker holding the same lock. Test-only accessor;
// the derivation itself stays unexported.
func LockKeyForTest(jobName string) int64 { return lockKey(jobName) }
