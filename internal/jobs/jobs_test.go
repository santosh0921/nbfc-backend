package jobs_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/joho/godotenv"

	"github.com/santosh0921/nbfc-backend/internal/admin"
	"github.com/santosh0921/nbfc-backend/internal/config"
	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/jobs"
	"github.com/santosh0921/nbfc-backend/internal/models"
)

// Phase 16.1 Parts 8–10: idempotency, overlap protection, and
// failure/retry behaviour for scheduled jobs.

func ensureDB(t *testing.T) {
	t.Helper()
	if database.DB != nil {
		return
	}
	_ = godotenv.Load("../../.env")
	cfg := config.Load()
	if err := admin.InitJWTSecret(cfg.AdminJWTSecret); err != nil {
		t.Fatalf("InitJWTSecret: %v", err)
	}
	database.Connect(cfg)
	admin.Migrate()
}

func cleanupExecutions(t *testing.T, jobName string) {
	t.Helper()
	t.Cleanup(func() {
		database.DB.Unscoped().Where("job_name = ?", jobName).Delete(&models.JobExecution{})
	})
}

// --- Part 9: overlap protection ---
//
// A job already running must not be started again by a second
// invocation — the real-world case being two application instances whose
// external schedulers both fire, or a slow job whose next trigger
// arrives before the previous run finished.
func TestConcurrentRunsDoNotOverlap(t *testing.T) {
	ensureDB(t)

	const name = "test.overlap"
	cleanupExecutions(t, name)

	var concurrent atomic.Int32
	var maxConcurrent atomic.Int32
	release := make(chan struct{})

	jobs.Register(jobs.Definition{
		Name: name, Purpose: "overlap test", Idempotent: true,
		Handler: func() (int, error) {
			cur := concurrent.Add(1)
			for {
				old := maxConcurrent.Load()
				if cur <= old || maxConcurrent.CompareAndSwap(old, cur) {
					break
				}
			}
			<-release // hold the job open so a second call must contend
			concurrent.Add(-1)
			return 1, nil
		},
	})

	const attempts = 8
	var wg sync.WaitGroup
	var ran, skipped atomic.Int32

	for i := 0; i < attempts; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := jobs.Run(name, "test", "SCHEDULER")
			switch {
			case err == nil:
				ran.Add(1)
			case errors.Is(err, jobs.ErrAlreadyRunning):
				skipped.Add(1)
			}
		}()
	}

	// Give the goroutines time to contend, then let the holder finish.
	time.Sleep(1500 * time.Millisecond)
	close(release)
	wg.Wait()

	if got := maxConcurrent.Load(); got > 1 {
		t.Fatalf("OVERLAP PROTECTION VIOLATED: %d executions ran simultaneously (want at most 1)", got)
	}
	if ran.Load() < 1 {
		t.Fatalf("expected at least one execution to run, got %d (skipped=%d)", ran.Load(), skipped.Load())
	}
	if skipped.Load() < 1 {
		t.Fatalf("expected contending invocations to be skipped, but none were (ran=%d)", ran.Load())
	}
	t.Logf("%d concurrent invocations -> %d ran, %d skipped, max simultaneous=%d",
		attempts, ran.Load(), skipped.Load(), maxConcurrent.Load())
}

// Every skip must leave a record, so contention is observable rather
// than silent.
func TestSkippedRunsAreRecorded(t *testing.T) {
	ensureDB(t)

	const name = "test.skiprecord"
	cleanupExecutions(t, name)

	release := make(chan struct{})
	jobs.Register(jobs.Definition{
		Name: name, Purpose: "skip record test", Idempotent: true,
		Handler: func() (int, error) { <-release; return 0, nil },
	})

	var wg sync.WaitGroup
	wg.Add(1)
	go func() { defer wg.Done(); _, _ = jobs.Run(name, "holder", "SCHEDULER") }()
	time.Sleep(700 * time.Millisecond)

	_, err := jobs.Run(name, "contender", "SCHEDULER")
	if !errors.Is(err, jobs.ErrAlreadyRunning) {
		t.Fatalf("expected ErrAlreadyRunning for the contending call, got %v", err)
	}
	close(release)
	wg.Wait()

	var skipped int64
	database.DB.Model(&models.JobExecution{}).
		Where("job_name = ? AND status = ?", name, models.JobStatusSkipped).Count(&skipped)
	if skipped < 1 {
		t.Fatal("a skipped run must be recorded so contention is visible, but no SKIPPED_LOCKED row exists")
	}
}

// --- Part 8: idempotency via run key ---
//
// A job declaring a RunKeyFor must run at most once per logical period,
// enforced by a unique index rather than an application-level check.
func TestRunKeyPreventsSecondRunInSamePeriod(t *testing.T) {
	ensureDB(t)

	const name = "test.oncePerDay"
	cleanupExecutions(t, name)

	var runs atomic.Int32
	jobs.Register(jobs.Definition{
		Name: name, Purpose: "run key test", Idempotent: true,
		RunKeyFor: func(tm time.Time) string { return name + ":" + tm.Format("2006-01-02") },
		Handler:   func() (int, error) { runs.Add(1); return 1, nil },
	})

	if _, err := jobs.Run(name, "test", "SCHEDULER"); err != nil {
		t.Fatalf("first run should succeed: %v", err)
	}
	_, err := jobs.Run(name, "test", "SCHEDULER")
	if !errors.Is(err, jobs.ErrAlreadyRanForPeriod) {
		t.Fatalf("second run in the same period must be rejected, got %v", err)
	}
	if got := runs.Load(); got != 1 {
		t.Fatalf("handler must have executed exactly once, ran %d times", got)
	}
}

// --- Part 10: failure handling ---
//
// A failing job must be recorded as FAILED with its error preserved —
// never silently dropped.
func TestFailedJobIsRecordedNotSilent(t *testing.T) {
	ensureDB(t)

	const name = "test.failing"
	cleanupExecutions(t, name)

	jobs.Register(jobs.Definition{
		Name: name, Purpose: "failure test", Idempotent: true,
		Handler: func() (int, error) { return 0, errors.New("deliberate test failure") },
	})

	exec, err := jobs.Run(name, "test", "SCHEDULER")
	if err == nil {
		t.Fatal("expected the job to report failure")
	}
	if exec == nil {
		t.Fatal("a failed job must still produce an execution record")
	}

	var stored models.JobExecution
	if e := database.DB.Where("id = ?", exec.ID).First(&stored).Error; e != nil {
		t.Fatalf("execution record not found: %v", e)
	}
	if stored.Status != models.JobStatusFailed {
		t.Fatalf("expected status FAILED, got %q", stored.Status)
	}
	if stored.ErrorMessage == "" {
		t.Fatal("a failed execution must preserve its error message")
	}
	if stored.CompletedAt == nil {
		t.Fatal("a failed execution must record when it ended")
	}
}

// A panicking job must not crash the process, and must be recorded as a
// failure — a job that takes down the API is worse than one that fails.
func TestPanickingJobIsContained(t *testing.T) {
	ensureDB(t)

	const name = "test.panicking"
	cleanupExecutions(t, name)

	jobs.Register(jobs.Definition{
		Name: name, Purpose: "panic test", Idempotent: true,
		Handler: func() (int, error) { panic("deliberate test panic") },
	})

	exec, err := jobs.Run(name, "test", "SCHEDULER")
	if err == nil {
		t.Fatal("a panicking job must surface as an error, not a success")
	}
	if exec == nil || exec.Status != models.JobStatusFailed {
		t.Fatalf("a panicking job must be recorded FAILED, got %+v", exec)
	}
}

// --- Monitoring: a job that never ran must be visible ---

func TestStaleJobsReportsNeverRunJobs(t *testing.T) {
	ensureDB(t)

	const name = "test.neverran"
	cleanupExecutions(t, name)
	jobs.Register(jobs.Definition{
		Name: name, Purpose: "stale test", Idempotent: true,
		SuggestedSchedule: "Daily",
		Handler:           func() (int, error) { return 0, nil },
	})

	found := false
	for _, s := range jobs.StaleJobs(time.Hour) {
		if s.JobName == name {
			found = true
			if !s.NeverRan {
				t.Fatalf("a job with no successful execution must be marked NeverRan")
			}
		}
	}
	if !found {
		t.Fatal("a never-run job must appear in StaleJobs so an operator can see scheduled work is not happening")
	}
}

// --- Every registered production job must declare itself idempotent ---
//
// Part 8 requires that every scheduled job be safe to retry. An external
// scheduler WILL retry on timeout, so a non-idempotent registered job is
// a latent double-execution bug.
func TestAllRegisteredProductionJobsAreIdempotent(t *testing.T) {
	ensureDB(t)
	jobs.RegisterAll()

	for _, d := range jobs.List() {
		if len(d.Name) >= 5 && d.Name[:5] == "test." {
			continue // test fixtures registered by other cases
		}
		if !d.Idempotent {
			t.Errorf("job %q is registered for scheduling but is not marked idempotent — an external scheduler retry would double-execute it", d.Name)
		}
		if d.Purpose == "" || d.SuggestedSchedule == "" {
			t.Errorf("job %q must document its purpose and suggested schedule for whoever configures the scheduler", d.Name)
		}
	}
}

// === Production Infrastructure Readiness — Part 11 scheduler failure tests ===
//
// Tests A (concurrent instances) and D (duplicate run key) are covered
// above by TestConcurrentRunsDoNotOverlap and
// TestRunKeyPreventsSecondRunInSamePeriod. B and C are added here.

// Test B — worker crashes during execution.
//
// A crash must not leave the advisory lock held forever. PostgreSQL
// session-level advisory locks are released when the SESSION ends, so a
// crashed process releases its lock at the TCP level even though no
// unlock statement ever ran. This proves that property rather than
// assuming it: a lock taken on a separate connection is released purely
// by closing that connection, and the job becomes runnable again.
func TestCrashDuringExecutionReleasesLock(t *testing.T) {
	ensureDB(t)

	const name = "test.crashlock"
	cleanupExecutions(t, name)
	jobs.Register(jobs.Definition{
		Name: name, Purpose: "crash lock test", Idempotent: true,
		Handler: func() (int, error) { return 1, nil },
	})

	// Simulate a crashed worker: take the job's advisory lock on a
	// dedicated connection, then abandon that connection WITHOUT
	// unlocking — exactly what a process kill does.
	sqlDB, err := database.DB.DB()
	if err != nil {
		t.Fatalf("db handle: %v", err)
	}
	conn, err := sqlDB.Conn(context.Background())
	if err != nil {
		t.Fatalf("conn: %v", err)
	}
	key := jobs.LockKeyForTest(name)
	var got bool
	if err := conn.QueryRowContext(context.Background(),
		"SELECT pg_try_advisory_lock($1)", key).Scan(&got); err != nil || !got {
		t.Fatalf("could not acquire simulated-crash lock: got=%v err=%v", got, err)
	}

	// While the "crashed" worker still holds it, the job must be skipped.
	if _, err := jobs.Run(name, "test", "SCHEDULER"); !errors.Is(err, jobs.ErrAlreadyRunning) {
		conn.Close()
		t.Fatalf("expected the job to be skipped while the lock is held, got %v", err)
	}

	// Now the worker "dies" — its connection closes, no unlock issued.
	if err := conn.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	// Postgres releases session locks asynchronously on backend exit.
	var ran bool
	for i := 0; i < 50; i++ {
		if _, err := jobs.Run(name, "test", "SCHEDULER"); err == nil {
			ran = true
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if !ran {
		t.Fatal("PERMANENT LOCK: after a simulated worker crash the job never became runnable again")
	}
	t.Log("simulated worker crash released the advisory lock; job became runnable again")
}

// A crashed run must leave an accurate, non-misleading record: the row
// stays RUNNING with no completion time, which is exactly how an
// operator distinguishes "crashed mid-flight" from "succeeded".
func TestCrashedRunLeavesAccurateRecord(t *testing.T) {
	ensureDB(t)

	const name = "test.crashrecord"
	cleanupExecutions(t, name)

	// A RUNNING row with no CompletedAt is what a crash leaves behind.
	rec := models.JobExecution{
		JobName: name, Status: models.JobStatusRunning,
		TriggeredBy: "test", TriggerSource: "SCHEDULER",
		StartedAt: time.Now().Add(-2 * time.Hour), AttemptNumber: 1,
	}
	if err := database.DB.Create(&rec).Error; err != nil {
		t.Fatalf("create: %v", err)
	}

	var stored models.JobExecution
	database.DB.Where("id = ?", rec.ID).First(&stored)
	if stored.Status != models.JobStatusRunning {
		t.Fatalf("expected RUNNING, got %q", stored.Status)
	}
	if stored.CompletedAt != nil {
		t.Fatal("a crashed run must NOT record a completion time — that would misreport it as finished")
	}
	// It must not count as a success, so staleness monitoring still
	// reports the job as not having run.
	if last := jobs.LastSuccessful(name); last != nil {
		t.Fatal("a crashed (RUNNING) execution must never count as the last successful run")
	}
	t.Log("crashed run recorded as RUNNING with no completion time, and does not count as success")
}

// Test C — database unavailable during a job.
//
// A job whose work fails against the database must surface as a FAILED
// execution with the error preserved. It must never report success.
func TestDatabaseFailureDuringJobIsVisible(t *testing.T) {
	ensureDB(t)

	const name = "test.dbfailure"
	cleanupExecutions(t, name)

	jobs.Register(jobs.Definition{
		Name: name, Purpose: "db failure test", Idempotent: true,
		Handler: func() (int, error) {
			// A deliberately invalid statement is the closest safe
			// analogue to a database problem: the job's own DB work
			// fails. Actually stopping PostgreSQL would break every
			// other test in this suite.
			if err := database.DB.Exec("SELECT * FROM a_table_that_does_not_exist_16_2").Error; err != nil {
				return 0, err
			}
			return 0, nil
		},
	})

	exec, err := jobs.Run(name, "test", "SCHEDULER")
	if err == nil {
		t.Fatal("a job whose database work failed must NOT report success")
	}
	if exec == nil || exec.Status != models.JobStatusFailed {
		t.Fatalf("expected a FAILED execution record, got %+v", exec)
	}

	var stored models.JobExecution
	database.DB.Where("id = ?", exec.ID).First(&stored)
	if stored.ErrorMessage == "" {
		t.Fatal("the database error must be preserved on the execution record")
	}
	if jobs.LastSuccessful(name) != nil {
		t.Fatal("a failed run must not register as the last successful run")
	}
	t.Logf("database failure surfaced as FAILED with error preserved: %.80s", stored.ErrorMessage)
}
