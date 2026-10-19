package models

import "time"

// VerificationReport is the signed PDF a field officer generates at the
// end of a loan-verification visit (customer + officer e-signatures
// embedded, see lib/employee_app/core/utils/pdf_generator.dart) — this
// record is what makes that report visible in the admin panel instead of
// staying only on the officer's device.
type VerificationReport struct {
	ID               uint      `gorm:"primaryKey" json:"id"`
	LoanID           uint      `gorm:"not null;index" json:"loanId"`
	EmployeeID       uint      `gorm:"not null;index" json:"employeeId"`
	CustomerName     string    `json:"customerName"`
	LoanType         string    `json:"loanType"`
	Recommendation   string    `json:"recommendation"`
	RiskAssessment   string    `json:"riskAssessment"`
	FileName         string    `json:"fileName"`
	MimeType         string    `json:"mimeType"`
	StorageReference string    `json:"-"`
	DataBase64       string    `json:"-" gorm:"type:text"`
	CreatedAt        time.Time `json:"createdAt"`
}
