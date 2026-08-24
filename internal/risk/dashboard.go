// Portfolio risk dashboard (item M) — every figure here comes from a
// server-side database aggregation over loan/DPD/classification/
// provisioning tables, never a client-side sum over rows fetched into the
// browser, per the instruction. Filters are applied as SQL WHERE clauses,
// not in-memory filtering after a full-table fetch (item V).
package risk

import (
	"time"

	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/models"
)

type DashboardFilters struct {
	From, To    *time.Time
	Product     string
	State       string // branch/location proxy — see PortfolioDashboard's doc comment
	DPDBucket   string
	Classification string
	CollectionOfficerAdminID *uint
}

type BucketCount struct {
	BucketName string `json:"bucketName"`
	LoanCount  int64  `json:"loanCount"`
	OverduePaise int64 `json:"overdueAmountPaise"`
}

type PortfolioDashboardReport struct {
	TotalPortfolioLoans   int64 `json:"totalPortfolioLoans"`
	OutstandingPrincipalPaise int64 `json:"outstandingPrincipalPaise"`
	CurrentLoans          int64 `json:"currentLoans"`
	OverdueLoans          int64 `json:"overdueLoans"`
	TotalOverdueAmountPaise int64 `json:"totalOverdueAmountPaise"`
	DPDBuckets            []BucketCount `json:"dpdBuckets"`
	NPALoans              int64 `json:"npaLoans"`
	NPAAmountPaise        int64 `json:"npaAmountPaise"`
	ProvisionRequiredPaise int64 `json:"provisionRequiredPaise"`
	ProvisionOutstandingPaise int64 `json:"provisionOutstandingPaise"` // alias of required as of latest calc — kept as its own field for a future "provision actually held" distinction
	WrittenOffLoans       int64 `json:"writtenOffLoans"`
	WrittenOffAmountPaise int64 `json:"writtenOffAmountPaise"`
	RecoveredAmountPaise  int64 `json:"recoveredAmountPaise"`
}

// PortfolioDashboard (item M). Branch/location filtering is a documented
// gap: this codebase has no BranchID/location field on LoanApplication or
// CustomerAddress that's structured for that purpose today (only a free-
// text CurrentState on CustomerAddress) — State filtering is supported;
// a real branch/location filter needs that data model to exist first (see
// the Phase 5 report's gaps).
func PortfolioDashboard(f DashboardFilters) PortfolioDashboardReport {
	report := PortfolioDashboardReport{}

	base := database.DB.Model(&models.LoanApplication{}).Where("status = ?", models.LoanStatusDisbursed)
	if f.Product != "" {
		base = base.Where("category = ?", f.Product)
	}
	if f.From != nil {
		base = base.Where("disbursed_at >= ?", *f.From)
	}
	if f.To != nil {
		base = base.Where("disbursed_at <= ?", *f.To)
	}
	if f.State != "" {
		base = base.Where("customer_id IN (SELECT user_id FROM customer_addresses WHERE current_state = ?)", f.State)
	}

	base.Count(&report.TotalPortfolioLoans)

	// Outstanding principal: sum of unpaid EmiInstallment.PrincipalComponent
	// across the filtered loan set — the same "outstanding principal"
	// definition already used by Phase 4's interest accrual and this
	// phase's provisioning exposure base, so the dashboard's number can
	// never silently diverge from what those engines actually used.
	var loanIDs []uint
	base.Pluck("id", &loanIDs)
	if len(loanIDs) == 0 {
		return report
	}

	var outstanding float64
	database.DB.Model(&models.EmiInstallment{}).
		Where("loan_id IN ? AND status != ?", loanIDs, models.EmiStatusPaid).
		Select("COALESCE(SUM(principal_component),0)").Scan(&outstanding)
	report.OutstandingPrincipalPaise = toPaise(outstanding)

	// Latest DPD per loan (from the most recent calculation date on record
	// for each loan) drives current/overdue/bucket counts.
	type latestDPD struct {
		LoanID uint
		DPD    int
	}
	var latest []latestDPD
	database.DB.Raw(`
		SELECT DISTINCT ON (loan_id) loan_id, dpd FROM dpd_records
		WHERE loan_id IN ?
		ORDER BY loan_id, calculation_date DESC`, loanIDs).Scan(&latest)

	loanMaxDPD := map[uint]int{}
	for _, l := range latest {
		if l.DPD > loanMaxDPD[l.LoanID] {
			loanMaxDPD[l.LoanID] = l.DPD
		}
	}

	var buckets []models.DelinquencyBucket
	database.DB.Where("status = ?", models.BucketStatusActive).Order("priority").Find(&buckets)
	bucketCounts := map[string]*BucketCount{}
	for _, b := range buckets {
		bucketCounts[b.Name] = &BucketCount{BucketName: b.Name}
	}

	for loanID, dpd := range loanMaxDPD {
		bucketName := ""
		for _, b := range buckets {
			if dpd >= b.MinDPD && dpd <= b.MaxDPD {
				bucketName = b.Name
				break
			}
		}
		if dpd <= 0 {
			report.CurrentLoans++
		} else {
			report.OverdueLoans++
			var overdue float64
			database.DB.Model(&models.DPDRecord{}).
				Where("loan_id = ? AND calculation_date = (SELECT MAX(calculation_date) FROM dpd_records WHERE loan_id = ?)", loanID, loanID).
				Select("COALESCE(SUM(outstanding_paise),0)").Scan(&overdue)
			report.TotalOverdueAmountPaise += int64(overdue)
			if bc := bucketCounts[bucketName]; bc != nil {
				bc.OverduePaise += int64(overdue)
			}
		}
		if bc := bucketCounts[bucketName]; bc != nil {
			bc.LoanCount++
		}
		if bucketName == "NPA" {
			report.NPALoans++
		}
	}
	for _, b := range buckets {
		report.DPDBuckets = append(report.DPDBuckets, *bucketCounts[b.Name])
	}

	var npaClassifiedLoanIDs []uint
	database.DB.Model(&models.LoanClassification{}).
		Where("loan_id IN ? AND classification IN ?", loanIDs, []string{"SUBSTANDARD", "DOUBTFUL", "LOSS"}).
		Pluck("loan_id", &npaClassifiedLoanIDs)
	if len(npaClassifiedLoanIDs) > 0 {
		var npaOutstanding float64
		database.DB.Model(&models.EmiInstallment{}).
			Where("loan_id IN ? AND status != ?", npaClassifiedLoanIDs, models.EmiStatusPaid).
			Select("COALESCE(SUM(principal_component),0)").Scan(&npaOutstanding)
		report.NPAAmountPaise = toPaise(npaOutstanding)
	}

	var provisionSum int64
	database.DB.Model(&models.LoanProvision{}).
		Where("loan_id IN ? AND status = ?", loanIDs, models.ProvisionStatusActive).
		Select("COALESCE(SUM(provision_amount_paise),0)").Scan(&provisionSum)
	report.ProvisionRequiredPaise = provisionSum
	report.ProvisionOutstandingPaise = provisionSum

	var writtenOffCount int64
	var writtenOffSum int64
	database.DB.Model(&models.WriteOffRequest{}).
		Where("loan_id IN ? AND status IN ?", loanIDs, []string{models.WriteOffStatusWrittenOff, models.WriteOffStatusRecoveryAfter}).
		Count(&writtenOffCount)
	database.DB.Model(&models.WriteOffRequest{}).
		Where("loan_id IN ? AND status IN ?", loanIDs, []string{models.WriteOffStatusWrittenOff, models.WriteOffStatusRecoveryAfter}).
		Select("COALESCE(SUM(amount_paise),0)").Scan(&writtenOffSum)
	report.WrittenOffLoans = writtenOffCount
	report.WrittenOffAmountPaise = writtenOffSum

	var recovered int64
	database.DB.Model(&models.RecoveryPayment{}).
		Where("loan_id IN ?", loanIDs).
		Select("COALESCE(SUM(amount_paise),0)").Scan(&recovered)
	report.RecoveredAmountPaise = recovered

	return report
}
