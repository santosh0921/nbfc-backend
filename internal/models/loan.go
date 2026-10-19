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

	// LoanStatusPendingClaim — a freshly submitted loan whose city has at
	// least one approved verification employee is broadcast to ALL of
	// them (AssignedEmployeeID stays nil) rather than auto-assigned to
	// one, first-come-first-served: whichever employee calls
	// POST /employee/loans/:id/claim first gets it, atomically, and it
	// disappears from every other matching employee's task list. Falls
	// back to the old single-employee least-loaded assignment
	// (LoanStatusAssigned immediately) only when no employee in that
	// city exists at all — see findVerificationEmployeesInCity.
	LoanStatusPendingClaim = "pending_claim"

	// Two-stage KYC review (Phase: document-first review). A freshly
	// submitted loan now sits here — visible to KYC with every uploaded
	// document, but not yet broadcast to any field employee — until a KYC
	// officer explicitly approves the documents (which is what flips it to
	// LoanStatusPendingClaim and triggers the employee broadcast that used
	// to happen immediately on submission). Symmetrically, once a field
	// employee submits their verification report the loan lands at
	// LoanStatusPendingKYCFinalReview rather than going straight to
	// LoanStatusVerified — a second, final KYC decision is required before
	// the loan is allowed to proceed to Credit Bureau/Underwriting.
	LoanStatusPendingKYCDocReview   = "pending_kyc_doc_review"
	LoanStatusPendingKYCFinalReview = "pending_kyc_final_review"
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

	// Two-stage KYC review — see LoanStatusPendingKYCDocReview /
	// LoanStatusPendingKYCFinalReview above. KYCDocApproved* is the first
	// decision (documents reviewed, loan released to field employees);
	// KYCFinalApproved* is the second (field report reviewed, loan
	// released to LoanStatusVerified and the rest of the pipeline).
	KYCDocApprovedByAdminID   *uint      `json:"kycDocApprovedByAdminId,omitempty"`
	KYCDocApprovedBy          string     `json:"kycDocApprovedBy,omitempty"`
	KYCDocApprovedAt          *time.Time `json:"kycDocApprovedAt,omitempty"`
	KYCFinalApprovedByAdminID *uint      `json:"kycFinalApprovedByAdminId,omitempty"`
	KYCFinalApprovedBy        string     `json:"kycFinalApprovedBy,omitempty"`
	KYCFinalApprovedAt        *time.Time `json:"kycFinalApprovedAt,omitempty"`
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

	// Home Loan only — the real-world builder/property details a
	// construction-linked property purchase actually needs, collected on
	// apply (see internal/loans/customer_handler.go SubmitLoanHandler).
	// Disbursement itself stays single-shot like every other category —
	// this is data collection only, not a tranche/demand-letter
	// disbursement workflow.
	BuilderName       string  `json:"builderName,omitempty"`
	ProjectName       string  `json:"projectName,omitempty"`
	PropertyAddress   string  `json:"propertyAddress,omitempty"`
	AgreementValue    float64 `json:"agreementValue,omitempty"`
	ConstructionStage string  `json:"constructionStage,omitempty"`

	// Gold Loan only — collected on apply so the branch an appraiser
	// works at knows what to expect before the customer physically
	// brings the gold in; VisitDate/VisitTimeSlot above double as the
	// appointment for this (same fields the Housing Loan flow already
	// uses for a field-verification visit). Purity/weight here are the
	// customer's own estimate — the branch's physical appraisal (not
	// modeled in this backend) is what actually sets the loan amount.
	GoldOrnamentType      string  `json:"goldOrnamentType,omitempty"`
	GoldApproxWeightGrams float64 `json:"goldApproxWeightGrams,omitempty"`
	GoldApproxPurityKarat int     `json:"goldApproxPurityKarat,omitempty"`
	GoldAlreadyPledged    bool    `json:"goldAlreadyPledged,omitempty"`
	GoldLoanBranch        string  `json:"goldLoanBranch,omitempty"`

	// Business Loan and MSME Loan — the business's own identity, separate
	// from the applicant's personal KYC. Real underwriting for either
	// product reads this (plus the already-existing GST Certificate /
	// Business Premises documents) instead of a salary slip. Both
	// categories share every field here except UdyamNumber, which only
	// means something for MSME (it's what qualifies a loan for
	// CGTMSE collateral-free processing) — left empty for Business Loan.
	BusinessName             string  `json:"businessName,omitempty"`
	BusinessRegistrationType string  `json:"businessRegistrationType,omitempty"`
	BusinessAddress          string  `json:"businessAddress,omitempty"`
	BusinessYearsInOperation int     `json:"businessYearsInOperation,omitempty"`
	BusinessMonthlyTurnover  float64 `json:"businessMonthlyTurnover,omitempty"`
	BusinessGSTNumber        string  `json:"businessGstNumber,omitempty"`
	// MSME Loan only.
	MSMEUdyamNumber string `json:"msmeUdyamNumber,omitempty"`

	// Farmer Loan only — land record details, the equivalent of a
	// property document for agricultural lending. State-specific record
	// type (7/12 extract, Patta/Chitta, Khasra-Khatauni, etc.) since
	// there's no unified national land registry to validate against.
	// KCCNumber is optional: an existing Kisan Credit Card ties this loan
	// to that government scheme's subsidized terms, same role
	// MSMEUdyamNumber plays for MSME.
	FarmerLandRecordType   string  `json:"farmerLandRecordType,omitempty"`
	FarmerLandRecordNumber string  `json:"farmerLandRecordNumber,omitempty"`
	FarmerLandAreaAcres    float64 `json:"farmerLandAreaAcres,omitempty"`
	FarmerLandLocation     string  `json:"farmerLandLocation,omitempty"`
	FarmerCropOrPurpose    string  `json:"farmerCropOrPurpose,omitempty"`
	FarmerKCCNumber        string  `json:"farmerKccNumber,omitempty"`

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
