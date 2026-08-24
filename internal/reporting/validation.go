// Report validation engine (item E). Generic, data-source-level checks —
// missing fields, duplicates, invalid references, negative amounts,
// cross-system mismatches — none of which depend on knowing a specific
// regulatory return's field meanings. A report-specific business-rule
// check (e.g. "field X must equal field Y per this return's instructions")
// is out of scope for this generic engine and must be added per
// definition once compliance specifies it (see the Phase 6 report §17).
package reporting

import (
	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/models"
)

// RunValidation (item E) executes every generic check against a run and
// records one RegulatoryReportException per finding. Always re-runs from
// scratch: prior exceptions for this run are cleared first, so a
// corrected/regenerated run's findings never accumulate stale entries.
func RunValidation(runID uint) []models.RegulatoryReportException {
	database.DB.Where("run_id = ?", runID).Delete(&models.RegulatoryReportException{})

	var findings []models.RegulatoryReportException
	findings = append(findings, checkAccountingMismatch(runID)...)
	findings = append(findings, checkNegativeAmounts(runID)...)
	findings = append(findings, checkOrphanReferences(runID)...)
	findings = append(findings, checkDuplicateJournalIdempotency(runID)...)

	for i := range findings {
		findings[i].RunID = runID
	}
	if len(findings) > 0 {
		database.DB.Create(&findings)
	}
	return findings
}

func newFinding(severity, check, detail string) models.RegulatoryReportException {
	return models.RegulatoryReportException{Severity: severity, Check: check, Detail: detail}
}

// checkAccountingMismatch reuses Phase 4/5's own consistency signal: any
// still-OPEN DataConsistencyException is surfaced as a WARNING here too,
// so a report reviewer doesn't have to separately remember to check the
// Exceptions screen.
func checkAccountingMismatch(runID uint) []models.RegulatoryReportException {
	var count int64
	database.DB.Model(&models.DataConsistencyException{}).Where("status = ?", models.ExceptionStatusOpen).Count(&count)
	if count == 0 {
		return nil
	}
	return []models.RegulatoryReportException{
		newFinding(models.ReportValidationWarning, "accounting_mismatch",
			itoaInt(int(count))+" open data-consistency exception(s) exist in the risk/accounting reconciliation — review before approving"),
	}
}

// checkNegativeAmounts — a regulatory return's underlying source data
// should never show a negative outstanding balance (Phase 5 already
// clamps this at calculation time, but a direct DB write could still
// produce one).
func checkNegativeAmounts(runID uint) []models.RegulatoryReportException {
	var count int64
	database.DB.Model(&models.DPDRecord{}).Where("outstanding_paise < 0").Count(&count)
	if count == 0 {
		return nil
	}
	return []models.RegulatoryReportException{
		newFinding(models.ReportValidationError, "negative_amount",
			itoaInt(int(count))+" DPDRecord row(s) show a negative outstanding amount — this is a blocking error"),
	}
}

// checkOrphanReferences — a LoanProvision or JournalEntry referencing a
// loan ID that no longer resolves to a real LoanApplication row would
// silently corrupt any report built from it.
func checkOrphanReferences(runID uint) []models.RegulatoryReportException {
	var count int64
	database.DB.Raw(`SELECT COUNT(*) FROM loan_provisions lp
		WHERE NOT EXISTS (SELECT 1 FROM loan_applications la WHERE la.id = lp.loan_id)`).Scan(&count)
	if count == 0 {
		return nil
	}
	return []models.RegulatoryReportException{
		newFinding(models.ReportValidationError, "invalid_loan_reference",
			itoaInt(int(count))+" LoanProvision row(s) reference a loan ID with no matching LoanApplication — blocking error"),
	}
}

// checkDuplicateJournalIdempotency — a duplicate IdempotencyKey should be
// structurally impossible (unique index, Phase 4), but this check exists
// as an independent verification a report reviewer can trust even if the
// index were ever somehow bypassed (e.g. a raw SQL migration).
func checkDuplicateJournalIdempotency(runID uint) []models.RegulatoryReportException {
	var rows []struct{ Cnt int }
	database.DB.Raw(`SELECT COUNT(*) as cnt FROM journal_entries GROUP BY idempotency_key HAVING COUNT(*) > 1`).Scan(&rows)
	if len(rows) == 0 {
		return nil
	}
	return []models.RegulatoryReportException{
		newFinding(models.ReportValidationError, "duplicate_journal",
			itoaInt(len(rows))+" idempotency key(s) have more than one JournalEntry — blocking error"),
	}
}
