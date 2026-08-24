// KPI dictionary (item AB — "This is mandatory") and data lineage
// (item AC).
//
// Unlike ManagementThreshold and Budget — which encode BUSINESS POLICY
// and are therefore never seeded — the KPI dictionary describes
// CALCULATIONS THIS CODEBASE ACTUALLY PERFORMS. Seeding it is
// documentation of existing behaviour, not a business decision, so it is
// safe and useful to seed. Each Formula below is the real formula used by
// the corresponding function in this package.
package analytics

import (
	"time"

	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/models"
)

// Lineage (item AC) is the dashboard → report → calculation → source →
// transaction chain for one KPI, returned alongside the KPI so a number
// can always be traced without reading the code.
type Lineage struct {
	Dashboard   string `json:"dashboard"`
	Report      string `json:"report"`
	Calculation string `json:"calculation"`
	Source      string `json:"source"`
	Transaction string `json:"transaction"`
}

type KPIWithLineage struct {
	models.KPIDefinition
	Lineage Lineage `json:"lineage"`
}

var kpiSeed = []struct {
	def     models.KPIDefinition
	lineage Lineage
}{
	{
		models.KPIDefinition{
			Key: "disbursed_amount", Name: "Total Disbursed",
			Definition: "Cash actually paid out to borrowers in the period. This is the creation of a loan receivable asset, NOT an expense.",
			Formula:    "SUM(disbursements.amount_paise) WHERE status = SUCCESS AND completed_at IN period",
			DataSource: "models.Disbursement", Frequency: "DAILY", Unit: "PAISE",
			Dependency: "None — independent of accounting period close.",
		},
		Lineage{"Growth Dashboard", "Disbursement Register", "analytics.disbursedSum", "disbursements table", "one row per successful disbursement"},
	},
	{
		models.KPIDefinition{
			Key: "collections_amount", Name: "Total Collected",
			Definition: "Cash received from borrowers in the period, INCLUDING principal repayment. This is not revenue and must never be presented as such.",
			Formula:    "SUM(journal_entry_lines.debit_paise) WHERE account_code = 1000 AND transaction_type = EMI_PAYMENT AND transaction_date IN period",
			DataSource: "General ledger (journal entries)", Frequency: "DAILY", Unit: "PAISE",
			Notes:      "Principal component of a collection reduces the loan receivable; only interest and fees are income.",
			Dependency: "Requires EMI payment postings to the general ledger (Phase 4).",
		},
		Lineage{"Growth Dashboard", "Collection MIS", "analytics.collectionsSum", "journal_entry_lines joined to journal_entries", "one journal entry per verified EMI payment"},
	},
	{
		models.KPIDefinition{
			Key: "revenue", Name: "Revenue",
			Definition: "Recognized income for the period: interest income, processing fee income, and penalty/other income. Excludes principal recovery entirely.",
			Formula:    "SUM(credit - debit) over accounts 4000 (interest), 4100 (processing fee), 4200 (penalty/other) for the period",
			DataSource: "General ledger via accounting.ProfitAndLoss", Frequency: "MONTHLY", Unit: "PAISE",
			Dependency: "Requires the accounting period's postings to be complete; figures for an open period are provisional.",
		},
		Lineage{"Growth Dashboard", "Revenue Quality Report", "accounting.ProfitAndLoss / analytics.incomeSum", "ledger income accounts 4000/4100/4200", "journal entries raised on EMI and fee recognition"},
	},
	{
		models.KPIDefinition{
			Key: "expenses", Name: "Operating Expenses",
			Definition: "Operating expenses recognized in the period. Excludes funding cost, which is tracked separately.",
			Formula:    "accounting.ProfitAndLoss(period).TotalExpensePaise",
			DataSource: "General ledger expense accounts", Frequency: "MONTHLY", Unit: "PAISE",
			Dependency: "Requires expense approval and posting workflow (Phase 8).",
		},
		Lineage{"Growth Dashboard", "Expense Efficiency Report", "accounting.ProfitAndLoss", "ledger expense accounts (5000 series)", "journal entry per paid expense"},
	},
	{
		models.KPIDefinition{
			Key: "funding_cost", Name: "Funding Cost",
			Definition: "Interest expense on the NBFC's own borrowings for the period.",
			Formula:    "SUM(journal_entry_lines.debit_paise) WHERE account_code = 5200 AND transaction_date IN period",
			DataSource: "General ledger account 5200, sourced from models.BorrowingTransaction", Frequency: "MONTHLY", Unit: "PAISE",
			Notes:      "Reads zero until real FundingSource and BorrowingTransaction rows are recorded. Zero here means 'no borrowings recorded', not 'no funding cost exists'.",
			Dependency: "Requires funding sources to be entered by finance (Phase 8 model, admin-entered).",
		},
		Lineage{"Profitability", "Funding MIS", "analytics.FundingCost", "ledger account 5200", "journal entry per borrowing interest charge"},
	},
	{
		models.KPIDefinition{
			Key: "pbt", Name: "Profit Before Tax",
			Definition: "Revenue less operating expenses less funding cost. Not a cash figure and not a bank balance.",
			Formula:    "revenue - expenses - funding_cost",
			DataSource: "General ledger", Frequency: "MONTHLY", Unit: "PAISE",
			Notes:      "PAT is deliberately not derived from this: no corporate income-tax provision exists in this system, and TDS is not a substitute for one.",
			Dependency: "REQUIRES CA VALIDATION before use in any statutory or external context.",
		},
		Lineage{"Profitability", "Profitability Report", "analytics.GenerateProfitability", "ledger income and expense accounts", "all journal entries in the period"},
	},
	{
		models.KPIDefinition{
			Key: "outstanding_principal", Name: "Outstanding Principal",
			Definition: "Unpaid principal component of installments on disbursed loans.",
			Formula:    "SUM(emi_installments.principal_component) WHERE loan disbursed AND installment unpaid",
			DataSource: "models.EmiInstallment", Frequency: "DAILY", Unit: "PAISE",
			Dependency: "None.",
		},
		Lineage{"Portfolio", "Portfolio MIS", "analytics.outstandingPrincipalAsOf", "emi_installments joined to loan_applications", "one row per scheduled installment"},
	},
	{
		models.KPIDefinition{
			Key: "collection_efficiency", Name: "Collection Efficiency",
			Definition: "Cash collected in the period as a percentage of installment amounts that fell due in the period.",
			Formula:    "collections_amount / SUM(emi_installments.amount WHERE due_date IN period) * 100",
			DataSource: "General ledger + models.EmiInstallment", Frequency: "MONTHLY", Unit: "PERCENT",
			Notes:      "Can exceed 100% when arrears from earlier periods are collected. That is expected, not an error.",
			Dependency: "None.",
		},
		Lineage{"Collections", "Collection Risk Report", "analytics.GenerateCollectionRisk", "journal entries + emi_installments", "EMI payments and the schedule they settle"},
	},
	{
		models.KPIDefinition{
			Key: "approval_rate", Name: "Approval Rate",
			Definition: "Applications reaching sanction or beyond, as a percentage of applications created in the period.",
			Formula:    "COUNT(status IN sanctioned/ready/processing/disbursed) / COUNT(all applications) * 100",
			DataSource: "models.LoanApplication", Frequency: "MONTHLY", Unit: "PERCENT",
			Notes:      "A period's rate is provisional while applications created in it are still in progress.",
			Dependency: "None.",
		},
		Lineage{"Growth Dashboard", "Loan Funnel", "analytics.GenerateGrowthDashboard", "loan_applications table", "one row per application"},
	},
	{
		models.KPIDefinition{
			Key: "dpd_90_plus_share", Name: "90+ DPD Portfolio Share",
			Definition: "Unpaid past-due amount aged more than 90 days as a percentage of outstanding principal. An OPERATIONAL arrears measure.",
			Formula:    "SUM(unpaid past-due installment amounts with DPD > 90) / outstanding_principal * 100",
			DataSource: "models.EmiInstallment", Frequency: "MONTHLY", Unit: "PERCENT",
			Notes:      "NOT a regulatory NPA figure. Regulatory asset classification requires approved ClassificationRule rows and is produced only by internal/risk.",
			Dependency: "REQUIRES RISK/COMPLIANCE VALIDATION before any regulatory use.",
		},
		Lineage{"Risk", "DPD Trend Report", "analytics.dpdBucketsAsOf", "emi_installments due dates and paid_at", "one row per scheduled installment"},
	},
}

// SeedKPIDictionary is idempotent — it inserts missing KPIs and refreshes
// the descriptive fields of existing ones, so a formula change in code is
// reflected in the dictionary rather than silently diverging from it.
func SeedKPIDictionary() {
	for _, k := range kpiSeed {
		var existing models.KPIDefinition
		if err := database.DB.Where("key = ?", k.def.Key).First(&existing).Error; err != nil {
			database.DB.Create(&k.def)
			continue
		}
		database.DB.Model(&existing).Updates(map[string]interface{}{
			"name": k.def.Name, "definition": k.def.Definition, "formula": k.def.Formula,
			"data_source": k.def.DataSource, "frequency": k.def.Frequency, "unit": k.def.Unit,
			"notes": k.def.Notes, "dependency": k.def.Dependency,
		})
	}
}

func ListKPIs() []KPIWithLineage {
	var defs []models.KPIDefinition
	database.DB.Order("key").Find(&defs)

	lineageByKey := map[string]Lineage{}
	for _, k := range kpiSeed {
		lineageByKey[k.def.Key] = k.lineage
	}

	out := make([]KPIWithLineage, 0, len(defs))
	for _, d := range defs {
		out = append(out, KPIWithLineage{KPIDefinition: d, Lineage: lineageByKey[d.Key]})
	}
	return out
}

// MetricValue resolves a KPI key to its current value for a period. It is
// the single lookup the alert engine uses, so an alert can only ever fire
// on a KPI that is defined in the dictionary — thresholds cannot be
// attached to undocumented numbers.
func MetricValue(key string, from, to time.Time) (float64, bool) {
	d := GenerateGrowthDashboard(from, to)
	for _, group := range [][]Metric{d.Customers, d.Applications, d.Loans, d.Disbursements,
		d.Collections, d.Portfolio, d.Revenue, d.Expenses, d.Profitability} {
		for _, m := range group {
			if m.Key == key {
				return m.Current, true
			}
		}
	}
	return 0, false
}
