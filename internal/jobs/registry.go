package jobs

import (
	"context"
	"time"

	"github.com/santosh0921/nbfc-backend/internal/analytics"
	"github.com/santosh0921/nbfc-backend/internal/collections"
	"github.com/santosh0921/nbfc-backend/internal/compliance"
	"github.com/santosh0921/nbfc-backend/internal/integrity"
	"github.com/santosh0921/nbfc-backend/internal/risk"
)

func dbCtx() context.Context { return context.Background() }

// RegisterAll wires every job that Phase 16.1 Part 7 identified as
// needing scheduled execution.
//
// SELECTION CRITERIA — a job appears here only if it is:
//   * currently reachable ONLY by a human clicking an admin endpoint,
//   * genuinely periodic (its value decays if it does not run), and
//   * idempotent, so an external scheduler firing twice is harmless.
//
// Jobs deliberately NOT registered, with reasons:
//
//   * Report/export generation — already asynchronous and user-initiated.
//     It is triggered by a specific person wanting a specific report;
//     running it on a timer would produce reports nobody asked for.
//   * Write-off processing, sanction approval, disbursement execution —
//     these are maker-checker decisions requiring human judgement. Item
//     "do not assume every job should be automated" applies directly:
//     automating an approval would defeat the control.
//   * Data cleanup / retention deletion — retention periods are seeded
//     UNVALIDATED at 0 years and require compliance sign-off. Automating
//     deletion against unvalidated policy could destroy records the law
//     requires be kept. Stays manual until compliance configures it.
//   * Notification dispatch — no evidence it is currently batched; it
//     happens inline with the events that produce it.
func RegisterAll() {
	Register(Definition{
		Name:              "risk.daily",
		Purpose:           "Recompute DPD for every active loan, refresh delinquency buckets, apply approved classification/provisioning rules, and mark broken payment promises.",
		SuggestedSchedule: "Daily, after end-of-day (e.g. 01:00 local)",
		Idempotent:        true,
		FinancialImpact:   true,
		// At most once per calendar day: DPD is a point-in-time
		// measure and a second same-day run would create a duplicate
		// RiskJobRun record for the same business day.
		RunKeyFor: func(t time.Time) string { return "risk.daily:" + t.Format("2006-01-02") },
		Handler: func() (int, error) {
			run, err := risk.RunDailyRiskJob(time.Now())
			if err != nil {
				return 0, err
			}
			if run == nil {
				return 0, nil
			}
			return run.LoansProcessed, nil
		},
	})

	Register(Definition{
		Name:              "risk.consistency",
		Purpose:           "Detect loan/ledger consistency exceptions (negative outstanding, paid-but-overdue, duplicate provisions, missing journal references).",
		SuggestedSchedule: "Daily, after risk.daily",
		Idempotent:        true,
		FinancialImpact:   true,
		Handler: func() (int, error) {
			return risk.RunConsistencyChecks(time.Now()), nil
		},
	})

	Register(Definition{
		Name:              "collections.broken_promises",
		Purpose:           "Flag payment promises whose promised date has passed without the promised amount being received.",
		SuggestedSchedule: "Daily",
		Idempotent:        true,
		FinancialImpact:   true,
		Handler: func() (int, error) {
			return collections.MarkBrokenPromises(time.Now()), nil
		},
	})

	Register(Definition{
		Name:              "compliance.sweep",
		Purpose:           "Refresh overdue compliance calendar items and CAPAs, and raise due-soon/overdue alerts.",
		SuggestedSchedule: "Daily, early morning",
		Idempotent:        true,
		FinancialImpact:   false,
		Handler: func() (int, error) {
			return compliance.RunDailySweep(), nil
		},
	})

	Register(Definition{
		Name:              "analytics.data_quality",
		Purpose:           "Detect analytics data-quality exceptions (negative amounts, disbursed loans without a schedule, future-dated transactions, orphaned installments, disbursement/ledger mismatch).",
		SuggestedSchedule: "Daily",
		Idempotent:        true,
		FinancialImpact:   false,
		Handler: func() (int, error) {
			return analytics.RunDataQualityChecks(time.Now()), nil
		},
	})

	Register(Definition{
		Name:              "analytics.risk_signals",
		Purpose:           "Detect review-worthy patterns (shared bank accounts, repeated failed payments, multiple applications in a short window).",
		SuggestedSchedule: "Daily",
		Idempotent:        true,
		FinancialImpact:   false,
		Handler: func() (int, error) {
			return analytics.RunSignalDetection(time.Now()), nil
		},
	})

	Register(Definition{
		Name:              "integrity.financial",
		Purpose:           "Run every financial integrity invariant (debit=credit, journal-number uniqueness, closed-period, duplicate disbursement, negative amounts, orphaned lines, disbursement/ledger agreement). Read-only.",
		SuggestedSchedule: "Hourly, and immediately after any deployment",
		Idempotent:        true,
		FinancialImpact:   false, // reports on financial data; never modifies it
		Handler: func() (int, error) {
			r := integrity.RunAll()
			// A failing invariant is reported as items processed and an
			// error, so the execution record shows FAILED and an alert
			// can key on it — rather than a "successful" run that
			// quietly contained a critical finding.
			failing := r.CriticalCount + r.HighCount + r.WarningCount
			if !r.Passed {
				return failing, &IntegrityFailure{Critical: r.CriticalCount, High: r.HighCount, Warning: r.WarningCount}
			}
			return 0, nil
		},
	})
}

// IntegrityFailure makes a failing integrity run a first-class error so
// the job execution is recorded FAILED and alerting can trigger on it.
type IntegrityFailure struct {
	Critical, High, Warning int
}

func (e *IntegrityFailure) Error() string {
	return "financial integrity checks failed: " +
		itoa(e.Critical) + " critical, " + itoa(e.High) + " high, " + itoa(e.Warning) + " warning — investigate via GET /admin/ops/integrity. NOTHING has been auto-corrected."
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}
