// Policy register + review workflow (items J/K). "Do not claim that a
// policy exists unless evidence is uploaded/recorded" (item J) is
// enforced by CreatePolicy requiring a real EvidenceID (a
// ComplianceEvidence row that must already exist — i.e. the document was
// actually uploaded) before a Policy row can be created at all.
package compliance

import (
	"errors"
	"time"

	"github.com/santosh0921/nbfc-backend/internal/audit"
	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/models"
)

func CreatePolicy(p *models.Policy, actor string) (*models.Policy, error) {
	if p.Name == "" {
		return nil, errors.New("compliance: policy name is required")
	}
	if p.EvidenceID == nil {
		return nil, errors.New("compliance: a policy cannot be recorded without an uploaded evidence document (item J)")
	}
	var ev models.ComplianceEvidence
	if err := database.DB.First(&ev, *p.EvidenceID).Error; err != nil {
		return nil, errors.New("compliance: referenced evidence document not found")
	}
	p.Status = models.PolicyStatusDraft
	p.CreatedBy = actor
	if err := database.DB.Create(p).Error; err != nil {
		return nil, err
	}
	audit.RecordDetailed(nil, audit.Entry{
		ActorType: "admin", ActorRef: actor,
		Action: "compliance.policy.created", TargetType: "policy", TargetID: itoa(p.ID),
		Result: audit.ResultSuccess, Details: p.Name,
	})
	return p, nil
}

// AdvancePolicyStatus (item K's lifecycle: Draft -> Review -> Approval ->
// Effective -> Revision -> Superseded). Every transition is audited.
func AdvancePolicyStatus(id uint, newStatus, actor string) error {
	switch newStatus {
	case models.PolicyStatusDraft, models.PolicyStatusReview, models.PolicyStatusApproval,
		models.PolicyStatusEffective, models.PolicyStatusRevision, models.PolicyStatusSuperseded:
	default:
		return errors.New("compliance: invalid policy status")
	}
	var p models.Policy
	if err := database.DB.First(&p, id).Error; err != nil {
		return err
	}
	before := p.Status
	p.Status = newStatus
	if err := database.DB.Save(&p).Error; err != nil {
		return err
	}
	audit.RecordDetailed(nil, audit.Entry{
		ActorType: "admin", ActorRef: actor,
		Action: "compliance.policy.status_changed", TargetType: "policy", TargetID: itoa(id),
		Result: audit.ResultSuccess, Details: before + " -> " + newStatus,
	})
	return nil
}

// CreateRevision — a new Policy row (new version), never editing the
// prior one in place — same version-preservation convention as evidence.
func CreateRevision(previousID uint, newEvidenceID uint, actor string) (*models.Policy, error) {
	var previous models.Policy
	if err := database.DB.First(&previous, previousID).Error; err != nil {
		return nil, err
	}
	revision := previous
	revision.ID = 0
	revision.Version = previous.Version + 1
	revision.PreviousPolicyID = &previousID
	revision.EvidenceID = &newEvidenceID
	revision.Status = models.PolicyStatusDraft
	revision.CreatedBy = actor
	if err := database.DB.Create(&revision).Error; err != nil {
		return nil, err
	}
	database.DB.Model(&previous).Update("status", models.PolicyStatusSuperseded)

	audit.RecordDetailed(nil, audit.Entry{
		ActorType: "admin", ActorRef: actor,
		Action: "compliance.policy.revised", TargetType: "policy", TargetID: itoa(revision.ID),
		Result: audit.ResultSuccess, Details: "supersedes policy #" + itoa(previousID),
	})
	return &revision, nil
}

// FindOverduePolicyReviews (item K: "track overdue policy reviews").
func FindOverduePolicyReviews() []models.Policy {
	var policies []models.Policy
	database.DB.Where("status = ? AND next_review IS NOT NULL AND next_review < ?", models.PolicyStatusEffective, time.Now()).Find(&policies)
	return policies
}
