// Package models — Phase 4 accounting foundation. Every monetary field in
// this file is an integer number of paise (int64), the same
// never-use-float-for-money convention already established for
// Disbursement/Payment (Phase 2) — NOT a floating-point rupee value.
package models

import "time"

// Account types — the five classical categories. NormalBalance is derived
// from AccountType by NormalBalanceFor(), not stored, so it can never drift
// out of sync with the type.
const (
	AccountTypeAsset     = "ASSET"
	AccountTypeLiability = "LIABILITY"
	AccountTypeEquity    = "EQUITY"
	AccountTypeIncome    = "INCOME"
	AccountTypeExpense   = "EXPENSE"
)

const (
	AccountStatusActive   = "ACTIVE"
	AccountStatusInactive = "INACTIVE"
)

// NormalBalanceFor reports which side (DEBIT/CREDIT) increases a balance
// for a given account type — Assets/Expenses are debit-normal,
// Liabilities/Equity/Income are credit-normal. Used by reports, never
// stored on the row itself.
func NormalBalanceFor(accountType string) string {
	switch accountType {
	case AccountTypeAsset, AccountTypeExpense:
		return "DEBIT"
	default:
		return "CREDIT"
	}
}

// LedgerAccount is one node in the Chart of Accounts (item A). The
// business logic never hard-codes an account's numeric ID — every posting
// function looks accounts up by AccountCode (see internal/accounting
// package), so the chart can be extended/reconfigured without a code
// change.
type LedgerAccount struct {
	ID              uint    `gorm:"primaryKey" json:"accountId"`
	AccountCode     string  `gorm:"uniqueIndex;not null" json:"accountCode"`
	AccountName     string  `gorm:"not null" json:"accountName"`
	AccountType     string  `gorm:"not null;index" json:"accountType"`
	ParentAccountID *uint   `gorm:"index" json:"parentAccountId,omitempty"`
	Description     string  `json:"description,omitempty"`
	Status          string  `gorm:"not null;default:'ACTIVE'" json:"status"`

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// AccountingPeriod (item M). Only OPEN periods accept new postings —
// SOFT_CLOSED is a documented extension point (e.g. "closed to routine
// entries but still open for period-end adjustments") not yet enforced
// differently from OPEN in this phase; CLOSED periods reject all posting.
const (
	PeriodStatusOpen       = "OPEN"
	PeriodStatusSoftClosed = "SOFT_CLOSED"
	PeriodStatusClosed     = "CLOSED"
	PeriodStatusReopened   = "REOPENED"
)

type AccountingPeriod struct {
	ID        uint      `gorm:"primaryKey" json:"periodId"`
	Code      string    `gorm:"uniqueIndex;not null" json:"code"` // e.g. "2026-08"
	StartDate time.Time `gorm:"not null" json:"startDate"`
	EndDate   time.Time `gorm:"not null" json:"endDate"`
	Status    string    `gorm:"not null;default:'OPEN'" json:"status"`

	ClosedBy     string     `json:"closedBy,omitempty"`
	ClosedAt     *time.Time `json:"closedAt,omitempty"`
	ReopenedBy   string     `json:"reopenedBy,omitempty"`
	ReopenedAt   *time.Time `json:"reopenedAt,omitempty"`
	ReopenReason string     `json:"reopenReason,omitempty"`

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// JournalEntry statuses. POSTED entries are immutable (item C/T) — a
// correction is always a new REVERSAL entry, never an edit. DRAFT exists
// only as a schema extension point; AccountingService.PostJournalEntry
// today always posts atomically (validates balance, then commits POSTED
// in one DB transaction) rather than leaving a journal in DRAFT, per the
// "prevent partial journal posting" requirement.
const (
	JournalStatusPosted   = "POSTED"
	JournalStatusReversed = "REVERSED" // this entry has since been reversed by another entry
	JournalStatusReversal = "REVERSAL" // this entry IS a reversal of another entry
)

// JournalEntry is one atomic double-entry posting. IdempotencyKey is the
// single mechanism preventing the same business event (e.g. one
// Disbursement, one Payment) from posting twice — see
// AccountingService.PostJournalEntry's unique-constraint-based dedup.
type JournalEntry struct {
	ID             uint   `gorm:"primaryKey" json:"journalId"`
	JournalNumber  string `gorm:"uniqueIndex;not null" json:"journalNumber"` // sequential, human-readable (e.g. "JE-2026-000001")
	TransactionType string `gorm:"not null;index" json:"transactionType"`    // e.g. "DISBURSEMENT","EMI_PAYMENT","INTEREST_ACCRUAL","PROCESSING_FEE","REVERSAL"
	ReferenceType  string `gorm:"not null;index" json:"referenceType"`       // e.g. "disbursement","payment","interest_accrual"
	ReferenceID    string `gorm:"not null;index" json:"referenceId"`

	// IdempotencyKey is unique across the whole table — normally
	// "<referenceType>:<referenceId>" for a first-time posting, so a
	// second attempt to post the same event fails the unique constraint
	// instead of creating a duplicate journal.
	IdempotencyKey string `gorm:"uniqueIndex;not null" json:"idempotencyKey"`

	TransactionDate time.Time `gorm:"not null" json:"transactionDate"`
	PostingDate     time.Time `gorm:"not null" json:"postingDate"`
	PeriodID        uint      `gorm:"not null;index" json:"periodId"`

	Description string `json:"description,omitempty"`
	Status      string `gorm:"not null;index" json:"status"`

	CreatedBy  string `json:"createdBy"`  // admin username, or "system" for an automated posting
	ApprovedBy string `json:"approvedBy,omitempty"`

	// Reversal linkage — nil on a normal entry. Exactly one of these is
	// set on a reversal/reversed pair.
	ReversalOfJournalID *uint `gorm:"index" json:"reversalOfJournalId,omitempty"`
	ReversedByJournalID *uint `gorm:"index" json:"reversedByJournalId,omitempty"`
	ReversalReason      string `json:"reversalReason,omitempty"`

	CreatedAt time.Time `json:"createdAt"`

	Lines []JournalEntryLine `gorm:"foreignKey:JournalID" json:"lines,omitempty"`
}

// JournalEntryLine is one debit or credit leg. Exactly one of
// DebitPaise/CreditPaise is nonzero per line — enforced by
// AccountingService, not a DB constraint, to keep the schema portable.
type JournalEntryLine struct {
	ID          uint   `gorm:"primaryKey" json:"journalLineId"`
	JournalID   uint   `gorm:"not null;index" json:"journalId"`
	AccountID   uint   `gorm:"not null;index" json:"accountId"`
	DebitPaise  int64  `gorm:"not null;default:0" json:"debitPaise"`
	CreditPaise int64  `gorm:"not null;default:0" json:"creditPaise"`
	Description string `json:"description,omitempty"`
}

// InterestAccrual (item F) — one row per loan per accrual period. The
// unique index on (loan_id, accrual_period) is what makes accrual
// idempotent: a second run for the same loan+period fails the constraint
// instead of double-accruing.
const (
	AccrualStatusPosted  = "POSTED"
	AccrualStatusFailed  = "FAILED"
)

type InterestAccrual struct {
	ID               uint   `gorm:"primaryKey" json:"accrualId"`
	LoanID           uint   `gorm:"not null;uniqueIndex:idx_loan_accrual_period" json:"loanId"`
	AccrualPeriod    string `gorm:"not null;uniqueIndex:idx_loan_accrual_period" json:"accrualPeriod"` // "2026-08"
	CalculationBasis string `json:"calculationBasis"`                                                  // e.g. "reducing-balance, actual/365"
	AmountPaise      int64  `gorm:"not null" json:"amountPaise"`
	Status           string `gorm:"not null" json:"status"`
	JournalReference string `json:"journalReference,omitempty"` // JournalEntry.JournalNumber, once posted

	// ExtensionPhase5 is an explicit, empty-today hook: Phase 5 IRAC/NPA
	// classification will set this to something like "NPA_INCOME_REVERSED"
	// to mark an accrual whose income recognition was suspended per RBI
	// IRAC norms, without requiring a schema change to this table.
	ExtensionPhase5 string `json:"extensionPhase5,omitempty"`

	CreatedAt time.Time `json:"createdAt"`
}
