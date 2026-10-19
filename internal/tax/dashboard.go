// Tax dashboard (item P/M) — server-side aggregation over TDSTransaction/
// TDSChallan/TDSException, never a client-side sum.
package tax

import (
	"time"

	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/models"
)

type TaxDashboard struct {
	PeriodFrom time.Time `json:"periodFrom"`
	PeriodTo   time.Time `json:"periodTo"`

	TDSDeductedPaise   int64 `json:"tdsDeductedPaise"`
	TDSDepositedPaise  int64 `json:"tdsDepositedPaise"`
	TDSPayablePaise    int64 `json:"tdsPayablePaise"` // deducted - deposited, i.e. outstanding liability
	TDSOverduePaise    int64 `json:"tdsOverduePaise"`
	TDSDueSoonPaise    int64 `json:"tdsDueSoonPaise"` // due within 7 days

	ExceptionCount     int64 `json:"exceptionCount"`
	ReconciledCount    int64 `json:"reconciledCount"`   // DEDUCTED transactions with a ChallanID set
	UnreconciledCount  int64 `json:"unreconciledCount"` // DEDUCTED transactions with no ChallanID

	DeducteeCount    int64 `json:"deducteeCount"`
	TransactionCount int64 `json:"transactionCount"`

	// GST breakdown (item H, GSTTransaction) — one row per taxable event
	// (today: processing fee at disbursement). CGST+SGST apply to
	// intra-state supply, IGST to inter-state — never both on the same
	// transaction, so CGSTPaise+SGSTPaise+IGSTPaise always equals
	// GSTPayablePaise exactly.
	CGSTPaise       int64 `json:"cgstPaise"`
	SGSTPaise       int64 `json:"sgstPaise"`
	IGSTPaise       int64 `json:"igstPaise"`
	GSTPayablePaise int64 `json:"gstPayablePaise"` // cgst + sgst + igst
	GSTTaxableValuePaise int64 `json:"gstTaxableValuePaise"`
	GSTTransactionCount  int64 `json:"gstTransactionCount"`

	GeneratedAt time.Time `json:"generatedAt"`
}

func GenerateTaxDashboard(from, to time.Time) TaxDashboard {
	d := TaxDashboard{PeriodFrom: from, PeriodTo: to, GeneratedAt: time.Now()}

	database.DB.Model(&models.TDSTransaction{}).
		Where("status IN ? AND transaction_date BETWEEN ? AND ?", []string{models.TDSStatusDeducted, models.TDSStatusReversed}, from, to).
		Select("COALESCE(SUM(tds_amount_paise), 0)").Scan(&d.TDSDeductedPaise)

	database.DB.Model(&models.TDSChallan{}).
		Where("status = ? AND actual_deposit_date BETWEEN ? AND ?", models.ChallanStatusPaid, from, to).
		Select("COALESCE(SUM(amount_paise), 0)").Scan(&d.TDSDepositedPaise)

	d.TDSPayablePaise = d.TDSDeductedPaise - d.TDSDepositedPaise

	database.DB.Model(&models.TDSChallan{}).
		Where("status IN ? AND deposit_due_date < ?", []string{models.ChallanStatusPending, models.ChallanStatusDue}, time.Now()).
		Select("COALESCE(SUM(amount_paise), 0)").Scan(&d.TDSOverduePaise)

	dueSoonCutoff := time.Now().AddDate(0, 0, 7)
	database.DB.Model(&models.TDSChallan{}).
		Where("status IN ? AND deposit_due_date BETWEEN ? AND ?",
			[]string{models.ChallanStatusPending, models.ChallanStatusDue}, time.Now(), dueSoonCutoff).
		Select("COALESCE(SUM(amount_paise), 0)").Scan(&d.TDSDueSoonPaise)

	database.DB.Model(&models.TDSException{}).Where("status = ?", models.TDSExceptionStatusOpen).Count(&d.ExceptionCount)
	database.DB.Model(&models.TDSTransaction{}).Where("status = ? AND challan_id IS NOT NULL", models.TDSStatusDeducted).Count(&d.ReconciledCount)
	database.DB.Model(&models.TDSTransaction{}).Where("status = ? AND challan_id IS NULL", models.TDSStatusDeducted).Count(&d.UnreconciledCount)

	database.DB.Model(&models.TDSDeductee{}).Count(&d.DeducteeCount)
	database.DB.Model(&models.TDSTransaction{}).Where("transaction_date BETWEEN ? AND ?", from, to).Count(&d.TransactionCount)

	database.DB.Model(&models.GSTTransaction{}).
		Where("created_at BETWEEN ? AND ?", from, to).
		Select("COALESCE(SUM(cgst_paise), 0)").Scan(&d.CGSTPaise)
	database.DB.Model(&models.GSTTransaction{}).
		Where("created_at BETWEEN ? AND ?", from, to).
		Select("COALESCE(SUM(sgst_paise), 0)").Scan(&d.SGSTPaise)
	database.DB.Model(&models.GSTTransaction{}).
		Where("created_at BETWEEN ? AND ?", from, to).
		Select("COALESCE(SUM(igst_paise), 0)").Scan(&d.IGSTPaise)
	database.DB.Model(&models.GSTTransaction{}).
		Where("created_at BETWEEN ? AND ?", from, to).
		Select("COALESCE(SUM(taxable_value_paise), 0)").Scan(&d.GSTTaxableValuePaise)
	database.DB.Model(&models.GSTTransaction{}).Where("created_at BETWEEN ? AND ?", from, to).Count(&d.GSTTransactionCount)
	d.GSTPayablePaise = d.CGSTPaise + d.SGSTPaise + d.IGSTPaise

	return d
}
