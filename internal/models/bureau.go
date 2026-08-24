package models

import (
	"time"

	"github.com/google/uuid"
)

// Bureau identifiers — internal/bureau.Provider implementations are named
// after these. Only one concrete provider exists today (SANDBOX, gated to
// non-production — see internal/bureau/sandbox_provider.go); CIBIL/
// EXPERIAN/CRIF/EQUIFAX are architecture placeholders requiring real
// credentials this environment does not have (see the Phase 3 report).
const (
	BureauSandbox  = "SANDBOX"
	BureauCIBIL    = "CIBIL"
	BureauExperian = "EXPERIAN"
	BureauCRIF     = "CRIF"
	BureauEquifax  = "EQUIFAX"
)

const (
	BureauRequestStatusPending   = "PENDING"
	BureauRequestStatusCompleted = "COMPLETED"
	BureauRequestStatusFailed    = "FAILED"
	BureauRequestStatusBlocked   = "BLOCKED" // no provider configured for this bureau in this environment
)

// BureauConsent records the customer's explicit consent BEFORE any bureau
// pull — internal/bureau.Service.PullScore refuses to proceed without a
// matching, unexpired consent row (see that package). One customer can
// have multiple consent records over time (e.g. per application, or a
// renewed annual consent) — never overwritten in place, so the history of
// what was actually consented to is preserved.
type BureauConsent struct {
	ID         uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	CustomerID uuid.UUID `gorm:"type:uuid;index;not null" json:"customerId"`

	Bureau         string `gorm:"not null" json:"bureau"`
	Purpose        string `gorm:"not null" json:"purpose"` // e.g. "loan underwriting"
	ConsentVersion string `gorm:"not null" json:"consentVersion"`
	ApplicationRef string `json:"applicationRef,omitempty"` // loan ID this consent was collected for, if any

	IPAddress  string `json:"ipAddress,omitempty"`
	DeviceInfo string `json:"deviceInfo,omitempty"`

	ConsentedAt time.Time `json:"consentedAt"`
	CreatedAt   time.Time `json:"createdAt"`
}

// BureauRequest is one bureau pull attempt. The RAW bureau report is
// deliberately never stored in full here — ResponseHash (SHA-256 of the
// raw response, for integrity/audit) and a small set of decision-relevant
// fields (Score, RiskIndicators — a short summary string, not the full
// report body) are what persist; ReportReference is an opaque pointer a
// human could use to pull the full report back from the provider's own
// portal if ever needed, not a copy stored here.
type BureauRequest struct {
	ID         uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"bureauRequestId"`
	CustomerID uuid.UUID `gorm:"type:uuid;index;not null" json:"customerId"`

	Bureau          string    `gorm:"not null" json:"bureau"`
	RequestType     string    `json:"requestType"` // "score" | "full_report"
	ConsentID       uuid.UUID `gorm:"type:uuid;not null" json:"consentId"`
	Status          string    `gorm:"not null" json:"status"`
	ProviderReference string  `json:"providerReference,omitempty"`
	ResponseHash    string    `json:"responseHash,omitempty"`
	ReportReference string    `json:"reportReference,omitempty"`

	Score          int    `json:"score,omitempty"`
	RiskIndicators string `json:"riskIndicators,omitempty"` // short summary only, not the full report
	FailureReason  string `json:"failureReason,omitempty"`

	RequestedAt time.Time  `json:"requestedAt"`
	CompletedAt *time.Time `json:"completedAt,omitempty"`
}

// CreditDecision separates the bureau data from the underwriting rules
// that consumed it (Phase 3, item H) — a decision record independent of
// both the BureauRequest and the LoanApplication row, so "what policy
// version made this call, on what inputs" survives even if the policy
// constants change later.
type CreditDecision struct {
	ID uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	// LoanID is nullable: an apply-time eligibility rejection happens
	// BEFORE any LoanApplication row is created, so there is no loan to
	// attach it to — CustomerID identifies who the decision was about in
	// that case.
	LoanID     *uint     `gorm:"index" json:"loanId,omitempty"`
	CustomerID uuid.UUID `gorm:"type:uuid;index" json:"customerId,omitempty"`

	PolicyVersion string `gorm:"not null" json:"policyVersion"`
	// InputsJSON is a small, deliberately non-sensitive summary (score
	// band, FOIR ratio, tenure/amount) — never raw bureau report content
	// or PII.
	InputsJSON  string `gorm:"type:text" json:"inputsJson"`
	Decision    string `gorm:"not null" json:"decision"` // "ELIGIBLE" | "REJECTED" | "SANCTIONED" | "SANCTION_REJECTED"
	ReasonCodes string `json:"reasonCodes,omitempty"`     // comma-separated short codes, e.g. "CIBIL_BELOW_THRESHOLD,FOIR_EXCEEDED"

	ReviewerOrSystem string    `gorm:"not null" json:"reviewerOrSystem"` // "system" for an automated eligibility check, admin username for a sanction decision
	CreatedAt        time.Time `json:"createdAt"`
}
