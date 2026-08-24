package models

import "time"

// --- H/I) Evidence repository + version control ---

const (
	EvidenceStatusActive     = "ACTIVE"
	EvidenceStatusSuperseded = "SUPERSEDED"
	EvidenceStatusExpired    = "EXPIRED"
)

// ComplianceEvidence — one document/record, stored via internal/storage's
// existing AES-256-GCM encrypted provider (Phase 3) — the same mechanism
// KYC documents use, not a new, separate storage path. Never publicly
// accessible: StorageReference is an opaque reference only
// internal/storage.Active().Retrieve can resolve, exactly like a KYC
// document.
type ComplianceEvidence struct {
	ID              uint   `gorm:"primaryKey" json:"evidenceId"`
	RequirementID   *uint  `gorm:"index" json:"requirementId,omitempty"`
	CalendarItemID  *uint  `gorm:"index" json:"calendarItemId,omitempty"`
	Title           string `gorm:"not null" json:"title"`
	Category        string `json:"category,omitempty"` // e.g. "Policy","Audit Report","VAPT Report","Board Minutes" — free text, item H's list is examples, not an enum

	StorageReference string `json:"storageReference,omitempty"`
	Hash             string `json:"hash,omitempty"` // SHA-256 of the uploaded content, same integrity convention as Phase 3 documents
	MimeType         string `json:"mimeType,omitempty"`
	SizeBytes        int    `json:"sizeBytes,omitempty"`

	Version         int   `gorm:"not null;default:1" json:"version"`
	PreviousVersionID *uint `json:"previousVersionId,omitempty"` // item I — links to the prior version, never overwritten

	UploadedByAdminID uint      `json:"uploadedByAdminId"`
	UploadedBy        string    `json:"uploadedBy"`
	UploadedAt        time.Time `json:"uploadedAt"`

	EffectiveDate *time.Time `json:"effectiveDate,omitempty"`
	ExpiryDate    *time.Time `json:"expiryDate,omitempty"`

	ReviewerName string     `json:"reviewerName,omitempty"`
	ApprovedBy   string     `json:"approvedBy,omitempty"`
	ApprovedAt   *time.Time `json:"approvedAt,omitempty"`

	Status string `gorm:"not null;default:'ACTIVE'" json:"status"`
	Notes  string `gorm:"type:text" json:"notes,omitempty"`

	CreatedAt time.Time `json:"createdAt"`
}

// --- J/K) Policy register + review workflow ---

const (
	PolicyStatusDraft      = "DRAFT"
	PolicyStatusReview     = "REVIEW"
	PolicyStatusApproval   = "APPROVAL"
	PolicyStatusEffective  = "EFFECTIVE"
	PolicyStatusRevision   = "REVISION"
	PolicyStatusSuperseded = "SUPERSEDED"
)

type Policy struct {
	ID           uint   `gorm:"primaryKey" json:"policyId"`
	Name         string `gorm:"not null" json:"name"`
	Category     string `json:"category,omitempty"` // free text, item J's list is examples only
	Version      int    `gorm:"not null;default:1" json:"version"`
	PreviousPolicyID *uint `json:"previousPolicyId,omitempty"`

	EffectiveDate time.Time  `json:"effectiveDate,omitempty"`
	ReviewDate    *time.Time `json:"reviewDate,omitempty"`
	NextReview    *time.Time `json:"nextReview,omitempty"`

	OwnerAdminID    *uint  `json:"ownerAdminId,omitempty"`
	OwnerName       string `json:"ownerName,omitempty"`
	ApproverName    string `json:"approverName,omitempty"`

	Status         string `gorm:"not null;default:'DRAFT'" json:"status"`
	EvidenceID     *uint  `json:"evidenceId,omitempty"` // links to the ComplianceEvidence row holding the actual document — a policy is never "claimed to exist" without one (item J)

	CreatedBy string    `json:"createdBy"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// --- L/M/N) Internal audit + findings + CAPA ---

const (
	AuditStatusPlanned    = "PLANNED"
	AuditStatusInProgress = "IN_PROGRESS"
	AuditStatusReporting  = "REPORTING"
	AuditStatusClosed     = "CLOSED"
)

type InternalAudit struct {
	ID         uint      `gorm:"primaryKey" json:"auditId"`
	AuditType  string    `json:"auditType,omitempty"` // free text, e.g. "Internal Audit","Compliance Review","Security Audit"
	Scope      string     `gorm:"type:text" json:"scope,omitempty"`
	Period     string     `json:"period,omitempty"`
	AuditorName string    `json:"auditorName,omitempty"`
	Department string     `json:"department,omitempty"`
	RiskRating string     `json:"riskRating,omitempty"`
	StartDate  time.Time  `json:"startDate,omitempty"`
	EndDate    *time.Time `json:"endDate,omitempty"`
	Status     string     `gorm:"not null;default:'PLANNED'" json:"status"`

	CreatedBy string    `json:"createdBy"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

const (
	FindingSeverityCritical    = "CRITICAL"
	FindingSeverityHigh        = "HIGH"
	FindingSeverityMedium      = "MEDIUM"
	FindingSeverityLow         = "LOW"
	FindingSeverityObservation = "OBSERVATION"

	FindingStatusOpen               = "OPEN"
	FindingStatusManagementResponse = "MANAGEMENT_RESPONSE"
	FindingStatusCorrectiveAction   = "CORRECTIVE_ACTION"
	FindingStatusPendingVerification = "PENDING_VERIFICATION"
	FindingStatusClosed             = "CLOSED"
)

// AuditFinding — item M's full lifecycle. Closure (item M: "do not allow
// a finding to be marked closed without required evidence/reviewer
// approval") is enforced by the service layer requiring both
// VerificationEvidence and a VerifiedBy name before Status can become
// CLOSED.
type AuditFinding struct {
	ID       uint   `gorm:"primaryKey" json:"findingId"`
	AuditID  uint   `gorm:"not null;index" json:"auditId"`
	Title    string `gorm:"not null" json:"title"`
	Description string `gorm:"type:text" json:"description,omitempty"`
	RootCause   string `gorm:"type:text" json:"rootCause,omitempty"`
	Risk        string `gorm:"type:text" json:"risk,omitempty"`
	Recommendation string `gorm:"type:text" json:"recommendation,omitempty"`

	Severity string `gorm:"not null" json:"severity"`
	Status   string `gorm:"not null;default:'OPEN';index" json:"status"`

	ManagementResponse string     `gorm:"type:text" json:"managementResponse,omitempty"`
	ManagementRespondedBy string  `json:"managementRespondedBy,omitempty"`
	ManagementRespondedAt *time.Time `json:"managementRespondedAt,omitempty"`

	OwnerAdminID *uint      `json:"ownerAdminId,omitempty"`
	OwnerName    string     `json:"ownerName,omitempty"`
	DueDate      *time.Time `json:"dueDate,omitempty"`

	VerificationEvidence string     `json:"verificationEvidence,omitempty"`
	VerifiedBy           string     `json:"verifiedBy,omitempty"`
	VerifiedAt            *time.Time `json:"verifiedAt,omitempty"`
	ClosedAt               *time.Time `json:"closedAt,omitempty"`

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

const (
	CAPAStatusOpen                = "OPEN"
	CAPAStatusInProgress          = "IN_PROGRESS"
	CAPAStatusPendingVerification = "PENDING_VERIFICATION"
	CAPAStatusClosed              = "CLOSED"
	CAPAStatusOverdue             = "OVERDUE"
	CAPAStatusRejected            = "REJECTED"
)

type CAPA struct {
	ID         uint   `gorm:"primaryKey" json:"capaId"`
	FindingID  uint   `gorm:"not null;index" json:"findingId"`
	RootCause  string `gorm:"type:text" json:"rootCause,omitempty"`
	Action     string `gorm:"type:text;not null" json:"action"`
	OwnerAdminID *uint `json:"ownerAdminId,omitempty"`
	OwnerName  string `json:"ownerName,omitempty"`
	TargetDate time.Time `json:"targetDate,omitempty"`
	Priority   string `json:"priority,omitempty"`
	Status     string `gorm:"not null;default:'OPEN';index" json:"status"`

	Evidence   string     `json:"evidence,omitempty"`
	ReviewerName string   `json:"reviewerName,omitempty"`
	VerifiedAt *time.Time `json:"verifiedAt,omitempty"`
	ClosedAt   *time.Time `json:"closedAt,omitempty"`
	RejectionReason string `json:"rejectionReason,omitempty"`

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// --- X) Compliance alerts ---

const (
	ComplianceAlertStatusOpen     = "OPEN"
	ComplianceAlertStatusResolved = "RESOLVED"
)

type ComplianceAlert struct {
	ID        uint   `gorm:"primaryKey" json:"alertId"`
	AlertType string `gorm:"not null;index" json:"alertType"`
	Severity  string `gorm:"not null" json:"severity"`
	Message   string `gorm:"not null" json:"message"`
	ReferenceType string `json:"referenceType,omitempty"`
	ReferenceID   string `json:"referenceId,omitempty"`

	OwnerName string `json:"ownerName,omitempty"`
	Status    string `gorm:"not null;default:'OPEN'" json:"status"`
	Resolution string `json:"resolution,omitempty"`

	CreatedAt  time.Time  `json:"createdAt"`
	ResolvedAt *time.Time `json:"resolvedAt,omitempty"`
}
