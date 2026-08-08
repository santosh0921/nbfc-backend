package models

import (
	"time"

	"github.com/google/uuid"
)

// CustomerKYCReview holds the admin's overall approve/reject decision for a
// customer's KYC, separate from the per-document verification status
// produced by Surepass (PAN/Aadhaar/Selfie). One row per user.
type CustomerKYCReview struct {
	ID uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`

	UserID uuid.UUID `gorm:"type:uuid;not null;uniqueIndex"`

	Status string `gorm:"size:20;not null;default:'PENDING'"`

	ReviewedBy string `gorm:"size:100"`

	ReviewedAt *time.Time

	RejectionReason string `gorm:"size:255"`

	CreatedAt time.Time
	UpdatedAt time.Time
}
