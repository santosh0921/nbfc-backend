// Regulatory return tracking (items E/F/G). Distinct from Phase 6's
// RegulatoryReportRun (the generic report-generation workflow with
// validation engine) — this is the compliance-team-facing tracking shape
// item E describes, with maker-checker and its own data-lineage
// exception list (item F). Both can coexist for the same real-world
// filing; this package does not merge them.
package compliance

import (
	"errors"
	"time"

	"github.com/santosh0921/nbfc-backend/internal/audit"
	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/models"
)

var (
	ErrSameActor      = errors.New("compliance: the preparer cannot also review/approve their own submission")
	ErrNotDraft       = errors.New("compliance: return is not in DRAFT/IN_PREPARATION")
	ErrNotApproved    = errors.New("compliance: return must be APPROVED before submission")
	ErrHasOpenExceptions = errors.New("compliance: this return has unresolved data-lineage exceptions and cannot be approved (item F)")
	ErrFrozen         = errors.New("compliance: a SUBMITTED/ACCEPTED return is frozen — create a correction instead")
)

func CreateReturn(ret *models.RegulatoryReturnTracking, preparedByAdminID uint, preparedBy string) (*models.RegulatoryReturnTracking, error) {
	if ret.Regulator == "" || ret.ReturnType == "" || ret.ReportingPeriod == "" {
		return nil, errors.New("compliance: regulator, returnType, and reportingPeriod are required")
	}
	ret.Status = models.ReturnTrackingStatusDraft
	ret.PreparedByAdminID = preparedByAdminID
	ret.PreparedBy = preparedBy
	if err := database.DB.Create(ret).Error; err != nil {
		return nil, err
	}
	audit.RecordDetailed(nil, audit.Entry{
		ActorType: "admin", ActorID: preparedByAdminID, ActorRef: preparedBy,
		Action: "compliance.return.created", TargetType: "regulatory_return_tracking", TargetID: itoa(ret.ID),
		Result: audit.ResultSuccess, Details: ret.ReturnType + " " + ret.ReportingPeriod,
	})
	return ret, nil
}

// ReviewReturn moves DRAFT/IN_PREPARATION to PENDING_REVIEW's next step —
// simplified two-step maker-checker (prepare -> review+approve in one
// checker action), consistent with the lighter-weight pattern already
// used for Phase 5 write-offs. Blocks approval while any
// RegulatoryDataException remains OPEN for this return (item F: "do not
// allow a report to be marked ready without validation").
func ReviewAndApproveReturn(id, reviewerAdminID uint, reviewerName, comments string) (*models.RegulatoryReturnTracking, error) {
	var ret models.RegulatoryReturnTracking
	if err := database.DB.First(&ret, id).Error; err != nil {
		return nil, err
	}
	if ret.Status != models.ReturnTrackingStatusDraft && ret.Status != models.ReturnTrackingStatusInPreparation && ret.Status != models.ReturnTrackingStatusPendingReview {
		return nil, ErrNotDraft
	}
	if ret.PreparedByAdminID == reviewerAdminID {
		return nil, ErrSameActor
	}
	var openExceptions int64
	database.DB.Model(&models.RegulatoryDataException{}).
		Where("return_tracking_id = ? AND status = ?", id, "OPEN").Count(&openExceptions)
	if openExceptions > 0 {
		return nil, ErrHasOpenExceptions
	}

	now := time.Now()
	ret.Status = models.ReturnTrackingStatusApproved
	ret.ReviewedByAdminID = &reviewerAdminID
	ret.ReviewedBy = reviewerName
	ret.ApprovedByAdminID = &reviewerAdminID
	ret.ApprovedBy = reviewerName
	ret.ApprovalDate = &now
	ret.ApprovalComments = comments
	if err := database.DB.Save(&ret).Error; err != nil {
		return nil, err
	}
	audit.RecordDetailed(nil, audit.Entry{
		ActorType: "admin", ActorID: reviewerAdminID, ActorRef: reviewerName,
		Action: "compliance.return.approved", TargetType: "regulatory_return_tracking", TargetID: itoa(ret.ID),
		Result: audit.ResultSuccess, Reason: comments,
	})
	return &ret, nil
}

// SubmitReturn — freezes the row (item Q: "do not allow a report to
// silently change after approval").
func SubmitReturn(id uint, submissionRef, submittedBy string) (*models.RegulatoryReturnTracking, error) {
	var ret models.RegulatoryReturnTracking
	if err := database.DB.First(&ret, id).Error; err != nil {
		return nil, err
	}
	if ret.Status != models.ReturnTrackingStatusApproved {
		return nil, ErrNotApproved
	}
	now := time.Now()
	ret.Status = models.ReturnTrackingStatusSubmitted
	ret.SubmissionRef = submissionRef
	ret.SubmittedBy = submittedBy
	ret.SubmittedDate = &now
	if err := database.DB.Save(&ret).Error; err != nil {
		return nil, err
	}
	audit.RecordDetailed(nil, audit.Entry{
		ActorType: "admin", ActorRef: submittedBy,
		Action: "compliance.return.submitted", TargetType: "regulatory_return_tracking", TargetID: itoa(ret.ID),
		Result: audit.ResultSuccess, Details: submissionRef,
	})
	return &ret, nil
}

// CreateCorrection — the ONLY way to change a SUBMITTED/ACCEPTED return's
// content: a brand-new row linked via SupersedesID, with the original
// left completely untouched.
func CreateCorrection(originalID uint, reason string, correctedByAdminID uint, correctedByName string) (*models.RegulatoryReturnTracking, error) {
	var original models.RegulatoryReturnTracking
	if err := database.DB.First(&original, originalID).Error; err != nil {
		return nil, err
	}
	if original.Status != models.ReturnTrackingStatusSubmitted && original.Status != models.ReturnTrackingStatusAccepted && original.Status != models.ReturnTrackingStatusRejected {
		return nil, errors.New("compliance: corrections are only created against a submitted/accepted/rejected return")
	}
	correction := original
	correction.ID = 0
	correction.Status = models.ReturnTrackingStatusDraft
	correction.SupersedesID = &originalID
	correction.SubmissionRef = ""
	correction.SubmittedBy = ""
	correction.SubmittedDate = nil
	correction.ReviewedByAdminID = nil
	correction.ApprovedByAdminID = nil
	correction.ApprovalDate = nil
	correction.Corrections = reason
	correction.PreparedByAdminID = correctedByAdminID
	correction.PreparedBy = correctedByName
	if err := database.DB.Create(&correction).Error; err != nil {
		return nil, err
	}
	audit.RecordDetailed(nil, audit.Entry{
		ActorType: "admin", ActorID: correctedByAdminID, ActorRef: correctedByName,
		Action: "compliance.return.correction_created", TargetType: "regulatory_return_tracking", TargetID: itoa(correction.ID),
		Result: audit.ResultSuccess, Reason: reason, Details: "supersedes " + itoa(originalID),
	})
	return &correction, nil
}

// RecordDataException (item F) — a detected lineage problem, never
// silently fixed.
func RecordDataException(returnTrackingID uint, exceptionType, detail string) (*models.RegulatoryDataException, error) {
	exc := models.RegulatoryDataException{
		ReturnTrackingID: returnTrackingID, ExceptionType: exceptionType, Detail: detail, Status: "OPEN",
	}
	if err := database.DB.Create(&exc).Error; err != nil {
		return nil, err
	}
	return &exc, nil
}

func ResolveDataException(id uint, resolvedBy string) error {
	now := time.Now()
	return database.DB.Model(&models.RegulatoryDataException{}).Where("id = ?", id).
		Updates(map[string]interface{}{"status": "RESOLVED", "resolved_at": now, "resolved_by": resolvedBy}).Error
}
