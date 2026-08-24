package accounting

import (
	"log"

	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/models"
)

// Account codes — every posting function in postings.go references these
// constants, never a raw literal, so the chart stays the single source of
// truth. Exported so admin tooling/tests can refer to the same codes.
const (
	AcctBankCash                = "1000"
	AcctLoanPrincipalReceivable = "1100"
	AcctInterestReceivable      = "1110"
	AcctProcessingFeeReceivable = "1120"
	AcctGSTInputReceivable      = "1200"

	AcctCGSTPayable = "2000"
	AcctSGSTPayable = "2010"
	AcctIGSTPayable = "2020"
	AcctTDSPayable  = "2100"
	AcctBorrowingsPayable = "2200"
	AcctProvisionForNPA   = "2900" // Phase 5 extension point — unused by any Phase 4 posting

	AcctEquityCapital    = "3000"
	AcctRetainedEarnings = "3900"

	AcctInterestIncome      = "4000"
	AcctProcessingFeeIncome = "4100"
	AcctPenaltyOtherIncome  = "4200"

	AcctOperatingExpenses     = "5000"
	AcctBankCharges           = "5100"
	AcctBorrowingInterestExp  = "5200"
	AcctProvisionExpenseNPA   = "5900" // Phase 5 extension point — unused by any Phase 4 posting
)

type seedAccount struct {
	code, name, accType, parent, desc string
}

// defaultChart is a sensible starting NBFC chart of accounts (item A) —
// NOT a claim of what the NBFC's CA will ultimately require. Every
// account can be renamed/reparented/deactivated after seeding via the
// admin Chart of Accounts screen; this seed only guarantees the codes
// internal/accounting's posting functions actually reference exist at
// first boot.
var defaultChart = []seedAccount{
	{AcctBankCash, "Bank / Cash", models.AccountTypeAsset, "", "Operating bank account(s) and cash-in-hand"},
	{AcctLoanPrincipalReceivable, "Loan Principal Receivable", models.AccountTypeAsset, "", "Outstanding principal owed by borrowers"},
	{AcctInterestReceivable, "Interest Receivable", models.AccountTypeAsset, "", "Accrued-but-uncollected interest"},
	{AcctProcessingFeeReceivable, "Processing Fee Receivable", models.AccountTypeAsset, "", "Processing fee + GST owed by the borrower, not yet collected"},
	{AcctGSTInputReceivable, "GST Input Receivable", models.AccountTypeAsset, "", "Input tax credit on the NBFC's own purchases, where applicable"},

	{AcctCGSTPayable, "CGST Payable", models.AccountTypeLiability, "", "Central GST collected, payable to the government"},
	{AcctSGSTPayable, "SGST Payable", models.AccountTypeLiability, "", "State GST collected, payable to the government"},
	{AcctIGSTPayable, "IGST Payable", models.AccountTypeLiability, "", "Integrated GST collected, payable to the government"},
	{AcctTDSPayable, "TDS Payable", models.AccountTypeLiability, "", "Tax deducted at source, payable to the government — see internal/tds"},
	{AcctBorrowingsPayable, "Borrowings Payable", models.AccountTypeLiability, "", "Principal outstanding on NBFC funding/borrowing lines"},
	{AcctProvisionForNPA, "Provision for NPA (Phase 5)", models.AccountTypeLiability, "", "Reserved for Phase 5 IRAC/NPA provisioning — not posted to in Phase 4"},

	{AcctEquityCapital, "Equity / Capital", models.AccountTypeEquity, "", "Paid-up capital"},
	{AcctRetainedEarnings, "Retained Earnings", models.AccountTypeEquity, "", "Accumulated profit/loss carried forward"},

	{AcctInterestIncome, "Interest Income", models.AccountTypeIncome, "", "Interest earned on loans, including accrued interest"},
	{AcctProcessingFeeIncome, "Processing Fee Income", models.AccountTypeIncome, "", "Processing fee revenue, exclusive of GST"},
	{AcctPenaltyOtherIncome, "Penalty / Other Income", models.AccountTypeIncome, "", "Late-payment penalties and other miscellaneous income"},

	{AcctOperatingExpenses, "Operating Expenses", models.AccountTypeExpense, "", "General operating expenditure"},
	{AcctBankCharges, "Bank Charges", models.AccountTypeExpense, AcctOperatingExpenses, "Payment-gateway/bank transfer charges"},
	{AcctBorrowingInterestExp, "Interest Expense on Borrowings", models.AccountTypeExpense, "", "Interest paid on NBFC funding/borrowing lines"},
	{AcctProvisionExpenseNPA, "Provision for NPA Expense (Phase 5)", models.AccountTypeExpense, "", "Reserved for Phase 5 — not posted to in Phase 4"},
}

// SeedChartOfAccounts is idempotent — safe to call on every startup. It
// only INSERTs accounts that don't already exist by code; it never
// overwrites a name/parent an admin has since edited via the Chart of
// Accounts screen.
func SeedChartOfAccounts() {
	codeToID := map[string]uint{}
	for _, a := range defaultChart {
		var existing models.LedgerAccount
		err := database.DB.Where("account_code = ?", a.code).First(&existing).Error
		if err == nil {
			codeToID[a.code] = existing.ID
			continue
		}
		var parentID *uint
		if a.parent != "" {
			if id, ok := codeToID[a.parent]; ok {
				parentID = &id
			}
		}
		acct := models.LedgerAccount{
			AccountCode: a.code, AccountName: a.name, AccountType: a.accType,
			ParentAccountID: parentID, Description: a.desc, Status: models.AccountStatusActive,
		}
		if err := database.DB.Create(&acct).Error; err != nil {
			log.Println("⚠️  failed to seed chart-of-accounts entry", a.code, ":", err)
			continue
		}
		codeToID[a.code] = acct.ID
	}
	log.Println("✅ Chart of Accounts seeded/verified")
}
