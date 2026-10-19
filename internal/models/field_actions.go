package models

import "time"

// GoldAppraisal is a field-officer's gold-loan appraisal recorded against
// a specific loan case (see loans.EmployeeGoldAppraisalHandler).
type GoldAppraisal struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	LoanID      uint      `gorm:"not null;index" json:"loanId"`
	EmployeeID  uint      `gorm:"not null;index" json:"employeeId"`
	WeightGrams float64   `json:"weightGrams"`
	PurityKarat float64   `json:"purityKarat"`
	CreatedAt   time.Time `json:"createdAt"`
}

// PropertyVerification is a field-officer's property-collateral
// verification recorded against a specific loan case (see
// loans.EmployeePropertyVerificationHandler).
type PropertyVerification struct {
	ID             uint      `gorm:"primaryKey" json:"id"`
	LoanID         uint      `gorm:"not null;index" json:"loanId"`
	EmployeeID     uint      `gorm:"not null;index" json:"employeeId"`
	Condition      string    `json:"condition"`
	EstimatedValue float64   `json:"estimatedValue"`
	CreatedAt      time.Time `json:"createdAt"`
}

// EmployeeAttendance is a field-officer's check-in/check-out record with
// device geolocation (see loans.EmployeeAttendanceHandler). Not tied to
// any loan case — one employee can have many rows per day.
type EmployeeAttendance struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	EmployeeID uint      `gorm:"not null;index" json:"employeeId"`
	Action     string    `json:"action"` // "checkin" | "checkout"
	Latitude   float64   `json:"latitude"`
	Longitude  float64   `json:"longitude"`
	CreatedAt  time.Time `json:"createdAt"`
}
