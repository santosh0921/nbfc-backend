// Package compliance implements the Phase 11 NBFC compliance-management
// and audit-evidence system.
//
// This package is itself NOT proof of regulatory compliance (item AC) —
// it tracks compliance activity (requirements, calendar items, evidence,
// findings, CAPAs) that a human compliance/audit function performs and
// records. Nothing here determines applicability of a real RBI
// requirement to this NBFC; every applicability/requirement/due-date
// value is admin-entered after independent verification, and this
// package seeds NO requirement, profile-field, or return rows — see
// seed.go's doc comment.
package compliance

import (
	"errors"
	"time"

	"github.com/santosh0921/nbfc-backend/internal/audit"
	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/models"
)

// GetOrCreateProfile — single-row NBFCProfile, created empty
// (REQUIRES_REVIEW) on first access if it doesn't exist yet. Every field
// starts blank; nothing is guessed from application data.
func GetOrCreateProfile() (*models.NBFCProfile, error) {
	var p models.NBFCProfile
	err := database.DB.First(&p).Error
	if err == nil {
		return &p, nil
	}
	p = models.NBFCProfile{ConfirmationStatus: models.ApplicabilityRequiresReview}
	if err := database.DB.Create(&p).Error; err != nil {
		return nil, err
	}
	return &p, nil
}

// UpdateProfile — a routine edit resets ConfirmationStatus back to
// REQUIRES_REVIEW (never silently kept "confirmed" after a change) —
// confirming an edited profile is always a separate, explicit
// ConfirmProfile call.
func UpdateProfile(updates map[string]interface{}, actor string) (*models.NBFCProfile, error) {
	p, err := GetOrCreateProfile()
	if err != nil {
		return nil, err
	}
	updates["confirmation_status"] = models.ApplicabilityRequiresReview
	updates["confirmed_by"] = ""
	updates["confirmed_at"] = nil
	updates["updated_by"] = actor
	if err := database.DB.Model(p).Updates(updates).Error; err != nil {
		return nil, err
	}
	audit.RecordDetailed(nil, audit.Entry{
		ActorType: "admin", ActorRef: actor,
		Action: "compliance.profile.updated", TargetType: "nbfc_profile", TargetID: "1",
		Result: audit.ResultSuccess,
	})
	database.DB.First(p)
	return p, nil
}

// ConfirmProfile — an explicit, separate action (item A) confirming the
// currently-saved profile fields are accurate. Requires a non-empty
// confirmedBy.
func ConfirmProfile(confirmedBy string) (*models.NBFCProfile, error) {
	if confirmedBy == "" {
		return nil, errors.New("compliance: a confirming admin name is required")
	}
	p, err := GetOrCreateProfile()
	if err != nil {
		return nil, err
	}
	now := time.Now()
	p.ConfirmationStatus = models.ApplicabilityApplicable
	p.ConfirmedBy = confirmedBy
	p.ConfirmedAt = &now
	if err := database.DB.Save(p).Error; err != nil {
		return nil, err
	}
	audit.RecordDetailed(nil, audit.Entry{
		ActorType: "admin", ActorRef: confirmedBy,
		Action: "compliance.profile.confirmed", TargetType: "nbfc_profile", TargetID: "1",
		Result: audit.ResultSuccess,
	})
	return p, nil
}
