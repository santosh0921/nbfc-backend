// Compliance alerts (item X) + the daily sweep that raises them.
package compliance

import (
	"time"

	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/models"
)

func CreateAlert(alertType, severity, message, refType, refID, owner string) {
	database.DB.Create(&models.ComplianceAlert{
		AlertType: alertType, Severity: severity, Message: message,
		ReferenceType: refType, ReferenceID: refID, OwnerName: owner, Status: models.ComplianceAlertStatusOpen,
	})
}

func ResolveAlert(id uint, resolution string) error {
	now := time.Now()
	return database.DB.Model(&models.ComplianceAlert{}).Where("id = ?", id).
		Updates(map[string]interface{}{"status": models.ComplianceAlertStatusResolved, "resolution": resolution, "resolved_at": now}).Error
}

// RunDailySweep (items D/X) — refreshes overdue calendar items/CAPAs and
// raises alerts for anything newly due-soon/overdue/critical. Idempotent
// in the sense that it only creates an alert when no still-OPEN alert of
// the same type+reference already exists.
func RunDailySweep() int {
	created := 0
	overdueCalendar := RefreshOverdueCalendarItems()
	if overdueCalendar > 0 {
		var items []models.ComplianceCalendarItem
		database.DB.Where("status = ?", models.CalendarStatusOverdue).Find(&items)
		for _, item := range items {
			created += alertOnce("COMPLIANCE_OVERDUE", "CRITICAL",
				"Compliance calendar item #"+itoa(item.ID)+" is overdue (due "+item.DueDate.Format("2006-01-02")+")",
				"compliance_calendar_item", itoa(item.ID), item.OwnerName)
		}
	}

	dueSoonCutoff := time.Now().AddDate(0, 0, 7)
	var dueSoon []models.ComplianceCalendarItem
	database.DB.Where("status NOT IN ? AND due_date BETWEEN ? AND ?",
		[]string{models.CalendarStatusCompleted, models.CalendarStatusOverdue, models.CalendarStatusNotApplicable},
		time.Now(), dueSoonCutoff).Find(&dueSoon)
	for _, item := range dueSoon {
		created += alertOnce("COMPLIANCE_DUE_SOON", "WARNING",
			"Compliance calendar item #"+itoa(item.ID)+" is due "+item.DueDate.Format("2006-01-02"),
			"compliance_calendar_item", itoa(item.ID), item.OwnerName)
	}

	overdueCAPAs := RefreshOverdueCAPAs()
	if overdueCAPAs > 0 {
		var capas []models.CAPA
		database.DB.Where("status = ?", models.CAPAStatusOverdue).Find(&capas)
		for _, c := range capas {
			created += alertOnce("CAPA_OVERDUE", "CRITICAL", "CAPA #"+itoa(c.ID)+" is overdue", "capa", itoa(c.ID), c.OwnerName)
		}
	}

	var criticalFindings []models.AuditFinding
	database.DB.Where("severity = ? AND status = ?", models.FindingSeverityCritical, models.FindingStatusOpen).Find(&criticalFindings)
	for _, f := range criticalFindings {
		created += alertOnce("CRITICAL_FINDING", "CRITICAL", "Critical finding #"+itoa(f.ID)+" ("+f.Title+") is open", "audit_finding", itoa(f.ID), f.OwnerName)
	}

	for _, p := range FindOverduePolicyReviews() {
		created += alertOnce("POLICY_REVIEW_OVERDUE", "WARNING", "Policy #"+itoa(p.ID)+" ("+p.Name+") is overdue for review", "policy", itoa(p.ID), p.OwnerName)
	}

	var expiringEvidence []models.ComplianceEvidence
	soon := time.Now().AddDate(0, 0, 30)
	database.DB.Where("status = ? AND expiry_date IS NOT NULL AND expiry_date < ?", models.EvidenceStatusActive, soon).Find(&expiringEvidence)
	for _, e := range expiringEvidence {
		created += alertOnce("EVIDENCE_EXPIRY", "WARNING", "Evidence #"+itoa(e.ID)+" ("+e.Title+") expires within 30 days", "compliance_evidence", itoa(e.ID), e.UploadedBy)
	}

	return created
}

func alertOnce(alertType, severity, message, refType, refID, owner string) int {
	var existing models.ComplianceAlert
	err := database.DB.Where("alert_type = ? AND reference_type = ? AND reference_id = ? AND status = ?",
		alertType, refType, refID, models.ComplianceAlertStatusOpen).First(&existing).Error
	if err == nil {
		return 0
	}
	CreateAlert(alertType, severity, message, refType, refID, owner)
	return 1
}
