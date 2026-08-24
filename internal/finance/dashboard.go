// Executive financial dashboard (items Q/U). Every figure is pulled from
// the Phase 4 accounting ledger / Phase 5 risk engine / Phase 2 loan
// tables directly — never a separately-maintained cache that could drift
// from the ledger. "Profit" is deliberately never labeled as such unless
// its components (Revenue, Operating Expenses, Funding Cost) are
// individually named — see FinancialDashboard's ProfitBeforeTaxPaise doc
// comment.
package finance

import (
	"time"

	"github.com/santosh0921/nbfc-backend/internal/accounting"
	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/models"
)

type FinancialDashboard struct {
	PeriodFrom time.Time `json:"periodFrom"`
	PeriodTo   time.Time `json:"periodTo"`

	TotalSanctionedPaise int64 `json:"totalSanctionedPaise"`
	TotalDisbursedPaise  int64 `json:"totalDisbursedPaise"`

	TotalPrincipalCollectedPaise int64 `json:"totalPrincipalCollectedPaise"`
	TotalInterestCollectedPaise  int64 `json:"totalInterestCollectedPaise"`

	TotalFeeIncomePaise   int64 `json:"totalFeeIncomePaise"`
	TotalOtherIncomePaise int64 `json:"totalOtherIncomePaise"`

	TotalExpensesPaise   int64 `json:"totalExpensesPaise"`
	TotalFundingCostPaise int64 `json:"totalFundingCostPaise"`

	TotalPrincipalOutstandingPaise int64 `json:"totalPrincipalOutstandingPaise"`
	TotalInterestReceivablePaise   int64 `json:"totalInterestReceivablePaise"`
	TotalReceivablePaise           int64 `json:"totalReceivablePaise"`
	TotalPayablePaise              int64 `json:"totalPayablePaise"`

	// Money-flow metrics (Phase 9 item T) — every figure below comes
	// directly from Payment/Refund/CrossSystemReconciliation rows, never
	// re-derived from the accounting ledger, since these describe the
	// gateway-level money movement (what was attempted, what failed, what
	// is still unreconciled), which is a different question from what the
	// ledger has posted.
	TotalRefundsPaise      int64 `json:"totalRefundsPaise"`
	TotalFailedPaymentsPaise int64 `json:"totalFailedPaymentsPaise"`
	TotalPendingPaymentsPaise int64 `json:"totalPendingPaymentsPaise"`
	ReconciledCount        int64 `json:"reconciledCount"`
	UnreconciledCount      int64 `json:"unreconciledCount"`

	// Revenue/ProfitBeforeTax are populated ONLY from
	// accounting.ProfitAndLoss — i.e. only from what was actually posted
	// to Income/Expense ledger accounts for the period, never derived by
	// this dashboard independently (e.g. never "total collected minus
	// total disbursed"). See the Phase 8 report §S for why
	// TotalPrincipalCollectedPaise above is explicitly NOT treated as
	// revenue.
	RevenuePaise          int64 `json:"revenuePaise"`
	OperatingExpensesPaise int64 `json:"operatingExpensesPaise"`
	ProfitBeforeTaxPaise  int64 `json:"profitBeforeTaxPaise"` // Revenue - OperatingExpenses, as posted — "profit after tax" is NOT computed anywhere in this codebase (no tax provisioning exists — see report)

	GeneratedAt time.Time `json:"generatedAt"`
}

func GenerateFinancialDashboard(from, to time.Time) FinancialDashboard {
	d := FinancialDashboard{PeriodFrom: from, PeriodTo: to, GeneratedAt: time.Now()}

	var sanctioned, disbursed float64
	database.DB.Model(&models.LoanApplication{}).
		Where("status IN ? AND created_at BETWEEN ? AND ?", []string{models.LoanStatusSanctioned, models.LoanStatusDisbursed}, from, to).
		Select("COALESCE(SUM(amount_requested), 0)").Scan(&sanctioned)
	d.TotalSanctionedPaise = toPaise(sanctioned)

	var disbursedPaise int64
	database.DB.Model(&models.Disbursement{}).
		Where("status = ? AND completed_at BETWEEN ? AND ?", models.DisbursementStatusSuccess, from, to).
		Select("COALESCE(SUM(amount_paise), 0)").Scan(&disbursedPaise)
	d.TotalDisbursedPaise = disbursedPaise
	_ = disbursed

	// Collections split via posted EMI_PAYMENT journal lines — the
	// authoritative source (Phase 4 posts principal/interest as separate
	// lines against distinct accounts), not a re-derivation from
	// EmiInstallment.
	d.TotalPrincipalCollectedPaise = journalCreditSum(accounting.AcctLoanPrincipalReceivable, "EMI_PAYMENT", from, to)
	d.TotalInterestCollectedPaise = journalCreditSum(accounting.AcctInterestReceivable, "EMI_PAYMENT", from, to)

	pl := accounting.ProfitAndLoss(from, to)
	for _, l := range pl.Income {
		switch l.AccountCode {
		case accounting.AcctProcessingFeeIncome:
			d.TotalFeeIncomePaise = l.AmountPaise
		case accounting.AcctPenaltyOtherIncome:
			d.TotalOtherIncomePaise = l.AmountPaise
		}
	}
	d.TotalExpensesPaise = pl.TotalExpensePaise
	for _, l := range pl.Expenses {
		if l.AccountCode == accounting.AcctBorrowingInterestExp {
			d.TotalFundingCostPaise = l.AmountPaise
		}
	}
	d.RevenuePaise = pl.TotalIncomePaise
	d.OperatingExpensesPaise = pl.TotalExpensePaise
	d.ProfitBeforeTaxPaise = pl.NetProfitPaise

	bs := accounting.BalanceSheet(to)
	for _, l := range bs.Assets {
		switch l.AccountCode {
		case accounting.AcctLoanPrincipalReceivable:
			d.TotalPrincipalOutstandingPaise = l.AmountPaise
		case accounting.AcctInterestReceivable:
			d.TotalInterestReceivablePaise = l.AmountPaise
		}
	}
	d.TotalReceivablePaise = d.TotalPrincipalOutstandingPaise + d.TotalInterestReceivablePaise
	for _, l := range bs.Liabilities {
		if l.AccountCode == accounting.AcctBorrowingsPayable {
			d.TotalPayablePaise += l.AmountPaise
		}
	}
	var vendorPayable int64
	database.DB.Model(&models.Expense{}).Where("status = ?", models.ExpenseStatusApproved).
		Select("COALESCE(SUM(net_payable_paise), 0)").Scan(&vendorPayable)
	d.TotalPayablePaise += vendorPayable

	database.DB.Model(&models.Refund{}).Where("created_at BETWEEN ? AND ?", from, to).
		Select("COALESCE(SUM(amount_paise), 0)").Scan(&d.TotalRefundsPaise)
	database.DB.Model(&models.Payment{}).Where("status = ? AND created_at BETWEEN ? AND ?", models.PaymentStatusFailed, from, to).
		Select("COALESCE(SUM(amount_paise), 0)").Scan(&d.TotalFailedPaymentsPaise)
	database.DB.Model(&models.Payment{}).Where("status IN ? AND created_at BETWEEN ? AND ?",
		[]string{models.PaymentStatusInitiated, models.PaymentStatusPending}, from, to).
		Select("COALESCE(SUM(amount_paise), 0)").Scan(&d.TotalPendingPaymentsPaise)

	database.DB.Model(&models.CrossSystemReconciliation{}).
		Where("detected_at BETWEEN ? AND ? AND status = ?", from, to, models.ReconStatusMatched).Count(&d.ReconciledCount)
	database.DB.Model(&models.CrossSystemReconciliation{}).
		Where("detected_at BETWEEN ? AND ? AND status != ?", from, to, models.ReconStatusMatched).Count(&d.UnreconciledCount)

	return d
}

func journalCreditSum(accountCode, transactionType string, from, to time.Time) int64 {
	var sum int64
	database.DB.Table("journal_entry_lines jel").
		Joins("JOIN journal_entries je ON je.id = jel.journal_id").
		Joins("JOIN ledger_accounts la ON la.id = jel.account_id").
		Where("la.account_code = ? AND je.transaction_type = ? AND je.transaction_date BETWEEN ? AND ?",
			accountCode, transactionType, from, to).
		Select("COALESCE(SUM(jel.credit_paise), 0)").Scan(&sum)
	return sum
}

func toPaise(rupees float64) int64 {
	return int64(rupees * 100)
}
