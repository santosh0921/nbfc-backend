// Package models — Phase 6 MIS / regulatory reporting / compliance
// monitoring / audit-readiness foundation. As with Phases 4-5, every
// monetary field is integer paise, never a float.
//
// IMPORTANT: no RBI/NBFC-specific return name, filing frequency, or
// statutory field list is hard-coded anywhere in this file.
// RegulatoryReportDefinition rows (the ONLY place a specific report's
// shape is described) start empty — see internal/reporting/seed.go's doc
// comment — and must be entered by the NBFC's compliance officer/CA
// before any regulatory report can be generated. Everything in this file
// is generic reporting INFRASTRUCTURE, not a specific regulatory claim.
package models

import "time"

// --- Generic report run tracking (item A) — used by every report type,
// MIS or regulatory, so every report shares one audit-friendly shape. ---

const (
	ReportStatusGenerated   = "GENERATED"
	ReportStatusFailed      = "FAILED"
	ReportValidationPass    = "PASS"
	ReportValidationWarning = "WARNING"
	ReportValidationError   = "ERROR"
)

// ReportRun is one execution of any report (MIS or regulatory) — the
// generic "report_id/report_type/reporting_period/generated_at/
// generated_by/status/validation_status" shape item A asks every report
// to carry. RegulatoryReportRun (below) additionally carries the
// maker-checker/versioning fields regulatory filings specifically need;
// this table covers ad hoc MIS/operational reports that don't go through
// that heavier workflow.
type ReportRun struct {
	ID               uint       `gorm:"primaryKey" json:"reportId"`
	ReportType       string     `gorm:"not null;index" json:"reportType"` // e.g. "PORTFOLIO_SUMMARY","GST_SUMMARY","TRIAL_BALANCE"
	ReportingPeriod  string     `json:"reportingPeriod,omitempty"`        // e.g. "2026-08" or a from/to encoded by the caller
	Parameters       string     `gorm:"type:text" json:"parameters,omitempty"` // JSON-encoded filter parameters, for reproducibility
	GeneratedBy      string     `gorm:"not null" json:"generatedBy"`           // admin username, or "system"
	GeneratedAt      time.Time  `json:"generatedAt"`
	GenerationMillis int64      `json:"generationMillis"`
	DataSnapshotRef  string     `json:"dataSnapshotRef,omitempty"` // e.g. the RiskJobRun/AccountingPeriod id this report is consistent with, where applicable
	Status           string     `gorm:"not null;default:'GENERATED'" json:"status"`
	ValidationStatus string     `json:"validationStatus,omitempty"`
	ResultRef        string     `json:"resultRef,omitempty"` // storage reference for an exported file, if any — see ReportExport
	ChecksumSHA256   string     `json:"checksumSha256,omitempty"`
}

// --- Regulatory reporting framework (item C) ---

const (
	RegReportStatusOpen        = "OPEN"
	RegReportStatusGenerated   = "GENERATED"
	RegReportStatusUnderReview = "UNDER_REVIEW"
	RegReportStatusApproved    = "APPROVED"
	RegReportStatusSubmitted   = "SUBMITTED"
	RegReportStatusClosed      = "CLOSED"
	RegReportStatusRejected    = "REJECTED"
)

// RegulatoryReportDefinition describes ONE specific regulatory return —
// created only after the NBFC's compliance officer/CA has confirmed it
// applies to this NBFC. No row is seeded by this codebase (see
// internal/reporting/seed.go). ValidationRequired marks that the
// definition itself (not just individual runs) is still
// CONFIGURED_NOT_VALIDATED vs VALIDATED — see item T's compliance matrix.
const (
	DefinitionStatusConfiguredNotValidated = "CONFIGURED_NOT_VALIDATED"
	DefinitionStatusValidated              = "VALIDATED"
	DefinitionStatusProductionReady        = "PRODUCTION_READY"
)

type RegulatoryReportDefinition struct {
	ID               uint      `gorm:"primaryKey" json:"definitionId"`
	ReportType       string    `gorm:"uniqueIndex;not null" json:"reportType"` // a short code, e.g. "NBFC_RETURN_X" — assigned by compliance, not invented here
	ReportingAuthority string  `json:"reportingAuthority,omitempty"`           // e.g. "RBI" — left blank until compliance names it
	Description      string    `json:"description,omitempty"`
	Frequency        string    `json:"frequency,omitempty"` // e.g. "MONTHLY","QUARTERLY" — left blank until compliance confirms
	Status           string    `gorm:"not null;default:'CONFIGURED_NOT_VALIDATED'" json:"status"`

	CreatedBy string    `json:"createdBy"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// RegulatoryReportField is one field within a definition's version — the
// "data mapping" item C calls for. Kept as its own table (not a JSON blob
// on the version) so a validation run can check field-by-field
// completeness.
type RegulatoryReportField struct {
	ID                   uint   `gorm:"primaryKey" json:"fieldId"`
	DefinitionVersionID  uint   `gorm:"not null;index" json:"definitionVersionId"`
	FieldCode            string `gorm:"not null" json:"fieldCode"`
	FieldLabel           string `json:"fieldLabel,omitempty"`
	SourceDescription    string `json:"sourceDescription,omitempty"` // human-readable note on where this field's value comes from — not executable, reviewed by compliance
	Required             bool   `gorm:"default:true" json:"required"`
}

// RegulatoryReportVersion — item N: every generated report must be
// reproducible even if the definition later changes. A version is
// immutable once any run using it reaches APPROVED (enforced in
// internal/reporting, not by a DB trigger, to keep this schema portable).
type RegulatoryReportVersion struct {
	ID           uint      `gorm:"primaryKey" json:"versionId"`
	DefinitionID uint      `gorm:"not null;index" json:"definitionId"`
	VersionLabel string    `gorm:"not null" json:"versionLabel"` // e.g. "v1"
	Notes        string    `json:"notes,omitempty"`
	CreatedBy    string    `json:"createdBy"`
	CreatedAt    time.Time `json:"createdAt"`
}

// RegulatoryReportRun — item C/D/N/P: one generation of one report
// version for one reporting period, carrying the full maker-checker +
// versioning + submission trail.
type RegulatoryReportRun struct {
	ID                 uint       `gorm:"primaryKey" json:"runId"`
	DefinitionID       uint       `gorm:"not null;index" json:"definitionId"`
	DefinitionVersionID uint      `gorm:"not null" json:"definitionVersionId"` // frozen at generation time — never repointed to a newer version after the fact
	ReportingPeriod    string     `gorm:"not null" json:"reportingPeriod"`     // e.g. "2026-08" or "2026-Q2"
	Status             string     `gorm:"not null;default:'OPEN';index" json:"status"`

	Parameters      string `gorm:"type:text" json:"parameters,omitempty"`
	DataSnapshotRef string `json:"dataSnapshotRef,omitempty"`
	ResultRef       string `json:"resultRef,omitempty"` // storage reference for the generated file
	ChecksumSHA256  string `json:"checksumSha256,omitempty"`

	PreparedByAdminID uint       `json:"preparedByAdminId,omitempty"`
	PreparedBy        string     `json:"preparedBy,omitempty"`
	GeneratedAt       *time.Time `json:"generatedAt,omitempty"`

	CheckedByAdminID *uint      `json:"checkedByAdminId,omitempty"`
	CheckedBy        string     `json:"checkedBy,omitempty"`
	ReviewedAt       *time.Time `json:"reviewedAt,omitempty"`
	ReviewDecision   string     `json:"reviewDecision,omitempty"` // "APPROVED" | "REJECTED"
	ReviewReason     string     `json:"reviewReason,omitempty"`

	SubmittedAt        *time.Time `json:"submittedAt,omitempty"`
	SubmissionReference string    `json:"submissionReference,omitempty"` // an acknowledgment/filing reference number, entered manually — this codebase does not submit to any regulator API

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// RegulatoryReportReview — item C's explicit review-record table,
// separate from the summary fields on RegulatoryReportRun so a run that's
// reviewed more than once (rejected, corrected, resubmitted) keeps every
// review event, not just the latest.
type RegulatoryReportReview struct {
	ID             uint      `gorm:"primaryKey" json:"reviewId"`
	RunID          uint      `gorm:"not null;index" json:"runId"`
	ReviewerAdminID uint     `json:"reviewerAdminId"`
	ReviewerName   string    `json:"reviewerName"`
	Decision       string    `gorm:"not null" json:"decision"` // "APPROVED" | "REJECTED"
	Reason         string    `json:"reason,omitempty"`
	CreatedAt      time.Time `json:"createdAt"`
}

// RegulatoryReportSubmission records the act of marking a run SUBMITTED —
// kept as its own append-only table (rather than only fields on the run)
// so a resubmission history is preserved if one is ever needed.
type RegulatoryReportSubmission struct {
	ID                 uint      `gorm:"primaryKey" json:"submissionId"`
	RunID              uint      `gorm:"not null;index" json:"runId"`
	SubmittedByAdminID uint      `json:"submittedByAdminId"`
	SubmittedByName    string    `json:"submittedByName"`
	SubmissionReference string   `json:"submissionReference,omitempty"`
	Notes              string    `json:"notes,omitempty"`
	CreatedAt          time.Time `json:"createdAt"`
}

// RegulatoryReportException — item E's validation-engine findings for one
// run. A blocking ERROR-severity, unresolved exception is what
// internal/reporting's approval gate checks for (item E: "do not allow a
// report with blocking ERRORs to be approved").
type RegulatoryReportException struct {
	ID       uint   `gorm:"primaryKey" json:"exceptionId"`
	RunID    uint   `gorm:"not null;index" json:"runId"`
	Severity string `gorm:"not null" json:"severity"` // PASS | WARNING | ERROR
	// Check's column is explicitly named check_name — "check" is a
	// reserved word in Postgres and would require quoting in every raw
	// SQL query that touches it.
	Check string `gorm:"column:check_name;not null" json:"check"` // e.g. "missing_required_field","duplicate_record","accounting_mismatch"
	Detail   string `gorm:"type:text" json:"detail"`

	CreatedAt time.Time `json:"createdAt"`
}

// --- Data reconciliation dashboard (item F) ---

const (
	ReconStatusMatched        = "MATCHED"
	ReconStatusMismatch       = "MISMATCH"
	ReconStatusMissing        = "MISSING"
	ReconStatusDuplicate      = "DUPLICATE"
	ReconStatusPendingReview  = "PENDING_REVIEW"
)

// CrossSystemReconciliation is one detected comparison result between two
// of the systems named in item F (loan/accounting/payments/bank/
// collections/NPA/KYC). Distinct from Phase 2's ReconciliationException
// (bank-statement-vs-Disbursement only) and Phase 5's
// DataConsistencyException (risk/accounting internal checks only) — this
// is the broader, all-systems compliance view item F asks for; it may
// well be populated BY running those other checks, not duplicating their
// logic.
type CrossSystemReconciliation struct {
	ID          uint   `gorm:"primaryKey" json:"reconId"`
	Entity      string `gorm:"not null;index" json:"entity"` // e.g. "loan","payment","emi_installment"
	Reference   string `gorm:"not null;index" json:"reference"`
	SourceA     string `json:"sourceA"` // e.g. "loan_schedule"
	SourceB     string `json:"sourceB"` // e.g. "accounting_ledger"
	ExpectedValue string `json:"expectedValue,omitempty"`
	ActualValue   string `json:"actualValue,omitempty"`
	DifferenceValue string `json:"differenceValue,omitempty"`
	Status      string `gorm:"not null;index" json:"status"`
	Severity    string `json:"severity,omitempty"`

	AssignedToAdminID *uint      `json:"assignedToAdminId,omitempty"`
	AssignedToName    string     `json:"assignedToName,omitempty"`
	ResolutionStatus  string     `json:"resolutionStatus,omitempty"`
	ResolvedAt        *time.Time `json:"resolvedAt,omitempty"`
	ResolutionNote    string     `json:"resolutionNote,omitempty"`

	DetectedAt time.Time `json:"detectedAt"`
}

// --- Compliance task management (item J) ---

const (
	TaskStatusOpen          = "OPEN"
	TaskStatusAssigned      = "ASSIGNED"
	TaskStatusInProgress    = "IN_PROGRESS"
	TaskStatusPendingReview = "PENDING_REVIEW"
	TaskStatusResolved      = "RESOLVED"
	TaskStatusClosed        = "CLOSED"

	TaskCategoryKYC          = "KYC"
	TaskCategoryScreening    = "AML_SCREENING"
	TaskCategoryCredit       = "CREDIT"
	TaskCategoryCollections  = "COLLECTIONS"
	TaskCategoryAccounting   = "ACCOUNTING"
	TaskCategoryGST          = "GST"
	TaskCategoryAudit        = "AUDIT"
	TaskCategorySecurity     = "SECURITY"
	TaskCategoryRegReporting = "REGULATORY_REPORTING"
	TaskCategoryDataQuality  = "DATA_QUALITY"

	TaskSeverityLow      = "LOW"
	TaskSeverityMedium   = "MEDIUM"
	TaskSeverityHigh     = "HIGH"
	TaskSeverityCritical = "CRITICAL"
)

type ComplianceTask struct {
	ID          uint       `gorm:"primaryKey" json:"taskId"`
	Category    string     `gorm:"not null;index" json:"category"`
	Severity    string     `gorm:"not null" json:"severity"`
	Description string     `gorm:"not null" json:"description"`
	Reference   string     `json:"reference,omitempty"` // e.g. "loan:123"

	AssignedToAdminID *uint      `gorm:"index" json:"assignedToAdminId,omitempty"`
	AssignedToName    string     `json:"assignedToName,omitempty"`
	DueDate           *time.Time `json:"dueDate,omitempty"`
	Status            string     `gorm:"not null;default:'OPEN';index" json:"status"`
	Resolution        string     `json:"resolution,omitempty"`

	CreatedBy string     `json:"createdBy"` // "system" for an auto-generated task from a periodic check
	CreatedAt time.Time  `json:"createdAt"`
	ClosedAt  *time.Time `json:"closedAt,omitempty"`
}

// --- Exports (item L) ---

const (
	ExportFormatCSV   = "CSV"
	ExportFormatExcel = "EXCEL"
	ExportFormatPDF   = "PDF"

	ExportStatusQueued    = "QUEUED"
	ExportStatusRunning   = "RUNNING"
	ExportStatusReady     = "READY"
	ExportStatusFailed    = "FAILED"
)

// ReportExport logs every export request — item L: "log the export,
// identify exporter, record date/time, record report/filter parameters".
// Large exports run asynchronously (Status starts QUEUED, a background
// goroutine flips it to READY) so a request never blocks on generating a
// huge file inline.
type ReportExport struct {
	ID           uint      `gorm:"primaryKey" json:"exportId"`
	ReportType   string    `gorm:"not null" json:"reportType"`
	Format       string    `gorm:"not null" json:"format"`
	Parameters   string    `gorm:"type:text" json:"parameters,omitempty"`
	ExportedByAdminID uint `json:"exportedByAdminId"`
	ExportedByName    string `json:"exportedByName"`
	Status       string    `gorm:"not null;default:'QUEUED'" json:"status"`
	StorageReference string `json:"storageReference,omitempty"` // internal/storage reference (Phase 3's encrypted provider — never a public URL)
	FailureReason string   `json:"failureReason,omitempty"`

	RequestedAt time.Time  `json:"requestedAt"`
	CompletedAt *time.Time `json:"completedAt,omitempty"`
}

// --- Retention policy configuration (item M) ---

// RetentionPolicy is metadata only — it configures how long a record
// category SHOULD be retained per legal/regulatory advice, and is
// deliberately never auto-enforced by any deletion job in this codebase.
// Status stays UNVALIDATED until compliance confirms the period; even a
// VALIDATED policy still requires a separate, explicit, human-triggered
// purge action outside this phase's scope — see internal/reporting's
// retention package doc comment.
const (
	RetentionStatusUnvalidated = "UNVALIDATED"
	RetentionStatusValidated   = "VALIDATED"
)

type RetentionPolicy struct {
	ID              uint   `gorm:"primaryKey" json:"policyId"`
	RecordCategory  string `gorm:"uniqueIndex;not null" json:"recordCategory"` // e.g. "kyc_documents","loan_documents","audit_logs"
	RetentionYears  int    `json:"retentionYears,omitempty"`
	Status          string `gorm:"not null;default:'UNVALIDATED'" json:"status"`
	LegalBasisNote  string `json:"legalBasisNote,omitempty"` // free text, filled in by compliance — never inferred by this code

	ConfiguredBy string    `json:"configuredBy,omitempty"`
	ConfiguredAt time.Time `json:"configuredAt"`
}
