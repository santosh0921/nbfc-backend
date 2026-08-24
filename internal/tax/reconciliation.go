// TDS reconciliation + exceptions (items G/K). Mirrors the pattern
// established in Phase 6/9's internal/reporting reconciliation checks —
// server-side detection, never a silent auto-fix, one open exception per
// finding+reference to avoid re-raising duplicates on every run.
package tax

import (
	"strconv"
	"time"

	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/models"
)

// RunTDSReconciliation (item G/K) — checks item K names, each producing a
// TDSException row.
func RunTDSReconciliation() int {
	found := 0
	found += checkPANMissing()
	found += checkDeductedNotChallanMapped()
	found += checkChallanOverdue()
	found += checkDuplicateDeduction()
	return found
}

func recordException(exceptionType, severity, detail, refType, refID string) {
	var existing models.TDSException
	err := database.DB.Where("exception_type = ? AND reference_type = ? AND reference_id = ? AND status = ?",
		exceptionType, refType, refID, models.TDSExceptionStatusOpen).First(&existing).Error
	if err == nil {
		return
	}
	database.DB.Create(&models.TDSException{
		ExceptionType: exceptionType, Severity: severity, Detail: detail,
		ReferenceType: refType, ReferenceID: refID, Status: models.TDSExceptionStatusOpen,
	})
}

// checkPANMissing — a TDSTransaction calculated with PANStatusAtCalc not
// VALID is flagged (item J/K: "PAN missing"/"PAN invalid").
func checkPANMissing() int {
	var rows []models.TDSTransaction
	database.DB.Where("status = ? AND pan_status_at_calc != ?", models.TDSStatusApplicable, models.PANStatusValid).Find(&rows)
	for _, r := range rows {
		recordException("PAN_MISSING_OR_INVALID", models.TDSExceptionSeverityHigh,
			"TDS transaction #"+itoa(r.ID)+" ("+r.Deductee+") was calculated with PAN status "+r.PANStatusAtCalc,
			"tds_transaction", itoa(r.ID))
	}
	return len(rows)
}

// checkDeductedNotChallanMapped — a DEDUCTED TDSTransaction with no
// ChallanID after 7+ days needs attention (deducted but not yet tracked
// toward a deposit).
func checkDeductedNotChallanMapped() int {
	cutoff := time.Now().AddDate(0, 0, -7)
	var rows []models.TDSTransaction
	database.DB.Where("status = ? AND challan_id IS NULL AND calculation_date < ?", models.TDSStatusDeducted, cutoff).Find(&rows)
	for _, r := range rows {
		recordException("DEDUCTED_NOT_MAPPED_TO_CHALLAN", models.TDSExceptionSeverityMedium,
			"TDS transaction #"+itoa(r.ID)+" was deducted 7+ days ago but is not yet mapped to a challan",
			"tds_transaction", itoa(r.ID))
	}
	return len(rows)
}

// checkChallanOverdue — a challan with a configured due date that has
// passed without being marked PAID.
func checkChallanOverdue() int {
	var rows []models.TDSChallan
	database.DB.Where("status IN ? AND deposit_due_date IS NOT NULL AND deposit_due_date < ?",
		[]string{models.ChallanStatusPending, models.ChallanStatusDue}, time.Now()).Find(&rows)
	for _, r := range rows {
		recordException("DEPOSIT_OVERDUE", models.TDSExceptionSeverityCritical,
			"Challan #"+itoa(r.ID)+" ("+r.TaxYear+" "+r.Quarter+") is overdue for deposit",
			"tds_challan", itoa(r.ID))
	}
	return len(rows)
}

// checkDuplicateDeduction — more than one DEDUCTED TDSTransaction for the
// same referenceType+referenceID (should be structurally rare given each
// Expense is a distinct reference, but checked independently per this
// codebase's defense-in-depth convention, e.g. Phase 4/5's own
// duplicate-provision checks).
func checkDuplicateDeduction() int {
	var rows []struct {
		ReferenceType string
		ReferenceID   string
		Count         int
	}
	database.DB.Model(&models.TDSTransaction{}).
		Where("status = ?", models.TDSStatusDeducted).
		Group("reference_type, reference_id").Having("COUNT(*) > 1").
		Select("reference_type, reference_id, COUNT(*) as count").Scan(&rows)
	for _, r := range rows {
		recordException("DUPLICATE_DEDUCTION", models.TDSExceptionSeverityCritical,
			r.ReferenceType+" "+r.ReferenceID+" has "+strconv.Itoa(r.Count)+" DEDUCTED TDS transactions",
			r.ReferenceType, r.ReferenceID)
	}
	return len(rows)
}

// ResolveException — a human decision, always audited via the standard
// admin-action pattern (handler.go), not duplicated here.
func ResolveException(id uint, resolvedBy, resolution, evidence string) error {
	now := time.Now()
	return database.DB.Model(&models.TDSException{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status": models.TDSExceptionStatusResolved, "resolved_by": resolvedBy,
		"resolved_at": now, "resolution": resolution, "evidence": evidence,
	}).Error
}
