// Package models — Phase 5 loan portfolio risk / DPD / NPA classification /
// provisioning foundation. As with Phase 4, every monetary field here is
// integer paise (int64), never a float.
//
// IMPORTANT: none of the DPD ranges, classification labels, or
// provisioning rates anywhere in this file are regulatory claims. They are
// configuration rows (ClassificationRule/ProvisioningRule/
// DelinquencyBucket) that start EMPTY/UNAPPROVED — see
// internal/risk/seed.go's doc comment — and must be entered and approved
// by the NBFC's compliance officer/CA before the classification/
// provisioning engines can produce a result for any loan.
package models

import "time"

// --- DPD (item A) ---

const (
	DPDRecordStatusCalculated = "CALCULATED"
)

// DPDRecord is one installment's days-past-due snapshot as of
// CalculationDate. The unique index on (schedule_id, calculation_date)
// is what makes the daily job idempotent — a second run on the same date
// for the same installment updates nothing new, it just returns the
// existing row (see internal/risk/dpd.go).
type DPDRecord struct {
	ID          uint      `gorm:"primaryKey" json:"dpdRecordId"`
	LoanID      uint      `gorm:"not null;index" json:"loanId"`
	ScheduleID  uint      `gorm:"not null;uniqueIndex:idx_schedule_calc_date" json:"scheduleId"` // EmiInstallment.ID

	DueDate           time.Time `gorm:"not null" json:"dueDate"`
	DueAmountPaise    int64     `gorm:"not null" json:"dueAmountPaise"`
	PaidAmountPaise   int64     `gorm:"not null;default:0" json:"paidAmountPaise"`
	OutstandingPaise  int64     `gorm:"not null" json:"outstandingAmountPaise"`

	DPD             int    `gorm:"not null;index" json:"dpd"` // never negative — clamped to 0 at computation time
	CalculationDate time.Time `gorm:"not null;uniqueIndex:idx_schedule_calc_date" json:"calculationDate"`
	Status          string `gorm:"not null;default:'CALCULATED'" json:"status"`

	CreatedAt time.Time `json:"createdAt"`
}

// --- Delinquency buckets (item C) — configurable, not hard-coded ---

const (
	BucketStatusActive   = "ACTIVE"
	BucketStatusInactive = "INACTIVE"
)

// DelinquencyBucket is a configurable DPD range with a display name — the
// names CURRENT/EARLY_OVERDUE/DELINQUENT/HIGH_RISK/NPA/WRITTEN_OFF used in
// internal/risk/seed.go are an unapproved STARTING STRUCTURE, explicitly
// not a claim of the final regulatory bucket definitions (see the Phase 5
// report). RuleVersion + EffectiveFrom/To let the bucket definitions
// themselves change over time without losing which version applied to a
// historical DPDRecord's bucket assignment.
type DelinquencyBucket struct {
	ID             uint       `gorm:"primaryKey" json:"bucketId"`
	Name           string     `gorm:"not null" json:"name"`
	MinDPD         int        `gorm:"not null" json:"minDpd"`
	MaxDPD         int        `gorm:"not null" json:"maxDpd"` // inclusive; a very large number (e.g. 999999) represents "and above"
	Priority       int        `gorm:"not null" json:"priority"` // lower = evaluated first
	Status         string     `gorm:"not null;default:'ACTIVE'" json:"status"`
	EffectiveFrom  time.Time  `gorm:"not null" json:"effectiveFrom"`
	EffectiveTo    *time.Time `json:"effectiveTo,omitempty"`
	RuleVersion    string     `gorm:"not null" json:"ruleVersion"`

	CreatedAt time.Time `json:"createdAt"`
}

// --- Classification (item D/E) ---

const (
	ClassificationStatusDraft    = "DRAFT"    // created, not yet approved — cannot be used to classify a live loan
	ClassificationStatusApproved = "APPROVED" // maker-checker approved, usable
	ClassificationStatusRetired  = "RETIRED"
)

// ClassificationRule is one versioned rule mapping a DPD range (for a given
// product/loan type) to a classification label. Labels like
// STANDARD/SUBSTANDARD/DOUBTFUL/LOSS below are again a starting structure
// for compliance to confirm or replace — see internal/risk/seed.go.
// Nothing in internal/risk applies a rule whose Status isn't APPROVED
// (item D: "never allow... without appropriate authorization" — enforced
// by gating on Status, and Status only becomes APPROVED via the
// maker-checker admin flow in internal/risk/rules_handler.go).
type ClassificationRule struct {
	ID              uint       `gorm:"primaryKey" json:"ruleId"`
	RuleVersion     string     `gorm:"not null;index" json:"ruleVersion"`
	EffectiveDate   time.Time  `gorm:"not null" json:"effectiveDate"`
	Product         string     `json:"product,omitempty"`  // empty = applies to all products
	LoanType        string     `json:"loanType,omitempty"` // empty = applies to all loan types
	Classification  string     `gorm:"not null" json:"classification"`
	MinDPD          int        `gorm:"not null" json:"minDpd"`
	MaxDPD          int        `gorm:"not null" json:"maxDpd"`
	Status          string     `gorm:"not null;default:'DRAFT'" json:"status"`

	CreatedBy      string     `json:"createdBy"`
	ApprovedBy     string     `json:"approvedBy,omitempty"`
	ApprovalDate   *time.Time `json:"approvalDate,omitempty"`

	CreatedAt time.Time `json:"createdAt"`
}

// LoanClassification is the CURRENT classification for one loan — a
// single mutable row per loan, always written together with a new
// LoanClassificationHistory row in the same transaction (see
// internal/risk/classification.go) so the two can never disagree about
// what the current state is or how it got there.
type LoanClassification struct {
	LoanID         uint      `gorm:"primaryKey" json:"loanId"`
	Classification string    `gorm:"not null" json:"classification"`
	RuleVersion    string    `json:"ruleVersion"`
	CalculatedAt   time.Time `json:"calculatedAt"`
	CalculatedBy   string    `json:"calculatedBy"` // "system" for the daily job, an admin username for a manual review
}

// LoanClassificationHistory (item E) — append-only. No code path in this
// package updates or deletes a row here; a correction is always a new row
// with the new classification and a reason, same convention as Phase 4's
// posted-journal immutability.
type LoanClassificationHistory struct {
	ID                 uint       `gorm:"primaryKey" json:"historyId"`
	LoanID             uint       `gorm:"not null;index" json:"loanId"`
	OldClassification  string     `json:"oldClassification,omitempty"`
	NewClassification  string     `gorm:"not null" json:"newClassification"`
	EffectiveDate      time.Time  `gorm:"not null" json:"effectiveDate"`
	Reason             string     `gorm:"not null" json:"reason"`
	RuleVersion        string     `json:"ruleVersion,omitempty"`
	CalculatedBy       string     `gorm:"not null" json:"calculatedBy"` // "system" or an admin username
	ApprovedBy         string     `json:"approvedBy,omitempty"`         // set only for a manual override requiring approval

	CreatedAt time.Time `json:"createdAt"`
}

// --- Provisioning (item G) ---

const (
	ProvisionStatusActive   = "ACTIVE"
	ProvisionStatusReversed = "REVERSED"
	ProvisionStatusFailed   = "FAILED"
)

// ProvisioningRule maps a classification to a provision rate — again a
// versioned, approval-gated configuration row, never a hard-coded
// percentage. See ClassificationRule's doc comment for the same
// maker-checker/Status-APPROVED gating pattern.
type ProvisioningRule struct {
	ID                  uint       `gorm:"primaryKey" json:"ruleId"`
	RuleVersion         string     `gorm:"not null;index" json:"ruleVersion"`
	Classification      string     `gorm:"not null" json:"classification"`
	ProvisionRatePercent float64   `gorm:"not null" json:"provisionRatePercent"`
	EffectiveDate       time.Time  `gorm:"not null" json:"effectiveDate"`
	Status              string     `gorm:"not null;default:'DRAFT'" json:"status"`

	CreatedBy    string     `json:"createdBy"`
	ApprovedBy   string     `json:"approvedBy,omitempty"`
	ApprovalDate *time.Time `json:"approvalDate,omitempty"`

	CreatedAt time.Time `json:"createdAt"`
}

// LoanProvision is one calculated provision for one loan as of
// CalculationDate — a new row every time it's recalculated (e.g. after a
// classification change), never updated in place, so
// "what was required as of date X" is always answerable. The unique index
// prevents the daily job from double-provisioning the same loan on the
// same date (item U #10).
type LoanProvision struct {
	ID                   uint      `gorm:"primaryKey" json:"provisionId"`
	LoanID               uint      `gorm:"not null;uniqueIndex:idx_loan_provision_calc_date" json:"loanId"`
	Classification       string    `gorm:"not null" json:"classification"`
	ExposureBasePaise    int64     `gorm:"not null" json:"exposureBasePaise"`
	ProvisionRatePercent float64   `gorm:"not null" json:"provisionRatePercent"`
	ProvisionAmountPaise int64     `gorm:"not null" json:"provisionAmountPaise"`
	RuleVersion          string    `gorm:"not null" json:"ruleVersion"`
	CalculationDate      time.Time `gorm:"not null;uniqueIndex:idx_loan_provision_calc_date" json:"calculationDate"`
	Status               string    `gorm:"not null;default:'ACTIVE'" json:"status"`
	JournalReference     string    `json:"journalReference,omitempty"`

	// SupersededByID links a REVERSED provision to the new ACTIVE one that
	// replaced it, when classification changes trigger a recalculation —
	// the old row is never deleted, only marked REVERSED.
	SupersededByID *uint `json:"supersededById,omitempty"`

	CreatedAt time.Time `json:"createdAt"`
}

// --- Daily job tracking (item F) ---

const (
	RiskJobStatusRunning   = "RUNNING"
	RiskJobStatusCompleted = "COMPLETED"
	RiskJobStatusFailed    = "FAILED"
	RiskJobStatusPartial   = "PARTIAL_FAILURE" // some loans succeeded, some failed — never silently treated as success
)

// RiskJobRun tracks one execution of the daily portfolio risk job. The
// unique index on RunDate means the job is idempotent at the run level —
// a second trigger for a date that already has a COMPLETED run is refused
// (see internal/risk/job.go); a FAILED or PARTIAL_FAILURE run for that
// date CAN be safely retried, since individual loan processing within a
// run is itself keyed by (loan_id, calculation_date) uniqueness at the
// DPDRecord/LoanProvision level.
type RiskJobRun struct {
	ID             uint       `gorm:"primaryKey" json:"jobRunId"`
	RunDate        time.Time  `gorm:"not null;uniqueIndex" json:"runDate"`
	Status         string     `gorm:"not null" json:"status"`
	LoansProcessed int        `json:"loansProcessed"`
	LoansFailed    int        `json:"loansFailed"`
	FailureDetail  string     `gorm:"type:text" json:"failureDetail,omitempty"`
	StartedAt      time.Time  `json:"startedAt"`
	CompletedAt    *time.Time `json:"completedAt,omitempty"`
}

// --- Alerts (item O) ---

const (
	AlertSeverityInfo     = "INFO"
	AlertSeverityWarning  = "WARNING"
	AlertSeverityCritical = "CRITICAL"

	AlertStatusOpen         = "OPEN"
	AlertStatusAcknowledged = "ACKNOWLEDGED"
	AlertStatusResolved     = "RESOLVED"
)

// RiskAlert is an INTERNAL operational alert (admin panel / ops visibility)
// — deliberately separate from any customer-facing notification/SMS/email
// channel (internal/notifications), per item O's instruction not to
// conflate the two.
type RiskAlert struct {
	ID        uint   `gorm:"primaryKey" json:"alertId"`
	AlertType string `gorm:"not null;index" json:"alertType"` // e.g. "EMI_OVERDUE","BUCKET_CHANGE","NPA_CLASSIFICATION","BROKEN_PTP","JOB_FAILED"
	Severity  string `gorm:"not null" json:"severity"`
	LoanID    *uint  `gorm:"index" json:"loanId,omitempty"`
	Message   string `gorm:"not null" json:"message"`
	Status    string `gorm:"not null;default:'OPEN'" json:"status"`

	AcknowledgedBy string     `json:"acknowledgedBy,omitempty"`
	AcknowledgedAt *time.Time `json:"acknowledgedAt,omitempty"`
	ResolvedBy     string     `json:"resolvedBy,omitempty"`
	ResolvedAt     *time.Time `json:"resolvedAt,omitempty"`

	CreatedAt time.Time `json:"createdAt"`
}
