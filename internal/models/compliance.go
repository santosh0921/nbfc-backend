// Package models — Phase 11 NBFC compliance-management and audit-evidence
// foundation.
//
// CRITICAL, non-negotiable design constraint carried through this whole
// file, matching the same discipline established in Phase 5 (risk rules)
// and Phase 10 (tax rules): NOTHING here asserts an NBFC category,
// applicable regulation, due date, or "compliant" status as fact. Every
// applicability/requirement/due-date row is admin-entered after
// verification against an official source, and this codebase seeds NO
// requirement rows and NO NBFC-profile field values — see
// internal/compliance/seed.go's doc comment. This system tracks
// compliance ACTIVITY; it is not itself proof of compliance (item AC).
package models

import "time"

// --- A) NBFC profile / applicability engine ---

const (
	ApplicabilityApplicable    = "APPLICABLE"
	ApplicabilityNotApplicable = "NOT_APPLICABLE"
	ApplicabilityConditional   = "CONDITIONAL"
	ApplicabilityRequiresReview = "REQUIRES_REVIEW" // the default and safe fallback — never auto-resolved to APPLICABLE/NOT_APPLICABLE
)

// NBFCProfile is a single-row configuration table (one org). Every field
// starts empty/"REQUIRES_CONFIRMATION" — nothing here is inferred from
// the existing codebase (e.g. from what loan products happen to exist in
// the DB), since the legal NBFC category/layer is a licensing fact only
// management/compliance can state, not something derivable from
// application data.
type NBFCProfile struct {
	ID uint `gorm:"primaryKey" json:"id"`

	LegalEntityName   string `json:"legalEntityName,omitempty"`
	NBFCType          string `json:"nbfcType,omitempty"`          // e.g. "NBFC-ICC" — admin-entered, never inferred
	RegulatoryLayer   string `json:"regulatoryLayer,omitempty"`   // e.g. "Base Layer"/"Middle Layer" under RBI's scale-based regulation — admin-entered
	PrincipalBusinessActivity string `json:"principalBusinessActivity,omitempty"`
	LendingProducts   string `gorm:"type:text" json:"lendingProducts,omitempty"`
	GeographicScope   string `json:"geographicScope,omitempty"`
	AcceptsDeposits   *bool  `json:"acceptsDeposits,omitempty"` // nil = REQUIRES CONFIRMATION, never defaulted to false
	PublicFundsStatus string `json:"publicFundsStatus,omitempty"`

	CIN               string `json:"cin,omitempty"`
	RBIRegistrationNo string `json:"rbiRegistrationNo,omitempty"`
	PAN               string `json:"pan,omitempty"`
	OtherRegistrations string `gorm:"type:text" json:"otherRegistrations,omitempty"`

	ComplianceOfficerAdminID *uint  `json:"complianceOfficerAdminId,omitempty"`
	ComplianceOfficerName    string `json:"complianceOfficerName,omitempty"`

	// ConfirmationStatus stays REQUIRES_REVIEW until an authorized admin
	// explicitly confirms the profile is accurate — this is a distinct,
	// deliberate action (ConfirmProfile), never implied by merely saving
	// the row.
	ConfirmationStatus string     `gorm:"not null;default:'REQUIRES_REVIEW'" json:"confirmationStatus"`
	ConfirmedBy        string     `json:"confirmedBy,omitempty"`
	ConfirmedAt        *time.Time `json:"confirmedAt,omitempty"`

	UpdatedBy string    `json:"updatedBy,omitempty"`
	UpdatedAt time.Time `json:"updatedAt"`
	CreatedAt time.Time `json:"createdAt"`
}

// --- B/C) Regulatory requirement master + catalogue ---

// RequirementCategory is a free string an admin assigns — item C lists
// example categories (Governance, KYC/AML, Fair Practices, ...) as a
// menu of possibilities, NOT a claim that all of them apply; this code
// enforces no fixed enum so a category that doesn't fit isn't forced into
// one that does.
const (
	RequirementStatusActive   = "ACTIVE"
	RequirementStatusRetired  = "RETIRED"
	RequirementStatusDraft    = "DRAFT" // not yet confirmed by compliance — never used to generate calendar items
)

type ComplianceRequirement struct {
	ID uint `gorm:"primaryKey" json:"requirementId"`

	Regulator      string `gorm:"not null" json:"regulator"` // e.g. "RBI" — admin-entered
	RegulationRef  string `json:"regulationRef,omitempty"`   // the direction/master circular name
	SectionRef     string `json:"sectionRef,omitempty"`
	Title          string `gorm:"not null" json:"title"`
	Description    string `gorm:"type:text" json:"description,omitempty"`
	Category       string `json:"category,omitempty"` // free text, see doc comment above

	Applicability     string `gorm:"not null;default:'REQUIRES_REVIEW'" json:"applicability"`
	ApplicabilityNote string `gorm:"type:text" json:"applicabilityNote,omitempty"`

	Frequency      string `json:"frequency,omitempty"` // free text, e.g. "Quarterly" — admin-entered, no hard-coded due-date rule
	DueDateRule    string `json:"dueDateRule,omitempty"`
	ResponsibleDepartment string `json:"responsibleDepartment,omitempty"`
	ResponsibleAdminID    *uint  `json:"responsibleAdminId,omitempty"`
	ResponsibleName       string `json:"responsibleName,omitempty"`
	ReviewerName          string `json:"reviewerName,omitempty"`

	EvidenceRequired string `gorm:"type:text" json:"evidenceRequired,omitempty"`
	DataSource       string `json:"dataSource,omitempty"`
	ReportingFormat  string `json:"reportingFormat,omitempty"`

	EffectiveFrom time.Time  `json:"effectiveFrom,omitempty"`
	EffectiveTo   *time.Time `json:"effectiveTo,omitempty"`

	Status       string     `gorm:"not null;default:'DRAFT'" json:"status"`
	RiskRating   string     `json:"riskRating,omitempty"` // free text (e.g. HIGH/MEDIUM/LOW) — admin-assigned
	LastReviewed *time.Time `json:"lastReviewed,omitempty"`
	NextReview   *time.Time `json:"nextReview,omitempty"`

	// Official-source provenance (item AB) — required before Status can
	// become ACTIVE, enforced by the service layer.
	SourceReference string     `gorm:"type:text" json:"sourceReference,omitempty"`
	LastVerifiedDate *time.Time `json:"lastVerifiedDate,omitempty"`

	CreatedBy string    `json:"createdBy"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// --- D) Compliance calendar ---

const (
	CalendarStatusNotStarted   = "NOT_STARTED"
	CalendarStatusInProgress   = "IN_PROGRESS"
	CalendarStatusPendingReview = "PENDING_REVIEW"
	CalendarStatusCompleted    = "COMPLETED"
	CalendarStatusOverdue      = "OVERDUE"
	CalendarStatusNotApplicable = "NOT_APPLICABLE"
	CalendarStatusException    = "EXCEPTION"
)

type ComplianceCalendarItem struct {
	ID              uint       `gorm:"primaryKey" json:"calendarItemId"`
	RequirementID   uint       `gorm:"not null;index" json:"requirementId"`
	Period          string     `json:"period,omitempty"` // e.g. "2026-Q1" — admin-entered
	DueDate         time.Time  `gorm:"not null;index" json:"dueDate"`
	OwnerAdminID    *uint      `json:"ownerAdminId,omitempty"`
	OwnerName       string     `json:"ownerName,omitempty"`
	ReviewerName    string     `json:"reviewerName,omitempty"`
	Status          string     `gorm:"not null;default:'NOT_STARTED';index" json:"status"`
	Priority        string     `json:"priority,omitempty"`
	Evidence        string     `json:"evidence,omitempty"` // free-text note or ComplianceEvidence ID reference
	SubmissionRef   string     `json:"submissionRef,omitempty"`
	CompletionDate  *time.Time `json:"completionDate,omitempty"`
	ReviewDate      *time.Time `json:"reviewDate,omitempty"`
	Remarks         string     `gorm:"type:text" json:"remarks,omitempty"`

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// ComplianceReminderConfig (item D) — configurable reminder offsets, not
// hard-coded. Seeded with the example values item D itself lists
// (30/15/7/3/1 days + due date), explicitly labeled as an editable
// starting point, not a regulatory requirement.
type ComplianceReminderConfig struct {
	ID           uint `gorm:"primaryKey" json:"reminderConfigId"`
	DaysBeforeDue int `gorm:"not null;uniqueIndex" json:"daysBeforeDue"` // 0 = due date itself; negative = days overdue
	Label        string `json:"label"`
	Active       bool `gorm:"default:true" json:"active"`
}

// --- E) Regulatory return data foundation ---
// (Deliberately separate from Phase 6's RegulatoryReportRun — that table
// is the generic report-generation workflow; this one is specifically
// the return-tracking shape item E asks for, with its own status
// vocabulary. Where the two overlap for an actual filing, an admin can
// cross-reference by ID; this phase does not merge them, to avoid
// disturbing Phase 6's existing, working implementation.)

const (
	ReturnTrackingStatusDraft              = "DRAFT"
	ReturnTrackingStatusInPreparation      = "IN_PREPARATION"
	ReturnTrackingStatusPendingReview      = "PENDING_REVIEW"
	ReturnTrackingStatusApproved           = "APPROVED"
	ReturnTrackingStatusSubmitted          = "SUBMITTED"
	ReturnTrackingStatusAccepted           = "ACCEPTED"
	ReturnTrackingStatusRejected           = "REJECTED"
	ReturnTrackingStatusCorrectionRequired = "CORRECTION_REQUIRED"
	ReturnTrackingStatusOverdue            = "OVERDUE"
)

type RegulatoryReturnTracking struct {
	ID              uint      `gorm:"primaryKey" json:"returnTrackingId"`
	Regulator       string    `gorm:"not null" json:"regulator"`
	ReturnType      string    `gorm:"not null" json:"returnType"` // free text — never a hard-coded form name
	ApplicableEntity string   `json:"applicableEntity,omitempty"`
	ReportingPeriod string    `gorm:"not null" json:"reportingPeriod"`
	Frequency       string    `json:"frequency,omitempty"`
	DueDate         time.Time `json:"dueDate,omitempty"`

	DataSource string `json:"dataSource,omitempty"`

	Status           string `gorm:"not null;default:'DRAFT';index" json:"status"`
	SubmissionRef    string `json:"submissionRef,omitempty"`
	SubmittedBy      string `json:"submittedBy,omitempty"`
	SubmittedDate    *time.Time `json:"submittedDate,omitempty"`
	Evidence         string `json:"evidence,omitempty"`
	Corrections      string `gorm:"type:text" json:"corrections,omitempty"`
	Remarks          string `gorm:"type:text" json:"remarks,omitempty"`

	// Maker-checker (item G)
	PreparedByAdminID uint       `json:"preparedByAdminId,omitempty"`
	PreparedBy        string     `json:"preparedBy,omitempty"`
	ReviewedByAdminID *uint      `json:"reviewedByAdminId,omitempty"`
	ReviewedBy        string     `json:"reviewedBy,omitempty"`
	ApprovedByAdminID *uint      `json:"approvedByAdminId,omitempty"`
	ApprovedBy        string     `json:"approvedBy,omitempty"`
	ApprovalDate      *time.Time `json:"approvalDate,omitempty"`
	ApprovalComments  string     `json:"approvalComments,omitempty"`

	// Once SUBMITTED/ACCEPTED, the row is frozen (item Q: "do not allow a
	// report to silently change after approval") — enforced by the
	// service layer, not the DB; a correction is always a NEW row linked
	// via SupersedesID.
	SupersedesID *uint `json:"supersedesId,omitempty"`

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// RegulatoryDataException (item F) — a detected data-lineage problem for
// one return, distinct from Phase 6/9's CrossSystemReconciliation (which
// is generic loan/payment/accounting reconciliation) — this one is
// specifically scoped to "does this regulatory return's numbers trace
// back correctly to source data."
type RegulatoryDataException struct {
	ID              uint   `gorm:"primaryKey" json:"exceptionId"`
	ReturnTrackingID uint  `gorm:"not null;index" json:"returnTrackingId"`
	ExceptionType   string `gorm:"not null" json:"exceptionType"` // e.g. "MISSING_RECORDS","DUPLICATE_RECORDS","OUTSTANDING_BALANCE_MISMATCH"
	Detail          string `gorm:"type:text" json:"detail"`
	Status          string `gorm:"not null;default:'OPEN'" json:"status"`
	CreatedAt       time.Time `json:"createdAt"`
	ResolvedAt      *time.Time `json:"resolvedAt,omitempty"`
	ResolvedBy      string     `json:"resolvedBy,omitempty"`
}
