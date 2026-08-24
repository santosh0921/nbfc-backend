// Revenue quality (item R), expense efficiency (item S), profitability
// intelligence (item T), unit economics (item U), cohorts (item V) and
// retention (item W).
//
// Revenue composition comes from the chart of accounts (interest 4000,
// processing fee 4100, penalty/other 4200), so "revenue" here is always
// recognized income. Principal recovery is a balance-sheet movement and
// appears nowhere in this file — item R.
package analytics

import (
	"time"

	"github.com/santosh0921/nbfc-backend/internal/accounting"
	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/models"
)

// --- Revenue quality (item R) ---

type RevenueComponent struct {
	Component   string  `json:"component"`
	AmountPaise int64   `json:"amountPaise"`
	MixPercent  float64 `json:"mixPercent"`
}

type RevenueQualityReport struct {
	PeriodFrom time.Time `json:"periodFrom"`
	PeriodTo   time.Time `json:"periodTo"`

	TotalRevenuePaise int64              `json:"totalRevenuePaise"`
	Components        []RevenueComponent `json:"components"`

	RevenuePerActiveLoanPaise float64 `json:"revenuePerActiveLoanPaise"`
	RevenuePerCustomerPaise   float64 `json:"revenuePerCustomerPaise"`

	GrowthPercent *float64 `json:"growthPercent"`
	GrowthNote    string   `json:"growthNote,omitempty"`

	Note        string    `json:"note"`
	GeneratedAt time.Time `json:"generatedAt"`
}

func GenerateRevenueQuality(from, to time.Time) RevenueQualityReport {
	r := RevenueQualityReport{
		PeriodFrom: from, PeriodTo: to, GeneratedAt: time.Now(),
		Note: "Revenue is recognized income from the general ledger. It excludes principal recovery, which is a reduction of the loan receivable and never income.",
	}

	interest := incomeSum(accounting.AcctInterestIncome, from, to)
	fees := incomeSum(accounting.AcctProcessingFeeIncome, from, to)
	other := incomeSum(accounting.AcctPenaltyOtherIncome, from, to)
	r.TotalRevenuePaise = interest + fees + other

	for _, c := range []struct {
		name   string
		amount int64
	}{{"Interest Income", interest}, {"Fee Income", fees}, {"Penalty / Other Income", other}} {
		r.Components = append(r.Components, RevenueComponent{
			Component: c.name, AmountPaise: c.amount,
			MixPercent: pctOf(c.amount, r.TotalRevenuePaise),
		})
	}

	active, _ := activeAndClosedLoans()
	var customers int64
	database.DB.Model(&models.User{}).Count(&customers)
	r.RevenuePerActiveLoanPaise = avg(r.TotalRevenuePaise, active)
	r.RevenuePerCustomerPaise = avg(r.TotalRevenuePaise, customers)

	pf, pt := PriorPeriod(from, to)
	priorTotal := incomeSum(accounting.AcctInterestIncome, pf, pt) +
		incomeSum(accounting.AcctProcessingFeeIncome, pf, pt) +
		incomeSum(accounting.AcctPenaltyOtherIncome, pf, pt)
	r.GrowthPercent = safeChangeFloat(float64(r.TotalRevenuePaise), float64(priorTotal))
	if r.GrowthPercent == nil {
		r.GrowthNote = NoComparableBase
	}

	return r
}

// incomeSum reads the CREDIT side for income accounts — income accounts
// carry credit balances, so summing debits would understate them.
func incomeSum(accountCode string, from, to time.Time) int64 {
	var sum int64
	database.DB.Table("journal_entry_lines jel").
		Joins("JOIN journal_entries je ON je.id = jel.journal_id").
		Joins("JOIN ledger_accounts la ON la.id = jel.account_id").
		Where("la.account_code = ? AND je.transaction_date BETWEEN ? AND ?", accountCode, from, to).
		Select("COALESCE(SUM(jel.credit_paise),0) - COALESCE(SUM(jel.debit_paise),0)").Scan(&sum)
	return sum
}

// --- Expense efficiency (item S) ---

type ExpenseEfficiencyReport struct {
	PeriodFrom time.Time `json:"periodFrom"`
	PeriodTo   time.Time `json:"periodTo"`

	TotalExpensePaise int64              `json:"totalExpensePaise"`
	ByCategory        []RevenueComponent `json:"byCategory"`

	ExpensePerCustomerPaise    float64 `json:"expensePerCustomerPaise"`
	ExpensePerActiveLoanPaise  float64 `json:"expensePerActiveLoanPaise"`
	ExpensePerRupeeDisbursed   float64 `json:"expensePerRupeeDisbursed"`
	ExpensePerRupeeCollected   float64 `json:"expensePerRupeeCollected"`

	GrowthPercent *float64 `json:"growthPercent"`
	GrowthNote    string   `json:"growthNote,omitempty"`

	// TopCostDrivers are the largest categories by amount. Item S is
	// explicit that size alone does not make a cost inefficient, so this
	// field is named for what it measures — magnitude — and the note says
	// so rather than implying a judgement.
	TopCostDrivers []RevenueComponent `json:"topCostDrivers"`

	Note        string    `json:"note"`
	GeneratedAt time.Time `json:"generatedAt"`
}

func GenerateExpenseEfficiency(from, to time.Time) ExpenseEfficiencyReport {
	r := ExpenseEfficiencyReport{
		PeriodFrom: from, PeriodTo: to, GeneratedAt: time.Now(),
		Note: "Cost drivers are ranked by absolute size only. A large cost is not thereby an inefficient one — assess each against its own trend and an appropriate denominator before drawing any conclusion.",
	}

	pl := accounting.ProfitAndLoss(from, to)
	r.TotalExpensePaise = pl.TotalExpensePaise

	type row struct {
		Category    string
		AmountPaise int64
	}
	var rows []row
	// Gross amount is the cost incurred. Dated by expense_date (when the
	// cost belongs), not created_at (when it was keyed in) — those differ
	// and using the wrong one would put costs in the wrong period.
	database.DB.Raw(`
		SELECT COALESCE(NULLIF(category,''),'(uncategorised)') AS category,
		       COALESCE(SUM(gross_amount_paise),0) AS amount_paise
		FROM expenses
		WHERE status = ? AND expense_date BETWEEN ? AND ?
		GROUP BY 1 ORDER BY amount_paise DESC
	`, models.ExpenseStatusPaid, from, to).Scan(&rows)
	for _, x := range rows {
		r.ByCategory = append(r.ByCategory, RevenueComponent{
			Component: x.Category, AmountPaise: x.AmountPaise,
			MixPercent: pctOf(x.AmountPaise, r.TotalExpensePaise),
		})
	}
	if len(r.ByCategory) > 5 {
		r.TopCostDrivers = r.ByCategory[:5]
	} else {
		r.TopCostDrivers = r.ByCategory
	}

	active, _ := activeAndClosedLoans()
	var customers int64
	database.DB.Model(&models.User{}).Count(&customers)
	disbursed, _ := disbursedSum(from, to)
	collected := collectionsSum(from, to)

	r.ExpensePerCustomerPaise = avg(r.TotalExpensePaise, customers)
	r.ExpensePerActiveLoanPaise = avg(r.TotalExpensePaise, active)
	if disbursed > 0 {
		r.ExpensePerRupeeDisbursed = float64(r.TotalExpensePaise) / float64(disbursed)
	}
	if collected > 0 {
		r.ExpensePerRupeeCollected = float64(r.TotalExpensePaise) / float64(collected)
	}

	pf, pt := PriorPeriod(from, to)
	r.GrowthPercent = safeChangeFloat(float64(r.TotalExpensePaise),
		float64(accounting.ProfitAndLoss(pf, pt).TotalExpensePaise))
	if r.GrowthPercent == nil {
		r.GrowthNote = NoComparableBase
	}

	return r
}

// --- Profitability intelligence (item T) ---

type ProfitabilityReport struct {
	PeriodFrom time.Time `json:"periodFrom"`
	PeriodTo   time.Time `json:"periodTo"`

	RevenuePaise          int64 `json:"revenuePaise"`
	OperatingExpensePaise int64 `json:"operatingExpensePaise"`
	FundingCostPaise      int64 `json:"fundingCostPaise"`
	PBTPaise              int64 `json:"pbtPaise"`

	ProfitMarginPercent      float64 `json:"profitMarginPercent"`
	ProfitPerActiveLoanPaise float64 `json:"profitPerActiveLoanPaise"`
	ProfitPerCustomerPaise   float64 `json:"profitPerCustomerPaise"`

	Trend string `json:"trend"`

	// PAT is not produced — see PATNote.
	PATNote string `json:"patNote"`
	// Unavailable lists what item T asks for that cannot be produced.
	Unavailable []string `json:"unavailable,omitempty"`

	GeneratedAt time.Time `json:"generatedAt"`
}

// GenerateProfitability computes PBT as Revenue − Operating Expenses −
// Funding Cost. Funding cost is real (account 5200, from Phase 8's
// borrowing model) and reads zero until borrowings are recorded, which
// is an honest zero rather than an omission.
func GenerateProfitability(from, to time.Time) ProfitabilityReport {
	pl := accounting.ProfitAndLoss(from, to)
	funding := FundingCost(from, to)

	r := ProfitabilityReport{
		PeriodFrom: from, PeriodTo: to, GeneratedAt: time.Now(),
		RevenuePaise:          pl.TotalIncomePaise,
		OperatingExpensePaise: pl.TotalExpensePaise,
		FundingCostPaise:      funding,
		PATNote: "PAT is not calculated. This system has no corporate income-tax computation — Phase 10 computes TDS withholding, which is a different thing entirely and must never be used as the tax charge in a PAT calculation. PAT requires a tax provision reviewed by a CA.",
		Unavailable: []string{
			"Profit by product — NOT RELIABLY CALCULABLE: no expense or funding cost in this codebase carries a product attribution, and item T forbids inventing a cost allocation.",
		},
	}

	r.PBTPaise = r.RevenuePaise - r.OperatingExpensePaise - r.FundingCostPaise
	r.ProfitMarginPercent = pctOf(r.PBTPaise, r.RevenuePaise)

	active, _ := activeAndClosedLoans()
	var customers int64
	database.DB.Model(&models.User{}).Count(&customers)
	r.ProfitPerActiveLoanPaise = avg(r.PBTPaise, active)
	r.ProfitPerCustomerPaise = avg(r.PBTPaise, customers)

	pf, pt := PriorPeriod(from, to)
	priorPL := accounting.ProfitAndLoss(pf, pt)
	priorPBT := priorPL.TotalIncomePaise - priorPL.TotalExpensePaise - FundingCost(pf, pt)
	switch {
	case priorPBT == 0:
		r.Trend = TrendInsufficient
	case r.PBTPaise > priorPBT:
		r.Trend = TrendImproving
	case r.PBTPaise < priorPBT:
		r.Trend = TrendDeteriorating
	default:
		r.Trend = TrendStable
	}

	return r
}

// --- Cohorts (item V) ---

type CohortRow struct {
	Cohort            string `json:"cohort"` // YYYY-MM of first disbursement
	Customers         int64  `json:"customers"`
	Loans             int64  `json:"loans"`
	DisbursedPaise    int64  `json:"disbursedPaise"`
	RepaidPaise       int64  `json:"repaidPaise"`
	OutstandingPaise  int64  `json:"outstandingPaise"`
	OverdueLoans      int64  `json:"overdueLoans"`
	ClosedLoans       int64  `json:"closedLoans"`
}

type CohortReport struct {
	Cohorts []CohortRow `json:"cohorts"`
	// Item V: do not create cohort metrics where insufficient data
	// exists. A cohort month with no disbursed loans is simply absent.
	Note        string    `json:"note"`
	GeneratedAt time.Time `json:"generatedAt"`
}

func GenerateCohorts(months int) CohortReport {
	if months < 1 {
		months = 12
	}
	since := startOfMonth(time.Now().AddDate(0, -months, 0))

	r := CohortReport{
		GeneratedAt: time.Now(),
		Note:        "Cohorts group loans by the calendar month of disbursement. Months with no disbursed loans are omitted rather than shown as empty cohorts.",
	}
	// Each loan's cohort and its own repayment figures are resolved per
	// loan first, then aggregated once. Correlating the installment sums
	// back to a grouped outer query is both invalid SQL and needlessly
	// expensive.
	database.DB.Raw(`
		WITH cohort_loans AS (
			SELECT la.id,
			       la.customer_id,
			       TO_CHAR(la.disbursed_at,'YYYY-MM') AS cohort,
			       (la.disbursed_amount*100)::bigint  AS disbursed_paise
			FROM loan_applications la
			WHERE la.status = ? AND la.disbursed_at IS NOT NULL AND la.disbursed_at >= ?
		),
		loan_emi AS (
			SELECT cl.id,
			       COALESCE(SUM(CASE WHEN ei.status = 'paid'  THEN ei.amount END)*100,0)::bigint AS repaid_paise,
			       COALESCE(SUM(CASE WHEN ei.status <> 'paid' THEN ei.principal_component END)*100,0)::bigint AS outstanding_paise,
			       COUNT(ei.id)                                            AS installments,
			       COUNT(CASE WHEN ei.status = 'overdue' THEN 1 END)       AS overdue_installments,
			       COUNT(CASE WHEN ei.status <> 'paid'   THEN 1 END)       AS unpaid_installments
			FROM cohort_loans cl
			LEFT JOIN emi_installments ei ON ei.loan_id = cl.id
			GROUP BY cl.id
		)
		SELECT cl.cohort,
		       COUNT(DISTINCT cl.customer_id)     AS customers,
		       COUNT(DISTINCT cl.id)              AS loans,
		       COALESCE(SUM(cl.disbursed_paise),0)::bigint AS disbursed_paise,
		       COALESCE(SUM(le.repaid_paise),0)::bigint      AS repaid_paise,
		       COALESCE(SUM(le.outstanding_paise),0)::bigint AS outstanding_paise,
		       COUNT(CASE WHEN le.overdue_installments > 0 THEN 1 END) AS overdue_loans,
		       COUNT(CASE WHEN le.installments > 0 AND le.unpaid_installments = 0 THEN 1 END) AS closed_loans
		FROM cohort_loans cl
		JOIN loan_emi le ON le.id = cl.id
		GROUP BY cl.cohort ORDER BY cl.cohort
	`, models.LoanStatusDisbursed, since).Scan(&r.Cohorts)

	return r
}

// --- Retention (item W) ---

type RetentionReport struct {
	NewBorrowers    int64   `json:"newBorrowers"`
	RepeatBorrowers int64   `json:"repeatBorrowers"`
	RepeatPercent   float64 `json:"repeatPercent"`

	ClosedThenBorrowedAgain int64 `json:"closedThenBorrowedAgain"`

	Note        string    `json:"note"`
	GeneratedAt time.Time `json:"generatedAt"`
}

// GenerateRetention counts borrowers with more than one disbursed loan.
// Item W warns against inferring retention from incomplete data, so this
// counts only actual repeat disbursements — it does not model churn or
// project a retention rate from partial history.
func GenerateRetention() RetentionReport {
	r := RetentionReport{
		GeneratedAt: time.Now(),
		Note:        "Retention here is the observed count of borrowers with more than one disbursed loan. No churn rate or projected retention is inferred, because this codebase has no loan-closure date series complete enough to support one.",
	}

	repeat := repeatCustomers()
	total := activeCustomers()
	r.RepeatBorrowers = repeat
	r.NewBorrowers = total - repeat
	r.RepeatPercent = pctOf(repeat, total)

	database.DB.Raw(`
		SELECT COUNT(*) FROM (
			SELECT la.customer_id
			FROM loan_applications la
			WHERE la.status = ?
			  AND NOT EXISTS (SELECT 1 FROM emi_installments ei WHERE ei.loan_id = la.id AND ei.status != 'paid')
			  AND EXISTS (SELECT 1 FROM emi_installments ei WHERE ei.loan_id = la.id)
			  AND EXISTS (
			      SELECT 1 FROM loan_applications l2
			      WHERE l2.customer_id = la.customer_id AND l2.id != la.id
			        AND l2.status = ? AND l2.disbursed_at > la.disbursed_at
			  )
			GROUP BY la.customer_id
		) t`, models.LoanStatusDisbursed, models.LoanStatusDisbursed).Scan(&r.ClosedThenBorrowedAgain)

	return r
}
