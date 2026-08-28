package models

import (
	"time"

	"github.com/google/uuid"
)

const (
	ComplaintStatusOpen       = "open"
	ComplaintStatusInProgress = "in_progress"
	ComplaintStatusResolved   = "resolved"
	ComplaintStatusClosed     = "closed"
)

// Complaint is a customer-raised grievance — distinct from a SupportThread
// (a free-form chat) in that it carries a category, a fixed status
// lifecycle, and an admin-facing resolution note, matching how the
// customer app's "Raise a Complaint" form already presented this to
// customers before it turned out nothing behind that form was real (the
// previous implementation kept complaints in an in-memory list on the
// device, never sent anywhere — this table is what that should have been
// hitting all along).
type Complaint struct {
	ID          uint       `gorm:"primaryKey" json:"id"`
	CustomerID  uuid.UUID  `gorm:"type:uuid;index;not null" json:"customerId"`
	Category    string     `gorm:"size:64;not null" json:"category"`
	Description string     `gorm:"type:text;not null" json:"description"`
	Status      string     `gorm:"size:20;not null;default:open;index" json:"status"`
	AdminNote   string     `gorm:"type:text" json:"adminNote,omitempty"`
	ResolvedAt  *time.Time `json:"resolvedAt,omitempty"`
	CreatedAt   time.Time  `json:"createdAt"`
	UpdatedAt   time.Time  `json:"updatedAt"`
}
