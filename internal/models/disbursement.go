package models

import "time"

// Disbursement status machine. A loan's own Status only becomes
// LoanStatusDisbursed once the linked Disbursement reaches
// DisbursementStatusSuccess — see internal/disbursement and
// internal/loans/maker_checker_handler.go. Approval by the checker only
// gets a Disbursement to INITIATED/PROCESSING; final SUCCESS requires
// independent confirmation (a provider webhook, or — since no live
// payout-API credentials are configured — a matched bank statement line
// during reconciliation; see internal/reconciliation).
const (
	DisbursementStatusPendingApproval = "PENDING_APPROVAL" // prepared, awaiting checker
	DisbursementStatusApproved        = "APPROVED"         // checker approved; transfer not yet initiated
	DisbursementStatusInitiated       = "INITIATED"        // transfer request sent to provider/bank
	DisbursementStatusProcessing      = "PROCESSING"       // provider/bank acknowledged, in flight
	DisbursementStatusSuccess         = "SUCCESS"          // confirmed complete — UTR captured
	DisbursementStatusFailed          = "FAILED"
	DisbursementStatusReversed        = "REVERSED"
)

// Disbursement is the record of an actual (or actual-once-a-real-rail-is-
// configured) money transfer out to a borrower for one sanctioned loan.
// Distinct from LoanApplication's own Disbursement* fields (added in
// Phase 1 for the maker-checker approval trail) — this table is the
// financial record of the transfer itself, including whichever provider
// handled it and its UTR, and is designed so a real bank/payout API can
// be plugged in later (internal/disbursement.Provider) without a schema
// change.
type Disbursement struct {
	ID         uint   `gorm:"primaryKey" json:"id"`
	LoanID     uint   `gorm:"uniqueIndex;not null" json:"loanId"` // one active disbursement per loan — enforced by this unique index
	CustomerID string `gorm:"index;not null" json:"customerId"`

	AmountPaise int64  `gorm:"not null" json:"amountPaise"`
	Currency    string `gorm:"not null;default:INR" json:"currency"`
	Status      string `gorm:"index;not null" json:"status"`

	// BankAccountReference is deliberately NOT a full account number —
	// masked display form only (e.g. "XXXX1234 · HDFC0001234"), per the
	// "do not expose complete bank account numbers unnecessarily"
	// requirement. No structured bank-account KYC model exists yet
	// (out of scope for this phase); this is admin/ops-entered at
	// prepare time.
	BankAccountReference string `json:"bankAccountReference,omitempty"`

	Provider          string `gorm:"not null" json:"provider"`    // "manual_bank_transfer" today; abstraction allows a real payout API later
	ProviderReference string `json:"providerReference,omitempty"` // provider's own transfer/transaction id, once initiated
	UTR               string `gorm:"index" json:"utr,omitempty"`  // captured once the bank confirms — matched against reconciliation import

	FailureReason string `json:"failureReason,omitempty"`

	// Maker-checker attribution mirrors LoanApplication's own fields
	// (Phase 1) so this table is self-contained for reporting even
	// without joining back to the loan.
	PreparedByAdminID *uint  `json:"preparedByAdminId,omitempty"`
	PreparedBy        string `json:"preparedBy,omitempty"`
	ApprovedByAdminID *uint  `json:"approvedByAdminId,omitempty"`
	ApprovedBy        string `json:"approvedBy,omitempty"`

	InitiatedAt *time.Time `json:"initiatedAt,omitempty"`
	CompletedAt *time.Time `json:"completedAt,omitempty"`
	VerifiedAt  *time.Time `json:"verifiedAt,omitempty"` // when SUCCESS was independently confirmed (webhook or reconciliation match)

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}
