package models

import (
	"time"

	"github.com/google/uuid"
)

const (
	DocumentStatusActive  = "ACTIVE"
	DocumentStatusRemoved = "REMOVED"
)

// Document is a generic uploaded file (KYC document photo, verification
// visit photo, etc.).
//
// Phase 3: new uploads no longer store plaintext file bytes in this
// table at all — StorageReference points at an AES-256-GCM-encrypted
// file written by internal/storage instead (see that package's doc
// comment), and Hash/SizeBytes/MimeType are metadata the database DOES
// still hold, per the "store metadata/reference, not raw bytes"
// requirement. DataBase64 is kept, still populated, ONLY for rows
// created before this change (or if no storage provider is configured —
// see internal/documents/handler.go) — never written to for a document
// that has a StorageReference.
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

	// DataBase64 — legacy inline storage. See type doc comment above:
	// never written for a document that has StorageReference set.
	DataBase64 string `gorm:"type:text" json:"-"`

	// StorageReference is the opaque reference internal/storage.Provider
	// returned for this document's encrypted bytes — never a raw
	// filesystem path or anything else a client could turn into direct
	// file access.
	StorageReference string `json:"storageReference,omitempty"`

	// Hash is SHA-256 of the DECODED (plaintext) file bytes — an
	// integrity check independent of wherever the bytes are actually
	// stored, and a way to detect a duplicate re-upload without needing
	// to decrypt/compare the stored copies.
	Hash      string `gorm:"index" json:"hash,omitempty"`
	SizeBytes int    `json:"sizeBytes,omitempty"`

	Status string `gorm:"default:'ACTIVE'" json:"status"`

	// RetentionExpiry is a placeholder for a data-retention policy — no
	// automated purge job exists yet (out of scope for this phase); the
	// field exists so one can be added without a schema change. See the
	// Phase 3 report for this as a flagged remaining gap.
	RetentionExpiry *time.Time `json:"retentionExpiry,omitempty"`

	UploadedByType string `json:"uploadedByType"` // "customer" | "employee"
	UploadedByRef  string `json:"uploadedByRef"`  // customer user ID (uuid string) or employee code
	CreatedBy      string `json:"createdBy,omitempty"`

	CreatedAt time.Time `json:"createdAt"`

	VerifiedAt *time.Time `json:"verifiedAt,omitempty"`
}
