// Report queries — every one of these is built directly from posted
// journal_entry_lines (joined to ledger_accounts), never from
// loan_applications/emi_installments/payments directly, per the Phase 4
// instruction that loan tables must not be treated as accounting books.
package accounting

import (
	"time"

	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/models"
)

type TrialBalanceRow struct {
	AccountCode   string `json:"accountCode"`
	AccountName   string `json:"accountName"`
	AccountType   string `json:"accountType"`
	OpeningDebit  int64  `json:"openingDebitPaise"`
	OpeningCredit int64  `json:"openingCreditPaise"`
	PeriodDebit   int64  `json:"periodDebitPaise"`
	PeriodCredit  int64  `json:"periodCreditPaise"`
	ClosingDebit  int64  `json:"closingDebitPaise"`
	ClosingCredit int64  `json:"closingCreditPaise"`
}

type TrialBalanceReport struct {
	Rows              []TrialBalanceRow `json:"rows"`
	TotalOpeningDebit  int64 `json:"totalOpeningDebitPaise"`
	TotalOpeningCredit int64 `json:"totalOpeningCreditPaise"`
	TotalPeriodDebit   int64 `json:"totalPeriodDebitPaise"`
	TotalPeriodCredit  int64 `json:"totalPeriodCreditPaise"`
	TotalClosingDebit  int64 `json:"totalClosingDebitPaise"`
	TotalClosingCredit int64 `json:"totalClosingCreditPaise"`
	Balanced           bool  `json:"balanced"` // TotalClosingDebit == TotalClosingCredit
}

type acctSums struct {
	AccountID uint
	Debit     int64
	Credit    int64
}

func sumLines(from, to *time.Time, accountID *uint) []acctSums {
	q := database.DB.Table("journal_entry_lines jel").
		Joins("JOIN journal_entries je ON je.id = jel.journal_id").
		Select("jel.account_id as account_id, COALESCE(SUM(jel.debit_paise),0) as debit, COALESCE(SUM(jel.credit_paise),0) as credit").
		Group("jel.account_id")
	if from != nil {
		q = q.Where("je.transaction_date >= ?", *from)
	}
	if to != nil {
		q = q.Where("je.transaction_date <= ?", *to)
	}
	if accountID != nil {
		q = q.Where("jel.account_id = ?", *accountID)
	}
	var rows []acctSums
	q.Scan(&rows)
	return rows
}

// TrialBalance (item N) — opening = everything posted before periodStart,
// period = everything within [periodStart, periodEnd], closing = opening+period.
func TrialBalance(periodStart, periodEnd time.Time) TrialBalanceReport {
	var accounts []models.LedgerAccount
	database.DB.Where("status = ?", models.AccountStatusActive).Order("account_code").Find(&accounts)

	openingBefore := periodStart.Add(-time.Nanosecond)
	opening := sumsByAccount(sumLines(nil, &openingBefore, nil))
	period := sumsByAccount(sumLines(&periodStart, &periodEnd, nil))

	report := TrialBalanceReport{}
	for _, a := range accounts {
		op := opening[a.ID]
		pe := period[a.ID]
		normal := models.NormalBalanceFor(a.AccountType)

		openDebit, openCredit := netByNormal(op.Debit, op.Credit, normal)
		periodDebit, periodCredit := pe.Debit, pe.Credit
		closeDebit, closeCredit := netByNormal(op.Debit+pe.Debit, op.Credit+pe.Credit, normal)

		row := TrialBalanceRow{
			AccountCode: a.AccountCode, AccountName: a.AccountName, AccountType: a.AccountType,
			OpeningDebit: openDebit, OpeningCredit: openCredit,
			PeriodDebit: periodDebit, PeriodCredit: periodCredit,
			ClosingDebit: closeDebit, ClosingCredit: closeCredit,
		}
		report.Rows = append(report.Rows, row)
		report.TotalOpeningDebit += openDebit
		report.TotalOpeningCredit += openCredit
		report.TotalPeriodDebit += periodDebit
		report.TotalPeriodCredit += periodCredit
		report.TotalClosingDebit += closeDebit
		report.TotalClosingCredit += closeCredit
	}
	report.Balanced = report.TotalClosingDebit == report.TotalClosingCredit
	return report
}

func sumsByAccount(rows []acctSums) map[uint]acctSums {
	out := make(map[uint]acctSums, len(rows))
	for _, r := range rows {
		out[r.AccountID] = r
	}
	return out
}

// netByNormal collapses a debit/credit pair into a single normal-balance
// figure (shown on whichever side the account naturally sits) purely for
// display — the underlying ledger always keeps both legs.
func netByNormal(debit, credit int64, normal string) (displayDebit, displayCredit int64) {
	net := debit - credit
	if normal == "DEBIT" {
		if net >= 0 {
			return net, 0
		}
		return 0, -net
	}
	net = credit - debit
	if net >= 0 {
		return 0, net
	}
	return -net, 0
}

// ProfitAndLossReport (item O) — Income accounts minus Expense accounts,
// for the given date range, from posted journal lines only.
type PLLine struct {
	AccountCode string `json:"accountCode"`
	AccountName string `json:"accountName"`
	AmountPaise int64  `json:"amountPaise"`
}
type ProfitAndLossReport struct {
	Income          []PLLine `json:"income"`
	Expenses        []PLLine `json:"expenses"`
	TotalIncomePaise   int64 `json:"totalIncomePaise"`
	TotalExpensePaise  int64 `json:"totalExpensePaise"`
	NetProfitPaise     int64 `json:"netProfitPaise"`
	ProvisionExpenseNote string `json:"provisionExpenseNote"`
}

func ProfitAndLoss(from, to time.Time) ProfitAndLossReport {
	var accounts []models.LedgerAccount
	database.DB.Where("status = ? AND account_type IN ?", models.AccountStatusActive,
		[]string{models.AccountTypeIncome, models.AccountTypeExpense}).Order("account_code").Find(&accounts)

	sums := sumsByAccount(sumLines(&from, &to, nil))
	report := ProfitAndLossReport{
		ProvisionExpenseNote: "NPA/IRAC provisioning is a Phase 5 feature — the Provision for NPA Expense account (5900) exists in the chart of accounts but nothing posts to it in Phase 4.",
	}
	for _, a := range accounts {
		s := sums[a.ID]
		if a.AccountType == models.AccountTypeIncome {
			amt := s.Credit - s.Debit
			report.Income = append(report.Income, PLLine{AccountCode: a.AccountCode, AccountName: a.AccountName, AmountPaise: amt})
			report.TotalIncomePaise += amt
		} else {
			amt := s.Debit - s.Credit
			report.Expenses = append(report.Expenses, PLLine{AccountCode: a.AccountCode, AccountName: a.AccountName, AmountPaise: amt})
			report.TotalExpensePaise += amt
		}
	}
	report.NetProfitPaise = report.TotalIncomePaise - report.TotalExpensePaise
	return report
}

// BalanceSheetReport (item P) — Assets/Liabilities/Equity as of asOf,
// validating Assets = Liabilities + Equity.
type BSLine struct {
	AccountCode string `json:"accountCode"`
	AccountName string `json:"accountName"`
	AmountPaise int64  `json:"amountPaise"`
}
type BalanceSheetReport struct {
	Assets      []BSLine `json:"assets"`
	Liabilities []BSLine `json:"liabilities"`
	Equity      []BSLine `json:"equity"`
	TotalAssetsPaise      int64 `json:"totalAssetsPaise"`
	TotalLiabilitiesPaise int64 `json:"totalLiabilitiesPaise"`
	TotalEquityPaise      int64 `json:"totalEquityPaise"`
	Balanced bool `json:"balanced"` // Assets == Liabilities + Equity
}

// BalanceSheet includes every posted Income/Expense entry's net effect as
// an implicit "Current Year Earnings" equity line, in addition to the
// real Equity-type accounts (Capital, Retained Earnings). This is the
// standard double-entry convention for an interim balance sheet: no
// period-close journal exists in this codebase to sweep Income/Expense
// into Retained Earnings, so any transaction touching a P&L account (e.g.
// Phase 5's provision expense) would otherwise make Assets != Liabilities
// + Equity even though every individual journal entry is itself balanced
// — the accounting equation only holds end-to-end once unclosed earnings
// are counted as part of equity.
func BalanceSheet(asOf time.Time) BalanceSheetReport {
	var accounts []models.LedgerAccount
	database.DB.Where("status = ? AND account_type IN ?", models.AccountStatusActive,
		[]string{models.AccountTypeAsset, models.AccountTypeLiability, models.AccountTypeEquity}).
		Order("account_code").Find(&accounts)

	sums := sumsByAccount(sumLines(nil, &asOf, nil))
	report := BalanceSheetReport{}
	for _, a := range accounts {
		s := sums[a.ID]
		normal := models.NormalBalanceFor(a.AccountType)
		var amt int64
		if normal == "DEBIT" {
			amt = s.Debit - s.Credit
		} else {
			amt = s.Credit - s.Debit
		}
		line := BSLine{AccountCode: a.AccountCode, AccountName: a.AccountName, AmountPaise: amt}
		switch a.AccountType {
		case models.AccountTypeAsset:
			report.Assets = append(report.Assets, line)
			report.TotalAssetsPaise += amt
		case models.AccountTypeLiability:
			report.Liabilities = append(report.Liabilities, line)
			report.TotalLiabilitiesPaise += amt
		case models.AccountTypeEquity:
			report.Equity = append(report.Equity, line)
			report.TotalEquityPaise += amt
		}
	}

	epoch := time.Time{}
	pl := ProfitAndLoss(epoch, asOf)
	if pl.NetProfitPaise != 0 {
		report.Equity = append(report.Equity, BSLine{
			AccountCode: "9999", AccountName: "Current Year Earnings (unclosed P&L)", AmountPaise: pl.NetProfitPaise,
		})
		report.TotalEquityPaise += pl.NetProfitPaise
	}

	report.Balanced = report.TotalAssetsPaise == report.TotalLiabilitiesPaise+report.TotalEquityPaise
	return report
}

// GeneralLedger returns every posted line for one account within a date
// range, in chronological order — the "show your work" behind any Trial
// Balance figure.
type GeneralLedgerLine struct {
	JournalNumber   string    `json:"journalNumber"`
	TransactionDate time.Time `json:"transactionDate"`
	Description     string    `json:"description"`
	DebitPaise      int64     `json:"debitPaise"`
	CreditPaise     int64     `json:"creditPaise"`
}

func GeneralLedger(accountCode string, from, to time.Time) ([]GeneralLedgerLine, error) {
	var account models.LedgerAccount
	if err := database.DB.Where("account_code = ?", accountCode).First(&account).Error; err != nil {
		return nil, ErrUnknownAccount
	}
	var lines []GeneralLedgerLine
	database.DB.Table("journal_entry_lines jel").
		Joins("JOIN journal_entries je ON je.id = jel.journal_id").
		Select("je.journal_number, je.transaction_date, jel.description, jel.debit_paise, jel.credit_paise").
		Where("jel.account_id = ? AND je.transaction_date BETWEEN ? AND ?", account.ID, from, to).
		Order("je.transaction_date, je.id").
		Scan(&lines)
	return lines, nil
}
