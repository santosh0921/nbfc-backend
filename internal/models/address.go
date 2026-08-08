package models

import (
	"time"

	"github.com/google/uuid"
)

type CustomerAddress struct {
	ID uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`

	UserID uuid.UUID `gorm:"type:uuid;not null;uniqueIndex"`

	CurrentAddress   string
	CurrentState     string
	CurrentDistrict  string
	CurrentCity      string
	CurrentPincode   string

	PermanentAddress string
	PermanentState   string
	PermanentDistrict string
	PermanentCity    string
	PermanentPincode string

	CreatedAt time.Time
	UpdatedAt time.Time
}