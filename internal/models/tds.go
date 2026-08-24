// Package models — Phase 10 TDS/tax-rule foundation.
//
// CRITICAL, non-negotiable design constraint carried through every model
// in this file: NOTHING here hard-codes a section number, a rate, a
// threshold, a form name, or a due date as fact. Every one of those is a
// field on a TDSRule/TaxComplianceCalendarItem row that a CA/tax
// professional enters and approves — this codebase seeds NO rule rows
// (see internal/tax/seed.go's doc comment) and asserts no view on
// whether Income-tax Act, 1961 or Income-tax Act, 2025 provisions apply
// to any given transaction. The Act/section/threshold/rate fields below
// are free-text/numeric fields an authorized human fills in, never
// literals or enums this code defines.
package models

import "time"

// TDSRule status mirrors the maker-checker pattern already established
// for risk classification/provisioning rules (Phase 5) and regulatory
// report definitions (Phase 6) — DRAFT is never usable for a real
// calculation; only APPROVED rules are eligible, and approval requires a
// different admin than the drafter.
const (
	TDSRuleStatusDraft    = "DRAFT"
	TDSRuleStatusApproved = "APPROVED"
	TDSRuleStatusRetired  = "RETIRED"
)

// TDSRule is one versioned tax-deduction rule. RuleVersion +
// EffectiveFrom/EffectiveTo let multiple versions of "the rule for nature
// of payment X" coexist — a historical transaction's TDSTransaction row
// stores which specific TDSRule.ID (and therefore which version) was
// actually used, so a later rule change never silently reaches back and
// changes what a past transaction owes (item O).
type TDSRule struct {
	ID uint `gorm:"primaryKey" json:"ruleId"`

	// TaxAct/SectionRef are free text an authorized admin enters after
	// confirming against an official source (item Y) — e.g. "Income-tax
	// Act, 2025" + "Section 393" for a post-01-Apr-2026 rule, or
	// "Income-tax Act, 1961" + "194J" for an older one. This code never
	// compares these strings against a hard-coded list.
	TaxAct        string `gorm:"not null" json:"taxAct"`
	SectionRef    string `gorm:"not null" json:"sectionRef"`
	RuleVersion   string `gorm:"not null" json:"ruleVersion"`

	NatureOfPayment string `gorm:"not null;index" json:"natureOfPayment"` // free text, e.g. "Professional Fees" — matched against Expense.Category by an admin's own mapping choice, not inferred
	PayeeType       string `json:"payeeType,omitempty"`                  // free text, e.g. "resident_vendor" | "non_resident" — admin-defined vocabulary, not an enum this code enforces
	ResidentStatus  string `json:"residentStatus,omitempty"`

	ThresholdPaise int64   `json:"thresholdPaise"` // 0 is a valid configured value (no threshold) — never defaulted to a nonzero guess
	RatePercent    float64 `json:"ratePercent"`
	SurchargePercent float64 `json:"surchargePercent,omitempty"`
	CessPercent      float64 `json:"cessPercent,omitempty"`

	RequiresPAN         bool   `json:"requiresPan"`
	NoPANRatePercent    float64 `json:"noPanRatePercent,omitempty"` // the (typically higher) rate when PAN is missing/invalid — left 0 until a CA confirms it applies
	ApplicabilityConditions string `gorm:"type:text" json:"applicabilityConditions,omitempty"`
	ExceptionsNote          string `gorm:"type:text" json:"exceptionsNote,omitempty"`
	SourceReference         string `gorm:"type:text" json:"sourceReference,omitempty"` // item Y: the official source this rule was verified against — required before APPROVED, enforced by the service layer, not the DB

	EffectiveFrom time.Time  `gorm:"not null" json:"effectiveFrom"`
	EffectiveTo   *time.Time `json:"effectiveTo,omitempty"`

	Status       string     `gorm:"not null;default:'DRAFT'" json:"status"`
	CreatedBy    string     `json:"createdBy"`
	ApprovedBy   string     `json:"approvedBy,omitempty"`
	ApprovedAt   *time.Time `json:"approvedAt,omitempty"`

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// TDSDeductee is the payee/PAN master (item C/J) — one row per vendor (or
// other payee) tracked for TDS purposes. Kept separate from Vendor
// (Phase 8) since not every TDS deductee need be a formal Vendor (e.g. a
// one-off professional payment), though the common case links VendorID.
const (
	PANStatusNotProvided = "NOT_PROVIDED"
	PANStatusValid       = "VALID"
	PANStatusInvalid     = "INVALID"
	PANStatusUnverified  = "UNVERIFIED" // present but not checked against any validation source
)

type TDSDeductee struct {
	ID       uint  `gorm:"primaryKey" json:"deducteeId"`
	VendorID *uint `gorm:"index" json:"vendorId,omitempty"`
	Name     string `gorm:"not null" json:"name"`

	PANEncrypted string `json:"-"`
	PANLast4     string `json:"panLast4,omitempty"`
	PANStatus    string `gorm:"not null;default:'NOT_PROVIDED'" json:"panStatus"`
	PANValidatedAt     *time.Time `json:"panValidatedAt,omitempty"`
	PANValidationSource string    `json:"panValidationSource,omitempty"` // e.g. "manual" | a verification provider name — never fabricated
	PANFailureReason    string    `json:"panFailureReason,omitempty"`

	ResidentStatus string `json:"residentStatus,omitempty"` // free text, admin-entered

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// TDSChallan (item F) — one government-deposit event. Deliberately
// distinct from "TDS deducted" (TDSTransaction) — this system never
// marks deducted TDS as deposited automatically (item E); only an
// explicit challan-recording action, tied to real challan/bank evidence,
// does that.
const (
	ChallanStatusPending               = "PENDING"
	ChallanStatusDue                   = "DUE"
	ChallanStatusPaid                  = "PAID"
	ChallanStatusLate                  = "LATE"
	ChallanStatusPartiallyPaid         = "PARTIALLY_PAID"
	ChallanStatusReconciliationRequired = "RECONCILIATION_REQUIRED"
	ChallanStatusCancelled             = "CANCELLED"
)

type TDSChallan struct {
	ID            uint      `gorm:"primaryKey" json:"challanId"`
	ChallanNumber string    `gorm:"uniqueIndex" json:"challanNumber,omitempty"` // set once the actual government challan is available
	BSRCode       string    `json:"bsrCode,omitempty"`
	TaxYear       string    `gorm:"not null;index" json:"taxYear"` // e.g. "2026-27" — admin-entered, this code doesn't compute FY boundaries
	Quarter       string    `gorm:"index" json:"quarter,omitempty"`

	AmountPaise int64  `gorm:"not null" json:"amountPaise"`
	Status      string `gorm:"not null;default:'PENDING'" json:"status"`

	DeductionPeriodFrom time.Time  `json:"deductionPeriodFrom"`
	DeductionPeriodTo   time.Time  `json:"deductionPeriodTo"`
	DepositDueDate      *time.Time `json:"depositDueDate,omitempty"` // left nil unless an admin/compliance calendar entry supplies a verified due date — never computed from a hard-coded rule
	ActualDepositDate   *time.Time `json:"actualDepositDate,omitempty"`

	BankReference     string `json:"bankReference,omitempty"`
	JournalReference  string `json:"journalReference,omitempty"` // the TDS-payable-to-cash posting once paid

	ResponsibleAdminID *uint  `json:"responsibleAdminId,omitempty"`
	ResponsibleName    string `json:"responsibleName,omitempty"`
	ApprovedByAdminID  *uint  `json:"approvedByAdminId,omitempty"`
	ApprovedByName     string `json:"approvedByName,omitempty"`

	ReconciliationStatus string `json:"reconciliationStatus,omitempty"`

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// TDSException (item K) — one detected data-quality/compliance gap.
const (
	TDSExceptionSeverityLow      = "LOW"
	TDSExceptionSeverityMedium   = "MEDIUM"
	TDSExceptionSeverityHigh     = "HIGH"
	TDSExceptionSeverityCritical = "CRITICAL"

	TDSExceptionStatusOpen     = "OPEN"
	TDSExceptionStatusResolved = "RESOLVED"
)

type TDSException struct {
	ID          uint   `gorm:"primaryKey" json:"exceptionId"`
	ExceptionType string `gorm:"not null;index" json:"exceptionType"` // e.g. "PAN_MISSING","RULE_MISSING","CHALLAN_MISSING","DEPOSIT_OVERDUE","DUPLICATE_DEDUCTION"
	Severity      string `gorm:"not null" json:"severity"`
	Detail        string `gorm:"type:text" json:"detail"`
	ReferenceType string `json:"referenceType,omitempty"`
	ReferenceID   string `json:"referenceId,omitempty"`

	Status         string     `gorm:"not null;default:'OPEN';index" json:"status"`
	OwnerAdminID   *uint      `json:"ownerAdminId,omitempty"`
	OwnerName      string     `json:"ownerName,omitempty"`
	DueDate        *time.Time `json:"dueDate,omitempty"`
	Resolution     string     `json:"resolution,omitempty"`
	Evidence       string     `json:"evidence,omitempty"`
	ResolvedBy     string     `json:"resolvedBy,omitempty"`
	ResolvedAt     *time.Time `json:"resolvedAt,omitempty"`

	CreatedAt time.Time `json:"createdAt"`
}

// TaxComplianceCalendarItem (item Q) — a configurable compliance-calendar
// entry. No due date is ever computed by this code from a hard-coded
// rule; every row is admin-entered after confirming the applicable date
// from an official source.
const (
	ComplianceItemStatusPending  = "PENDING"
	ComplianceItemStatusDue      = "DUE"
	ComplianceItemStatusComplete = "COMPLETE"
	ComplianceItemStatusOverdue  = "OVERDUE"
)

type TaxComplianceCalendarItem struct {
	ID              uint       `gorm:"primaryKey" json:"calendarItemId"`
	ComplianceType  string     `gorm:"not null" json:"complianceType"` // free text, e.g. "TDS Quarterly Return" — never a hard-coded form name
	TaxYear         string     `gorm:"not null;index" json:"taxYear"`
	Quarter         string     `json:"quarter,omitempty"`
	DueDate         time.Time  `gorm:"not null" json:"dueDate"`
	ApplicableEntity string    `json:"applicableEntity,omitempty"`
	ResponsibleAdminID *uint   `json:"responsibleAdminId,omitempty"`
	ResponsibleName    string  `json:"responsibleName,omitempty"`

	Status         string     `gorm:"not null;default:'PENDING'" json:"status"`
	ReminderSentAt *time.Time `json:"reminderSentAt,omitempty"`
	Evidence       string     `json:"evidence,omitempty"`
	CompletionDate *time.Time `json:"completionDate,omitempty"`
	ReviewerName   string     `json:"reviewerName,omitempty"`

	Source string `json:"source,omitempty"` // official source this due date was verified against (item Y)

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// TDSReturnFiling (item H) — tracks preparation of a periodic TDS return.
// FormReference is free text an admin enters after confirming the
// currently-applicable form identifier from the Income Tax Department —
// this code does not assert a form name/number anywhere.
const (
	ReturnStatusDraft     = "DRAFT"
	ReturnStatusPrepared  = "PREPARED"
	ReturnStatusFiled     = "FILED"
	ReturnStatusRevised   = "REVISED"
)

type TDSReturnFiling struct {
	ID            uint      `gorm:"primaryKey" json:"returnFilingId"`
	FormReference string    `gorm:"not null" json:"formReference"` // admin-entered, e.g. "Form 140" — verify before use, see the Phase 10 report
	TaxYear       string    `gorm:"not null;index" json:"taxYear"`
	Quarter       string    `gorm:"not null" json:"quarter"`
	Status        string    `gorm:"not null;default:'DRAFT'" json:"status"`

	DeducteeCount   int   `json:"deducteeCount"`
	TransactionCount int  `json:"transactionCount"`
	TotalGrossPaise int64 `json:"totalGrossPaise"`
	TotalTDSPaise   int64 `json:"totalTdsPaise"`

	PreparedByAdminID uint       `json:"preparedByAdminId,omitempty"`
	PreparedBy        string     `json:"preparedBy,omitempty"`
	FiledAt           *time.Time `json:"filedAt,omitempty"`
	AcknowledgementRef string    `json:"acknowledgementRef,omitempty"`

	ExportRef string `json:"exportRef,omitempty"` // ReportExport reference for the generated file

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}
