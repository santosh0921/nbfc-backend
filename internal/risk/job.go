package risk

import (
	"log"
	"time"

	"github.com/santosh0921/nbfc-backend/internal/collections"
	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/models"
)

// batchSize keeps the daily job from ever loading the whole loan book
// into memory at once (item V) — gorm's FindInBatches streams rows in
// chunks of this size, processing and discarding each batch before
// fetching the next.
const batchSize = 200

// RunDailyRiskJob (item F) is the single entry point for the scheduled
// daily portfolio risk calculation. Idempotent at the run level: if a
// COMPLETED RiskJobRun already exists for runDate, it returns immediately
// without reprocessing anything. A previous FAILED/PARTIAL_FAILURE run
// for the same date is safely retried in place (same row, updated
// counters) — individual loan processing is itself idempotent (DPDRecord/
// LoanProvision unique indexes), so a retry never double-posts anything
// that already succeeded on an earlier attempt.
//
// Per-loan failures do not abort the run — item F: "do not silently
// continue after a partial failure" is honored by recording every
// failure's detail and ending the run as PARTIAL_FAILURE (never silently
// reported as COMPLETED) whenever LoansFailed > 0, while still finishing
// the loans that DID succeed rather than aborting the whole portfolio
// over one bad row.
func RunDailyRiskJob(runDate time.Time) (*models.RiskJobRun, error) {
	runDate = time.Date(runDate.Year(), runDate.Month(), runDate.Day(), 0, 0, 0, 0, runDate.Location())

	var jobRun models.RiskJobRun
	err := database.DB.Where("run_date = ?", runDate).First(&jobRun).Error
	if err == nil {
		if jobRun.Status == models.RiskJobStatusCompleted {
			log.Println("risk job: already COMPLETED for", runDate.Format("2006-01-02"), "— skipping (idempotent)")
			return &jobRun, nil
		}
		// FAILED or PARTIAL_FAILURE — safe to retry against the same row.
		jobRun.Status = models.RiskJobStatusRunning
		jobRun.LoansProcessed = 0
		jobRun.LoansFailed = 0
		jobRun.FailureDetail = ""
		database.DB.Save(&jobRun)
	} else {
		jobRun = models.RiskJobRun{RunDate: runDate, Status: models.RiskJobStatusRunning, StartedAt: time.Now()}
		if createErr := database.DB.Create(&jobRun).Error; createErr != nil {
			return nil, createErr
		}
	}

	processed, failed, failureDetail := processLoansInBatches(runDate)
	collections.MarkBrokenPromises(runDate)
	RunConsistencyChecks(runDate)

	now := time.Now()
	jobRun.LoansProcessed = processed
	jobRun.LoansFailed = failed
	jobRun.FailureDetail = failureDetail
	jobRun.CompletedAt = &now
	if failed > 0 {
		jobRun.Status = models.RiskJobStatusPartial
		CreateAlert("JOB_FAILED", models.AlertSeverityCritical, nil,
			"Daily risk job for "+runDate.Format("2006-01-02")+" completed with "+itoaInt(failed)+" loan failure(s) — see FailureDetail")
	} else {
		jobRun.Status = models.RiskJobStatusCompleted
	}
	database.DB.Save(&jobRun)

	return &jobRun, nil
}
