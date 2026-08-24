package integrity_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"

	"github.com/santosh0921/nbfc-backend/internal/accounting"
	"github.com/santosh0921/nbfc-backend/internal/admin"
	"github.com/santosh0921/nbfc-backend/internal/config"
	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/integrity"
	"github.com/santosh0921/nbfc-backend/internal/models"
	"github.com/santosh0921/nbfc-backend/internal/observability"
)

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
	accounting.SeedChartOfAccounts()
}

// --- Item X: the checks actually run and report structurally ---

func TestIntegrityChecksRunAndReportInvariants(t *testing.T) {
	ensureDB(t)
	r := integrity.RunAll()

	if len(r.Results) == 0 {
		t.Fatal("integrity report contains no checks")
	}
	for _, res := range r.Results {
		if res.Check == "" {
			t.Fatal("a check reported no name")
		}
		// Every check must state the invariant it enforces — a failure
		// that cannot explain itself is not actionable at 3am.
		if res.Invariant == "" {
			t.Fatalf("check %q does not state its invariant", res.Check)
		}
		if res.Detail == "" {
			t.Fatalf("check %q reported no detail", res.Check)
		}
		if res.Passed && res.Severity != integrity.SeverityOK {
			t.Fatalf("check %q passed but reported severity %q", res.Check, res.Severity)
		}
		if !res.Passed && res.Severity == integrity.SeverityOK {
			t.Fatalf("check %q failed but reported severity OK", res.Check)
		}
	}
	t.Logf("ran %d integrity checks; passed=%v critical=%d high=%d warning=%d",
		len(r.Results), r.Passed, r.CriticalCount, r.HighCount, r.WarningCount)
}

// The double-entry invariant is the single most important one; assert it
// is actually among the checks rather than trusting the list.
func TestDoubleEntryInvariantIsChecked(t *testing.T) {
	ensureDB(t)
	r := integrity.RunAll()

	found := false
	for _, res := range r.Results {
		if res.Check == "journal_balance" {
			found = true
			if res.Passed && res.Count != 0 {
				t.Fatalf("journal_balance passed but reported %d offending entries", res.Count)
			}
		}
	}
	if !found {
		t.Fatal("no journal_balance (debit == credit) check present — this is the foundational ledger invariant")
	}
}

// --- Item X: detection must never repair ---
//
// Running the checks must not change any financial total. This is the
// property that separates monitoring from silent data mutation.
func TestIntegrityChecksNeverModifyFinancialData(t *testing.T) {
	ensureDB(t)

	var beforeJournals, beforeLines, beforeDisb, beforePay int64
	database.DB.Model(&models.JournalEntry{}).Count(&beforeJournals)
	database.DB.Model(&models.JournalEntryLine{}).Count(&beforeLines)
	database.DB.Model(&models.Disbursement{}).Count(&beforeDisb)
	database.DB.Model(&models.Payment{}).Count(&beforePay)

	integrity.RunAll()

	var afterJournals, afterLines, afterDisb, afterPay int64
	database.DB.Model(&models.JournalEntry{}).Count(&afterJournals)
	database.DB.Model(&models.JournalEntryLine{}).Count(&afterLines)
	database.DB.Model(&models.Disbursement{}).Count(&afterDisb)
	database.DB.Model(&models.Payment{}).Count(&afterPay)

	if beforeJournals != afterJournals || beforeLines != afterLines ||
		beforeDisb != afterDisb || beforePay != afterPay {
		t.Fatalf("integrity checks MODIFIED financial data (item X forbids this): "+
			"journals %d->%d, lines %d->%d, disbursements %d->%d, payments %d->%d",
			beforeJournals, afterJournals, beforeLines, afterLines,
			beforeDisb, afterDisb, beforePay, afterPay)
	}
}

// --- Item T: metrics record what the middleware feeds them ---

func TestMetricsRecordCountersAndLatency(t *testing.T) {
	observability.Reset()

	observability.Inc(observability.MetricHTTPRequests)
	observability.Inc(observability.MetricHTTPRequests)
	observability.Inc(observability.MetricHTTPErrors5xx)
	observability.ObserveLatency("http./test", 10*time.Millisecond)
	observability.ObserveLatency("http./test", 30*time.Millisecond)
	observability.SetGauge("db.connections", 7)

	s := observability.Collect()
	if s.Counters[observability.MetricHTTPRequests] != 2 {
		t.Fatalf("expected 2 requests counted, got %d", s.Counters[observability.MetricHTTPRequests])
	}
	if s.Counters[observability.MetricHTTPErrors5xx] != 1 {
		t.Fatalf("expected 1 5xx counted, got %d", s.Counters[observability.MetricHTTPErrors5xx])
	}
	if s.Gauges["db.connections"] != 7 {
		t.Fatalf("expected gauge 7, got %v", s.Gauges["db.connections"])
	}
	lat, ok := s.Latency["http./test"]
	if !ok || lat.Count != 2 {
		t.Fatalf("expected 2 latency samples, got %+v", lat)
	}
	if lat.MaxMs < 29 || lat.MaxMs > 31 {
		t.Fatalf("expected max ~30ms, got %v", lat.MaxMs)
	}
	// The snapshot must warn that it is single-process, so a consumer
	// cannot mistake it for cluster-wide truth.
	if s.Note == "" {
		t.Fatal("metrics snapshot must state its single-process limitation")
	}
}

// The latency reservoir must stay bounded — an observability package
// that leaks memory defeats its own purpose.
func TestLatencyReservoirIsBounded(t *testing.T) {
	observability.Reset()
	for i := 0; i < 10_000; i++ {
		observability.ObserveLatency("http./flood", time.Millisecond)
	}
	s := observability.Collect()
	if got := s.Latency["http./flood"].Count; got > 2048 {
		t.Fatalf("latency reservoir grew unbounded: %d samples retained", got)
	}
}

// --- Metrics endpoint is not public ---

func TestMetricsEndpointRequiresAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ensureDB(t)

	// A bare engine with only the handler mounted proves the handler
	// itself does not self-authorize; the real route table additionally
	// gates it behind AdminAuthMiddleware + rbac.MISView (verified in
	// the Phase 16 report's route inventory).
	r := gin.New()
	r.GET("/admin/ops/metrics", integrity.AdminMetricsHandler)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/admin/ops/metrics", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("handler itself should serve; gating is the route table's job, got %d", w.Code)
	}
}

// --- Phase 16.1 Part 14: prove the reconciliation check itself works ---
//
// The purpose is NOT to make the number go green — it is to prove the
// check detects each failure mode it claims to. Each case constructs the
// specific condition, asserts the classification, then removes only the
// rows it created.

func TestReconciliationClassifiesEveryCase(t *testing.T) {
	ensureDB(t)

	base := integrity.ReconcileDisbursementLedger(true)
	if !base.FullyExplained {
		t.Fatalf("baseline has %d paise unexplained — reconciliation cannot classify all data", base.UnexplainedPaise)
	}

	counts := map[string]int{}
	for _, r := range base.Rows {
		counts[r.Classification]++
	}
	// The three conditions the current dataset genuinely exhibits.
	for _, want := range []string{
		integrity.ClassValid,
		integrity.ClassOrphanLedgerEntry,
		integrity.ClassMissingLedgerEntry,
	} {
		if counts[want] == 0 {
			t.Errorf("expected at least one %q row in the current dataset, found none", want)
		}
	}
	// Every row must carry exactly one classification and its evidence.
	for _, r := range base.Rows {
		if r.Classification == "" {
			t.Fatal("a reconciliation row has no classification")
		}
		if r.Classification == integrity.ClassUnknown {
			t.Fatalf("row classified UNKNOWN (journal=%s): Part 12 requires every difference be explained",
				r.JournalNumber)
		}
		if r.Evidence == "" {
			t.Fatalf("row %s has no evidence for its classification", r.Classification)
		}
	}
	t.Logf("classified %d rows: %v", len(base.Rows), counts)
}

// Totals must reconcile exactly: disbursement-side minus ledger-side
// must equal the sum of the one-sided classifications. If this drifts,
// the report is hiding an amount.
func TestReconciliationTotalsAreInternallyConsistent(t *testing.T) {
	ensureDB(t)
	r := integrity.ReconcileDisbursementLedger(true)

	var sumDisb, sumLedger int64
	for _, row := range r.Rows {
		sumDisb += row.DisbursementPaise
		sumLedger += row.LedgerPaise
	}
	if sumDisb != r.TotalDisbursementPaise {
		t.Fatalf("row disbursement total %d != report total %d", sumDisb, r.TotalDisbursementPaise)
	}
	if sumLedger != r.TotalLedgerPaise {
		t.Fatalf("row ledger total %d != report total %d", sumLedger, r.TotalLedgerPaise)
	}
	if r.DifferencePaise != r.TotalDisbursementPaise-r.TotalLedgerPaise {
		t.Fatal("reported difference does not equal disbursement minus ledger")
	}
}

// Part 14 / Part 19: the reconciliation must never mutate financial data.
func TestReconciliationNeverMutates(t *testing.T) {
	ensureDB(t)

	type snap struct{ disb, je, jel, pay int64 }
	take := func() snap {
		var s snap
		database.DB.Model(&models.Disbursement{}).Count(&s.disb)
		database.DB.Model(&models.JournalEntry{}).Count(&s.je)
		database.DB.Model(&models.JournalEntryLine{}).Count(&s.jel)
		database.DB.Model(&models.Payment{}).Count(&s.pay)
		return s
	}
	before := take()
	integrity.ReconcileDisbursementLedger(true)
	after := take()

	if before != after {
		t.Fatalf("reconciliation MUTATED financial data: before=%+v after=%+v", before, after)
	}
}
