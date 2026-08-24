// Package models — Phase 8 expense/vendor management. Genuinely new this
// phase — the Phase 1-7 audit found no existing expense or vendor
// tracking anywhere in this codebase (Phase 4's Chart of Accounts already
// has expense-type accounts like "Operating Expenses"/"Bank Charges", but
// nothing ever posted a real vendor bill against them). Every monetary
// field is integer paise, matching the convention established in Phase 4.
package models

import "time"

// Vendor categories are intentionally NOT an enum — item J says "do not
// hard-code only a few categories," so Category is a free string an admin
// enters, validated only for non-emptiness.
const (
	VendorStatusActive   = "ACTIVE"
	VendorStatusInactive = "INACTIVE"
)

// Vendor — PAN/bank details here carry the same sensitivity as customer
// PAN/bank data (Phase 7 item I classification) and must be masked in
// admin list views the same way (see internal/finance/handler.go).
type Vendor struct {
	ID       uint   `gorm:"primaryKey" json:"vendorId"`
	Name     string `gorm:"not null" json:"name"`
	Category string `json:"category,omitempty"`

	PANEncrypted string `json:"-"`                 // AES-256-GCM, internal/security — never returned raw
	PANLast4     string `json:"panLast4,omitempty"` // display-safe
	GSTIN        string `json:"gstin,omitempty"`

	BankAccountMasked string `json:"bankAccountMasked,omitempty"` // e.g. "XXXXXXXX1234" — same masking convention as models.BankAccount
	IFSC              string `json:"ifsc,omitempty"`

	Status string `gorm:"not null;default:'ACTIVE'" json:"status"`

	CreatedBy string    `json:"createdBy"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// Expense lifecycle (item I + Phase 7's maker-checker reuse, item M here).
const (
	ExpenseStatusPendingApproval = "PENDING_APPROVAL"
	ExpenseStatusApproved        = "APPROVED"
	ExpenseStatusRejected        = "REJECTED"
	ExpenseStatusPaid            = "PAID"
	// ExpenseStatusReversed (Phase 8 gap-closure) — a PAID expense found
	// to be wrong is never edited or deleted; ReverseExpense
	// (internal/finance/expense.go) posts an accounting reversal via
	// accounting.ReverseJournalEntry and moves the expense here, same
	// reversal-not-edit convention as every other posted-journal
	// correction in this codebase (Phase 4/5).
	ExpenseStatusReversed = "REVERSED"
)

// Expense — every field item I lists. AccountCode identifies which
// expense-type ledger account (Chart of Accounts, Phase 4) this posts
// against once approved+paid; left to the admin/config to pick, never
// inferred by this code from Category (that mapping is itself an
// accounting-policy decision requiring CA input — see the Phase 8
// report).
type Expense struct {
	ID          uint      `gorm:"primaryKey" json:"expenseId"`
	ExpenseDate time.Time `gorm:"not null" json:"expenseDate"`
	VendorID    *uint     `gorm:"index" json:"vendorId,omitempty"`
	Category    string    `gorm:"not null" json:"category"`
	Description string    `json:"description,omitempty"`
	InvoiceNumber string  `json:"invoiceNumber,omitempty"`

	GrossAmountPaise int64 `gorm:"not null" json:"grossAmountPaise"`
	TaxAmountPaise   int64 `gorm:"not null;default:0" json:"taxAmountPaise"`
	TDSAmountPaise   int64 `gorm:"not null;default:0" json:"tdsAmountPaise"` // framework only — see internal/tds-style caveat in the Phase 8 report; never auto-computed
	NetPayablePaise  int64 `gorm:"not null" json:"netPayablePaise"`          // Gross + Tax - TDS, computed at creation, never silently recomputed later

	AccountCode string `gorm:"not null" json:"accountCode"` // Chart of Accounts code this expense posts against once paid

	Status         string `gorm:"not null;default:'PENDING_APPROVAL'" json:"status"`
	RejectionReason string `json:"rejectionReason,omitempty"`

	PaymentStatus    string     `json:"paymentStatus,omitempty"` // "" | "PAID"
	PaymentDate      *time.Time `json:"paymentDate,omitempty"`
	PaymentReference string     `json:"paymentReference,omitempty"`

	SupportingDocumentRef string `json:"supportingDocumentRef,omitempty"` // internal/storage reference, Phase 3's encrypted provider

	JournalReference string `json:"journalReference,omitempty"`

	ReversalReason      string     `json:"reversalReason,omitempty"`
	ReversedByAdminID   *uint      `json:"reversedByAdminId,omitempty"`
	ReversedBy          string     `json:"reversedBy,omitempty"`
	ReversedAt          *time.Time `json:"reversedAt,omitempty"`
	ReversalJournalRef  string     `json:"reversalJournalRef,omitempty"`

	CreatedByAdminID  uint       `json:"createdByAdminId"`
	CreatedBy         string     `json:"createdBy"`
	ApprovedByAdminID *uint      `json:"approvedByAdminId,omitempty"`
	ApprovedBy        string     `json:"approvedBy,omitempty"`
	ApprovedAt        *time.Time `json:"approvedAt,omitempty"`

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}
