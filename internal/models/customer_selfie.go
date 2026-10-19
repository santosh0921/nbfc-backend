package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type CustomerSelfie struct {
	ID uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`

	UserID uuid.UUID `gorm:"type:uuid;not null;uniqueIndex"`

	// ImagePath: legacy column — pre-encryption rows have the raw
	// base64-encoded image inline here. New uploads leave this empty and
	// use StorageReference instead (see internal/storage), matching the
	// pattern already used for Document.DataBase64/StorageReference.
	ImagePath string `gorm:"type:text"`

	// StorageReference is the opaque reference into internal/storage's
	// encrypted-at-rest provider — never a raw filesystem path. Empty for
	// rows uploaded before this field existed (ImagePath holds those).
	StorageReference string `gorm:"size:255"`

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