package models

import "time"

// LoanReference is one of the two compulsory personal references a
// customer must supply per loan application. Aadhaar is handled the same
// way as CustomerAadhaar (hash + mask, raw number never stored); PAN is
// handled the same way as the (now-encrypted) CustomerPAN (AES-GCM
// ciphertext + plaintext last-4 for display).
type LoanReference struct {
	ID       uint   `gorm:"primaryKey" json:"id"`
	LoanID   uint   `gorm:"index;not null" json:"loanId"`
	Name     string `json:"name"`
	Relation string `json:"relation"`
	Phone    string `json:"phone"`

	AadhaarMasked string `gorm:"size:20" json:"-"`
	AadhaarHash   string `gorm:"size:64;index" json:"-"`

	PANEncrypted string `gorm:"type:text" json:"-"`
	PANLast4     string `gorm:"size:4" json:"-"`

	CreatedAt time.Time `json:"createdAt"`
}
