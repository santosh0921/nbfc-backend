package models

import "time"

// Phase 13 analytics models. Every one of these is DECISION SUPPORT —
// none of them makes a credit, regulatory-classification, accounting, or
// tax decision. The five layers the phase spec requires be kept separate
// are separate here by construction:
//
//	OBSERVED DATA        — existing loan/payment/journal tables (untouched)
//	CALCULATED METRIC    — computed at request time in internal/analytics
//	RISK INDICATOR       — RiskSignal (a signal, never an accusation)
//	MANAGEMENT THRESHOLD — ManagementThreshold (admin-configured, never hard-coded)
//	REGULATORY RULE      — ClassificationRule/ProvisioningRule (Phase 5, untouched)

// --- Risk signals (item O) ---

const (
	SignalSeverityInfo     = "INFO"
	SignalSeverityWarning  = "WARNING"
	SignalSeverityCritical = "CRITICAL"

	SignalStatusOpen         = "OPEN"
	SignalStatusUnderReview  = "UNDER_REVIEW"
	SignalStatusFalsePositive = "FALSE_POSITIVE"
	SignalStatusConfirmed    = "CONFIRMED"
	SignalStatusClosed       = "CLOSED"
)

// RiskSignal is an observation that MAY warrant human review. It is never
// a finding of fraud — item O is explicit that these are signals and that
// nothing here may automatically accuse a customer or employee. Status
// therefore starts OPEN and only a human reviewer moves it.
type RiskSignal struct {
	ID         uint   `gorm:"primaryKey" json:"riskSignalId"`
	SignalType string `gorm:"not null;index" json:"signalType"`
	Severity   string `gorm:"not null" json:"severity"`

	// Reason is the plain-language explanation of what was observed —
	// required so no signal can exist without a stated basis.
	Reason string `gorm:"not null" json:"reason"`
	// Source names the table/query the observation came from (item AC).
	Source string `gorm:"not null" json:"source"`

	SubjectType string `gorm:"index" json:"subjectType,omitempty"` // CUSTOMER / LOAN / PAYMENT / ADMIN
	SubjectRef  string `gorm:"index" json:"subjectRef,omitempty"`

	// DedupeKey stops the same standing observation being re-raised on
	// every sweep (same pattern Phase 11's compliance alerts use).
	DedupeKey string `gorm:"uniqueIndex;not null" json:"-"`

	Status         string     `gorm:"not null;default:'OPEN'" json:"status"`
	ReviewedBy     string     `json:"reviewedBy,omitempty"`
	ReviewedAt     *time.Time `json:"reviewedAt,omitempty"`
	ResolutionNote string     `json:"resolutionNote,omitempty"`

	ObservedAt time.Time `gorm:"not null;index" json:"observedAt"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

// --- Management thresholds and alerts (items Z / AA) ---

const (
	ThresholdDirectionAbove = "ABOVE" // fires when the metric rises above the value
	ThresholdDirectionBelow = "BELOW" // fires when the metric falls below the value
)

// ManagementThreshold — item Z: "Do not hard-code thresholds." Every
// threshold is an admin-created row; this phase seeds ZERO of them, so
// the alert engine produces no alerts until management configures what
// it considers material.
type ManagementThreshold struct {
	ID         uint   `gorm:"primaryKey" json:"thresholdId"`
	MetricKey  string `gorm:"not null;uniqueIndex" json:"metricKey"` // must match a KPIDefinition.Key
	Direction  string `gorm:"not null" json:"direction"`
	// Value is interpreted in the KPI's own unit (paise for money KPIs,
	// a plain number for counts, a percentage for rate KPIs) — the KPI
	// dictionary records which, so the threshold cannot be misread.
	Value    float64 `gorm:"not null" json:"value"`
	Severity string  `gorm:"not null" json:"severity"`
	Owner    string  `json:"owner,omitempty"`
	Active   bool    `gorm:"not null;default:true" json:"active"`

	CreatedBy string    `json:"createdBy,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

const (
	ManagementAlertStatusOpen         = "OPEN"
	ManagementAlertStatusAcknowledged = "ACKNOWLEDGED"
	ManagementAlertStatusResolved     = "RESOLVED"
)

// ManagementAlert — a threshold breach that actually occurred. Carries
// everything item AA requires an alert to explain (what/why/current/
// threshold/period/source), so the alert is self-explanatory without the
// reader re-deriving it.
type ManagementAlert struct {
	ID          uint   `gorm:"primaryKey" json:"managementAlertId"`
	ThresholdID uint   `gorm:"not null;index" json:"thresholdId"`
	MetricKey   string `gorm:"not null;index" json:"metricKey"`

	CurrentValue   float64 `json:"currentValue"`
	ThresholdValue float64 `json:"thresholdValue"`
	Direction      string  `json:"direction"`
	Severity       string  `gorm:"not null" json:"severity"`

	// Explanation is a factual statement of what was measured against
	// what — never a recommendation to take a specific business action.
	Explanation string `gorm:"not null" json:"explanation"`
	Source      string `gorm:"not null" json:"source"`

	PeriodFrom time.Time `json:"periodFrom"`
	PeriodTo   time.Time `json:"periodTo"`

	DedupeKey string `gorm:"uniqueIndex;not null" json:"-"`

	Status         string     `gorm:"not null;default:'OPEN'" json:"status"`
	Owner          string     `json:"owner,omitempty"`
	ResolvedBy     string     `json:"resolvedBy,omitempty"`
	ResolvedAt     *time.Time `json:"resolvedAt,omitempty"`
	ResolutionNote string     `json:"resolutionNote,omitempty"`

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// --- KPI dictionary (item AB — "This is mandatory") ---

// KPIDefinition documents exactly how a KPI is computed and what it
// depends on, so a dashboard number is never an unexplained figure.
// Unlike the threshold table, this one IS seeded — these are descriptions
// of calculations this codebase actually performs, not business policy.
type KPIDefinition struct {
	ID         uint   `gorm:"primaryKey" json:"kpiId"`
	Key        string `gorm:"not null;uniqueIndex" json:"key"`
	Name       string `gorm:"not null" json:"name"`
	Definition string `gorm:"not null" json:"definition"`
	Formula    string `gorm:"not null" json:"formula"`
	DataSource string `gorm:"not null" json:"dataSource"`
	Frequency  string `json:"frequency,omitempty"`
	Unit       string `gorm:"not null" json:"unit"` // PAISE / COUNT / PERCENT
	Owner      string `json:"owner,omitempty"`
	Notes      string `json:"notes,omitempty"`
	// Dependency records any accounting/compliance precondition — e.g.
	// "requires approved ClassificationRule rows" — so a KPI that cannot
	// legitimately be produced yet says so in its own definition.
	Dependency string `json:"dependency,omitempty"`

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// --- Budget vs actual (item Y — foundation only, zero rows seeded) ---

// Budget is the data-model foundation item Y asks for. Item Y is explicit:
// "Do not invent budgets." No row is ever created by this codebase.
type Budget struct {
	ID        uint      `gorm:"primaryKey" json:"budgetId"`
	MetricKey string    `gorm:"not null;index" json:"metricKey"`
	PeriodFrom time.Time `gorm:"not null" json:"periodFrom"`
	PeriodTo   time.Time `gorm:"not null" json:"periodTo"`
	TargetValue float64  `gorm:"not null" json:"targetValue"`
	Unit       string    `gorm:"not null" json:"unit"`
	Notes      string    `json:"notes,omitempty"`

	CreatedBy string    `json:"createdBy,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// --- Data quality exceptions (item AI) ---

const (
	DataQualityStatusOpen     = "OPEN"
	DataQualityStatusReviewed = "REVIEWED"
	DataQualityStatusResolved = "RESOLVED"
)

// AnalyticsDataQualityException records a detected data problem. Item AI
// is explicit that historical financial data must NOT be silently fixed —
// nothing in this codebase mutates source data in response to one of
// these; they are recorded for human resolution only. Named distinctly
// from Phase 5's DataConsistencyException (loan/ledger integrity) because
// this one covers analytics-input quality and the two are reviewed by
// different people.
type AnalyticsDataQualityException struct {
	ID            uint   `gorm:"primaryKey" json:"exceptionId"`
	ExceptionType string `gorm:"not null;index" json:"exceptionType"`
	Severity      string `gorm:"not null" json:"severity"`
	Detail        string `gorm:"not null" json:"detail"`
	Source        string `gorm:"not null" json:"source"`

	DedupeKey string `gorm:"uniqueIndex;not null" json:"-"`

	Status         string     `gorm:"not null;default:'OPEN'" json:"status"`
	ReviewedBy     string     `json:"reviewedBy,omitempty"`
	ReviewedAt     *time.Time `json:"reviewedAt,omitempty"`
	ResolutionNote string     `json:"resolutionNote,omitempty"`

	DetectedAt time.Time `gorm:"not null;index" json:"detectedAt"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
}
