// Growth analytics (item N) — month-over-month and year-over-year
// comparisons over the same authoritative counts/sums the rest of this
// package already uses, never a separately-tracked "growth" number.
package mis

import (
	"time"

	"github.com/santosh0921/nbfc-backend/internal/accounting"
	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/models"
)

type GrowthMetric struct {
	Label        string  `json:"label"`
	CurrentPeriod  int64  `json:"currentPeriodPaise,omitempty"` // paise for money metrics, plain count for counts (see IsCount)
	PriorPeriod    int64  `json:"priorPeriodPaise,omitempty"`
	IsCount        bool   `json:"isCount"`
	ChangePercent  *float64 `json:"changePercent"` // nil when the prior period is zero — never a fabricated/infinite percentage
}

// SafePercentChange (item N: "handle zero/negative denominators safely.
// Do not produce misleading percentages") — returns nil rather than
// +Inf/NaN or a silently-clamped number when prior is zero.
func SafePercentChange(current, prior int64) *float64 {
	if prior == 0 {
		return nil
	}
	pct := (float64(current) - float64(prior)) / float64(prior) * 100
	return &pct
}

type GrowthReport struct {
	CurrentFrom time.Time `json:"currentFrom"`
	CurrentTo   time.Time `json:"currentTo"`
	PriorFrom   time.Time `json:"priorFrom"`
	PriorTo     time.Time `json:"priorTo"`
	Metrics []GrowthMetric `json:"metrics"`
	GeneratedAt time.Time `json:"generatedAt"`
}

// GenerateGrowthReport compares [currentFrom,currentTo] against the
// immediately preceding period of equal length — works for MoM, QoQ, YoY,
// or any custom range the caller supplies, since "prior period" is
// always derived by shifting back the same duration rather than assuming
// calendar months.
func GenerateGrowthReport(currentFrom, currentTo time.Time) GrowthReport {
	duration := currentTo.Sub(currentFrom)
	priorTo := currentFrom.Add(-time.Second)
	priorFrom := priorTo.Add(-duration)

	r := GrowthReport{CurrentFrom: currentFrom, CurrentTo: currentTo, PriorFrom: priorFrom, PriorTo: priorTo, GeneratedAt: time.Now()}

	addCount := func(label string, model interface{}, dateCol string, extraWhere string, args ...interface{}) {
		var cur, prior int64
		curQ := database.DB.Model(model).Where(dateCol+" BETWEEN ? AND ?", currentFrom, currentTo)
		priorQ := database.DB.Model(model).Where(dateCol+" BETWEEN ? AND ?", priorFrom, priorTo)
		if extraWhere != "" {
			curQ = curQ.Where(extraWhere, args...)
			priorQ = priorQ.Where(extraWhere, args...)
		}
		curQ.Count(&cur)
		priorQ.Count(&prior)
		r.Metrics = append(r.Metrics, GrowthMetric{Label: label, CurrentPeriod: cur, PriorPeriod: prior, IsCount: true, ChangePercent: SafePercentChange(cur, prior)})
	}

	addCount("New Customers", &models.User{}, "created_at", "")
	addCount("Loan Applications", &models.LoanApplication{}, "created_at", "")
	addCount("Approved (Sanctioned/Disbursed)", &models.LoanApplication{}, "created_at",
		"status IN ?", []string{models.LoanStatusSanctioned, models.LoanStatusDisbursed})

	var curDisb, priorDisb int64
	database.DB.Model(&models.Disbursement{}).Where("status = ? AND completed_at BETWEEN ? AND ?", models.DisbursementStatusSuccess, currentFrom, currentTo).
		Select("COALESCE(SUM(amount_paise),0)").Scan(&curDisb)
	database.DB.Model(&models.Disbursement{}).Where("status = ? AND completed_at BETWEEN ? AND ?", models.DisbursementStatusSuccess, priorFrom, priorTo).
		Select("COALESCE(SUM(amount_paise),0)").Scan(&priorDisb)
	r.Metrics = append(r.Metrics, GrowthMetric{Label: "Disbursement (paise)", CurrentPeriod: curDisb, PriorPeriod: priorDisb, ChangePercent: SafePercentChange(curDisb, priorDisb)})

	curColl := journalDebitSum(accounting.AcctBankCash, "EMI_PAYMENT", currentFrom, currentTo)
	priorColl := journalDebitSum(accounting.AcctBankCash, "EMI_PAYMENT", priorFrom, priorTo)
	r.Metrics = append(r.Metrics, GrowthMetric{Label: "Collection (paise)", CurrentPeriod: curColl, PriorPeriod: priorColl, ChangePercent: SafePercentChange(curColl, priorColl)})

	curRev := revenueSum(currentFrom, currentTo)
	priorRev := revenueSum(priorFrom, priorTo)
	r.Metrics = append(r.Metrics, GrowthMetric{Label: "Revenue (paise)", CurrentPeriod: curRev, PriorPeriod: priorRev, ChangePercent: SafePercentChange(curRev, priorRev)})

	curExp := expenseSum(currentFrom, currentTo)
	priorExp := expenseSum(priorFrom, priorTo)
	r.Metrics = append(r.Metrics, GrowthMetric{Label: "Expenses (paise)", CurrentPeriod: curExp, PriorPeriod: priorExp, ChangePercent: SafePercentChange(curExp, priorExp)})

	curProfit := curRev - curExp
	priorProfit := priorRev - priorExp
	r.Metrics = append(r.Metrics, GrowthMetric{Label: "Profit Before Tax (paise)", CurrentPeriod: curProfit, PriorPeriod: priorProfit, ChangePercent: SafePercentChange(curProfit, priorProfit)})

	return r
}

func journalDebitSum(accountCode, transactionType string, from, to time.Time) int64 {
	var sum int64
	database.DB.Table("journal_entry_lines jel").
		Joins("JOIN journal_entries je ON je.id = jel.journal_id").
		Joins("JOIN ledger_accounts la ON la.id = jel.account_id").
		Where("la.account_code = ? AND je.transaction_type = ? AND je.transaction_date BETWEEN ? AND ?",
			accountCode, transactionType, from, to).
		Select("COALESCE(SUM(jel.debit_paise), 0)").Scan(&sum)
	return sum
}

func revenueSum(from, to time.Time) int64 {
	pl := accounting.ProfitAndLoss(from, to)
	return pl.TotalIncomePaise
}

func expenseSum(from, to time.Time) int64 {
	pl := accounting.ProfitAndLoss(from, to)
	return pl.TotalExpensePaise
}
