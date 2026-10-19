package models

import "time"

// Collection case states (item I) — OPERATIONAL states, deliberately
// distinct from the NPA/IRAC asset CLASSIFICATION states in risk.go. A
// loan's classification is a regulatory/accounting fact; a collection
// case's status is just where ops is in working the account.
const (
	CollectionStatusCurrent            = "CURRENT"
	CollectionStatusReminder           = "REMINDER"
	CollectionStatusOverdue            = "OVERDUE"
	CollectionStatusAssigned           = "COLLECTION_ASSIGNED"
	CollectionStatusEscalated          = "ESCALATED"
	CollectionStatusLegalRecoveryReview = "LEGAL_RECOVERY_REVIEW"
	CollectionStatusClosed             = "CLOSED"
	CollectionStatusResolved           = "RESOLVED"
)

// CollectionCase is one delinquent loan's collection workflow record — at
// most one OPEN case per loan at a time (enforced by the service layer,
// not a DB constraint, since a closed case followed by a new delinquency
// legitimately opens a new case).
type CollectionCase struct {
	ID         uint   `gorm:"primaryKey" json:"caseId"`
	LoanID     uint   `gorm:"not null;index" json:"loanId"`
	CustomerID string `gorm:"not null;index" json:"customerId"`

	DPD             int    `json:"dpd"`
	OverdueAmountPaise int64 `json:"overdueAmountPaise"`
	Status          string `gorm:"not null;index" json:"status"`
	Priority        string `json:"priority,omitempty"` // e.g. "LOW"/"MEDIUM"/"HIGH" — ops-assigned, not a regulatory field

	AssignedOfficerAdminID *uint  `json:"assignedOfficerAdminId,omitempty"`
	AssignedOfficerName    string `json:"assignedOfficerName,omitempty"`

	NextActionDate *time.Time `json:"nextActionDate,omitempty"`
	Resolution     string     `json:"resolution,omitempty"`
	ResolvedAt     *time.Time `json:"resolvedAt,omitempty"`

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// CollectionAssignment is the append-only history of who a case was
// assigned/reassigned to — CollectionCase.AssignedOfficer* always reflects
// the latest row here, kept in sync by internal/collections' assignment
// function rather than ever being written independently.
type CollectionAssignment struct {
	ID                 uint      `gorm:"primaryKey" json:"assignmentId"`
	CaseID             uint      `gorm:"not null;index" json:"caseId"`
	AssignedToAdminID  uint      `gorm:"not null" json:"assignedToAdminId"`
	AssignedToName     string    `json:"assignedToName"`
	AssignedByAdminID  uint      `json:"assignedByAdminId"`
	AssignedByName     string    `json:"assignedByName"`
	Reason             string    `json:"reason,omitempty"`
	CreatedAt          time.Time `json:"createdAt"`
}

// Collection action types (item J).
const (
	ActionTypeCall        = "CALL"
	ActionTypeSMS         = "SMS"
	ActionTypeEmail       = "EMAIL"
	ActionTypeReminder    = "REMINDER"
	ActionTypePaymentLink = "PAYMENT_LINK"
	ActionTypeFieldVisit  = "FIELD_VISIT"
	ActionTypePTP         = "PROMISE_TO_PAY"
	ActionTypeFollowUp    = "FOLLOW_UP"
	ActionTypeEscalation  = "ESCALATION"
	ActionTypeResolution  = "RESOLUTION"
)

// CollectionAction is one logged interaction against a case. Notes are
// free text entered by the collection officer — deliberately no field for
// raw customer financial/KYC data here; if something more sensitive needs
// recording it belongs in the customer's own KYC/document records, not a
// collection note.
type CollectionAction struct {
	ID         uint   `gorm:"primaryKey" json:"actionId"`
	CaseID     uint   `gorm:"not null;index" json:"caseId"`
	ActorAdminID uint `json:"actorAdminId"`
	ActorName  string `json:"actorName"`
	ActionType string `gorm:"not null" json:"actionType"`
	Result     string `json:"result,omitempty"` // e.g. "answered","no_response","promised","refused"
	Notes      string `json:"notes,omitempty"`
	NextAction *time.Time `json:"nextAction,omitempty"`
	CommunicationRef string `json:"communicationRef,omitempty"` // e.g. an SMS/email provider message id, where applicable

	CreatedAt time.Time `json:"createdAt"`
}

// CollectionNote is a freeform note on a case not tied to a specific
// action (e.g. context added after a manager review) — kept as its own
// table per the spec's explicit collection_notes table, distinct from
// CollectionAction.Notes which is always attached to one logged action.
type CollectionNote struct {
	ID         uint      `gorm:"primaryKey" json:"noteId"`
	CaseID     uint      `gorm:"not null;index" json:"caseId"`
	ActorAdminID uint    `json:"actorAdminId"`
	ActorName  string    `json:"actorName"`
	Note       string    `gorm:"not null" json:"note"`
	CreatedAt  time.Time `json:"createdAt"`
}

// Promise-to-pay status (item K). A promise NEVER auto-transitions to
// PAID by itself — only a verified Payment settling the relevant
// installment(s) can do that (see internal/collections.CheckPromiseStatus,
// called after a verified payment, never on a timer alone claiming
// success).
const (
	PTPStatusOpen    = "OPEN"
	PTPStatusPaid    = "PAID"
	PTPStatusBroken  = "BROKEN"
)

type CollectionPromise struct {
	ID              uint       `gorm:"primaryKey" json:"promiseId"`
	CaseID          uint       `gorm:"not null;index" json:"caseId"`
	LoanID          uint       `gorm:"not null;index" json:"loanId"`
	PromisedAmountPaise int64  `gorm:"not null" json:"promisedAmountPaise"`
	PromisedDate    time.Time  `gorm:"not null" json:"promisedDate"`
	Status          string     `gorm:"not null;default:'OPEN'" json:"status"`

	CreatedByAdminID uint      `json:"createdByAdminId"`
	CreatedByName    string    `json:"createdByName"`

	SettledPaymentID *uint      `json:"settledPaymentId,omitempty"` // the verified Payment that satisfied this PTP, if PAID
	ResolvedAt       *time.Time `json:"resolvedAt,omitempty"`

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// --- Write-off / recovery (item Q) ---

const (
	WriteOffStatusReview          = "WRITE_OFF_REVIEW"
	WriteOffStatusApproved        = "WRITE_OFF_APPROVED"
	WriteOffStatusWrittenOff      = "WRITTEN_OFF"
	WriteOffStatusRecoveryAfter   = "RECOVERY_AFTER_WRITE_OFF"
	WriteOffStatusRejected        = "REJECTED"
)

// WriteOffRequest — the loan and its payment history are NEVER deleted by
// any code path that touches this table; write-off is purely a status +
// accounting event layered on top of the existing loan record (item Q:
// "Do not delete the underlying loan or payment history").
type WriteOffRequest struct {
	ID       uint   `gorm:"primaryKey" json:"writeOffId"`
	LoanID   uint   `gorm:"not null;index" json:"loanId"`
	AmountPaise int64 `gorm:"not null" json:"amountPaise"`
	Reason   string `gorm:"not null" json:"reason"`
	Status   string `gorm:"not null;default:'WRITE_OFF_REVIEW'" json:"status"`

	RequestedByAdminID uint       `json:"requestedByAdminId"`
	RequestedByName    string     `json:"requestedByName"`
	RequestedAt        time.Time  `json:"requestedAt"`

	ApprovedByAdminID *uint      `json:"approvedByAdminId,omitempty"`
	ApprovedByName    string     `json:"approvedByName,omitempty"`
	ApprovedAt        *time.Time `json:"approvedAt,omitempty"`
	RejectionReason   string     `json:"rejectionReason,omitempty"`

	JournalReference string `json:"journalReference,omitempty"`

	UpdatedAt time.Time `json:"updatedAt"`
}

// RecoveryPayment records money recovered after a loan has been written
// off — distinct from the normal EMI-payment path since the loan is no
// longer being serviced on its original schedule.
type RecoveryPayment struct {
	ID              uint      `gorm:"primaryKey" json:"recoveryPaymentId"`
	WriteOffID      uint      `gorm:"not null;index" json:"writeOffId"`
	LoanID          uint      `gorm:"not null;index" json:"loanId"`
	AmountPaise     int64     `gorm:"not null" json:"amountPaise"`
	ReceivedAt      time.Time `gorm:"not null" json:"receivedAt"`
	RecordedByAdminID uint    `json:"recordedByAdminId"`
	RecordedByName  string    `json:"recordedByName"`
	Reference       string    `json:"reference,omitempty"` // bank/UTR reference for the recovered amount
	JournalReference string   `json:"journalReference,omitempty"`

	CreatedAt time.Time `json:"createdAt"`
}
