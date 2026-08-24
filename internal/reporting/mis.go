// MIS dashboard (item B) — every number is a server-side database
// aggregation over the authoritative tables Phases 1-5 already own
// (customers, loans, payments, DPD, provisioning, accounting journals).
// Nothing here recomputes a figure using different logic than the engine
// that owns it — e.g. outstanding principal is the same "sum of unpaid
// EmiInstallment.PrincipalComponent" definition internal/risk and
// internal/accounting already use.
package reporting

import (
	"time"

	"github.com/santosh0921/nbfc-backend/internal/accounting"
	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/models"
	"github.com/santosh0921/nbfc-backend/internal/risk"
)

type MISFilters struct {
	From, To *time.Time
	Product  string
	State    string
}

type CustomerMetrics struct {
	TotalCustomers int64 `json:"totalCustomers"`
	NewCustomers   int64 `json:"newCustomers"`
	KYCPending     int64 `json:"kycPending"`
	KYCVerified    int64 `json:"kycVerified"`
	KYCRejected    int64 `json:"kycRejected"`
}

type LoanMetrics struct {
	Applications        int64 `json:"applications"`
	Approved            int64 `json:"approved"`
	Rejected            int64 `json:"rejected"`
	Disbursed           int64 `json:"disbursed"`
	ActiveLoans         int64 `json:"activeLoans"`
	ClosedLoans         int64 `json:"closedLoans"`
	OutstandingPrincipalPaise int64 `json:"outstandingPrincipalPaise"`
	OutstandingInterestPaise  int64 `json:"outstandingInterestPaise"`
	TotalPortfolioPaise       int64 `json:"totalPortfolioPaise"`
}

type CollectionMetrics struct {
	EMIDueCount       int64 `json:"emiDueCount"`
	EMICollectedCount int64 `json:"emiCollectedCount"`
	CollectionRatePercent float64 `json:"collectionRatePercent"`
	OverdueAmountPaise int64 `json:"overdueAmountPaise"`
	DPDBuckets        []risk.BucketCount `json:"dpdBuckets"`
	RecoveryRatePercent float64 `json:"recoveryRatePercent"`
}

type RiskMetrics struct {
	CurrentLoans        int64 `json:"currentLoans"`
	DelinquentLoans     int64 `json:"delinquentLoans"`
	NPALoans            int64 `json:"npaLoans"`
	NPAAmountPaise      int64 `json:"npaAmountPaise"`
	ProvisionRequiredPaise int64 `json:"provisionRequiredPaise"`
	ProvisionBookedPaise   int64 `json:"provisionBookedPaise"`
}

type FinancialMetrics struct {
	InterestIncomePaise      int64 `json:"interestIncomePaise"`
	ProcessingFeeIncomePaise int64 `json:"processingFeeIncomePaise"`
	GSTCollectedPaise        int64 `json:"gstCollectedPaise"`
	GSTPayablePaise          int64 `json:"gstPayablePaise"`
	ExpensesPaise            int64 `json:"expensesPaise"`
	NetProfitPaise           int64 `json:"netProfitPaise"`
	BankBalancePaise         int64 `json:"bankBalancePaise"`
	BorrowingsOutstandingPaise int64 `json:"borrowingsOutstandingPaise"`
}

type MISDashboard struct {
	Customer  CustomerMetrics  `json:"customer"`
	Loan      LoanMetrics      `json:"loan"`
	Collection CollectionMetrics `json:"collection"`
	Risk      RiskMetrics      `json:"risk"`
	Financial FinancialMetrics `json:"financial"`
	LastUpdated time.Time      `json:"lastUpdated"`
}

// GenerateMISDashboard (item B) — computes the complete MIS in one call.
// Callers should treat the whole struct as generated together (the
// LastUpdated timestamp applies to every section), even though each
// section's query runs independently.
func GenerateMISDashboard(f MISFilters) MISDashboard {
	return MISDashboard{
		Customer:    customerMetrics(f),
		Loan:        loanMetrics(f),
		Collection:  collectionMetrics(f),
		Risk:        riskMetrics(f),
		Financial:   financialMetrics(f),
		LastUpdated: time.Now(),
	}
}

func customerMetrics(f MISFilters) CustomerMetrics {
	var m CustomerMetrics
	database.DB.Model(&models.User{}).Count(&m.TotalCustomers)
	q := database.DB.Model(&models.User{})
	if f.From != nil {
		q = q.Where("created_at >= ?", *f.From)
	}
	if f.To != nil {
		q = q.Where("created_at <= ?", *f.To)
	}
	q.Count(&m.NewCustomers)

	database.DB.Model(&models.CustomerKYC{}).Where("status IN ?",
		[]string{models.KYCStatusInitiated, models.KYCStatusDocumentsPending, models.KYCStatusVerificationPending}).Count(&m.KYCPending)
	database.DB.Model(&models.CustomerKYC{}).Where("status = ?", models.KYCStatusVerified).Count(&m.KYCVerified)
	database.DB.Model(&models.CustomerKYC{}).Where("status = ?", models.KYCStatusRejected).Count(&m.KYCRejected)
	return m
}

func loanMetrics(f MISFilters) LoanMetrics {
	var m LoanMetrics
	base := database.DB.Model(&models.LoanApplication{})
	if f.Product != "" {
		base = base.Where("category = ?", f.Product)
	}
	if f.From != nil {
		base = base.Where("created_at >= ?", *f.From)
	}
	if f.To != nil {
		base = base.Where("created_at <= ?", *f.To)
	}
	base.Count(&m.Applications)

	countStatus := func(status string) int64 {
		var c int64
		q := database.DB.Model(&models.LoanApplication{}).Where("status = ?", status)
		if f.Product != "" {
			q = q.Where("category = ?", f.Product)
		}
		q.Count(&c)
		return c
	}
	m.Approved = countStatus(models.LoanStatusSanctioned) + countStatus(models.LoanStatusDisbursed)
	m.Rejected = countStatus(models.LoanStatusRejected)
	m.Disbursed = countStatus(models.LoanStatusDisbursed)
	m.ActiveLoans = countStatus(models.LoanStatusDisbursed)

	var loanIDs []uint
	q2 := database.DB.Model(&models.LoanApplication{}).Where("status = ?", models.LoanStatusDisbursed)
	if f.Product != "" {
		q2 = q2.Where("category = ?", f.Product)
	}
	q2.Pluck("id", &loanIDs)

	if len(loanIDs) > 0 {
		var principal, interest float64
		database.DB.Model(&models.EmiInstallment{}).
			Where("loan_id IN ? AND status != ?", loanIDs, models.EmiStatusPaid).
			Select("COALESCE(SUM(principal_component),0)").Scan(&principal)
		database.DB.Model(&models.EmiInstallment{}).
			Where("loan_id IN ? AND status != ?", loanIDs, models.EmiStatusPaid).
			Select("COALESCE(SUM(interest_component),0)").Scan(&interest)
		m.OutstandingPrincipalPaise = toPaise(principal)
		m.OutstandingInterestPaise = toPaise(interest)
		m.TotalPortfolioPaise = m.OutstandingPrincipalPaise + m.OutstandingInterestPaise
	}
	return m
}

func collectionMetrics(f MISFilters) CollectionMetrics {
	var m CollectionMetrics
	from := time.Now().AddDate(0, -1, 0)
	to := time.Now()
	if f.From != nil {
		from = *f.From
	}
	if f.To != nil {
		to = *f.To
	}

	database.DB.Model(&models.EmiInstallment{}).Where("due_date BETWEEN ? AND ?", from, to).Count(&m.EMIDueCount)
	database.DB.Model(&models.EmiInstallment{}).
		Where("due_date BETWEEN ? AND ? AND status = ?", from, to, models.EmiStatusPaid).Count(&m.EMICollectedCount)
	if m.EMIDueCount > 0 {
		m.CollectionRatePercent = round2(float64(m.EMICollectedCount) / float64(m.EMIDueCount) * 100)
	}

	dashboard := risk.PortfolioDashboard(risk.DashboardFilters{From: f.From, To: f.To, Product: f.Product, State: f.State})
	m.OverdueAmountPaise = dashboard.TotalOverdueAmountPaise
	m.DPDBuckets = dashboard.DPDBuckets

	var recovered int64
	database.DB.Model(&models.RecoveryPayment{}).Select("COALESCE(SUM(amount_paise),0)").Scan(&recovered)
	var writtenOff int64
	database.DB.Model(&models.WriteOffRequest{}).Where("status IN ?",
		[]string{models.WriteOffStatusWrittenOff, models.WriteOffStatusRecoveryAfter}).
		Select("COALESCE(SUM(amount_paise),0)").Scan(&writtenOff)
	if writtenOff > 0 {
		m.RecoveryRatePercent = round2(float64(recovered) / float64(writtenOff) * 100)
	}
	return m
}

func riskMetrics(f MISFilters) RiskMetrics {
	dashboard := risk.PortfolioDashboard(risk.DashboardFilters{From: f.From, To: f.To, Product: f.Product, State: f.State})
	return RiskMetrics{
		CurrentLoans: dashboard.CurrentLoans, DelinquentLoans: dashboard.OverdueLoans,
		NPALoans: dashboard.NPALoans, NPAAmountPaise: dashboard.NPAAmountPaise,
		ProvisionRequiredPaise: dashboard.ProvisionRequiredPaise, ProvisionBookedPaise: dashboard.ProvisionOutstandingPaise,
	}
}

func financialMetrics(f MISFilters) FinancialMetrics {
	from := time.Now().AddDate(0, -1, 0)
	to := time.Now()
	if f.From != nil {
		from = *f.From
	}
	if f.To != nil {
		to = *f.To
	}
	pl := accounting.ProfitAndLoss(from, to)
	var interestIncome, feeIncome int64
	for _, l := range pl.Income {
		switch l.AccountCode {
		case accounting.AcctInterestIncome:
			interestIncome = l.AmountPaise
		case accounting.AcctProcessingFeeIncome:
			feeIncome = l.AmountPaise
		}
	}

	var gstPayable int64
	database.DB.Model(&models.GSTTransaction{}).Select("COALESCE(SUM(cgst_paise + sgst_paise + igst_paise),0)").Scan(&gstPayable)

	bs := accounting.BalanceSheet(to)
	var bankBalance, borrowings int64
	for _, l := range bs.Assets {
		if l.AccountCode == accounting.AcctBankCash {
			bankBalance = l.AmountPaise
		}
	}
	for _, l := range bs.Liabilities {
		if l.AccountCode == accounting.AcctBorrowingsPayable {
			borrowings = l.AmountPaise
		}
	}

	return FinancialMetrics{
		InterestIncomePaise: interestIncome, ProcessingFeeIncomePaise: feeIncome,
		GSTCollectedPaise: gstPayable, GSTPayablePaise: gstPayable,
		ExpensesPaise: pl.TotalExpensePaise, NetProfitPaise: pl.NetProfitPaise,
		BankBalancePaise: bankBalance, BorrowingsOutstandingPaise: borrowings,
	}
}

func toPaise(rupees float64) int64 {
	return int64(rupees * 100)
}

func round2(v float64) float64 {
	return float64(int64(v*100+0.5)) / 100
}
