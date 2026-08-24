// Disbursement ↔ ledger reconciliation (Phase 16.1 Parts 11–12).
//
// Phase 16's `disbursement_ledger_agreement` check reported only a total
// difference. A total is not actionable: it says money disagrees without
// saying which records disagree or why. This produces the per-record
// reconciliation Part 11 requires, with every row classified into
// exactly one of the Part 12 categories.
//
// READ-ONLY, like every other check in this package. It never alters a
// disbursement, a journal entry, or a journal line — proven by
// TestReconciliationNeverMutates.
package integrity

import (
	"time"

	"github.com/santosh0921/nbfc-backend/internal/database"
)

// Classification values — exactly the Part 12 vocabulary. Every
// reconciliation row carries precisely one.
const (
	ClassValid              = "VALID"
	ClassOrphanLedgerEntry  = "ORPHAN LEDGER ENTRY"
	ClassMissingLedgerEntry = "MISSING LEDGER ENTRY"
	ClassAmountMismatch     = "WRONG AMOUNT"
	ClassLedgerForNonSuccess = "ORPHAN LEDGER ENTRY (non-success disbursement)"
	ClassUnknown            = "UNKNOWN"
)

// ReconciliationRow is one disbursement/ledger pairing.
type ReconciliationRow struct {
	Classification string `json:"classification"`

	JournalNumber   string `json:"journalNumber,omitempty"`
	JournalID       *uint  `json:"journalId,omitempty"`
	DisbursementID  *uint  `json:"disbursementId,omitempty"`
	LoanID          *uint  `json:"loanId,omitempty"`

	DisbursementPaise int64 `json:"disbursementPaise"`
	LedgerPaise       int64 `json:"ledgerPaise"`
	DifferencePaise   int64 `json:"differencePaise"`

	DisbursementStatus string    `json:"disbursementStatus,omitempty"`
	CreatedBy          string    `json:"createdBy,omitempty"`
	TransactionDate    time.Time `json:"transactionDate,omitempty"`

	// LoanExists distinguishes a record whose parent loan is still
	// present from one whose loan has also been removed. A production
	// loan is never deleted, so a missing loan is strong evidence of
	// test-fixture cleanup rather than a financial defect.
	LoanExists bool `json:"loanExists"`

	// Evidence records why this row was classified as it was, so the
	// classification can be audited rather than taken on trust.
	Evidence string `json:"evidence,omitempty"`
}

type ClassificationTotal struct {
	Classification string `json:"classification"`
	Rows           int    `json:"rows"`
	LedgerPaise    int64  `json:"ledgerPaise"`
	DisbursementPaise int64 `json:"disbursementPaise"`
}

type ReconciliationReport struct {
	RanAt time.Time `json:"ranAt"`

	TotalDisbursementPaise int64 `json:"totalDisbursementPaise"` // SUCCESS rows only
	TotalLedgerPaise       int64 `json:"totalLedgerPaise"`
	DifferencePaise        int64 `json:"differencePaise"`

	Totals []ClassificationTotal `json:"totals"`
	Rows   []ReconciliationRow   `json:"rows"`

	// UnexplainedPaise is the amount that could NOT be attributed to a
	// specific classification. Part 12: any material UNKNOWN amount
	// means the financial integrity gate FAILS.
	UnexplainedPaise int64 `json:"unexplainedPaise"`
	FullyExplained   bool  `json:"fullyExplained"`

	Note string `json:"note"`
}

// ReconcileDisbursementLedger builds the full per-record reconciliation.
// includeRows=false returns totals only (cheap enough for a dashboard);
// includeRows=true returns every row (for investigation and export).
func ReconcileDisbursementLedger(includeRows bool) ReconciliationReport {
	r := ReconciliationReport{
		RanAt: time.Now(),
		Note:  "Read-only. This report classifies differences; it never alters a disbursement, journal entry, or journal line. A classification is evidence for a human decision, not an automatic correction.",
	}

	// One query covering BOTH directions: ledger entries that have no
	// disbursement, and disbursements that have no ledger entry. A FULL
	// OUTER JOIN is used deliberately so neither side can be silently
	// omitted — the Phase 16 check only compared totals and therefore
	// could not distinguish these cases.
	type row struct {
		JournalID          *uint
		JournalNumber      string
		DisbursementID     *uint
		LoanID             *uint
		DisbursementPaise  int64
		LedgerPaise        int64
		DisbursementStatus string
		CreatedBy          string
		TransactionDate    *time.Time
		LoanExists         bool
	}
	var rows []row

	database.DB.Raw(`
		WITH ledger AS (
			SELECT je.id AS journal_id,
			       je.journal_number,
			       NULLIF(je.reference_id, '')::bigint AS disbursement_ref,
			       je.created_by,
			       je.transaction_date,
			       SUM(jel.debit_paise) AS ledger_paise
			FROM journal_entries je
			JOIN journal_entry_lines jel ON jel.journal_id = je.id
			JOIN ledger_accounts la ON la.id = jel.account_id
			WHERE je.transaction_type = 'DISBURSEMENT'
			  AND la.account_code = '1100'
			  AND je.reference_type = 'disbursement'
			GROUP BY 1,2,3,4,5
		),
		disb AS (
			SELECT d.id, d.loan_id, d.amount_paise, d.status
			FROM disbursements d
			WHERE d.status = 'SUCCESS'
		)
		SELECT l.journal_id,
		       COALESCE(l.journal_number,'')            AS journal_number,
		       d.id                                     AS disbursement_id,
		       d.loan_id                                AS loan_id,
		       COALESCE(d.amount_paise,0)               AS disbursement_paise,
		       COALESCE(l.ledger_paise,0)               AS ledger_paise,
		       COALESCE(d.status,'')                    AS disbursement_status,
		       COALESCE(l.created_by,'')                AS created_by,
		       l.transaction_date,
		       COALESCE((SELECT true FROM loan_applications la WHERE la.id = d.loan_id), false) AS loan_exists
		FROM ledger l
		FULL OUTER JOIN disb d ON d.id = l.disbursement_ref
		ORDER BY l.journal_id NULLS LAST, d.id
	`).Scan(&rows)

	totals := map[string]*ClassificationTotal{}
	for _, x := range rows {
		rr := ReconciliationRow{
			JournalID: x.JournalID, JournalNumber: x.JournalNumber,
			DisbursementID: x.DisbursementID, LoanID: x.LoanID,
			DisbursementPaise: x.DisbursementPaise, LedgerPaise: x.LedgerPaise,
			DifferencePaise:    x.DisbursementPaise - x.LedgerPaise,
			DisbursementStatus: x.DisbursementStatus,
			CreatedBy:          x.CreatedBy,
			LoanExists:         x.LoanExists,
		}
		if x.TransactionDate != nil {
			rr.TransactionDate = *x.TransactionDate
		}

		switch {
		case x.JournalID != nil && x.DisbursementID == nil:
			rr.Classification = ClassOrphanLedgerEntry
			rr.Evidence = "A ledger DISBURSEMENT posting exists but the disbursement row it references (reference_id) is absent."
		case x.JournalID == nil && x.DisbursementID != nil:
			rr.Classification = ClassMissingLedgerEntry
			rr.Evidence = "A SUCCESS disbursement exists with no corresponding ledger DISBURSEMENT posting."
		case x.DisbursementPaise == x.LedgerPaise:
			rr.Classification = ClassValid
			rr.Evidence = "Disbursement amount and ledger posting agree exactly."
		default:
			rr.Classification = ClassAmountMismatch
			rr.Evidence = "Disbursement row and ledger posting both exist but the amounts differ."
		}

		// A missing parent loan is corroborating evidence, not a
		// classification of its own — recorded so a reviewer can weigh it.
		if !x.LoanExists && x.DisbursementID != nil {
			rr.Evidence += " The referenced loan_application no longer exists."
		}

		t, ok := totals[rr.Classification]
		if !ok {
			t = &ClassificationTotal{Classification: rr.Classification}
			totals[rr.Classification] = t
		}
		t.Rows++
		t.LedgerPaise += rr.LedgerPaise
		t.DisbursementPaise += rr.DisbursementPaise

		r.TotalDisbursementPaise += rr.DisbursementPaise
		r.TotalLedgerPaise += rr.LedgerPaise

		if rr.Classification == ClassUnknown {
			d := rr.DifferencePaise
			if d < 0 {
				d = -d
			}
			r.UnexplainedPaise += d
		}
		if includeRows {
			r.Rows = append(r.Rows, rr)
		}
	}

	for _, t := range totals {
		r.Totals = append(r.Totals, *t)
	}
	r.DifferencePaise = r.TotalDisbursementPaise - r.TotalLedgerPaise
	r.FullyExplained = r.UnexplainedPaise == 0

	return r
}
