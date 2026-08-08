package otp

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type OTPVerification struct {
	ID         uuid.UUID `gorm:"type:uuid;primaryKey"`
	Mobile     string    `gorm:"size:15;index;not null"`
	OTP        string    `gorm:"size:6;not null"`
	Verified   bool      `gorm:"default:false"`
	Attempts   int       `gorm:"default:0"`
	ExpiresAt  time.Time `gorm:"not null"`
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

func (o *OTPVerification) BeforeCreate(tx *gorm.DB) error {
	o.ID = uuid.New()
	return nil
}