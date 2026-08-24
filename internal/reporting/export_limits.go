// Export resource limits (Phase 16 item AK) — closing the Phase 15
// finding that exports were asynchronous (good) but had NO caps on rows,
// date range, file size, concurrency, or execution time, so a single
// authorized admin could request an export spanning all time and consume
// the process's memory and a database connection for as long as it took.
//
// The Phase 9 rate limiter bounds export FREQUENCY; these bound export
// SIZE and COST, which is a different failure mode.
//
// Every limit is configurable via environment variable with a safe
// default, per item AK's requirement that they be configurable rather
// than hard-coded.
package reporting

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"sync"
	"time"
)

var (
	ErrExportTooManyRows   = errors.New("export exceeds the maximum allowed row count")
	ErrExportRangeTooWide  = errors.New("export date range exceeds the maximum allowed span")
	ErrExportTooLarge      = errors.New("generated export exceeds the maximum allowed file size")
	ErrTooManyConcurrent   = errors.New("too many exports are already running; retry shortly")
	ErrExportTimedOut      = errors.New("export exceeded the maximum allowed execution time")
)

// Defaults are deliberately generous enough for real reporting work and
// small enough to bound worst-case cost on a single instance.
const (
	defaultMaxRows          = 200_000
	defaultMaxRangeDays     = 400 // just over a financial year, so a full-FY export always fits
	defaultMaxFileSizeBytes = 100 << 20
	defaultMaxConcurrent    = 3
	defaultMaxExecution     = 5 * time.Minute
)

func envInt(name string, def int) int {
	if v := os.Getenv(name); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return def
}

// ExportLimits is the resolved limit set. Read per call rather than
// cached so an operator can change a limit by restarting with new
// configuration, without a code change.
type ExportLimits struct {
	MaxRows          int
	MaxRangeDays     int
	MaxFileSizeBytes int
	MaxConcurrent    int
	MaxExecution     time.Duration
}

func CurrentExportLimits() ExportLimits {
	return ExportLimits{
		MaxRows:          envInt("EXPORT_MAX_ROWS", defaultMaxRows),
		MaxRangeDays:     envInt("EXPORT_MAX_RANGE_DAYS", defaultMaxRangeDays),
		MaxFileSizeBytes: envInt("EXPORT_MAX_FILE_BYTES", defaultMaxFileSizeBytes),
		MaxConcurrent:    envInt("EXPORT_MAX_CONCURRENT", defaultMaxConcurrent),
		MaxExecution:     time.Duration(envInt("EXPORT_MAX_EXECUTION_SECONDS", int(defaultMaxExecution.Seconds()))) * time.Second,
	}
}

// ValidateExportRange rejects an excessive date span BEFORE any rows are
// built — the cheapest possible point to refuse, since it costs no
// database work at all.
func ValidateExportRange(from, to time.Time) error {
	l := CurrentExportLimits()
	if from.IsZero() || to.IsZero() {
		return nil // an unbounded-by-date report type; row cap still applies
	}
	if days := to.Sub(from).Hours() / 24; days > float64(l.MaxRangeDays) {
		return fmt.Errorf("%w: requested %.0f days, maximum is %d",
			ErrExportRangeTooWide, days, l.MaxRangeDays)
	}
	return nil
}

// ValidateExportRows rejects an oversized result set after building but
// before serialization, so the process never renders a workbook it
// cannot afford to hold in memory.
func ValidateExportRows(n int) error {
	l := CurrentExportLimits()
	if n > l.MaxRows {
		return fmt.Errorf("%w: %d rows, maximum is %d", ErrExportTooManyRows, n, l.MaxRows)
	}
	return nil
}

// --- Concurrency gate ---
//
// A counting semaphore rather than a queue: an export that cannot start
// now is refused with a clear error the caller can retry, which is
// better than silently queueing work the requester cannot see the status
// of. Per-process, matching the fact that exports run in-process.
var exportSlots = make(chan struct{}, defaultMaxConcurrent)
var exportSlotsOnce sync.Once

func acquireExportSlot() error {
	exportSlotsOnce.Do(func() {
		if n := CurrentExportLimits().MaxConcurrent; n != defaultMaxConcurrent {
			exportSlots = make(chan struct{}, n)
		}
	})
	select {
	case exportSlots <- struct{}{}:
		return nil
	default:
		return fmt.Errorf("%w: maximum is %d concurrent exports",
			ErrTooManyConcurrent, cap(exportSlots))
	}
}

func releaseExportSlot() {
	select {
	case <-exportSlots:
	default:
	}
}

// ValidateExportSize rejects a generated artifact that exceeds the file
// size cap, so an oversized file is never written to storage.
func ValidateExportSize(n int) error {
	l := CurrentExportLimits()
	if n > l.MaxFileSizeBytes {
		return fmt.Errorf("%w: %d bytes, maximum is %d", ErrExportTooLarge, n, l.MaxFileSizeBytes)
	}
	return nil
}
