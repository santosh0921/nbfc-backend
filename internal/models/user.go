package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type User struct {
	ID uuid.UUID `gorm:"type:uuid;primaryKey"`

	Mobile string `gorm:"size:15;uniqueIndex;not null"`

	// Store bcrypt hash of MPIN, never plain MPIN
	MPIN string `gorm:"size:255;not null"`

	IsMobileVerified bool `gorm:"default:false"`

	// Whether user enabled fingerprint/Face ID
	BiometricEnabled bool `gorm:"default:false"`

	IsActive bool `gorm:"default:true"`

	FailedLoginAttempts int        `gorm:"default:0"`
    LockedUntil         *time.Time

	// CibilScore is a deterministic mock score (550-850) generated once per
	// customer at their first loan application and reused for every
	// subsequent application (see internal/loans.getOrCreateCibilScore).
	// 0 means "not generated yet".
	CibilScore int `gorm:"default:0"`

	CreatedAt time.Time
	UpdatedAt time.Time
}

func (u *User) BeforeCreate(tx *gorm.DB) error {
	u.ID = uuid.New()
	return nil
}