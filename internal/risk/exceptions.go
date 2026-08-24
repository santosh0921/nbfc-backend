package risk

import (
	"time"

	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/models"
)

// RunConsistencyChecks (item T) detects mismatches across the loan
// schedule, verified payments, DPD, and provisioning records, recording
// one DataConsistencyException per finding — it never silently corrects
// anything; every finding needs a human to look at it via the admin
// Exceptions screen (internal/risk/handler.go's
// AdminListExceptionsHandler/AdminResolveExceptionHandler).
func RunConsistencyChecks(asOf time.Time) int {
	found := 0
	found += checkNegativeOutstanding()
	found += checkPaidStillOverdue()
	found += checkDuplicateProvisions(asOf)
	found += checkMissingJournalReferences()
	return found
}

func recordException(exceptionType string, loanID *uint, detail string) {
	// Avoid re-raising the exact same unresolved finding on every run —
	// only create a new row if no OPEN exception of this type already
	// exists for this loan.
	var existing models.DataConsistencyException
	q := database.DB.Where("exception_type = ? AND status = ?", exceptionType, models.ExceptionStatusOpen)
	if loanID != nil {
		q = q.Where("loan_id = ?", *loanID)
	} else {
		q = q.Where("loan_id IS NULL")
	}
	if q.First(&existing).Error == nil {
		return
	}
	database.DB.Create(&models.DataConsistencyException{
		ExceptionType: exceptionType, LoanID: loanID, Detail: detail,
		Status: models.ExceptionStatusOpen, DetectedAt: time.Now(),
	})
}

// checkNegativeOutstanding — a DPDRecord's outstanding amount should
// never be negative (the calculation clamps it, but a direct DB write
// bypassing CalculateDPDForLoan — e.g. a bad migration or manual fix —
// could still produce one).
func checkNegativeOutstanding() int {
	var rows []struct {
		LoanID uint
	}
	database.DB.Model(&models.DPDRecord{}).
		Where("outstanding_paise < 0").
		Select("DISTINCT loan_id").Scan(&rows)
	for _, r := range rows {
		loanID := r.LoanID
		recordException(models.ExceptionTypeNegativeOutstanding, &loanID, "loan #"+itoa(loanID)+" has a DPDRecord with negative outstanding_paise")
	}
	return len(rows)
}

// checkPaidStillOverdue — an EmiInstallment marked paid but whose most
// recent DPDRecord still shows a positive outstanding balance means the
// two systems disagree about whether it was actually collected.
func checkPaidStillOverdue() int {
	var rows []struct {
		LoanID     uint
		ScheduleID uint
	}
	database.DB.Table("emi_installments ei").
		Joins(`JOIN dpd_records dr ON dr.schedule_id = ei.id AND dr.calculation_date = (
			SELECT MAX(calculation_date) FROM dpd_records WHERE schedule_id = ei.id)`).
		Where("ei.status = ? AND dr.outstanding_paise > 0", models.EmiStatusPaid).
		Select("ei.loan_id, ei.id as schedule_id").Scan(&rows)
	for _, r := range rows {
		loanID := r.LoanID
		recordException(models.ExceptionTypePaidStillOverdue, &loanID,
			"installment #"+itoa(r.ScheduleID)+" on loan #"+itoa(loanID)+" is marked paid but its latest DPDRecord still shows an outstanding balance")
	}
	return len(rows)
}

// checkDuplicateProvisions — the unique index on (loan_id,
// calculation_date) should make this impossible going forward, but this
// check exists to catch anything from before that index existed or from
// a direct DB write.
func checkDuplicateProvisions(asOf time.Time) int {
	var rows []struct {
		LoanID uint
		Count  int
	}
	database.DB.Model(&models.LoanProvision{}).
		Where("calculation_date = ?", asOf).
		Group("loan_id").Having("COUNT(*) > 1").
		Select("loan_id, COUNT(*) as count").Scan(&rows)
	for _, r := range rows {
		loanID := r.LoanID
		recordException(models.ExceptionTypeDuplicateProvision, &loanID,
			"loan #"+itoa(loanID)+" has "+itoaInt(r.Count)+" LoanProvision rows for the same calculation date "+asOf.Format("2006-01-02"))
	}
	return len(rows)
}

// checkMissingJournalReferences — an ACTIVE provision with a nonzero
// amount should always have a JournalReference; one without means the
// accounting posting failed or was skipped silently somewhere.
func checkMissingJournalReferences() int {
	var rows []struct {
		LoanID uint
		ID     uint
	}
	database.DB.Model(&models.LoanProvision{}).
		Where("status = ? AND provision_amount_paise > 0 AND (journal_reference IS NULL OR journal_reference = '')", models.ProvisionStatusActive).
		Select("loan_id, id").Scan(&rows)
	for _, r := range rows {
		loanID := r.LoanID
		recordException(models.ExceptionTypeMissingJournalRef, &loanID,
			"LoanProvision #"+itoa(r.ID)+" for loan #"+itoa(loanID)+" is ACTIVE with a nonzero amount but has no journal reference")
	}
	return len(rows)
}
