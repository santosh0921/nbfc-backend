// Package analytics implements the Phase 13 management intelligence
// layer. It is DECISION SUPPORT ONLY — nothing here makes a credit,
// regulatory-classification, accounting, or tax decision, and nothing
// here writes to a source financial table.
//
// Like Phase 12's internal/mis, this package COMPOSES the existing,
// already-tested calculations from Phases 4/5/8/10/11 rather than
// re-deriving them: revenue and expenses come from
// accounting.ProfitAndLoss, portfolio figures from risk.PortfolioDashboard,
// growth deltas from mis.SafePercentChange. A number shown here is the
// same number the corresponding report shows, by construction.
package analytics

import (
	"time"

	"github.com/santosh0921/nbfc-backend/internal/accounting"
	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/mis"
	"github.com/santosh0921/nbfc-backend/internal/models"
	"github.com/santosh0921/nbfc-backend/internal/risk"
)

// NoComparableBase is item C's mandated replacement for an infinite
// growth percentage. When the prior period is zero, ChangePercent is nil
// and this string is returned instead — never "∞%", never a silent 0%.
const NoComparableBase = "New / No comparable prior-period base"

// NotCalculable is item U's mandated marker for a metric whose required
// data source does not exist in this codebase.
const NotCalculable = "NOT CALCULABLE — REQUIRED DATA MISSING"

// Metric is one measured value with its prior-period comparison. Unit
// makes the number self-describing so a PAISE metric can never be read
// as a COUNT or a PERCENT (this same Unit vocabulary is what
// ManagementThreshold values are interpreted in).
type Metric struct {
	Key      string `json:"key"` // matches a KPIDefinition.Key where one exists
	Label    string `json:"label"`
	Unit     string `json:"unit"` // PAISE / COUNT / PERCENT

	Current float64 `json:"current"`
	Prior   float64 `json:"prior"`

	// ChangePercent is nil whenever the prior period is zero — item C.
	ChangePercent *float64 `json:"changePercent"`
	// ChangeNote carries NoComparableBase exactly when ChangePercent is
	// nil, so a UI never has to invent its own wording for that case.
	ChangeNote string `json:"changeNote,omitempty"`
}

func newMetric(key, label, unit string, current, prior float64) Metric {
	m := Metric{Key: key, Label: label, Unit: unit, Current: current, Prior: prior}
	m.ChangePercent = safeChangeFloat(current, prior)
	if m.ChangePercent == nil {
		m.ChangeNote = NoComparableBase
	}
	return m
}

// safeChangeFloat handles item C's three required cases: zero previous
// period (nil, not infinity), negative values, and missing periods
// (which arrive here as zero and are treated as zero-base). A negative
// prior is divided by its ABSOLUTE value so that a loss shrinking from
// -100 to -50 reads as +50% improvement rather than -50%.
func safeChangeFloat(current, prior float64) *float64 {
	if prior == 0 {
		return nil
	}
	denom := prior
	if denom < 0 {
		denom = -denom
	}
	pct := (current - prior) / denom * 100
	return &pct
}

type GrowthDashboard struct {
	CurrentFrom time.Time `json:"currentFrom"`
	CurrentTo   time.Time `json:"currentTo"`
	PriorFrom   time.Time `json:"priorFrom"`
	PriorTo     time.Time `json:"priorTo"`

	Customers      []Metric `json:"customers"`
	Applications   []Metric `json:"applications"`
	Loans          []Metric `json:"loans"`
	Disbursements  []Metric `json:"disbursements"`
	Collections    []Metric `json:"collections"`
	Portfolio      []Metric `json:"portfolio"`
	Revenue        []Metric `json:"revenue"`
	Expenses       []Metric `json:"expenses"`
	Profitability  []Metric `json:"profitability"`

	GeneratedAt time.Time `json:"generatedAt"`
}

// GenerateGrowthDashboard (item B). The prior period is always the
// immediately preceding window of equal length, so the same function
// serves MoM, QoQ, YoY and any custom range (item C) without assuming
// calendar months.
func GenerateGrowthDashboard(from, to time.Time) GrowthDashboard {
	priorFrom, priorTo := PriorPeriod(from, to)

	d := GrowthDashboard{
		CurrentFrom: from, CurrentTo: to,
		PriorFrom: priorFrom, PriorTo: priorTo,
		GeneratedAt: time.Now(),
	}

	// --- Customers ---
	newCur := countBetween(&models.User{}, "created_at", from, to)
	newPrior := countBetween(&models.User{}, "created_at", priorFrom, priorTo)
	var totalCustomers int64
	database.DB.Model(&models.User{}).Count(&totalCustomers)
	d.Customers = []Metric{
		newMetric("customers_total", "Total Customers", "COUNT", float64(totalCustomers), float64(totalCustomers-newCur)),
		newMetric("customers_new", "New Customers", "COUNT", float64(newCur), float64(newPrior)),
		newMetric("customers_active", "Active Customers (with a disbursed loan)", "COUNT",
			float64(activeCustomers()), 0),
		newMetric("customers_repeat", "Repeat Customers (2+ disbursed loans)", "COUNT",
			float64(repeatCustomers()), 0),
	}

	// --- Applications ---
	appsCur := countBetween(&models.LoanApplication{}, "created_at", from, to)
	appsPrior := countBetween(&models.LoanApplication{}, "created_at", priorFrom, priorTo)
	apprCur := countStatusBetween(approvedStatuses(), from, to)
	apprPrior := countStatusBetween(approvedStatuses(), priorFrom, priorTo)
	rejCur := countStatusBetween([]string{models.LoanStatusRejected}, from, to)
	rejPrior := countStatusBetween([]string{models.LoanStatusRejected}, priorFrom, priorTo)
	d.Applications = []Metric{
		newMetric("applications_total", "Applications", "COUNT", float64(appsCur), float64(appsPrior)),
		newMetric("approval_rate", "Approval Rate", "PERCENT", pctOf(apprCur, appsCur), pctOf(apprPrior, appsPrior)),
		newMetric("rejection_rate", "Rejection Rate", "PERCENT", pctOf(rejCur, appsCur), pctOf(rejPrior, appsPrior)),
	}

	// --- Loans ---
	active, closed := activeAndClosedLoans()
	d.Loans = []Metric{
		newMetric("loans_active", "Active Loans", "COUNT", float64(active), 0),
		newMetric("loans_closed", "Closed Loans (all installments paid)", "COUNT", float64(closed), 0),
		newMetric("loans_new", "New Loans Disbursed", "COUNT",
			float64(disbursementCount(from, to)), float64(disbursementCount(priorFrom, priorTo))),
	}

	// --- Disbursements ---
	disbCur, cntCur := disbursedSum(from, to)
	disbPrior, cntPrior := disbursedSum(priorFrom, priorTo)
	d.Disbursements = []Metric{
		newMetric("disbursed_amount", "Total Disbursed", "PAISE", float64(disbCur), float64(disbPrior)),
		newMetric("disbursement_count", "Number of Disbursements", "COUNT", float64(cntCur), float64(cntPrior)),
		newMetric("avg_loan_size", "Average Loan Size", "PAISE", avg(disbCur, cntCur), avg(disbPrior, cntPrior)),
	}

	// --- Collections ---
	// Cash actually received. Deliberately NOT called revenue — it
	// includes principal recovery, which is a balance-sheet movement.
	collCur := collectionsSum(from, to)
	collPrior := collectionsSum(priorFrom, priorTo)
	dueCur := emiDueSum(from, to)
	d.Collections = []Metric{
		newMetric("collections_amount", "Total Collected (cash, incl. principal)", "PAISE", float64(collCur), float64(collPrior)),
		newMetric("collection_efficiency", "Collection Efficiency (collected / due in period)", "PERCENT",
			pctOf(collCur, dueCur), pctOf(collectionsSum(priorFrom, priorTo), emiDueSum(priorFrom, priorTo))),
	}

	// --- Portfolio (from Phase 5's dashboard — not re-derived) ---
	pf := risk.PortfolioDashboard(risk.DashboardFilters{})
	d.Portfolio = []Metric{
		newMetric("outstanding_principal", "Outstanding Principal", "PAISE", float64(pf.OutstandingPrincipalPaise), 0),
		newMetric("overdue_amount", "Overdue Amount", "PAISE", float64(pf.TotalOverdueAmountPaise), 0),
	}

	// --- Revenue / Expenses / Profitability (from Phase 4's P&L) ---
	plCur := accounting.ProfitAndLoss(from, to)
	plPrior := accounting.ProfitAndLoss(priorFrom, priorTo)
	fundCur := FundingCost(from, to)
	fundPrior := FundingCost(priorFrom, priorTo)

	d.Revenue = []Metric{
		newMetric("revenue", "Revenue (recognized income, NOT collections)", "PAISE",
			float64(plCur.TotalIncomePaise), float64(plPrior.TotalIncomePaise)),
	}
	d.Expenses = []Metric{
		newMetric("expenses", "Operating Expenses", "PAISE",
			float64(plCur.TotalExpensePaise), float64(plPrior.TotalExpensePaise)),
		newMetric("funding_cost", "Funding Cost (borrowing interest expense)", "PAISE",
			float64(fundCur), float64(fundPrior)),
	}

	pbtCur := plCur.TotalIncomePaise - plCur.TotalExpensePaise
	pbtPrior := plPrior.TotalIncomePaise - plPrior.TotalExpensePaise
	d.Profitability = []Metric{
		newMetric("pbt", "Profit Before Tax", "PAISE", float64(pbtCur), float64(pbtPrior)),
		newMetric("profit_margin", "Profit Margin (PBT / Revenue)", "PERCENT",
			pctOf(pbtCur, plCur.TotalIncomePaise), pctOf(pbtPrior, plPrior.TotalIncomePaise)),
	}

	return d
}

// PriorPeriod returns the immediately preceding window of equal length.
func PriorPeriod(from, to time.Time) (priorFrom, priorTo time.Time) {
	d := to.Sub(from)
	priorTo = from.Add(-time.Second)
	priorFrom = priorTo.Add(-d)
	return
}

// MonthOverMonth and YearOverYear (item C) express the two named
// comparisons in terms of the same generic dashboard, so neither can
// drift from the other's definitions.
func MonthOverMonth(anchor time.Time) GrowthDashboard {
	from := time.Date(anchor.Year(), anchor.Month(), 1, 0, 0, 0, 0, anchor.Location())
	to := from.AddDate(0, 1, 0).Add(-time.Second)
	return GenerateGrowthDashboard(from, to)
}

// YearOverYear compares the current financial year to date against the
// same window one year earlier. It reuses Phase 12's April-March
// FinancialYearRange rather than assuming a calendar year — the two are
// different periods and the spec requires they stay distinguished.
func YearOverYear(anchor time.Time) GrowthDashboard {
	fyStart, _, _ := mis.FinancialYearRange(anchor)
	d := GenerateGrowthDashboard(fyStart, anchor)
	// Override the prior window to be the same span one year back rather
	// than the immediately preceding equal-length window.
	priorFrom := fyStart.AddDate(-1, 0, 0)
	priorTo := anchor.AddDate(-1, 0, 0)
	return rebase(d, fyStart, anchor, priorFrom, priorTo)
}

func rebase(_ GrowthDashboard, from, to, priorFrom, priorTo time.Time) GrowthDashboard {
	// Recomputing against the explicit prior window is cheaper to reason
	// about than patching each metric, and guarantees the prior figures
	// come from the same code path as the current ones.
	cur := GenerateGrowthDashboard(from, to)
	prior := GenerateGrowthDashboard(priorFrom, priorTo)
	cur.PriorFrom, cur.PriorTo = priorFrom, priorTo
	cur.Customers = mergePrior(cur.Customers, prior.Customers)
	cur.Applications = mergePrior(cur.Applications, prior.Applications)
	cur.Loans = mergePrior(cur.Loans, prior.Loans)
	cur.Disbursements = mergePrior(cur.Disbursements, prior.Disbursements)
	cur.Collections = mergePrior(cur.Collections, prior.Collections)
	cur.Portfolio = mergePrior(cur.Portfolio, prior.Portfolio)
	cur.Revenue = mergePrior(cur.Revenue, prior.Revenue)
	cur.Expenses = mergePrior(cur.Expenses, prior.Expenses)
	cur.Profitability = mergePrior(cur.Profitability, prior.Profitability)
	return cur
}

func mergePrior(current, prior []Metric) []Metric {
	priorByKey := map[string]float64{}
	for _, m := range prior {
		priorByKey[m.Key] = m.Current
	}
	out := make([]Metric, 0, len(current))
	for _, m := range current {
		out = append(out, newMetric(m.Key, m.Label, m.Unit, m.Current, priorByKey[m.Key]))
	}
	return out
}

// FundingCost (items K/T). Phase 12's report recorded funding cost as
// unavailable; that was incorrect — models.FundingSource /
// BorrowingTransaction exist (Phase 8) and borrowing interest posts to
// account 5200, so the cost IS derivable from the ledger. It reads zero
// until real funding rows are entered, which is honest rather than
// fabricated.
func FundingCost(from, to time.Time) int64 {
	var sum int64
	database.DB.Table("journal_entry_lines jel").
		Joins("JOIN journal_entries je ON je.id = jel.journal_id").
		Joins("JOIN ledger_accounts la ON la.id = jel.account_id").
		Where("la.account_code = ? AND je.transaction_date BETWEEN ? AND ?",
			accounting.AcctBorrowingInterestExp, from, to).
		Select("COALESCE(SUM(jel.debit_paise), 0)").Scan(&sum)
	return sum
}

// --- shared query helpers ---

func countBetween(model interface{}, dateCol string, from, to time.Time) int64 {
	var c int64
	database.DB.Model(model).Where(dateCol+" BETWEEN ? AND ?", from, to).Count(&c)
	return c
}

func approvedStatuses() []string {
	return []string{models.LoanStatusSanctioned, models.LoanStatusReadyForDisbursement,
		models.LoanStatusDisbursementProcessing, models.LoanStatusDisbursed}
}

func countStatusBetween(statuses []string, from, to time.Time) int64 {
	var c int64
	database.DB.Model(&models.LoanApplication{}).
		Where("status IN ? AND created_at BETWEEN ? AND ?", statuses, from, to).Count(&c)
	return c
}

func activeCustomers() int64 {
	var c int64
	database.DB.Raw(`SELECT COUNT(DISTINCT customer_id) FROM loan_applications WHERE status = ?`,
		models.LoanStatusDisbursed).Scan(&c)
	return c
}

func repeatCustomers() int64 {
	var c int64
	database.DB.Raw(`
		SELECT COUNT(*) FROM (
			SELECT customer_id FROM loan_applications WHERE status = ?
			GROUP BY customer_id HAVING COUNT(*) > 1
		) t`, models.LoanStatusDisbursed).Scan(&c)
	return c
}

// activeAndClosedLoans mirrors Phase 12's derivation exactly: this
// codebase's loan state machine has no "closed" status, so a loan counts
// as closed only when every one of its installments is actually paid.
func activeAndClosedLoans() (active, closed int64) {
	database.DB.Model(&models.LoanApplication{}).
		Where("status = ?", models.LoanStatusDisbursed).Count(&active)
	database.DB.Raw(`
		SELECT COUNT(*) FROM loan_applications la
		WHERE la.status = ?
		AND EXISTS (SELECT 1 FROM emi_installments ei WHERE ei.loan_id = la.id)
		AND NOT EXISTS (SELECT 1 FROM emi_installments ei WHERE ei.loan_id = la.id AND ei.status != 'paid')
	`, models.LoanStatusDisbursed).Scan(&closed)
	active -= closed
	return
}

func disbursedSum(from, to time.Time) (amount, count int64) {
	database.DB.Model(&models.Disbursement{}).
		Where("status = ? AND completed_at BETWEEN ? AND ?", models.DisbursementStatusSuccess, from, to).
		Select("COALESCE(SUM(amount_paise),0)").Scan(&amount)
	database.DB.Model(&models.Disbursement{}).
		Where("status = ? AND completed_at BETWEEN ? AND ?", models.DisbursementStatusSuccess, from, to).
		Count(&count)
	return
}

func disbursementCount(from, to time.Time) int64 {
	_, c := disbursedSum(from, to)
	return c
}

func collectionsSum(from, to time.Time) int64 {
	var sum int64
	database.DB.Table("journal_entry_lines jel").
		Joins("JOIN journal_entries je ON je.id = jel.journal_id").
		Joins("JOIN ledger_accounts la ON la.id = jel.account_id").
		Where("la.account_code = ? AND je.transaction_type = ? AND je.transaction_date BETWEEN ? AND ?",
			accounting.AcctBankCash, "EMI_PAYMENT", from, to).
		Select("COALESCE(SUM(jel.debit_paise), 0)").Scan(&sum)
	return sum
}

// emiDueSum is the denominator for collection efficiency: EMI amounts
// that fell due within the window, in paise.
func emiDueSum(from, to time.Time) int64 {
	var rupees float64
	database.DB.Model(&models.EmiInstallment{}).
		Where("due_date BETWEEN ? AND ?", from, to).
		Select("COALESCE(SUM(amount),0)").Scan(&rupees)
	return int64(rupees * 100)
}

func pctOf(part, whole int64) float64 {
	if whole == 0 {
		return 0
	}
	return float64(part) / float64(whole) * 100
}

func avg(total, count int64) float64 {
	if count == 0 {
		return 0
	}
	return float64(total) / float64(count)
}
