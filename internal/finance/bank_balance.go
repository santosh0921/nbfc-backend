// Bank/cash account tracking (item K). BankAccount itself already exists
// (Phase 2, internal/models/reconciliation.go) as the account statements
// import against — this adds the missing piece: a computed balance and
// transaction summary, derived from BankTransaction rows already
// imported, never a separately-maintained running total that could drift.
package finance

import (
	"time"

	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/models"
)

type BankAccountSummary struct {
	BankAccount        models.BankAccount `json:"bankAccount"`
	OpeningBalancePaise int64  `json:"openingBalancePaise"`
	TotalCreditsPaise  int64  `json:"totalCreditsPaise"` // deposits/collections/inflows
	TotalDebitsPaise   int64  `json:"totalDebitsPaise"`  // withdrawals/disbursements/expenses
	ClosingBalancePaise int64 `json:"closingBalancePaise"`
	TransactionCount   int64  `json:"transactionCount"`
	AsOf               time.Time `json:"asOf"`
}

// BankAccountBalance computes a bank account's balance strictly from its
// imported BankTransaction rows (item K: deposits/withdrawals/closing
// balance) — this is the bank's own statement data, the authoritative
// source for what actually happened in that account, not a figure
// re-derived from the loan/accounting side (that comparison is what
// reconciliation, item L/O, is for).
func BankAccountBalance(bankAccountID uint, asOf time.Time) (*BankAccountSummary, error) {
	var account models.BankAccount
	if err := database.DB.First(&account, bankAccountID).Error; err != nil {
		return nil, err
	}

	var credits, debits int64
	var count int64
	database.DB.Model(&models.BankTransaction{}).
		Where("bank_account_id = ? AND dr_cr = ? AND txn_date <= ?", bankAccountID, "CR", asOf).
		Select("COALESCE(SUM(amount_paise), 0)").Scan(&credits)
	database.DB.Model(&models.BankTransaction{}).
		Where("bank_account_id = ? AND dr_cr = ? AND txn_date <= ?", bankAccountID, "DR", asOf).
		Select("COALESCE(SUM(amount_paise), 0)").Scan(&debits)
	database.DB.Model(&models.BankTransaction{}).
		Where("bank_account_id = ? AND txn_date <= ?", bankAccountID, asOf).Count(&count)

	return &BankAccountSummary{
		BankAccount: account, OpeningBalancePaise: account.OpeningBalancePaise,
		TotalCreditsPaise: credits, TotalDebitsPaise: debits,
		ClosingBalancePaise: account.OpeningBalancePaise + credits - debits,
		TransactionCount: count, AsOf: asOf,
	}, nil
}

// AllBankAccountSummaries — item U's "Cash/Bank Summary" dashboard tile,
// server-aggregated (no per-account loop pulling raw rows into memory).
func AllBankAccountSummaries(asOf time.Time) []BankAccountSummary {
	var accounts []models.BankAccount
	database.DB.Where("active = ?", true).Find(&accounts)

	out := make([]BankAccountSummary, 0, len(accounts))
	for _, a := range accounts {
		s, err := BankAccountBalance(a.ID, asOf)
		if err == nil {
			out = append(out, *s)
		}
	}
	return out
}
