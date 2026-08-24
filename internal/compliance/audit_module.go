// Internal audit + finding lifecycle + CAPA (items L/M/N). "Do not
// fabricate audit findings" — this package never generates a finding
// itself; every AuditFinding row is created by an explicit admin action
// describing a real finding, never inferred from system data.
package compliance

import (
	"errors"
	"time"

	"github.com/santosh0921/nbfc-backend/internal/audit"
	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/models"
)

func CreateAudit(a *models.InternalAudit, actor string) (*models.InternalAudit, error) {
	if a.Status == "" {
		a.Status = models.AuditStatusPlanned
	}
	a.CreatedBy = actor
	if err := database.DB.Create(a).Error; err != nil {
		return nil, err
	}
	audit.RecordDetailed(nil, audit.Entry{
		ActorType: "admin", ActorRef: actor,
		Action: "compliance.audit.created", TargetType: "internal_audit", TargetID: itoa(a.ID),
		Result: audit.ResultSuccess, Details: a.AuditType,
	})
	return a, nil
}

func CreateFinding(f *models.AuditFinding, actor string) (*models.AuditFinding, error) {
	if f.Title == "" || f.Severity == "" {
		return nil, errors.New("compliance: finding title and severity are required")
	}
	f.Status = models.FindingStatusOpen
	if err := database.DB.Create(f).Error; err != nil {
		return nil, err
	}
	audit.RecordDetailed(nil, audit.Entry{
		ActorType: "admin", ActorRef: actor,
		Action: "compliance.finding.created", TargetType: "audit_finding", TargetID: itoa(f.ID),
		Result: audit.ResultSuccess, Details: f.Severity + ": " + f.Title,
	})
	return f, nil
}

func RecordManagementResponse(findingID uint, response, respondedBy string) error {
	now := time.Now()
	err := database.DB.Model(&models.AuditFinding{}).Where("id = ?", findingID).Updates(map[string]interface{}{
		"status": models.FindingStatusManagementResponse, "management_response": response,
		"management_responded_by": respondedBy, "management_responded_at": now,
	}).Error
	if err != nil {
		return err
	}
	audit.RecordDetailed(nil, audit.Entry{
		ActorType: "admin", ActorRef: respondedBy,
		Action: "compliance.finding.management_response", TargetType: "audit_finding", TargetID: itoa(findingID),
		Result: audit.ResultSuccess,
	})
	return nil
}

var ErrClosureRequiresEvidence = errors.New("compliance: a finding cannot be closed without verification evidence and a verifier name (item M)")

// CloseFinding — enforces item M's explicit requirement: verification
// evidence + a reviewer (VerifiedBy) are mandatory before Status can
// become CLOSED.
func CloseFinding(findingID uint, verificationEvidence, verifiedBy, actor string) error {
	if verificationEvidence == "" || verifiedBy == "" {
		return ErrClosureRequiresEvidence
	}
	var f models.AuditFinding
	if err := database.DB.First(&f, findingID).Error; err != nil {
		return err
	}
	now := time.Now()
	f.Status = models.FindingStatusClosed
	f.VerificationEvidence = verificationEvidence
	f.VerifiedBy = verifiedBy
	f.VerifiedAt = &now
	f.ClosedAt = &now
	if err := database.DB.Save(&f).Error; err != nil {
		return err
	}
	audit.RecordDetailed(nil, audit.Entry{
		ActorType: "admin", ActorRef: actor,
		Action: "compliance.finding.closed", TargetType: "audit_finding", TargetID: itoa(findingID),
		Result: audit.ResultSuccess, Details: "verified by " + verifiedBy,
	})
	return nil
}

// --- CAPA ---

func CreateCAPA(c *models.CAPA, actor string) (*models.CAPA, error) {
	if c.Action == "" {
		return nil, errors.New("compliance: CAPA action description is required")
	}
	c.Status = models.CAPAStatusOpen
	if err := database.DB.Create(c).Error; err != nil {
		return nil, err
	}
	audit.RecordDetailed(nil, audit.Entry{
		ActorType: "admin", ActorRef: actor,
		Action: "compliance.capa.created", TargetType: "capa", TargetID: itoa(c.ID),
		Result: audit.ResultSuccess, Details: "finding #" + itoa(c.FindingID),
	})
	return c, nil
}

func UpdateCAPAStatus(id uint, status, actor string) error {
	switch status {
	case models.CAPAStatusOpen, models.CAPAStatusInProgress, models.CAPAStatusPendingVerification,
		models.CAPAStatusClosed, models.CAPAStatusOverdue, models.CAPAStatusRejected:
	default:
		return errors.New("compliance: invalid CAPA status")
	}
	updates := map[string]interface{}{"status": status}
	if status == models.CAPAStatusClosed {
		now := time.Now()
		updates["closed_at"] = now
	}
	if err := database.DB.Model(&models.CAPA{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		return err
	}
	audit.RecordDetailed(nil, audit.Entry{
		ActorType: "admin", ActorRef: actor,
		Action: "compliance.capa.status_changed", TargetType: "capa", TargetID: itoa(id),
		Result: audit.ResultSuccess, Details: status,
	})
	return nil
}

// RefreshOverdueCAPAs — mirrors the calendar-item pattern; only flips
// status based on an already-set TargetDate, never invents one.
func RefreshOverdueCAPAs() int64 {
	result := database.DB.Model(&models.CAPA{}).
		Where("status IN ? AND target_date < ?", []string{models.CAPAStatusOpen, models.CAPAStatusInProgress}, time.Now()).
		Update("status", models.CAPAStatusOverdue)
	return result.RowsAffected
}
