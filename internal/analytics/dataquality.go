// Analytics data-quality validation (item AI).
//
// CRITICAL CONSTRAINT: item AI states historical financial data must not
// be silently fixed. Nothing in this file writes to any source financial
// table. Every check only RECORDS an exception for a human to resolve.
package analytics

import (
	"fmt"
	"time"

	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/models"
)

func recordDataQuality(exceptionType, severity, detail, source, dedupeKey string) {
	var existing models.AnalyticsDataQualityException
	if err := database.DB.Where("dedupe_key = ?", dedupeKey).First(&existing).Error; err == nil {
		return
	}
	database.DB.Create(&models.AnalyticsDataQualityException{
		ExceptionType: exceptionType, Severity: severity,
		Detail: detail, Source: source, DedupeKey: dedupeKey,
		DetectedAt: time.Now(),
	})
}

// RunDataQualityChecks executes every validation and returns how many new
// exceptions were recorded.
func RunDataQualityChecks(asOf time.Time) int {
	before := dataQualityCount()

	checkNegativeAmounts()
	checkDisbursedWithoutSchedule()
	checkFutureDatedTransactions(asOf)
	checkOrphanedInstallments()
	checkDisbursementLedgerMismatch(asOf)

	return int(dataQualityCount() - before)
}

func dataQualityCount() int64 {
	var c int64
	database.DB.Model(&models.AnalyticsDataQualityException{}).Count(&c)
	return c
}

// Impossible negative values.
func checkNegativeAmounts() {
	var ids []uint
	database.DB.Raw(`SELECT id FROM disbursements WHERE amount_paise < 0 LIMIT 100`).Scan(&ids)
	for _, id := range ids {
		recordDataQuality("NEGATIVE_DISBURSEMENT", models.SignalSeverityCritical,
			fmt.Sprintf("Disbursement %d has a negative amount, which is not a possible value.", id),
			"disbursements.amount_paise", fmt.Sprintf("NEGATIVE_DISBURSEMENT:%d", id))
	}

	var payIDs []uint
	database.DB.Raw(`SELECT id FROM payments WHERE amount_paise < 0 LIMIT 100`).Scan(&payIDs)
	for _, id := range payIDs {
		recordDataQuality("NEGATIVE_PAYMENT", models.SignalSeverityCritical,
			fmt.Sprintf("Payment %d has a negative amount, which is not a possible value.", id),
			"payments.amount_paise", fmt.Sprintf("NEGATIVE_PAYMENT:%d", id))
	}
}

// Broken relationship: a disbursed loan with no repayment schedule means
// every portfolio and DPD figure understates that loan.
func checkDisbursedWithoutSchedule() {
	var ids []uint
	database.DB.Raw(`
		SELECT la.id FROM loan_applications la
		WHERE la.status = ?
		  AND NOT EXISTS (SELECT 1 FROM emi_installments ei WHERE ei.loan_id = la.id)
		LIMIT 100`, models.LoanStatusDisbursed).Scan(&ids)
	for _, id := range ids {
		recordDataQuality("DISBURSED_WITHOUT_SCHEDULE", models.SignalSeverityCritical,
			fmt.Sprintf("Loan %d is disbursed but has no EMI installments. Portfolio, DPD, and collection-efficiency figures exclude this loan until a schedule exists.", id),
			"loan_applications joined to emi_installments", fmt.Sprintf("DISBURSED_WITHOUT_SCHEDULE:%d", id))
	}
}

// Date anomaly: a transaction dated in the future distorts every period
// comparison it falls into.
func checkFutureDatedTransactions(asOf time.Time) {
	var ids []uint
	database.DB.Raw(`SELECT id FROM disbursements WHERE completed_at > ? LIMIT 100`, asOf).Scan(&ids)
	for _, id := range ids {
		recordDataQuality("FUTURE_DATED_DISBURSEMENT", models.SignalSeverityWarning,
			fmt.Sprintf("Disbursement %d has a completion date in the future.", id),
			"disbursements.completed_at", fmt.Sprintf("FUTURE_DATED_DISBURSEMENT:%d", id))
	}
}

// Broken relationship: installments pointing at a loan that no longer
// exists would be counted in schedule-based aggregates but never in
// loan-based ones.
func checkOrphanedInstallments() {
	var count int64
	database.DB.Raw(`
		SELECT COUNT(*) FROM emi_installments ei
		WHERE NOT EXISTS (SELECT 1 FROM loan_applications la WHERE la.id = ei.loan_id)
	`).Scan(&count)
	if count > 0 {
		recordDataQuality("ORPHANED_INSTALLMENTS", models.SignalSeverityCritical,
			fmt.Sprintf("%d EMI installments reference a loan application that does not exist.", count),
			"emi_installments.loan_id", fmt.Sprintf("ORPHANED_INSTALLMENTS:%d", count))
	}
}

// Financial mismatch — item AI's own worked example. Compares total
// successful disbursement against what the general ledger recorded for
// the same period. A difference means one of the two is incomplete;
// nothing is adjusted, the discrepancy is reported.
func checkDisbursementLedgerMismatch(asOf time.Time) {
	from := startOfMonth(asOf)
	to := endOfMonth(asOf)

	operational, _ := disbursedSum(from, to)

	var ledger int64
	database.DB.Table("journal_entry_lines jel").
		Joins("JOIN journal_entries je ON je.id = jel.journal_id").
		Joins("JOIN ledger_accounts la ON la.id = jel.account_id").
		Where("la.account_code = ? AND je.transaction_type = ? AND je.transaction_date BETWEEN ? AND ?",
			"1100", "DISBURSEMENT", from, to).
		Select("COALESCE(SUM(jel.debit_paise),0)").Scan(&ledger)

	if operational != ledger {
		recordDataQuality("DISBURSEMENT_LEDGER_MISMATCH", models.SignalSeverityCritical,
			fmt.Sprintf("Operational disbursement total for %s is %d paise but the general ledger recorded %d paise (difference %d). No data has been altered — this requires accounting review.",
				from.Format("2006-01"), operational, ledger, operational-ledger),
			"disbursements vs journal_entry_lines (account 1100)",
			fmt.Sprintf("DISBURSEMENT_LEDGER_MISMATCH:%s", from.Format("2006-01")))
	}
}

// ReviewDataQualityException — resolution is a human recording what was
// decided; it never changes the underlying data.
func ReviewDataQualityException(id uint, status, reviewer, note string) error {
	switch status {
	case models.DataQualityStatusReviewed, models.DataQualityStatusResolved:
	default:
		return fmt.Errorf("invalid status %q", status)
	}
	if reviewer == "" {
		return fmt.Errorf("reviewer is required")
	}
	now := time.Now()
	return database.DB.Model(&models.AnalyticsDataQualityException{}).Where("id = ?", id).
		Updates(map[string]interface{}{
			"status": status, "reviewed_by": reviewer,
			"reviewed_at": &now, "resolution_note": note,
		}).Error
}
