package models

import "time"

// FundingSource (item K) — one NBFC borrowing/funding line. Business
// terms (lender name, rate, tenure) are always admin-entered, never
// invented by this codebase.
const (
	FundingStatusActive = "ACTIVE"
	FundingStatusClosed = "CLOSED"
)

type FundingSource struct {
	ID              uint    `gorm:"primaryKey" json:"fundingSourceId"`
	LenderName      string  `gorm:"not null" json:"lenderName"`
	PrincipalPaise  int64   `gorm:"not null" json:"principalPaise"`
	InterestRatePercent float64 `json:"interestRatePercent,omitempty"`
	MaturityDate    *time.Time `json:"maturityDate,omitempty"`
	OutstandingPaise int64  `gorm:"not null" json:"outstandingPaise"`
	Status          string  `gorm:"not null;default:'ACTIVE'" json:"status"`

	AccountingReference string `json:"accountingReference,omitempty"`

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// BorrowingTransaction — a draw-down, interest charge, or repayment
// against a FundingSource, each of which posts its own journal entry.
const (
	BorrowingTxnDrawdown   = "DRAWDOWN"
	BorrowingTxnInterest   = "INTEREST_CHARGE"
	BorrowingTxnRepayment  = "REPAYMENT"
)

type BorrowingTransaction struct {
	ID              uint   `gorm:"primaryKey" json:"borrowingTransactionId"`
	FundingSourceID uint   `gorm:"not null;index" json:"fundingSourceId"`
	TransactionType string `gorm:"not null" json:"transactionType"`
	AmountPaise     int64  `gorm:"not null" json:"amountPaise"`
	TransactionDate time.Time `gorm:"not null" json:"transactionDate"`

	IdempotencyKey   string `gorm:"uniqueIndex;not null" json:"idempotencyKey"`
	JournalReference string `json:"journalReference,omitempty"`

	CreatedAt time.Time `json:"createdAt"`
}
