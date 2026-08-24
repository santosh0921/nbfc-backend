// Package integrity implements Phase 16 item X — automated financial
// integrity monitoring.
//
// THE CENTRAL RULE: this package DETECTS and REPORTS. It never repairs.
// Item X is explicit that financial data must not be silently repaired,
// and the same discipline already governs Phase 13's data-quality
// exceptions and Phase 5's consistency checks. Nothing here issues an
// UPDATE or DELETE against any financial table.
//
// Each check returns a Result carrying its own severity, so an operator
// alert can be raised at the right level rather than treating a
// rounding-scale discrepancy and a broken ledger identically.
package integrity

import (
	"fmt"
	"time"

	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/observability"
)

const (
	SeverityOK       = "OK"
	SeverityWarning  = "WARNING"
	SeverityHigh     = "HIGH"
	SeverityCritical = "CRITICAL"
)

// Result is one integrity check's outcome.
type Result struct {
	Check    string `json:"check"`
	Passed   bool   `json:"passed"`
	Severity string `json:"severity"`
	// Detail states what was compared and what was found, in terms an
	// operator can act on without reading the query.
	Detail string `json:"detail"`
	// Invariant states, in plain language, the rule being enforced — so
	// a failure explains itself rather than requiring tribal knowledge.
	Invariant string `json:"invariant"`
	Count     int64  `json:"count"`
}

type Report struct {
	RanAt   time.Time `json:"ranAt"`
	Results []Result  `json:"results"`

	Passed         bool `json:"passed"`
	CriticalCount  int  `json:"criticalCount"`
	HighCount      int  `json:"highCount"`
	WarningCount   int  `json:"warningCount"`

	Note string `json:"note"`
}

// RunAll executes every integrity check. Safe to run on a schedule or
// on demand; it is read-only.
func RunAll() Report {
	r := Report{
		RanAt: time.Now(),
		Note:  "Detection only. No financial data is modified by these checks — a failure must be investigated and corrected deliberately, never auto-repaired.",
	}

	r.Results = []Result{
		checkJournalBalance(),
		checkJournalSequenceIntegrity(),
		checkClosedPeriodIntegrity(),
		checkDuplicateDisbursements(),
		checkNegativeAmounts(),
		checkOrphanedJournalLines(),
		checkDisbursementLedgerAgreement(),
	}

	r.Passed = true
	for _, res := range r.Results {
		if res.Passed {
			continue
		}
		r.Passed = false
		switch res.Severity {
		case SeverityCritical:
			r.CriticalCount++
		case SeverityHigh:
			r.HighCount++
		default:
			r.WarningCount++
		}
	}

	if !r.Passed {
		// Feeds the operational alert surface (item W/X) rather than
		// only appearing in an API response nobody is watching.
		observability.Inc(observability.MetricIntegrityFail)
	}
	return r
}

// --- Debit == Credit, the foundational double-entry invariant ---

func checkJournalBalance() int64Result {
	const invariant = "Every journal entry must have total debits equal to total credits."
	var count int64
	database.DB.Raw(`
		SELECT COUNT(*) FROM (
			SELECT jel.journal_id
			FROM journal_entry_lines jel
			GROUP BY jel.journal_id
			HAVING COALESCE(SUM(jel.debit_paise),0) <> COALESCE(SUM(jel.credit_paise),0)
		) t
	`).Scan(&count)

	return build("journal_balance", invariant, count, SeverityCritical,
		"All journal entries balance.",
		fmt.Sprintf("%d journal entr(ies) have debits != credits. The general ledger does not balance; financial reporting derived from it is unreliable until this is resolved.", count))
}

// --- Journal numbering: no duplicates ---
//
// Phase 15 fixed a COUNT-derived sequence that could reuse a number.
// This check is the standing guard that the property actually holds in
// the data, not just in the code path that generates it.
func checkJournalSequenceIntegrity() int64Result {
	const invariant = "A journal number must identify exactly one entry — numbers are never reused."
	var count int64
	database.DB.Raw(`
		SELECT COUNT(*) FROM (
			SELECT journal_number FROM journal_entries
			GROUP BY journal_number HAVING COUNT(*) > 1
		) t
	`).Scan(&count)

	return build("journal_sequence", invariant, count, SeverityCritical,
		"All journal numbers are unique.",
		fmt.Sprintf("%d journal number(s) appear on more than one entry — the audit sequence has been compromised.", count))
}

// --- Nothing posted into a closed accounting period ---

func checkClosedPeriodIntegrity() int64Result {
	const invariant = "No journal entry may exist against an accounting period that is CLOSED."
	var count int64
	database.DB.Raw(`
		SELECT COUNT(*)
		FROM journal_entries je
		JOIN accounting_periods ap ON ap.id = je.period_id
		WHERE ap.status = 'CLOSED' AND je.posting_date > ap.closed_at
	`).Scan(&count)

	return build("closed_period", invariant, count, SeverityHigh,
		"No postings after period close.",
		fmt.Sprintf("%d journal entr(ies) were posted after their accounting period was closed.", count))
}

// --- One successful disbursement per loan ---

func checkDuplicateDisbursements() int64Result {
	const invariant = "A loan may have at most one successful disbursement."
	var count int64
	database.DB.Raw(`
		SELECT COUNT(*) FROM (
			SELECT loan_id FROM disbursements
			WHERE status = 'SUCCESS'
			GROUP BY loan_id HAVING COUNT(*) > 1
		) t
	`).Scan(&count)

	return build("duplicate_disbursement", invariant, count, SeverityCritical,
		"No loan has more than one successful disbursement.",
		fmt.Sprintf("%d loan(s) have multiple successful disbursements — money may have been paid out twice.", count))
}

// --- No impossible negative money ---

func checkNegativeAmounts() int64Result {
	const invariant = "Disbursement and payment amounts must never be negative."
	var count int64
	database.DB.Raw(`
		SELECT
			(SELECT COUNT(*) FROM disbursements WHERE amount_paise < 0) +
			(SELECT COUNT(*) FROM payments WHERE amount_paise < 0)
	`).Scan(&count)

	return build("negative_amounts", invariant, count, SeverityHigh,
		"No negative disbursement or payment amounts.",
		fmt.Sprintf("%d row(s) hold a negative amount, which is not a representable business value.", count))
}

// --- Every journal line belongs to a journal entry ---

func checkOrphanedJournalLines() int64Result {
	const invariant = "Every journal line must belong to an existing journal entry."
	var count int64
	database.DB.Raw(`
		SELECT COUNT(*) FROM journal_entry_lines jel
		WHERE NOT EXISTS (SELECT 1 FROM journal_entries je WHERE je.id = jel.journal_id)
	`).Scan(&count)

	return build("orphaned_journal_lines", invariant, count, SeverityCritical,
		"No orphaned journal lines.",
		fmt.Sprintf("%d journal line(s) reference a journal entry that no longer exists — ledger totals include amounts with no parent entry.", count))
}

// --- Operational disbursement total agrees with the ledger ---
//
// Compares the disbursement table against what was posted to the loan
// receivable account. A divergence means one of the two is incomplete;
// which one is correct is a human determination, so this only reports.
func checkDisbursementLedgerAgreement() int64Result {
	const invariant = "Successful disbursements must equal what was posted to the loan-receivable account for the same events."

	var operational, ledger int64
	database.DB.Raw(`SELECT COALESCE(SUM(amount_paise),0) FROM disbursements WHERE status = 'SUCCESS'`).Scan(&operational)
	database.DB.Raw(`
		SELECT COALESCE(SUM(jel.debit_paise),0)
		FROM journal_entry_lines jel
		JOIN journal_entries je ON je.id = jel.journal_id
		JOIN ledger_accounts la ON la.id = jel.account_id
		WHERE la.account_code = '1100' AND je.transaction_type = 'DISBURSEMENT'
	`).Scan(&ledger)

	diff := operational - ledger
	if diff < 0 {
		diff = -diff
	}

	res := int64Result{
		Check:     "disbursement_ledger_agreement",
		Invariant: invariant,
		Count:     diff,
		Passed:    diff == 0,
		Severity:  SeverityOK,
	}
	if diff == 0 {
		res.Detail = fmt.Sprintf("Disbursement total and ledger agree (%d paise).", operational)
		return res
	}
	res.Severity = SeverityHigh
	res.Detail = fmt.Sprintf(
		"Disbursement table totals %d paise but the ledger recorded %d paise for DISBURSEMENT postings (difference %d). "+
			"This is reported, not corrected — accounting must determine which side is incomplete.",
		operational, ledger, diff)
	return res
}

// int64Result is an alias so each check reads as returning a Result
// while sharing the small build helper below.
type int64Result = Result

func build(check, invariant string, count int64, failSeverity, okDetail, failDetail string) Result {
	if count == 0 {
		return Result{
			Check: check, Invariant: invariant, Count: 0,
			Passed: true, Severity: SeverityOK, Detail: okDetail,
		}
	}
	return Result{
		Check: check, Invariant: invariant, Count: count,
		Passed: false, Severity: failSeverity, Detail: failDetail,
	}
}
