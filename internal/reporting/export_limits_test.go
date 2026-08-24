package reporting_test

import (
	"errors"
	"os"
	"testing"
	"time"

	"github.com/santosh0921/nbfc-backend/internal/reporting"
)

// Phase 16 item AK — export resource limits. These close the Phase 15
// finding that exports had no size, range, concurrency, or execution
// caps.

func TestExportRowLimitEnforced(t *testing.T) {
	l := reporting.CurrentExportLimits()

	if err := reporting.ValidateExportRows(l.MaxRows); err != nil {
		t.Fatalf("a row count exactly at the limit must be allowed: %v", err)
	}
	err := reporting.ValidateExportRows(l.MaxRows + 1)
	if !errors.Is(err, reporting.ErrExportTooManyRows) {
		t.Fatalf("expected ErrExportTooManyRows one row over the limit, got %v", err)
	}
}

func TestExportDateRangeLimitEnforced(t *testing.T) {
	l := reporting.CurrentExportLimits()
	to := time.Now()

	within := to.AddDate(0, 0, -(l.MaxRangeDays - 1))
	if err := reporting.ValidateExportRange(within, to); err != nil {
		t.Fatalf("a range inside the limit must be allowed: %v", err)
	}

	tooWide := to.AddDate(0, 0, -(l.MaxRangeDays + 10))
	if err := reporting.ValidateExportRange(tooWide, to); !errors.Is(err, reporting.ErrExportRangeTooWide) {
		t.Fatalf("expected ErrExportRangeTooWide for an over-limit span, got %v", err)
	}

	// A report type with no date bounds must not be rejected by the
	// range check — the row cap still protects it.
	if err := reporting.ValidateExportRange(time.Time{}, time.Time{}); err != nil {
		t.Fatalf("an unbounded-by-date report must pass the range check: %v", err)
	}
}

func TestExportFileSizeLimitEnforced(t *testing.T) {
	l := reporting.CurrentExportLimits()
	if err := reporting.ValidateExportSize(l.MaxFileSizeBytes); err != nil {
		t.Fatalf("a file exactly at the size limit must be allowed: %v", err)
	}
	if err := reporting.ValidateExportSize(l.MaxFileSizeBytes + 1); !errors.Is(err, reporting.ErrExportTooLarge) {
		t.Fatalf("expected ErrExportTooLarge one byte over, got %v", err)
	}
}

// Limits must be configurable rather than hard-coded (item AK).
func TestExportLimitsAreConfigurable(t *testing.T) {
	t.Setenv("EXPORT_MAX_ROWS", "10")
	if got := reporting.CurrentExportLimits().MaxRows; got != 10 {
		t.Fatalf("EXPORT_MAX_ROWS should override the default, got %d", got)
	}
	if err := reporting.ValidateExportRows(11); !errors.Is(err, reporting.ErrExportTooManyRows) {
		t.Fatalf("configured limit must actually be enforced, got %v", err)
	}
	os.Unsetenv("EXPORT_MAX_ROWS")
}

// An over-limit export must be refused at request time, not queued and
// silently failed later.
func TestOversizedExportRefusedAtRequestTime(t *testing.T) {
	t.Setenv("EXPORT_MAX_ROWS", "5")

	rows := make([][]string, 50)
	for i := range rows {
		rows[i] = []string{"a", "b"}
	}

	_, err := reporting.RequestExport("TEST_REPORT", "CSV", "", 0, "phase16-test",
		[]string{"c1", "c2"}, rows)
	if !errors.Is(err, reporting.ErrExportTooManyRows) {
		t.Fatalf("an over-limit export must be refused up front, got %v", err)
	}
}
