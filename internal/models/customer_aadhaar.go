package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type CustomerAadhaar struct {
	ID uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`

	UserID uuid.UUID `gorm:"type:uuid;not null;uniqueIndex"`

	MaskedAadhaar string `gorm:"size:20"`
	Last4Digits   string `gorm:"size:4"`
	AadhaarHash string `gorm:"size:255;uniqueIndex"`

	NameOnAadhaar string `gorm:"size:150"`
	NameMatched bool `gorm:"default:false"`

	VerificationStatus string `gorm:"size:20;default:'PENDING'"`

	IsVerified bool `gorm:"default:false"`
    VerificationFailureReason string `gorm:"size:255"`
	VerificationProvider  string `gorm:"size:30"`
	VerificationReference string `gorm:"size:100"`
	VerificationRemarks string `gorm:"size:255"`

	VerifiedAt *time.Time

	CreatedAt time.Time
	UpdatedAt time.Time

	DeletedAt gorm.DeletedAt `gorm:"index"`
}