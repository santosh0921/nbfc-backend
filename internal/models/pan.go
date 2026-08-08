package models

import (
	"time"

	"gorm.io/gorm"


	"github.com/google/uuid"
)


type CustomerPAN struct {
	ID uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`

	UserID uuid.UUID `gorm:"type:uuid;not null;uniqueIndex"`

	// PANNumber is no longer written in plaintext (see internal/security) —
	// the column is kept only so any pre-existing rows from before the
	// encryption change remain readable; new rows leave it empty.
	PANNumber string `gorm:"size:10;index"`

	// PANEncrypted holds the AES-GCM ciphertext (base64) of the real PAN
	// number — see internal/security.Encrypt/Decrypt. Never serialize this.
	PANEncrypted string `gorm:"type:text"`

	// PANLast4 is the plaintext last 4 characters, kept unencrypted purely
	// for building a masked display value ("******1234") without a decrypt
	// round-trip.
	PANLast4 string `gorm:"size:4"`

	// PANHash is a SHA-256 hash of the PAN, used for uniqueness/dedup
	// lookups now that the PAN itself is encrypted (ciphertext isn't
	// equality-comparable across rows because of the random GCM nonce).
	PANHash string `gorm:"size:64;index"`

	NameOnPAN string `gorm:"size:150"`

	VerificationStatus string `gorm:"size:20;default:'PENDING'"`
	
	IsVerified bool `gorm:"default:false"`
    // provider details
	////////////////////////////////////////////////////
	VerificationProvider string `gorm:"size:30"`

	VerificationReference string `gorm:"size:100"`
//////////////////////////////////////////////////////
	VerificationRemarks string `gorm:"size:255"`
	VerificationFailureReason string `gorm:"size:255"`



	VerifiedAt *time.Time

	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`

}