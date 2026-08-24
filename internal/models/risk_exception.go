package models

import "time"

// DataConsistencyException (item T) — one detected mismatch between the
// loan schedule, verified payments, accounting ledger, DPD, and
// classification/provisioning records. Deliberately its own table, not
// reused from Phase 2's ReconciliationException, since that one is scoped
// to bank-statement-vs-Disbursement matching — this one is scoped to
// cross-system risk/accounting data consistency (a different check
// entirely, even though both feed an "Exceptions" admin screen).
const (
	ExceptionTypeNegativeOutstanding   = "NEGATIVE_OUTSTANDING"
	ExceptionTypePaidStillOverdue      = "PAID_EMI_STILL_OVERDUE"
	ExceptionTypeOverdueMismatch       = "OVERDUE_AMOUNT_MISMATCH"
	ExceptionTypeLedgerLoanMismatch    = "LEDGER_LOAN_MISMATCH"
	ExceptionTypeDuplicateAllocation   = "DUPLICATE_PAYMENT_ALLOCATION"
	ExceptionTypeDuplicateProvision    = "DUPLICATE_PROVISION"
	ExceptionTypeInvalidClassification = "INVALID_CLASSIFICATION"
	ExceptionTypeMissingJournalRef     = "MISSING_JOURNAL_REFERENCE"

	ExceptionStatusOpen     = "OPEN"
	ExceptionStatusResolved = "RESOLVED"
)

type DataConsistencyException struct {
	ID            uint   `gorm:"primaryKey" json:"exceptionId"`
	ExceptionType string `gorm:"not null;index" json:"exceptionType"`
	LoanID        *uint  `gorm:"index" json:"loanId,omitempty"`
	Detail        string `gorm:"type:text;not null" json:"detail"`
	Status        string `gorm:"not null;default:'OPEN';index" json:"status"`

	DetectedAt time.Time  `json:"detectedAt"`
	ResolvedBy string     `json:"resolvedBy,omitempty"`
	ResolvedAt *time.Time `json:"resolvedAt,omitempty"`
	ResolutionNote string  `json:"resolutionNote,omitempty"`
}
