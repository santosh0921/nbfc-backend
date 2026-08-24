// Regulatory reporting framework (item C/D/N/P). No specific RBI/NBFC
// return is hard-coded — CreateDefinition only records whatever
// reportType/authority/frequency an authorized admin enters, and this
// package never seeds one itself (see seed.go). Every run carries the
// full maker-checker + versioning trail item C/D/N/P calls for.
package reporting

import (
	"errors"
	"time"

	"gorm.io/gorm"

	"github.com/santosh0921/nbfc-backend/internal/audit"
	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/models"
)

var (
	ErrDefinitionNotFound = errors.New("reporting: report definition not found")
	ErrRunNotOpen         = errors.New("reporting: run is not in a state that allows this action")
	ErrSameActor          = errors.New("reporting: the preparer of a report cannot also approve/reject it")
	ErrBlockingErrors     = errors.New("reporting: this report has unresolved blocking ERROR-severity validation findings and cannot be approved")
	ErrImmutableVersion   = errors.New("reporting: an APPROVED or SUBMITTED report run cannot be regenerated in place — create a new run instead")
)

// CreateDefinition — requires an authorized compliance/admin permission
// (enforced by the HTTP handler). Starts CONFIGURED_NOT_VALIDATED (item
// T) — only a compliance officer flipping it to VALIDATED/PRODUCTION_READY
// via UpdateDefinitionStatus (a deliberate, explicit, separate action)
// changes that.
func CreateDefinition(reportType, authority, description, frequency, createdBy string) (*models.RegulatoryReportDefinition, error) {
	if reportType == "" {
		return nil, errors.New("reporting: reportType is required")
	}
	def := models.RegulatoryReportDefinition{
		ReportType: reportType, ReportingAuthority: authority, Description: description, Frequency: frequency,
		Status: models.DefinitionStatusConfiguredNotValidated, CreatedBy: createdBy,
	}
	if err := database.DB.Create(&def).Error; err != nil {
		return nil, err
	}
	audit.RecordDetailed(nil, audit.Entry{
		ActorType: "admin", ActorRef: createdBy,
		Action: "reporting.definition.created", TargetType: "regulatory_report_definition", TargetID: itoa(def.ID),
		Result: audit.ResultSuccess, Details: reportType,
	})
	return &def, nil
}

func UpdateDefinitionStatus(definitionID uint, status, actor string) error {
	switch status {
	case models.DefinitionStatusConfiguredNotValidated, models.DefinitionStatusValidated, models.DefinitionStatusProductionReady:
	default:
		return errors.New("reporting: invalid definition status")
	}
	var def models.RegulatoryReportDefinition
	if err := database.DB.First(&def, definitionID).Error; err != nil {
		return ErrDefinitionNotFound
	}
	before := def.Status
	def.Status = status
	database.DB.Save(&def)
	audit.RecordDetailed(nil, audit.Entry{
		ActorType: "admin", ActorRef: actor,
		Action: "reporting.definition.status_changed", TargetType: "regulatory_report_definition", TargetID: itoa(def.ID),
		Result: audit.ResultSuccess, Details: before + " -> " + status,
	})
	return nil
}

// CreateVersion — a new immutable version snapshot of a definition's
// field mapping (item N). Fields are supplied by the caller; this
// function only persists them, it does not infer a mapping.
func CreateVersion(definitionID uint, versionLabel, notes, createdBy string, fields []models.RegulatoryReportField) (*models.RegulatoryReportVersion, error) {
	var def models.RegulatoryReportDefinition
	if err := database.DB.First(&def, definitionID).Error; err != nil {
		return nil, ErrDefinitionNotFound
	}
	version := models.RegulatoryReportVersion{
		DefinitionID: definitionID, VersionLabel: versionLabel, Notes: notes, CreatedBy: createdBy,
	}
	err := database.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&version).Error; err != nil {
			return err
		}
		for i := range fields {
			fields[i].DefinitionVersionID = version.ID
		}
		if len(fields) > 0 {
			return tx.Create(&fields).Error
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &version, nil
}

// GenerateRun (item C: "Report Generated -> Prepared By") — creates a new
// RegulatoryReportRun bound to a frozen DefinitionVersionID and runs the
// validation engine immediately, so a run's ValidationStatus (via its
// exceptions) is known before anyone attempts to review it. dataJSON is
// the caller-supplied, already-assembled report payload (this package
// does not know how to build every possible regulatory return's content —
// that assembly logic lives with whichever admin tool/report builder
// produces dataJSON; this function's job is the workflow/audit
// scaffolding around it).
func GenerateRun(definitionID, versionID uint, reportingPeriod, parametersJSON, preparedBy string, preparedByAdminID uint) (*models.RegulatoryReportRun, error) {
	now := time.Now()
	run := models.RegulatoryReportRun{
		DefinitionID: definitionID, DefinitionVersionID: versionID, ReportingPeriod: reportingPeriod,
		Status: models.RegReportStatusGenerated, Parameters: parametersJSON,
		PreparedByAdminID: preparedByAdminID, PreparedBy: preparedBy, GeneratedAt: &now,
	}
	if err := database.DB.Create(&run).Error; err != nil {
		return nil, err
	}
	audit.RecordDetailed(nil, audit.Entry{
		ActorType: "admin", ActorID: preparedByAdminID, ActorRef: preparedBy,
		Action: "reporting.run.generated", TargetType: "regulatory_report_run", TargetID: itoa(run.ID),
		Result: audit.ResultSuccess, Details: reportingPeriod,
	})
	RunValidation(run.ID)
	return &run, nil
}

// SubmitForReview moves a GENERATED run to UNDER_REVIEW.
func SubmitForReview(runID uint, actor string) error {
	var run models.RegulatoryReportRun
	if err := database.DB.First(&run, runID).Error; err != nil {
		return ErrRunNotOpen
	}
	if run.Status != models.RegReportStatusGenerated {
		return ErrRunNotOpen
	}
	run.Status = models.RegReportStatusUnderReview
	database.DB.Save(&run)
	audit.RecordDetailed(nil, audit.Entry{
		ActorType: "admin", ActorRef: actor,
		Action: "reporting.run.submitted_for_review", TargetType: "regulatory_report_run", TargetID: itoa(run.ID),
		Result: audit.ResultSuccess,
	})
	return nil
}

// ReviewRun (item P) — the checker's decision. Refuses the same admin who
// prepared the run (item P: "the preparer cannot independently approve
// their own regulatory report"). Approval is blocked by any unresolved
// ERROR-severity exception (item E).
func ReviewRun(runID, reviewerAdminID uint, reviewerName, decision, reason string) (*models.RegulatoryReportRun, error) {
	var run models.RegulatoryReportRun
	if err := database.DB.First(&run, runID).Error; err != nil {
		return nil, ErrRunNotOpen
	}
	if run.Status != models.RegReportStatusUnderReview {
		return nil, ErrRunNotOpen
	}
	if run.PreparedByAdminID == reviewerAdminID {
		return nil, ErrSameActor
	}
	if decision == models.RegReportStatusApproved {
		if hasBlockingErrors(runID) {
			return nil, ErrBlockingErrors
		}
	}

	now := time.Now()
	run.CheckedByAdminID = &reviewerAdminID
	run.CheckedBy = reviewerName
	run.ReviewedAt = &now
	run.ReviewDecision = decision
	run.ReviewReason = reason
	if decision == models.RegReportStatusApproved {
		run.Status = models.RegReportStatusApproved
	} else {
		run.Status = models.RegReportStatusRejected
	}
	database.DB.Save(&run)

	database.DB.Create(&models.RegulatoryReportReview{
		RunID: runID, ReviewerAdminID: reviewerAdminID, ReviewerName: reviewerName,
		Decision: decision, Reason: reason,
	})
	audit.RecordDetailed(nil, audit.Entry{
		ActorType: "admin", ActorID: reviewerAdminID, ActorRef: reviewerName,
		Action: "reporting.run.reviewed", TargetType: "regulatory_report_run", TargetID: itoa(run.ID),
		Result: audit.ResultSuccess, Reason: reason, Details: decision,
	})
	return &run, nil
}

// SubmitRun (item C/D) — marks an APPROVED run SUBMITTED. This codebase
// never files to a regulator's API directly — submissionReference is a
// manually-entered acknowledgment number for whatever offline/portal
// filing process the NBFC actually uses.
func SubmitRun(runID, submittedByAdminID uint, submittedByName, submissionReference, notes string) (*models.RegulatoryReportRun, error) {
	var run models.RegulatoryReportRun
	if err := database.DB.First(&run, runID).Error; err != nil {
		return nil, ErrRunNotOpen
	}
	if run.Status != models.RegReportStatusApproved {
		return nil, ErrRunNotOpen
	}
	now := time.Now()
	run.Status = models.RegReportStatusSubmitted
	run.SubmittedAt = &now
	run.SubmissionReference = submissionReference
	database.DB.Save(&run)

	database.DB.Create(&models.RegulatoryReportSubmission{
		RunID: runID, SubmittedByAdminID: submittedByAdminID, SubmittedByName: submittedByName,
		SubmissionReference: submissionReference, Notes: notes,
	})
	audit.RecordDetailed(nil, audit.Entry{
		ActorType: "admin", ActorID: submittedByAdminID, ActorRef: submittedByName,
		Action: "reporting.run.submitted", TargetType: "regulatory_report_run", TargetID: itoa(run.ID),
		Result: audit.ResultSuccess, Details: submissionReference,
	})
	return &run, nil
}

// CloseRun (item D) — the final state; once CLOSED, the run's
// Parameters/ResultRef/ChecksumSHA256 are permanently fixed (no code path
// in this package updates them after this point — item N).
func CloseRun(runID uint, actor string) error {
	var run models.RegulatoryReportRun
	if err := database.DB.First(&run, runID).Error; err != nil {
		return ErrRunNotOpen
	}
	if run.Status != models.RegReportStatusSubmitted {
		return ErrRunNotOpen
	}
	run.Status = models.RegReportStatusClosed
	database.DB.Save(&run)
	audit.RecordDetailed(nil, audit.Entry{
		ActorType: "admin", ActorRef: actor,
		Action: "reporting.run.closed", TargetType: "regulatory_report_run", TargetID: itoa(run.ID),
		Result: audit.ResultSuccess,
	})
	return nil
}

func hasBlockingErrors(runID uint) bool {
	var count int64
	database.DB.Model(&models.RegulatoryReportException{}).
		Where("run_id = ? AND severity = ?", runID, models.ReportValidationError).Count(&count)
	return count > 0
}
