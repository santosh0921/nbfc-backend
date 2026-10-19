package models

import "time"

// Gold collateral lifecycle. Unlike every other loan category here, the
// collateral is a physical object, not a document — this is the one
// place the system actually tracks custody of something in a vault.
const (
	GoldCollateralStatusPendingAppraisal = "PENDING_APPRAISAL"
	GoldCollateralStatusAppraised        = "APPRAISED"         // branch submitted actual weight/purity; awaiting checker sign-off
	GoldCollateralStatusApprovedInVault  = "APPROVED_IN_VAULT" // checker approved — this IS the sealed, vaulted state
	GoldCollateralStatusReleased         = "RELEASED"          // customer repaid in full, gold handed back
	GoldCollateralStatusAuctioned        = "AUCTIONED"         // customer defaulted, gold sold to recover the loan
)

// GoldCollateral is the branch's real record of one Gold Loan's physical
// collateral — one row per loan (uniqueIndex on LoanID; a loan is never
// re-pledged under this system). The customer's own weight/purity
// estimate at apply time (LoanApplication.GoldApprox*) is never
// authoritative — this is: the branch's actual XRF/touchstone test
// result, which is what the loan amount is really sized against.
type GoldCollateral struct {
	ID     uint `gorm:"primaryKey" json:"id"`
	LoanID uint `gorm:"not null;uniqueIndex" json:"loanId"`

	Status string `gorm:"not null;default:'PENDING_APPRAISAL';index" json:"status"`

	// Appraisal — the branch's physical test result.
	ActualWeightGrams  float64 `json:"actualWeightGrams,omitempty"`
	ActualPurityKarat  int     `json:"actualPurityKarat,omitempty"`
	GoldRatePerGram    float64 `json:"goldRatePerGram,omitempty"` // that day's market rate, admin-entered (no live gold-rate feed integrated)
	LTVPercent         float64 `json:"ltvPercent,omitempty"`      // loan-to-value cap applied, e.g. 75
	ApprovedLoanAmount float64 `json:"approvedLoanAmount,omitempty"`
	PacketTagNumber    string  `json:"packetTagNumber,omitempty"` // the sealed packet's physical tag/barcode
	VaultBranch        string  `json:"vaultBranch,omitempty"`

	AppraisedByAdminID *uint      `json:"appraisedByAdminId,omitempty"`
	AppraisedBy        string     `json:"appraisedBy,omitempty"`
	AppraisedAt        *time.Time `json:"appraisedAt,omitempty"`

	// Checker sign-off on the appraisal — maker (branch appraiser) ≠
	// checker, same separation every other money decision in this system
	// enforces.
	ApprovedByAdminID *uint      `json:"approvedByAdminId,omitempty"`
	ApprovedBy        string     `json:"approvedBy,omitempty"`
	ApprovedAt        *time.Time `json:"approvedAt,omitempty"`

	// Release — on full repayment, the gold is physically handed back.
	ReleasedByAdminID *uint      `json:"releasedByAdminId,omitempty"`
	ReleasedBy        string     `json:"releasedBy,omitempty"`
	ReleasedAt        *time.Time `json:"releasedAt,omitempty"`
	ReleaseNotes      string     `json:"releaseNotes,omitempty"`

	// Auction — on default, the lender's real recovery mechanism for gold
	// (no court/repossession process needed since the collateral is
	// already in hand).
	AuctionedByAdminID *uint      `json:"auctionedByAdminId,omitempty"`
	AuctionedBy        string     `json:"auctionedBy,omitempty"`
	AuctionedAt        *time.Time `json:"auctionedAt,omitempty"`
	AuctionNotes       string     `json:"auctionNotes,omitempty"`
	AuctionProceeds    float64    `json:"auctionProceeds,omitempty"`

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}
