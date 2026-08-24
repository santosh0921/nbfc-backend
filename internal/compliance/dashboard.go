// Internal compliance tracking dashboard (item O). Deliberately named
// "Internal Compliance Tracking Score," never "compliance score" or any
// phrase implying legal/regulatory certification — item O is explicit
// that this is a self-reported completion metric over admin-entered
// calendar items, not an external audit outcome.
package compliance

import (
	"time"

	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/models"
)

type ComplianceTrackingDashboard struct {
	TotalRequirements      int64 `json:"totalRequirements"`
	ApplicableRequirements int64 `json:"applicableRequirements"`

	TotalCalendarItems int64 `json:"totalCalendarItems"`
	Completed          int64 `json:"completed"`
	InProgress         int64 `json:"inProgress"`
	DueSoon            int64 `json:"dueSoon"` // within 7 days, not yet completed
	Overdue            int64 `json:"overdue"`
	Exceptions         int64 `json:"exceptions"`

	CriticalFindings int64 `json:"criticalFindings"`
	HighFindings     int64 `json:"highFindings"`
	OpenCAPAs        int64 `json:"openCapas"`
	OverdueCAPAs     int64 `json:"overdueCapas"`

	OverduePolicyReviews int64 `json:"overduePolicyReviews"`

	// InternalComplianceTrackingScorePercent — Completed / ApplicableCalendarItems
	// * 100, where "applicable" excludes NOT_APPLICABLE items. Explicitly
	// NOT a claim of regulatory compliance — see the package doc comment
	// and item O's own instruction.
	InternalComplianceTrackingScorePercent float64 `json:"internalComplianceTrackingScorePercent"`

	GeneratedAt time.Time `json:"generatedAt"`
}

func GenerateDashboard() ComplianceTrackingDashboard {
	var d ComplianceTrackingDashboard
	database.DB.Model(&models.ComplianceRequirement{}).Count(&d.TotalRequirements)
	database.DB.Model(&models.ComplianceRequirement{}).Where("applicability = ?", models.ApplicabilityApplicable).Count(&d.ApplicableRequirements)

	applicableQ := database.DB.Model(&models.ComplianceCalendarItem{}).Where("status != ?", models.CalendarStatusNotApplicable)
	applicableQ.Count(&d.TotalCalendarItems)
	database.DB.Model(&models.ComplianceCalendarItem{}).Where("status = ?", models.CalendarStatusCompleted).Count(&d.Completed)
	database.DB.Model(&models.ComplianceCalendarItem{}).Where("status = ?", models.CalendarStatusInProgress).Count(&d.InProgress)
	database.DB.Model(&models.ComplianceCalendarItem{}).
		Where("status NOT IN ? AND due_date BETWEEN ? AND ?",
			[]string{models.CalendarStatusCompleted, models.CalendarStatusNotApplicable}, time.Now(), time.Now().AddDate(0, 0, 7)).
		Count(&d.DueSoon)
	database.DB.Model(&models.ComplianceCalendarItem{}).Where("status = ?", models.CalendarStatusOverdue).Count(&d.Overdue)
	database.DB.Model(&models.ComplianceCalendarItem{}).Where("status = ?", models.CalendarStatusException).Count(&d.Exceptions)

	database.DB.Model(&models.AuditFinding{}).Where("severity = ? AND status != ?", models.FindingSeverityCritical, models.FindingStatusClosed).Count(&d.CriticalFindings)
	database.DB.Model(&models.AuditFinding{}).Where("severity = ? AND status != ?", models.FindingSeverityHigh, models.FindingStatusClosed).Count(&d.HighFindings)
	database.DB.Model(&models.CAPA{}).Where("status NOT IN ?", []string{models.CAPAStatusClosed, models.CAPAStatusRejected}).Count(&d.OpenCAPAs)
	database.DB.Model(&models.CAPA{}).Where("status = ?", models.CAPAStatusOverdue).Count(&d.OverdueCAPAs)

	d.OverduePolicyReviews = int64(len(FindOverduePolicyReviews()))

	if d.TotalCalendarItems > 0 {
		d.InternalComplianceTrackingScorePercent = float64(d.Completed) / float64(d.TotalCalendarItems) * 100
	}
	d.GeneratedAt = time.Now()
	return d
}
