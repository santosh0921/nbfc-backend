package models

import "time"

const (
	BankTxnStatusUnmatched        = "UNMATCHED"
	BankTxnStatusMatched          = "MATCHED"           // auto- or manually- matched, not yet finalized
	BankTxnStatusReconciled       = "RECONCILED"        // match confirmed and its financial effect applied (e.g. Disbursement -> SUCCESS)
	BankTxnStatusException        = "EXCEPTION"         // matched a candidate but amount/date didn't line up cleanly
	BankTxnStatusManuallyResolved = "MANUALLY_RESOLVED" // an exception a human reviewed and resolved by hand
)

// BankAccount is the NBFC's own settlement/disbursement bank account that
// statements are imported against — a reconciliation import must be
// scoped to a known account, not a free-floating file.
type BankAccount struct {
	ID                  uint      `gorm:"primaryKey" json:"id"`
	AccountName         string    `gorm:"not null" json:"accountName"`
	AccountNumberMasked string    `gorm:"not null" json:"accountNumberMasked"` // last 4 digits only, e.g. "XXXXXXXX1234"
	IFSC                string    `json:"ifsc,omitempty"`
	BankName            string    `json:"bankName,omitempty"`
	// OpeningBalancePaise (Phase 8 item K) — the balance as of when this
	// account was first tracked in this system, admin-entered (this
	// application has no way to independently verify a historical opening
	// balance). ClosingBalance in finance.BankAccountBalance is always
	// OpeningBalancePaise + credits - debits from imported statement
	// lines, never a separately-maintained running total.
	OpeningBalancePaise int64     `gorm:"not null;default:0" json:"openingBalancePaise"`
	Active              bool      `gorm:"default:true" json:"active"`
	CreatedAt           time.Time `json:"createdAt"`
}

// BankStatementImport is one CSV upload batch — kept so every imported
// transaction traces back to who imported it, when, and from which file,
// which the raw BankTransaction rows alone wouldn't capture.
type BankStatementImport struct {
	ID                uint      `gorm:"primaryKey" json:"id"`
	BankAccountID     uint      `gorm:"index;not null" json:"bankAccountId"`
	FileName          string    `json:"fileName"`
	RowCount          int       `json:"rowCount"`
	ImportedBy        string    `json:"importedBy"`
	ImportedByAdminID uint      `json:"importedByAdminId"`
	CreatedAt         time.Time `json:"createdAt"`
}

// BankTransaction is a single line from an imported bank statement.
type BankTransaction struct {
	ID            uint `gorm:"primaryKey" json:"id"`
	ImportID      uint `gorm:"index;not null" json:"importId"`
	BankAccountID uint `gorm:"index;not null" json:"bankAccountId"`

	BankTxnID   string    `json:"bankTxnId,omitempty"` // the bank's own transaction id, if the statement format includes one
	TxnDate     time.Time `json:"txnDate"`
	ValueDate   time.Time `json:"valueDate"`
	AmountPaise int64     `gorm:"not null" json:"amountPaise"`
	DrCr        string    `gorm:"not null" json:"drCr"` // "DR" | "CR"
	Description string    `gorm:"type:text" json:"description"`
	Reference   string    `gorm:"index" json:"reference,omitempty"` // UTR/reference extracted from the statement line

	Status string `gorm:"index;not null;default:UNMATCHED" json:"status"`

	// MatchedEntityType/ID point at whatever this line was matched
	// against — "disbursement" | "payment" — kept generic rather than two
	// nullable foreign keys since a statement line matches at most one
	// entity.
	MatchedEntityType string `json:"matchedEntityType,omitempty"`
	MatchedEntityID   uint   `json:"matchedEntityId,omitempty"`

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// ReconciliationRecord is created every time a BankTransaction is matched
// (automatically or manually) — the audit-relevant "who/when/why" for a
// match, kept distinct from the BankTransaction row itself so re-matching
// history isn't lost by overwriting fields in place.
type ReconciliationRecord struct {
	ID                  uint      `gorm:"primaryKey" json:"id"`
	BankTransactionID   uint      `gorm:"index;not null" json:"bankTransactionId"`
	MatchedEntityType   string    `gorm:"not null" json:"matchedEntityType"`
	MatchedEntityID     uint      `gorm:"not null" json:"matchedEntityId"`
	MatchType           string    `gorm:"not null" json:"matchType"`         // "auto" | "manual"
	Reason              string    `gorm:"type:text" json:"reason,omitempty"` // required for manual matches
	ReconciledBy        string    `json:"reconciledBy"`                      // "system" for auto-matches
	ReconciledByAdminID *uint     `json:"reconciledByAdminId,omitempty"`
	ReconciledAt        time.Time `json:"reconciledAt"`
}

// ReconciliationException flags a BankTransaction the matcher couldn't
// cleanly resolve (e.g. a UTR match with a mismatched amount) — surfaced
// to admins for manual review rather than silently auto-matched or
// silently dropped.
type ReconciliationException struct {
	ID                  uint       `gorm:"primaryKey" json:"id"`
	BankTransactionID   uint       `gorm:"uniqueIndex;not null" json:"bankTransactionId"`
	Reason              string     `gorm:"type:text;not null" json:"reason"`
	CandidateEntityType string     `json:"candidateEntityType,omitempty"`
	CandidateEntityID   uint       `json:"candidateEntityId,omitempty"`
	ResolvedAt          *time.Time `json:"resolvedAt,omitempty"`
	ResolvedBy          string     `json:"resolvedBy,omitempty"`
	ResolvedByAdminID   *uint      `json:"resolvedByAdminId,omitempty"`
	ResolutionNote      string     `json:"resolutionNote,omitempty"`
	CreatedAt           time.Time  `json:"createdAt"`
}
