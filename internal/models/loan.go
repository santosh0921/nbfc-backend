package models

import (
	"time"

	"github.com/google/uuid"
)

// Loan application status vocabulary (single source of truth — see
// internal/constants/verification.go for the aliases used by handlers).
const (
	LoanStatusSubmitted    = "submitted"
	LoanStatusAutoVerified = "auto_verified"
	LoanStatusAssigned     = "assigned"
	LoanStatusVerified     = "verified"
	LoanStatusSanctioned   = "sanctioned"
	LoanStatusRejected     = "rejected"
	LoanStatusDisbursed    = "disbursed"

	// Maker-checker states (Phase 1 security work) — a loan no longer goes
	// straight from "verified" to "sanctioned" under a single admin's own
	// decision. LoanStatusSanctioned is still the terminal "approved,
	// ready to disburse" state (kept exactly as before so every existing
	// downstream check — eSign, EMI eligibility, letters — keeps working
	// unchanged), it's just no longer reachable in one step by one admin.
	LoanStatusPendingSanctionApproval = "pending_sanction_approval"
	LoanStatusReadyForDisbursement    = "ready_for_disbursement"

	// LoanStatusDisbursementProcessing (Phase 2) — the checker approving a
	// disbursement no longer flips a loan straight to LoanStatusDisbursed.
	// It moves here while the actual bank transfer is in flight
	// (internal/disbursement.Disbursement), and only becomes
	// LoanStatusDisbursed once that transfer is independently confirmed
	// (internal/loans.ConfirmDisbursement) — a real bank/payout
	// confirmation, or a matched bank-statement line during reconciliation.
	LoanStatusDisbursementProcessing = "disbursement_processing"
)

type LoanApplication struct {
	ID                         uint       `gorm:"primaryKey" json:"id"`
	CustomerID                 uuid.UUID  `gorm:"type:uuid;index" json:"customerId"`
	Category                   string     `json:"category"`
	AmountRequested            float64    `json:"amountRequested"`
	Purpose                    string     `json:"purpose"`
	ApplicantName              string     `json:"applicantName"`
	ApplicantPhone             string     `json:"applicantPhone"`
	AddressLine                string     `json:"addressLine"`
	City                       string     `json:"city"`
	Status                     string     `json:"status"`
	AutoVerified               bool       `json:"autoVerified"`
	AutoVerifiedNote           string     `json:"autoVerifiedNote"`
	AssignedEmployeeID         *uint      `json:"assignedEmployeeId"`
	VerificationReport         string     `json:"verificationReport,omitempty"`
	VerificationFindings       string     `json:"verificationFindings,omitempty"`
	VerificationRisk           string     `json:"verificationRisk,omitempty"`
	VerificationRecommendation string     `json:"verificationRecommendation,omitempty"`
	DecisionRemarks            string     `json:"decisionRemarks,omitempty"`
	DecidedBy                  string     `json:"decidedBy,omitempty"`
	DecidedAt                  *time.Time `json:"decidedAt,omitempty"`
	DisbursedAmount            float64    `json:"disbursedAmount,omitempty"`
	DisbursedAt                *time.Time `json:"disbursedAt,omitempty"`
	TenureMonths               *int       `json:"tenureMonths,omitempty"`
	InterestRatePercent        *float64   `json:"interestRatePercent,omitempty"`
	ParentLoanID               *uint      `json:"parentLoanId,omitempty"`
	CibilScore                 int        `json:"cibilScore,omitempty"`

	// What the customer actually asked for and was quoted at apply time
	// (internal/loans/eligibility.go) — kept separate from TenureMonths/
	// InterestRatePercent above, which stay reserved for whatever the
	// admin actually sets at sanction (may differ from the applicant's
	// request). Mainly for the admin panel to show what was requested.
	RequestedTenureMonths        *int     `json:"requestedTenureMonths,omitempty"`
	EstimatedInterestRatePercent *float64 `json:"estimatedInterestRatePercent,omitempty"`
	EstimatedMonthlyEmi          *float64 `json:"estimatedMonthlyEmi,omitempty"`

	// Collected on the customer's "Verification Scheduling" apply step —
	// previously only ever shown back to the customer on their own review
	// screen and never sent to the backend at all, so the employee who
	// actually has to call/visit them had no way to know it.
	TimingPreference string     `json:"timingPreference,omitempty"`
	VisitDate        *time.Time `json:"visitDate,omitempty"`
	VisitTimeSlot    string     `json:"visitTimeSlot,omitempty"`

	// GST on the NBFC processing fee — computed once at sanction time (see
	// internal/loans/gst.go) and kept as its own line items throughout:
	// principal, processing fee, and tax are never merged into a single
	// number anywhere they're stored or displayed. CustomerState is a
	// snapshot of the place-of-supply state used for the calculation, kept
	// separately from CustomerAddress so a later address edit can't
	// silently change which tax rate this loan was actually charged under.
	ProcessingFeeBase          float64 `json:"processingFeeBase,omitempty"`
	ProcessingFeeCGST          float64 `json:"processingFeeCgst,omitempty"`
	ProcessingFeeSGST          float64 `json:"processingFeeSgst,omitempty"`
	ProcessingFeeIGST          float64 `json:"processingFeeIgst,omitempty"`
	ProcessingFeeGSTTotal      float64 `json:"processingFeeGstTotal,omitempty"`
	ProcessingFeeTotal         float64 `json:"processingFeeTotal,omitempty"`
	ProcessingFeeGSTType       string  `json:"processingFeeGstType,omitempty"` // "CGST_SGST" | "IGST"
	ProcessingFeeCustomerState string  `json:"processingFeeCustomerState,omitempty"`

	// Maker-checker: sanction. RecommendedBy is the maker (LOAN_REVIEW
	// permission); DecidedBy (existing field, reused) is the checker who
	// approved/rejected the recommendation (LOAN_SANCTION permission) — the
	// handler enforces RecommendedByAdminID != approving admin's ID, so
	// the same person can never be both.
	RecommendedByAdminID           *uint      `json:"recommendedByAdminId,omitempty"`
	RecommendedBy                  string     `json:"recommendedBy,omitempty"` // admin username, for display
	RecommendedAt                  *time.Time `json:"recommendedAt,omitempty"`
	RecommendedTenureMonths        *int       `json:"recommendedTenureMonths,omitempty"`
	RecommendedInterestRatePercent *float64   `json:"recommendedInterestRatePercent,omitempty"`
	RecommendationRemarks          string     `json:"recommendationRemarks,omitempty"`
	DecidedByAdminID               *uint      `json:"decidedByAdminId,omitempty"`

	// Maker-checker: disbursement. DisbursementPreparedByAdminID is the
	// maker (LOAN_DISBURSE_PREPARE); DisbursementApprovedByAdminID is the
	// checker who released the funds (LOAN_DISBURSE) — same
	// maker != checker enforcement as sanction, checked independently.
	DisbursementPreparedByAdminID *uint      `json:"disbursementPreparedByAdminId,omitempty"`
	DisbursementPreparedBy        string     `json:"disbursementPreparedBy,omitempty"`
	DisbursementPreparedAt        *time.Time `json:"disbursementPreparedAt,omitempty"`
	DisbursementPreparedAmount    *float64   `json:"disbursementPreparedAmount,omitempty"`
	DisbursementApprovedByAdminID *uint      `json:"disbursementApprovedByAdminId,omitempty"`
	DisbursementApprovedBy        string     `json:"disbursementApprovedBy,omitempty"`
	DisbursementApprovedAt        *time.Time `json:"disbursementApprovedAt,omitempty"`

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// CIBIL decision-rule thresholds/constants (see AdminDecisionHandler).
const (
	CibilRejectThreshold = 650 // < this: sanction must be refused
	CibilPrimeThreshold  = 750 // > this: minimum rate applies
	CibilMidRatePercent  = 12.0
	CibilMinRatePercent  = 8.0 // rate offered to CIBIL > 750 applicants
)
