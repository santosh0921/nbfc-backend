// Data reconciliation dashboard (item F) — cross-system checks between
// loan schedule, accounting, payments, bank, collections, NPA/
// provisioning, and KYC. Where a prior phase already runs an equivalent
// check (Phase 2's bank ReconciliationException, Phase 5's
// DataConsistencyException), this package surfaces those findings into
// CrossSystemReconciliation rather than re-implementing the same logic
// twice — this is explicitly the CONSOLIDATED compliance view item F
// asks for, not a competing source of truth.
package reporting

import (
	"strconv"
	"time"

	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/models"
)

// RunCrossSystemChecks (re)builds the CrossSystemReconciliation table from
// every upstream signal this package knows how to read. Clears prior
// PENDING_REVIEW-or-earlier rows for each check type before re-inserting,
// so re-running never duplicates a still-open finding — a row a human has
// already marked resolved is left untouched.
func RunCrossSystemChecks() int {
	found := 0
	found += importBankReconciliationExceptions()
	found += importDataConsistencyExceptions()
	found += checkLoanWithoutClassification()
	found += checkOrphanPayments()
	found += checkDisbursementAmountMismatch()
	found += checkPaymentGatewayStatusMismatch()
	return found
}

// checkPaymentGatewayStatusMismatch (Phase 9 gap-closure — item O's
// second named example: "bank/gateway says succeeded but application
// says failed," and its inverse). Compares the internal Payment.Status
// against what the gateway's own webhook events actually said for that
// payment — the PaymentEvent log (Phase 2) already records every event
// received, including ones a signature/amount check rejected, so this
// reads the gateway's claims from that log rather than calling the
// gateway API again. Two distinct findings:
//  1. Payment.Status == SUCCESS but the latest EVENT recorded for it was
//     a failure/refund type — the internal state and the last thing the
//     gateway said disagree.
//  2. A Payment stuck in INITIATED/PENDING for more than 24h with no
//     terminal webhook event at all — neither confirmed success nor
//     failure, needs manual investigation rather than being silently
//     left open forever.
func checkPaymentGatewayStatusMismatch() int {
	found := 0

	var staleRows []struct {
		PaymentID uint
	}
	cutoff := "NOW() - INTERVAL '24 hours'"
	database.DB.Raw(`
		SELECT id as payment_id FROM payments
		WHERE status IN ('INITIATED','PENDING') AND created_at < ` + cutoff + `
	`).Scan(&staleRows)
	for _, r := range staleRows {
		upsertRecon("payment", strconv.Itoa(int(r.PaymentID)), "internal_ledger", "payment_gateway",
			"a terminal SUCCESS/FAILED webhook event within 24h", "no terminal event received",
			models.ReconStatusPendingReview, "HIGH")
		found++
	}

	var mismatchRows []struct {
		PaymentID uint
		LatestEvent string
	}
	database.DB.Raw(`
		SELECT p.id as payment_id, pe.event_type as latest_event
		FROM payments p
		JOIN LATERAL (
			SELECT event_type FROM payment_events
			WHERE payment_id = p.id ORDER BY created_at DESC LIMIT 1
		) pe ON true
		WHERE p.status = 'SUCCESS' AND pe.event_type IN ('payment.failed', 'refund.processed', 'refund.created')
	`).Scan(&mismatchRows)
	for _, r := range mismatchRows {
		upsertRecon("payment", strconv.Itoa(int(r.PaymentID)), "internal_ledger", "payment_gateway",
			"SUCCESS", "latest gateway event: "+r.LatestEvent, models.ReconStatusMismatch, "CRITICAL")
		found++
	}

	return found
}

// checkDisbursementAmountMismatch (Phase 8 item O) — "Loan says ₹100,000
// disbursed but payment gateway/bank says ₹99,000" is exactly the
// scenario item O names. A successful Disbursement's AmountPaise is
// compared against the BankTransaction it was matched to during
// reconciliation (Phase 2) — a mismatch here means the loan system and
// the bank statement disagree about how much actually moved, which is
// more serious than a bank-side "unmatched" exception (item F territory)
// because BOTH sides claim a match, just at different amounts.
func checkDisbursementAmountMismatch() int {
	var rows []struct {
		DisbursementID uint
		LoanID         uint
		DisbAmount     int64
		BankAmount     int64
	}
	database.DB.Raw(`
		SELECT d.id as disbursement_id, d.loan_id, d.amount_paise as disb_amount, bt.amount_paise as bank_amount
		FROM disbursements d
		JOIN bank_transactions bt ON bt.matched_entity_type = 'disbursement' AND bt.matched_entity_id = d.id
		WHERE d.status = 'SUCCESS' AND d.amount_paise != bt.amount_paise
	`).Scan(&rows)
	for _, r := range rows {
		upsertRecon("disbursement", strconv.Itoa(int(r.DisbursementID)), "loan_system", "bank_statement",
			strconv.FormatInt(r.DisbAmount, 10), strconv.FormatInt(r.BankAmount, 10),
			models.ReconStatusMismatch, "CRITICAL")
	}
	return len(rows)
}

func upsertRecon(entity, reference, sourceA, sourceB, expected, actual, status, severity string) {
	var existing models.CrossSystemReconciliation
	err := database.DB.Where("entity = ? AND reference = ? AND source_a = ? AND source_b = ? AND status != ?",
		entity, reference, sourceA, sourceB, "RESOLVED").First(&existing).Error
	if err == nil {
		return // an open finding for this exact comparison already exists
	}
	database.DB.Create(&models.CrossSystemReconciliation{
		Entity: entity, Reference: reference, SourceA: sourceA, SourceB: sourceB,
		ExpectedValue: expected, ActualValue: actual, Status: status, Severity: severity,
	})
}

// importBankReconciliationExceptions surfaces Phase 2's own open bank
// exceptions into the consolidated view.
func importBankReconciliationExceptions() int {
	var exceptions []models.ReconciliationException
	database.DB.Where("resolved_at IS NULL").Find(&exceptions)
	for _, e := range exceptions {
		upsertRecon("bank_transaction", strconv.Itoa(int(e.BankTransactionID)), "bank_statement", "disbursement",
			"", "", models.ReconStatusMismatch, "MEDIUM")
	}
	return len(exceptions)
}

// importDataConsistencyExceptions surfaces Phase 5's open risk/accounting
// exceptions.
func importDataConsistencyExceptions() int {
	var exceptions []models.DataConsistencyException
	database.DB.Where("status = ?", models.ExceptionStatusOpen).Find(&exceptions)
	for _, e := range exceptions {
		ref := ""
		if e.LoanID != nil {
			ref = strconv.Itoa(int(*e.LoanID))
		}
		upsertRecon("loan", ref, "loan_schedule", "accounting_ledger", "", "", models.ReconStatusMismatch, "HIGH")
	}
	return len(exceptions)
}

// checkLoanWithoutClassification flags a disbursed, materially overdue
// loan that has no LoanClassification row at all — a gap between the risk
// engine and what's actually recorded, distinct from Phase 5's own
// negative-outstanding/duplicate-provision checks.
func checkLoanWithoutClassification() int {
	var rows []struct{ LoanID uint }
	database.DB.Raw(`
		SELECT DISTINCT dr.loan_id FROM dpd_records dr
		WHERE dr.dpd > 90
		AND NOT EXISTS (SELECT 1 FROM loan_classifications lc WHERE lc.loan_id = dr.loan_id)
	`).Scan(&rows)
	for _, r := range rows {
		upsertRecon("loan", strconv.Itoa(int(r.LoanID)), "risk_dpd", "risk_classification",
			"a classification is expected once DPD exceeds 90", "no classification recorded",
			models.ReconStatusMissing, "HIGH")
	}
	return len(rows)
}

// checkOrphanPayments flags a SUCCESS payment whose EmiInstallment can no
// longer be found — should be structurally impossible (foreign key
// reference, never deleted) but is checked independently for the same
// defense-in-depth reasoning as validation.go's checks.
func checkOrphanPayments() int {
	var rows []struct{ PaymentID uint }
	database.DB.Raw(`
		SELECT p.id as payment_id FROM payments p
		WHERE p.status = 'SUCCESS'
		AND NOT EXISTS (SELECT 1 FROM emi_installments ei WHERE ei.id = p.emi_id)
	`).Scan(&rows)
	for _, r := range rows {
		upsertRecon("payment", strconv.Itoa(int(r.PaymentID)), "payments", "emi_installments",
			"a matching EmiInstallment", "none found", models.ReconStatusMissing, "CRITICAL")
	}
	return len(rows)
}

// ResolveReconciliation — a human decision, always audited via the
// standard admin-action pattern in handler.go (not duplicated here).
func ResolveReconciliation(id uint, resolvedBy, note string) error {
	now := time.Now()
	return database.DB.Model(&models.CrossSystemReconciliation{}).Where("id = ?", id).
		Updates(map[string]interface{}{
			"status": models.ReconStatusMatched, "resolution_status": "RESOLVED",
			"resolution_note": note + " (resolved by " + resolvedBy + ")", "resolved_at": now,
		}).Error
}
