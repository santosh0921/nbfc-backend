package models

import "time"

// Tranche lifecycle. Deliberately separate from Disbursement's own status
// vocabulary — a tranche is never itself a Disbursement row (see
// LoanTranche's own comment for why) and has one extra real-world step
// neither regular disbursement nor Gold Loan has: a site visit to confirm
// the builder's claimed construction stage before any money moves.
const (
	TrancheStatusPendingSiteVisit = "PENDING_SITE_VISIT"
	TrancheStatusSiteRejected     = "SITE_REJECTED"
	TrancheStatusVerified         = "VERIFIED" // site visit confirmed the stage; ready for maker/checker
	TrancheStatusPrepared         = "PREPARED" // maker has set the release amount
	TrancheStatusDisbursed        = "DISBURSED"
)

// LoanTranche is one builder demand-letter cycle within a Home Loan's
// construction-linked disbursement. Deliberately NOT a models.Disbursement
// row: Disbursement has `LoanID uint gorm:"uniqueIndex"` — ONE disbursement
// per loan — which every other category's prepare/approve/confirm handler,
// reconciliation match and accounting posting assumes. Reusing it here
// would mean relaxing that constraint for every category just to support
// the one that needs many partial releases, risking the working path for
// every Personal/Instant/Gold/Business/MSME loan to support Home Loan's
// tranches. A parallel, self-contained table costs nothing those other
// categories ever see.
type LoanTranche struct {
	ID             uint `gorm:"primaryKey" json:"id"`
	LoanID         uint `gorm:"not null;index" json:"loanId"`
	TrancheNumber  int  `gorm:"not null" json:"trancheNumber"`
	IsFinalTranche bool `gorm:"not null;default:false" json:"isFinalTranche"`

	// What the builder's demand letter claimed — DemandLetterDocumentID
	// points at the actual uploaded letter (models.Document, DocType
	// "Demand Letter", same upload path every other document uses).
	ConstructionStageClaimed string    `gorm:"not null" json:"constructionStageClaimed"`
	RequestedAmount          float64   `gorm:"not null" json:"requestedAmount"`
	DemandLetterDocumentID   *string   `json:"demandLetterDocumentId,omitempty"`
	RaisedByAdminID          uint      `json:"raisedByAdminId"`
	RaisedByUsername         string    `json:"raisedByUsername"`
	CreatedAt                time.Time `json:"createdAt"`

	Status string `gorm:"not null;default:'PENDING_SITE_VISIT';index" json:"status"`

	// Site visit — confirms the construction stage actually matches the
	// demand letter before any maker/checker money action is even allowed.
	SiteVisitByAdminID *uint      `json:"siteVisitByAdminId,omitempty"`
	SiteVisitBy        string     `json:"siteVisitBy,omitempty"`
	SiteVisitAt        *time.Time `json:"siteVisitAt,omitempty"`
	SiteVisitNotes     string     `json:"siteVisitNotes,omitempty"`
	SiteRejectReason   string     `json:"siteRejectReason,omitempty"`

	// Maker/checker, same separation already enforced on the regular
	// disbursement path (AdminApproveDisbursementHandler) — the admin who
	// prepared a tranche's release amount cannot also approve it.
	PreparedByAdminID *uint      `json:"preparedByAdminId,omitempty"`
	PreparedBy        string     `json:"preparedBy,omitempty"`
	PreparedAt        *time.Time `json:"preparedAt,omitempty"`
	PreparedAmount    float64    `json:"preparedAmount,omitempty"`

	ApprovedByAdminID *uint      `json:"approvedByAdminId,omitempty"`
	ApprovedBy        string     `json:"approvedBy,omitempty"`
	ApprovedAt        *time.Time `json:"approvedAt,omitempty"`

	DisbursedAmount float64    `json:"disbursedAmount,omitempty"`
	DisbursedAt     *time.Time `json:"disbursedAt,omitempty"`
	UTR             string     `json:"utr,omitempty"`

	UpdatedAt time.Time `json:"updatedAt"`
}
