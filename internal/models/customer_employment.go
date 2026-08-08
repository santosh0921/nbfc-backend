package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type CustomerEmployment struct {
	ID uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`

	UserID uuid.UUID `gorm:"type:uuid;not null;uniqueIndex"`

	EmploymentType string `gorm:"size:30"`

	CompanyName string `gorm:"size:150"`

	Occupation string `gorm:"size:100"`

	MonthlyIncome float64

	WorkEmail string `gorm:"size:150"`

	ExperienceYears int

	SalaryDay int

	CreatedAt time.Time
	UpdatedAt time.Time

	DeletedAt gorm.DeletedAt `gorm:"index"`
}