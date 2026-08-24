package mis

import (
	"time"

	"github.com/santosh0921/nbfc-backend/internal/compliance"
	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/finance"
	"github.com/santosh0921/nbfc-backend/internal/models"
	"github.com/santosh0921/nbfc-backend/internal/risk"
	"github.com/santosh0921/nbfc-backend/internal/tax"
)

// BusinessCounts — the loan-funnel/customer counts item C's "TOP KPIs"
// asks for, computed fresh from the authoritative tables (LoanApplication/
// User), never cached or hand-entered.
type BusinessCounts struct {
	TotalCustomers int64 `json:"totalCustomers"`
	TotalLoans     int64 `json:"totalLoans"`
	ActiveLoans    int64 `json:"activeLoans"`
	ClosedLoans    int64 `json:"closedLoans"`
	Applications   int64 `json:"applications"`
	Approved       int64 `json:"approved"`
	Rejected       int64 `json:"rejected"`
}

func GenerateBusinessCounts(from, to time.Time) BusinessCounts {
	var b BusinessCounts
	database.DB.Model(&models.User{}).Count(&b.TotalCustomers)
	database.DB.Model(&models.LoanApplication{}).Count(&b.TotalLoans)
	database.DB.Model(&models.LoanApplication{}).Where("status = ?", models.LoanStatusDisbursed).Count(&b.ActiveLoans)
	// "Closed" is not a status this codebase's loan state machine ever
	// sets (confirmed by inspection — no LoanStatusClosed constant
	// exists anywhere in Phases 1-11). A loan is derived as closed here
	// only when EVERY one of its installments is actually paid — a real,
	// verified-payment-backed signal, never a status this code invents.
	// ClosedLoans therefore undercounts any loan closed by a mechanism
	// other than full EMI repayment (e.g. a write-off, Phase 5) — those
	// are tracked separately via internal/writeoff, not counted here.
	database.DB.Raw(`
		SELECT COUNT(*) FROM loan_applications la
		WHERE la.status = ?
		AND EXISTS (SELECT 1 FROM emi_installments ei WHERE ei.loan_id = la.id)
		AND NOT EXISTS (SELECT 1 FROM emi_installments ei WHERE ei.loan_id = la.id AND ei.status != 'paid')
	`, models.LoanStatusDisbursed).Scan(&b.ClosedLoans)
	b.ActiveLoans -= b.ClosedLoans
	database.DB.Model(&models.LoanApplication{}).Where("created_at BETWEEN ? AND ?", from, to).Count(&b.Applications)
	database.DB.Model(&models.LoanApplication{}).
		Where("status IN ? AND created_at BETWEEN ? AND ?", []string{models.LoanStatusSanctioned, models.LoanStatusDisbursed}, from, to).
		Count(&b.Approved)
	database.DB.Model(&models.LoanApplication{}).
		Where("status = ? AND created_at BETWEEN ? AND ?", models.LoanStatusRejected, from, to).Count(&b.Rejected)
	return b
}

// ExecutiveDashboard (item C) — literally embeds the pre-existing,
// already-tested dashboard structs from Phases 5/8/10/11 rather than
// re-deriving equivalent figures. This is what makes item B's "single
// source of truth" a structural guarantee rather than a discipline that
// could quietly drift: there is no second calculation of "total
// disbursed" anywhere in this file.
type ExecutiveDashboard struct {
	PeriodFrom time.Time `json:"periodFrom"`
	PeriodTo   time.Time `json:"periodTo"`

	Business  BusinessCounts                    `json:"business"`
	Financial finance.FinancialDashboard         `json:"financial"`
	Portfolio risk.PortfolioDashboardReport      `json:"portfolio"`
	Tax       tax.TaxDashboard                   `json:"tax"`
	Compliance compliance.ComplianceTrackingDashboard `json:"compliance"`

	GeneratedAt time.Time `json:"generatedAt"`
}

func GenerateExecutiveDashboard(from, to time.Time) ExecutiveDashboard {
	return ExecutiveDashboard{
		PeriodFrom: from, PeriodTo: to,
		Business:   GenerateBusinessCounts(from, to),
		Financial:  finance.GenerateFinancialDashboard(from, to),
		Portfolio:  risk.PortfolioDashboard(risk.DashboardFilters{From: &from, To: &to}),
		Tax:        tax.GenerateTaxDashboard(from, to),
		Compliance: compliance.GenerateDashboard(),
		GeneratedAt: time.Now(),
	}
}
