// Unit economics (item M). Every metric this codebase actually has the
// data to compute is computed from real source tables; every metric it
// does NOT have data for returns an explicit "NOT AVAILABLE — REQUIRED
// DATA SOURCE MISSING" marker rather than an estimate — this is the
// single most load-bearing instruction in item M ("do NOT estimate
// silently").
package mis

import (
	"time"

	"github.com/santosh0921/nbfc-backend/internal/accounting"
	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/models"
)

const NotAvailable = "NOT AVAILABLE — REQUIRED DATA SOURCE MISSING"

type UnitEconomicsMetric struct {
	Label       string  `json:"label"`
	ValuePaise  *int64  `json:"valuePaise,omitempty"`
	Unavailable string  `json:"unavailable,omitempty"` // set to NotAvailable when ValuePaise is nil
}

type UnitEconomicsReport struct {
	PeriodFrom time.Time `json:"periodFrom"`
	PeriodTo   time.Time `json:"periodTo"`
	Metrics              []UnitEconomicsMetric  `json:"metrics"`
	GeneratedAt          time.Time              `json:"generatedAt"`
}

func GenerateUnitEconomics(from, to time.Time) UnitEconomicsReport {
	r := UnitEconomicsReport{PeriodFrom: from, PeriodTo: to, GeneratedAt: time.Now()}

	var totalCustomers, activeLoans int64
	database.DB.Model(&models.User{}).Count(&totalCustomers)
	database.DB.Model(&models.LoanApplication{}).Where("status = ?", models.LoanStatusDisbursed).Count(&activeLoans)

	revenue := revenueSum(from, to)
	r.Metrics = append(r.Metrics, ratioMetric("Revenue per Customer (paise)", revenue, totalCustomers))
	r.Metrics = append(r.Metrics, ratioMetric("Revenue per Active Loan (paise)", revenue, activeLoans))

	var totalDisbursed int64
	var disbursementCount int64
	database.DB.Model(&models.Disbursement{}).Where("status = ?", models.DisbursementStatusSuccess).
		Select("COALESCE(SUM(amount_paise),0)").Scan(&totalDisbursed)
	database.DB.Model(&models.Disbursement{}).Where("status = ?", models.DisbursementStatusSuccess).Count(&disbursementCount)
	r.Metrics = append(r.Metrics, ratioMetric("Average Loan Size (paise)", totalDisbursed, disbursementCount))

	collected := journalDebitSum(accounting.AcctBankCash, "EMI_PAYMENT", from, to)
	var paymentCount int64
	database.DB.Model(&models.Payment{}).Where("status = ? AND verified_at BETWEEN ? AND ?", models.PaymentStatusSuccess, from, to).Count(&paymentCount)
	r.Metrics = append(r.Metrics, ratioMetric("Average Collection (paise)", collected, paymentCount))

	expenses := expenseSum(from, to)
	r.Metrics = append(r.Metrics, ratioMetric("Average Expense per Active Loan (paise)", expenses, activeLoans))

	// Item M explicitly names these; this codebase has no marketing-spend,
	// per-application acquisition-cost, or channel-attribution data
	// anywhere (confirmed by inspection — no such model/table exists in
	// Phases 1-11) — so these are honestly reported unavailable rather
	// than estimated from unrelated expense totals.
	r.Metrics = append(r.Metrics, unavailableMetric("Customer Acquisition Cost"))
	r.Metrics = append(r.Metrics, unavailableMetric("Collection Cost per Rupee Collected"))
	r.Metrics = append(r.Metrics, unavailableMetric("Technology Cost per Customer"))
	r.Metrics = append(r.Metrics, unavailableMetric("KYC Cost per Customer"))
	r.Metrics = append(r.Metrics, unavailableMetric("Credit Bureau Cost per Application"))

	return r
}

func ratioMetric(label string, numerator, denominator int64) UnitEconomicsMetric {
	if denominator == 0 {
		return unavailableMetric(label)
	}
	v := numerator / denominator
	return UnitEconomicsMetric{Label: label, ValuePaise: &v}
}

func unavailableMetric(label string) UnitEconomicsMetric {
	return UnitEconomicsMetric{Label: label, Unavailable: NotAvailable}
}
