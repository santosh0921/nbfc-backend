// Deductee/PAN master (items C/J). Reuses internal/security's AES-256-GCM
// encryption — the exact same mechanism as customer PAN (Phase 1/3) and
// vendor PAN (Phase 8) — deliberately, so PAN handling has one consistent
// implementation across the whole codebase rather than three variants.
package tax

import (
	"errors"
	"time"

	"github.com/santosh0921/nbfc-backend/internal/audit"
	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/models"
	"github.com/santosh0921/nbfc-backend/internal/security"
)

func CreateDeductee(name string, vendorID *uint, pan, residentStatus, createdBy string) (*models.TDSDeductee, error) {
	if name == "" {
		return nil, errors.New("tax: deductee name is required")
	}
	d := models.TDSDeductee{Name: name, VendorID: vendorID, ResidentStatus: residentStatus, PANStatus: models.PANStatusNotProvided}
	if pan != "" {
		encrypted, err := security.Encrypt(pan)
		if err != nil {
			return nil, err
		}
		d.PANEncrypted = encrypted
		if len(pan) >= 4 {
			d.PANLast4 = pan[len(pan)-4:]
		}
		d.PANStatus = models.PANStatusUnverified
	}
	if err := database.DB.Create(&d).Error; err != nil {
		return nil, err
	}
	audit.RecordDetailed(nil, audit.Entry{
		ActorType: "admin", ActorRef: createdBy,
		Action: "tax.deductee.created", TargetType: "tds_deductee", TargetID: itoa(d.ID),
		Result: audit.ResultSuccess, Details: name,
	})
	return &d, nil
}

// SetPANValidationStatus — records a PAN validation outcome. source must
// be non-empty and honest ("manual" for a human check, or a real
// verification provider's name) — never fabricated as if an automated
// check ran when it didn't.
func SetPANValidationStatus(deducteeID uint, status, source, failureReason, actor string) error {
	switch status {
	case models.PANStatusValid, models.PANStatusInvalid, models.PANStatusUnverified, models.PANStatusNotProvided:
	default:
		return errors.New("tax: invalid PAN status")
	}
	now := time.Now()
	err := database.DB.Model(&models.TDSDeductee{}).Where("id = ?", deducteeID).Updates(map[string]interface{}{
		"pan_status": status, "pan_validated_at": now, "pan_validation_source": source, "pan_failure_reason": failureReason,
	}).Error
	if err != nil {
		return err
	}
	audit.RecordDetailed(nil, audit.Entry{
		ActorType: "admin", ActorRef: actor,
		Action: "tax.deductee.pan_status_changed", TargetType: "tds_deductee", TargetID: itoa(deducteeID),
		Result: audit.ResultSuccess, Details: status + " via " + source,
	})
	return nil
}

// --- Compliance calendar (item Q) ---

func CreateCalendarItem(item *models.TaxComplianceCalendarItem, actor string) (*models.TaxComplianceCalendarItem, error) {
	if item.ComplianceType == "" || item.TaxYear == "" || item.DueDate.IsZero() {
		return nil, errors.New("tax: complianceType, taxYear, and dueDate are required")
	}
	if item.Status == "" {
		item.Status = models.ComplianceItemStatusPending
	}
	if err := database.DB.Create(item).Error; err != nil {
		return nil, err
	}
	audit.RecordDetailed(nil, audit.Entry{
		ActorType: "admin", ActorRef: actor,
		Action: "tax.calendar.created", TargetType: "tax_compliance_calendar_item", TargetID: itoa(item.ID),
		Result: audit.ResultSuccess, Details: item.ComplianceType + " due " + item.DueDate.Format("2006-01-02"),
	})
	return item, nil
}

func MarkCalendarItemComplete(id uint, reviewerName, evidence, actor string) error {
	now := time.Now()
	err := database.DB.Model(&models.TaxComplianceCalendarItem{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status": models.ComplianceItemStatusComplete, "completion_date": now, "reviewer_name": reviewerName, "evidence": evidence,
	}).Error
	if err != nil {
		return err
	}
	audit.RecordDetailed(nil, audit.Entry{
		ActorType: "admin", ActorRef: actor,
		Action: "tax.calendar.completed", TargetType: "tax_compliance_calendar_item", TargetID: itoa(id),
		Result: audit.ResultSuccess,
	})
	return nil
}

// RefreshOverdueCalendarItems — flips any PENDING/DUE item past its due
// date to OVERDUE. Never invents or shifts a due date itself.
func RefreshOverdueCalendarItems() int64 {
	result := database.DB.Model(&models.TaxComplianceCalendarItem{}).
		Where("status IN ? AND due_date < ?", []string{models.ComplianceItemStatusPending, models.ComplianceItemStatusDue}, time.Now()).
		Update("status", models.ComplianceItemStatusOverdue)
	return result.RowsAffected
}
