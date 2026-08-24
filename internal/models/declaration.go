package models

import (
	"time"

	"github.com/google/uuid"
)

// Declaration is a configurable customer declaration (e.g. FATCA/CRS
// self-certification) — Phase 3, item L. Body/Title are placeholder wording
// only, explicitly NOT approved legal text (see the Phase 3 report's
// compliance-validation section); do not present this wording to a real
// customer without CA/CS/legal sign-off first.
type Declaration struct {
	ID      uint   `gorm:"primaryKey" json:"id"`
	Type    string `gorm:"not null" json:"type"` // e.g. "FATCA_CRS"
	Version string `gorm:"not null" json:"version"`
	Title   string `gorm:"not null" json:"title"`
	Body    string `gorm:"type:text;not null" json:"body"`
	Required bool  `gorm:"not null;default:true" json:"required"`
	Active  bool   `gorm:"not null;default:true" json:"active"`

	CreatedAt time.Time `json:"createdAt"`
}

// CustomerDeclarationResponse records one customer's response to one
// Declaration version — never overwritten in place, so history of what was
// actually agreed to (and when) is preserved even if the declaration
// wording is later revised.
type CustomerDeclarationResponse struct {
	ID            uint      `gorm:"primaryKey" json:"id"`
	CustomerID    uuid.UUID `gorm:"type:uuid;index;not null" json:"customerId"`
	DeclarationID uint      `gorm:"not null" json:"declarationId"`

	Response       string `gorm:"not null" json:"response"` // "ACCEPTED" | "DECLINED"
	ConsentRef     string `json:"consentRef,omitempty"`
	IPAddress      string `json:"ipAddress,omitempty"`
	DeviceInfo     string `json:"deviceInfo,omitempty"`

	RespondedAt time.Time `json:"respondedAt"`
}
