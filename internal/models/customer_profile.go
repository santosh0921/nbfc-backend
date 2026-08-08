package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type CustomerProfile struct {
	ID uuid.UUID `gorm:"type:uuid;primaryKey"`

	UserID uuid.UUID `gorm:"type:uuid;uniqueIndex;not null"`

	FirstName string `gorm:"size:100"`
	LastName  string `gorm:"size:100"`

	Email string `gorm:"size:150"`

	DateOfBirth *time.Time

	Gender string `gorm:"size:20"`

	MaritalStatus string `gorm:"size:30"`

	Occupation string `gorm:"size:100"`

	ProfileCompleted bool `gorm:"default:false"`

	CreatedAt time.Time
	UpdatedAt time.Time
}

func (c *CustomerProfile) BeforeCreate(tx *gorm.DB) error {
	c.ID = uuid.New()
	return nil
}