package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type CustomerSelfie struct {
	ID uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`

	UserID uuid.UUID `gorm:"type:uuid;not null;uniqueIndex"`

	// TEXT rather than varchar(255): this currently stores the raw
	// base64-encoded selfie image inline (no object storage/upload step
	// wired up yet), which is far larger than a typical file path.
	ImagePath string `gorm:"type:text"`

	VerificationStatus string `gorm:"size:30;default:'PENDING'"`

	IsVerified bool `gorm:"default:false"`

	VerificationProvider string `gorm:"size:50"`

	VerificationReference string `gorm:"size:100"`

	VerificationFailureReason string `gorm:"size:255"`

	VerificationRemarks string `gorm:"size:255"`

	VerifiedAt *time.Time

	CreatedAt time.Time
	UpdatedAt time.Time

	DeletedAt gorm.DeletedAt `gorm:"index"`
}