package compliance

import (
	"errors"
	"time"

	"github.com/santosh0921/nbfc-backend/internal/audit"
	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/models"
)

// itoa/util shared across this package's files — see util.go.

var ErrSourceRequired = errors.New("compliance: a sourceReference (item AB — the official regulatory source this requirement was verified against) is required before it can be activated")

// CreateRequirement — starts DRAFT; Applicability starts REQUIRES_REVIEW
// unless the caller explicitly supplies one (still never defaulted by
// this function to APPLICABLE).
func CreateRequirement(req *models.ComplianceRequirement, createdBy string) (*models.ComplianceRequirement, error) {
	if req.Title == "" || req.Regulator == "" {
		return nil, errors.New("compliance: title and regulator are required")
	}
	if req.Applicability == "" {
		req.Applicability = models.ApplicabilityRequiresReview
	}
	req.Status = models.RequirementStatusDraft
	req.CreatedBy = createdBy
	if err := database.DB.Create(req).Error; err != nil {
		return nil, err
	}
	audit.RecordDetailed(nil, audit.Entry{
		ActorType: "admin", ActorRef: createdBy,
		Action: "compliance.requirement.created", TargetType: "compliance_requirement", TargetID: itoa(req.ID),
		Result: audit.ResultSuccess, Details: req.Title,
	})
	return req, nil
}

// ActivateRequirement — requires SourceReference (item AB), moves DRAFT
// to ACTIVE. Applicability itself is not changed here — that's a
// separate SetApplicability call, since determining applicability and
// activating the requirement record are distinct decisions.
func ActivateRequirement(id uint, actor string) (*models.ComplianceRequirement, error) {
	var req models.ComplianceRequirement
	if err := database.DB.First(&req, id).Error; err != nil {
		return nil, err
	}
	if req.SourceReference == "" {
		return nil, ErrSourceRequired
	}
	now := time.Now()
	req.Status = models.RequirementStatusActive
	req.LastVerifiedDate = &now
	if err := database.DB.Save(&req).Error; err != nil {
		return nil, err
	}
	audit.RecordDetailed(nil, audit.Entry{
		ActorType: "admin", ActorRef: actor,
		Action: "compliance.requirement.activated", TargetType: "compliance_requirement", TargetID: itoa(req.ID),
		Result: audit.ResultSuccess,
	})
	return &req, nil
}

// SetApplicability — always audited with before/after, since item W
// requires "applicability changes" to be tracked explicitly.
func SetApplicability(id uint, applicability, note, actor string) error {
	switch applicability {
	case models.ApplicabilityApplicable, models.ApplicabilityNotApplicable, models.ApplicabilityConditional, models.ApplicabilityRequiresReview:
	default:
		return errors.New("compliance: invalid applicability value")
	}
	var req models.ComplianceRequirement
	if err := database.DB.First(&req, id).Error; err != nil {
		return err
	}
	before := audit.Snapshot(req)
	req.Applicability = applicability
	req.ApplicabilityNote = note
	if err := database.DB.Save(&req).Error; err != nil {
		return err
	}
	audit.RecordDetailed(nil, audit.Entry{
		ActorType: "admin", ActorRef: actor,
		Action: "compliance.requirement.applicability_changed", TargetType: "compliance_requirement", TargetID: itoa(id),
		Result: audit.ResultSuccess, Reason: note, Details: before, After: req,
	})
	return nil
}

// --- Calendar (item D) ---

// CreateCalendarItem — requires the parent requirement to be ACTIVE
// (never generates a calendar item off a DRAFT/unverified requirement).
func CreateCalendarItem(item *models.ComplianceCalendarItem, actor string) (*models.ComplianceCalendarItem, error) {
	var req models.ComplianceRequirement
	if err := database.DB.First(&req, item.RequirementID).Error; err != nil {
		return nil, errors.New("compliance: requirement not found")
	}
	if req.Status != models.RequirementStatusActive {
		return nil, errors.New("compliance: cannot create a calendar item for a non-ACTIVE requirement")
	}
	if item.DueDate.IsZero() {
		return nil, errors.New("compliance: dueDate is required")
	}
	if item.Status == "" {
		item.Status = models.CalendarStatusNotStarted
	}
	if err := database.DB.Create(item).Error; err != nil {
		return nil, err
	}
	audit.RecordDetailed(nil, audit.Entry{
		ActorType: "admin", ActorRef: actor,
		Action: "compliance.calendar.created", TargetType: "compliance_calendar_item", TargetID: itoa(item.ID),
		Result: audit.ResultSuccess, Details: req.Title + " due " + item.DueDate.Format("2006-01-02"),
	})
	return item, nil
}

// UpdateCalendarItemStatus — every status change is audited with the
// before/after status (item W).
func UpdateCalendarItemStatus(id uint, status, remarks, actor string) error {
	var item models.ComplianceCalendarItem
	if err := database.DB.First(&item, id).Error; err != nil {
		return err
	}
	before := item.Status
	item.Status = status
	item.Remarks = remarks
	if status == models.CalendarStatusCompleted {
		now := time.Now()
		item.CompletionDate = &now
	}
	if err := database.DB.Save(&item).Error; err != nil {
		return err
	}
	audit.RecordDetailed(nil, audit.Entry{
		ActorType: "admin", ActorRef: actor,
		Action: "compliance.calendar.status_changed", TargetType: "compliance_calendar_item", TargetID: itoa(id),
		Result: audit.ResultSuccess, Details: before + " -> " + status,
	})
	return nil
}

// RefreshOverdueCalendarItems — flips any not-yet-completed item past its
// due date to OVERDUE. Never shifts or invents a due date.
func RefreshOverdueCalendarItems() int64 {
	result := database.DB.Model(&models.ComplianceCalendarItem{}).
		Where("status IN ? AND due_date < ?",
			[]string{models.CalendarStatusNotStarted, models.CalendarStatusInProgress, models.CalendarStatusPendingReview},
			time.Now()).
		Update("status", models.CalendarStatusOverdue)
	return result.RowsAffected
}
