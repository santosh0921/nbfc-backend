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
}
