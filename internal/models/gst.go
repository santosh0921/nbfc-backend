package models

import "time"

// Supply type drives whether GST splits into CGST+SGST (intra-state) or
// IGST (inter-state) — mirrors the existing calculateProcessingFeeGST
// logic in internal/loans (Phase 1/2), now given a persistent record.
const (
	GSTSupplyTypeIntraState = "INTRA_STATE"
	GSTSupplyTypeInterState = "INTER_STATE"
)

// GSTTransaction (item H) — one row per taxable event (today: processing
// fee at disbursement). All amounts are integer paise. GSTIN is never
// fabricated: Gstin is populated only from the NBFC's actual registered
// GSTIN (a config value), and CustomerGSTIN only if the customer has
// supplied one — both left blank rather than guessed.
type GSTTransaction struct {
	ID             uint   `gorm:"primaryKey" json:"gstTransactionId"`
	SupplyType     string `gorm:"not null" json:"supplyType"`
	PlaceOfSupply  string `json:"placeOfSupply,omitempty"` // customer's state, as used for the CGST+SGST/IGST split

	TaxableValuePaise int64 `gorm:"not null" json:"taxableValuePaise"`
	CGSTPaise         int64 `gorm:"not null;default:0" json:"cgstPaise"`
	SGSTPaise         int64 `gorm:"not null;default:0" json:"sgstPaise"`
	IGSTPaise         int64 `gorm:"not null;default:0" json:"igstPaise"`
	GSTRatePercent    float64 `gorm:"not null" json:"gstRatePercent"`

	Gstin         string `json:"gstin,omitempty"`         // NBFC's own registered GSTIN, from config — never invented
	CustomerGstin string `json:"customerGstin,omitempty"` // only if the customer actually supplied one

	InvoiceReference  string `json:"invoiceReference,omitempty"`
	FinancialReference string `gorm:"index" json:"financialReference"` // e.g. "loan:123" or "disbursement:45"

	JournalReference string `json:"journalReference,omitempty"`

	CreatedAt time.Time `json:"createdAt"`
}

// Tax invoice sequencing/status (item I). Numbering is sequential per
// InvoiceSeries (configurable), NOT globally random — GST law requires a
// consecutive serial number; the exact series/format must still be
// confirmed with the NBFC's CA (see Phase 4 report §18/20).
const (
	InvoiceStatusIssued    = "ISSUED"
	InvoiceStatusCancelled = "CANCELLED"
)

type TaxInvoice struct {
	ID            uint      `gorm:"primaryKey" json:"invoiceId"`
	InvoiceSeries string    `gorm:"not null;index" json:"invoiceSeries"` // e.g. "NBFC/24-25" — configurable
	InvoiceNumber string    `gorm:"uniqueIndex;not null" json:"invoiceNumber"` // series + sequential suffix, generated under a row lock — see internal/accounting
	InvoiceDate   time.Time `gorm:"not null" json:"invoiceDate"`

	SupplierName  string `gorm:"not null" json:"supplierName"`
	SupplierGstin string `json:"supplierGstin,omitempty"`

	CustomerName  string `gorm:"not null" json:"customerName"`
	CustomerGstin string `json:"customerGstin,omitempty"`

	PlaceOfSupply string `json:"placeOfSupply,omitempty"`

	TaxableValuePaise int64   `gorm:"not null" json:"taxableValuePaise"`
	GSTRatePercent    float64 `gorm:"not null" json:"gstRatePercent"`
	CGSTPaise         int64   `gorm:"not null;default:0" json:"cgstPaise"`
	SGSTPaise         int64   `gorm:"not null;default:0" json:"sgstPaise"`
	IGSTPaise         int64   `gorm:"not null;default:0" json:"igstPaise"`
	TotalPaise        int64   `gorm:"not null" json:"totalPaise"`

	ReferenceType string `json:"referenceType"` // "loan" | "payment" | "processing_fee"
	ReferenceID   string `json:"referenceId"`

	Status         string `gorm:"not null;default:'ISSUED'" json:"status"`
	CancelReason   string `json:"cancelReason,omitempty"`
	GSTTransactionID *uint `json:"gstTransactionId,omitempty"`

	CreatedAt time.Time `json:"createdAt"`
}

// TDSTransaction (item D) — one TDS calculation event, always produced by
// internal/tax's rule engine, never hand-entered with an invented rate.
// Status stays PENDING_REVIEW until an admin/accountant confirms
// applicability for a specific payment; nothing in this codebase
// auto-applies TDS. Every field below that traces "why this amount" is
// populated ONLY by internal/tax.CalculateTDS — see that package's doc
// comment (Phase 10).
const (
	TDSStatusPendingReview = "PENDING_REVIEW"
	TDSStatusApplicable    = "APPLICABLE"
	TDSStatusNotApplicable = "NOT_APPLICABLE"
	TDSStatusDeducted      = "DEDUCTED"
	TDSStatusReversed      = "REVERSED"
)

type TDSTransaction struct {
	ID              uint   `gorm:"primaryKey" json:"tdsTransactionId"`
	SectionRef      string `json:"sectionRef,omitempty"` // e.g. "194A" — must be confirmed by the CA before use, never inferred
	BaseAmountPaise int64  `gorm:"not null" json:"baseAmountPaise"`
	RatePercent     float64 `json:"ratePercent,omitempty"`
	TDSAmountPaise  int64  `gorm:"not null;default:0" json:"tdsAmountPaise"`

	Deductee   string `json:"deductee,omitempty"` // e.g. a vendor/borrowing-lender name
	DeducteeID *uint  `gorm:"index" json:"deducteeId,omitempty"`

	// Rule provenance (item D: "the system must be able to answer WHY was
	// this TDS amount calculated") — RuleID/RuleVersion are frozen at
	// calculation time and never repointed, so a later rule edit can
	// never silently change what a historical transaction says it used.
	RuleID           *uint     `json:"ruleId,omitempty"`
	RuleVersion      string    `json:"ruleVersion,omitempty"`
	TransactionDate  time.Time `json:"transactionDate,omitempty"`
	CalculationDate  time.Time `json:"calculationDate,omitempty"`
	ThresholdPaise   int64     `json:"thresholdPaise,omitempty"`
	PANStatusAtCalc  string    `json:"panStatusAtCalc,omitempty"`
	NetPayablePaise  int64     `json:"netPayablePaise,omitempty"`

	// Override fields — set only via internal/tax.OverrideTDS, never by
	// CalculateTDS itself (item L: overrides require reason/user/
	// timestamp/approval and must record the previous calculation).
	IsOverride         bool       `json:"isOverride,omitempty"`
	OverrideReason     string     `json:"overrideReason,omitempty"`
	OverriddenByAdminID *uint     `json:"overriddenByAdminId,omitempty"`
	OverriddenBy       string     `json:"overriddenBy,omitempty"`
	OverriddenAt       *time.Time `json:"overriddenAt,omitempty"`
	PreviousTDSAmountPaise int64  `json:"previousTdsAmountPaise,omitempty"`
	ApprovedByAdminID  *uint      `json:"approvedByAdminId,omitempty"`
	ApprovedBy         string     `json:"approvedBy,omitempty"`

	ChallanID *uint `gorm:"index" json:"challanId,omitempty"` // set once mapped to a TDSChallan deposit — see item G reconciliation

	Status            string `gorm:"not null;default:'PENDING_REVIEW'" json:"status"`
	CertificateRef    string `json:"certificateRef,omitempty"`
	JournalReference  string `json:"journalReference,omitempty"`

	ReferenceType string `json:"referenceType,omitempty"`
	ReferenceID   string `gorm:"index" json:"referenceId,omitempty"`

	CreatedAt time.Time `json:"createdAt"`
}
