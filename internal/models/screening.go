package models

import (
	"time"

	"github.com/google/uuid"
)

// Screening results (Phase 3, item K). A POTENTIAL_MATCH is never
// auto-treated as confirmed — it always routes to REVIEW_REQUIRED for a
// compliance officer's explicit decision.
const (
	ScreeningResultClear           = "CLEAR"
	ScreeningResultPotentialMatch  = "POTENTIAL_MATCH"
	ScreeningResultReviewRequired  = "REVIEW_REQUIRED"
)

const (
	ScreeningDecisionPending  = "PENDING"
	ScreeningDecisionCleared  = "CLEARED"
	ScreeningDecisionRejected = "REJECTED"
)

// ScreeningRequest is one PEP/sanctions screening attempt for a customer.
// Whenever no screening provider is configured, Result defaults to
// REVIEW_REQUIRED (fail-safe) — never CLEAR — so an unconfigured
// environment cannot silently pass every customer through.
type ScreeningRequest struct {
	ID         uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	CustomerID uuid.UUID `gorm:"type:uuid;index;not null" json:"customerId"`

	Provider  string `json:"provider,omitempty"`
	Result    string `gorm:"not null" json:"result"`
	Reference string `json:"reference,omitempty"`

	// Compliance decision — set only by a compliance-tier reviewer, never
	// by an ordinary loan/credit officer. Decision stays PENDING until a
	// compliance officer acts, regardless of Result.
	ReviewerAdminID uint       `json:"reviewerAdminId,omitempty"`
	ReviewerRef     string     `json:"reviewerRef,omitempty"`
	Decision        string     `gorm:"not null;default:'PENDING'" json:"decision"`
	DecisionReason  string     `json:"decisionReason,omitempty"`
	DecidedAt       *time.Time `json:"decidedAt,omitempty"`

	ScreenedAt time.Time `json:"screenedAt"`
}
