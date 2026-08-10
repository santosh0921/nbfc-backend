package otp

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type OTPVerification struct {
	ID uuid.UUID `gorm:"type:uuid;primaryKey"`
	// Mobile carries a unique index (not just a plain index) so at most one
	// active OTP row can exist per mobile number at the database level,
	// backing up GenerateOTP's delete-then-create with a hard guarantee —
	// two concurrent send-otp calls can no longer both leave a row behind.
	Mobile string `gorm:"size:15;uniqueIndex;not null"`
	// OTPHash is a SHA-256 hex digest of the OTP, never the plaintext code —
	// a database dump or read-replica leak no longer hands over live,
	// unexpired OTP codes. See hashOTP/VerifyOTP.
	OTPHash   string    `gorm:"size:64;not null"`
	Verified  bool      `gorm:"default:false"`
	Attempts  int       `gorm:"default:0"`
	ExpiresAt time.Time `gorm:"not null"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (o *OTPVerification) BeforeCreate(tx *gorm.DB) error {
	o.ID = uuid.New()
	return nil
}