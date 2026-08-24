package models

import "time"

// JobExecution records one attempt to run a scheduled job (Phase 16.1
// Parts 7–10).
//
// WHY THIS TABLE EXISTS: before Phase 16.1 a job left no trace that it
// ran. "Did the daily risk job execute last night?" was unanswerable,
// which means a silently-failing financial job could go unnoticed
// indefinitely. Every run now writes a row — including runs that were
// skipped because another instance held the lock, and runs that failed.
//
// A failed job must never silently disappear (item L / Part 10), so a
// FAILED row is a permanent record with its error text, not a log line
// that scrolls away.
const (
	JobStatusRunning   = "RUNNING"
	JobStatusSucceeded = "SUCCEEDED"
	JobStatusFailed    = "FAILED"
	// SKIPPED means another instance already held the job's lock. This
	// is normal, expected behaviour under horizontal scaling — not an
	// error — and is recorded distinctly so it never inflates failure
	// counts or triggers an alert.
	JobStatusSkipped = "SKIPPED_LOCKED"
)

type JobExecution struct {
	ID      uint   `gorm:"primaryKey" json:"jobExecutionId"`
	JobName string `gorm:"not null;index:idx_job_name_started" json:"jobName"`

	Status string `gorm:"not null;index" json:"status"`

	// TriggeredBy names what caused this run — an external scheduler's
	// identity, or the admin who invoked it manually. Auditable.
	TriggeredBy string `gorm:"not null" json:"triggeredBy"`
	// TriggerSource is "SCHEDULER", "MANUAL", or "CLI".
	TriggerSource string `gorm:"not null" json:"triggerSource"`

	StartedAt   time.Time  `gorm:"not null;index:idx_job_name_started" json:"startedAt"`
	CompletedAt *time.Time `json:"completedAt,omitempty"`
	DurationMs  int64      `json:"durationMs"`

	// ItemsProcessed is the job's own count of work done (rows updated,
	// alerts raised). Lets an operator spot a job that "succeeded" but
	// silently did nothing.
	ItemsProcessed int `json:"itemsProcessed"`

	ErrorMessage string `json:"errorMessage,omitempty"`
	// AttemptNumber supports retry accounting; 1 for a first attempt.
	AttemptNumber int `gorm:"not null;default:1" json:"attemptNumber"`

	// RunKey enforces idempotency for jobs that must run at most once
	// per logical period. Unique when set — a second attempt to run the
	// same job for the same period is rejected by the database, not by
	// an application-level "if already processed" check that could race.
	//
	// POINTER, NOT STRING, deliberately: PostgreSQL treats '' as a
	// value in a unique index but permits unlimited NULLs. With a plain
	// string, the FIRST row without a run key would occupy '' and every
	// subsequent keyless execution — including every SKIPPED_LOCKED
	// record — would be rejected as a duplicate. That would have made
	// job contention invisible, which is precisely what this table
	// exists to reveal. nil == "no key, unlimited rows allowed".
	RunKey *string `gorm:"uniqueIndex" json:"runKey,omitempty"`

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}
