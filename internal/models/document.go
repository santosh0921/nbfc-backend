package models

import (
	"time"

	"github.com/google/uuid"
)

// Document is a generic uploaded file (KYC document photo, verification
// visit photo, etc.) stored inline as base64 — same precedent as
// CustomerSelfie.ImagePath, fine for this demo's scale.
type Document struct {
	ID uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`

	CustomerID uuid.UUID `gorm:"type:uuid;index;not null" json:"customerId"`

	// LoanID is nullable — set when the document relates to a specific
	// loan application (employee-captured verification docs always have
	// this; customer KYC uploads may or may not).
	LoanID *uint `gorm:"index" json:"loanId,omitempty"`

	DocType string `json:"docType"`

	FileName string `json:"fileName"`

	MimeType string `json:"mimeType"`

	// TEXT: holds the base64-encoded file content inline (no object
	// storage/upload step wired up yet) — same approach as
	// CustomerSelfie.ImagePath.
	DataBase64 string `gorm:"type:text" json:"dataBase64"`

	UploadedByType string `json:"uploadedByType"` // "customer" | "employee"
	UploadedByRef  string `json:"uploadedByRef"`  // customer user ID (uuid string) or employee code

	CreatedAt time.Time `json:"createdAt"`
}
