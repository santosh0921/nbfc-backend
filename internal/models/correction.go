package models

import (
	"time"

	"github.com/google/uuid"
)

const (
	CorrectionDomainKYC    = "KYC"
	CorrectionDomainCredit = "CREDIT"

	CorrectionStatusOpen       = "OPEN"
	CorrectionStatusInProgress = "IN_PROGRESS"
	CorrectionStatusResolved   = "RESOLVED"
)

// CorrectionCase is raised whenever a KYC reviewer rejects a customer, or a
// credit officer flags a bureau result, with a reason that needs a human
// follow-up — a document that's missing or unreadable, a customer who
// didn't cooperate with verification, a bureau/income mismatch, etc. It is
// the queue the Correction Portal works from: a correction employee calls
// or otherwise follows up with the customer using the full context
// (customer info + documents, fetched live from the existing admin
// endpoints by CustomerID — never duplicated here), then resolves the
// case, which re-opens the original KYC/Credit review for a second look.
type CorrectionCase struct {
	ID uint `gorm:"primaryKey" json:"id"`

	CustomerID uuid.UUID `gorm:"type:uuid;not null;index" json:"customerId"`
	LoanID     *uint     `gorm:"index" json:"loanId,omitempty"`
	// LoanCategory is copied from the loan at creation time — informational
	// only, so the Correction Portal can filter/group by category (Instant
	// Loan, Personal Loan, Housing Loan, ...) without a second round-trip.
	LoanCategory string `json:"loanCategory,omitempty"`

	Domain       string `gorm:"size:10;not null;index" json:"domain"` // KYC | CREDIT
	ReasonCode   string `gorm:"size:60;not null" json:"reasonCode"`
	ReasonDetail string `json:"reasonDetail"`

	// DocCompletenessAtRaise snapshots how many of the 3 required KYC
	// documents (PAN/Aadhaar/Selfie) the customer had submitted at the
	// moment this case was raised — KYC domain only, always 0 for CREDIT.
	// The Correction Portal compares this against the customer's current
	// count to warn (not block) a resolve when nothing has actually
	// changed since — a correction can legitimately be resolved by phone
	// call with no new document, so this is a nudge, not a gate.
	DocCompletenessAtRaise int `json:"docCompletenessAtRaise"`

	// SourceRef points back at the record this correction resolves —
	// empty for KYC (one CustomerKYCReview per customer, found by
	// CustomerID alone), the BureauRequest ID for CREDIT (a customer can
	// have several bureau pulls over time, so the specific one matters).
	SourceRef string `json:"sourceRef,omitempty"`

	Status string `gorm:"size:20;not null;default:'OPEN';index" json:"status"`

	RaisedByAdminID  uint   `json:"raisedByAdminId"`
	RaisedByUsername string `json:"raisedByUsername"`
	RaisedByRole     string `json:"raisedByRole"`

	AssignedToAdminID  *uint  `json:"assignedToAdminId,omitempty"`
	AssignedToUsername string `json:"assignedToUsername,omitempty"`

	ResolutionNotes    string     `json:"resolutionNotes,omitempty"`
	ResolvedByAdminID  *uint      `json:"resolvedByAdminId,omitempty"`
	ResolvedByUsername string     `json:"resolvedByUsername,omitempty"`
	ResolvedAt         *time.Time `json:"resolvedAt,omitempty"`

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}
