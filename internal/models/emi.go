package models

import "time"

const (
	EmiStatusPending = "pending"
	EmiStatusPaid    = "paid"
	EmiStatusOverdue = "overdue"
)

type EmiInstallment struct {
	ID                 uint       `gorm:"primaryKey" json:"id"`
	LoanID             uint       `gorm:"index" json:"loanId"`
	InstallmentNumber  int        `json:"installmentNumber"`
	DueDate            time.Time  `json:"dueDate"`
	Amount             float64    `json:"amount"`
	PrincipalComponent float64    `json:"principalComponent"`
	InterestComponent  float64    `json:"interestComponent"`
	Status             string     `json:"status"`
	PaidAt             *time.Time `json:"paidAt,omitempty"`
	CreatedAt          time.Time  `json:"createdAt"`

	// PaymentReference is the gateway/UTR transaction reference the
	// customer's payment was made under. Unique so the same reference can
	// never be replayed to settle a second installment, and required
	// (non-empty) before an installment can be marked paid — see
	// CustomerPayEmiHandler in internal/loans/emi_handler.go.
	PaymentReference string `gorm:"uniqueIndex:idx_emi_payment_reference,where:payment_reference != ''" json:"paymentReference,omitempty"`
	PaymentMethod    string `json:"paymentMethod,omitempty"`
}
