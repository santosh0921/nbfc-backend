package risk

import (
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/santosh0921/nbfc-backend/internal/collections"
	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/models"
)

// processLoansInBatches streams every DISBURSED loan through FindInBatches
// (item V — never loads the whole book into memory) and runs the full
// DPD -> bucket -> classification -> provisioning pipeline for each. One
// loan's failure is caught and recorded, not propagated — the batch (and
// the run) continues with the next loan.
func processLoansInBatches(asOf time.Time) (processed, failed int, failureDetail string) {
	var failures []string

	database.DB.Model(&models.LoanApplication{}).
		Where("status = ?", models.LoanStatusDisbursed).
		FindInBatches(&[]models.LoanApplication{}, batchSize, func(tx *gorm.DB, batchNum int) error {
			var batch []models.LoanApplication
			tx.Find(&batch)
			for _, loan := range batch {
				if err := processOneLoan(loan, asOf); err != nil {
					failed++
					failures = append(failures, "loan #"+itoa(loan.ID)+": "+err.Error())
					continue
				}
				processed++
			}
			return nil
		})

	return processed, failed, strings.Join(failures, "; ")
}

func processOneLoan(loan models.LoanApplication, asOf time.Time) error {
	dpd, err := CalculateDPDForLoan(loan.ID, asOf)
	if err != nil {
		return err
	}

	bucket, err := FindBucketForDPD(dpd, asOf)
	if err != nil {
		return err
	}
	if bucket != nil {
		checkBucketChangeAlert(loan.ID, bucket.Name, dpd)
	}

	var classificationBefore models.LoanClassification
	hadClassificationBefore := database.DB.First(&classificationBefore, "loan_id = ?", loan.ID).Error == nil

	classification, err := ClassifyLoan(loan.ID, loan.Category, "", dpd, asOf, "system")
	if err != nil && err != ErrNoApprovedRule {
		return err
	}
	if classification != "" {
		// classification only actually CHANGED if there was no prior row, or
		// the prior row's value differs — ClassifyLoan itself is a silent
		// no-op (same row, no new history) when nothing changed, so this
		// durable before/after DB comparison is the real change signal, not
		// an in-memory flag that forgets everything on restart.
		if !hadClassificationBefore || classificationBefore.Classification != classification {
			checkClassificationAlert(loan.ID, classification)
		}
		if _, provErr := CalculateProvision(loan.ID, classification, asOf, "system"); provErr != nil && provErr != ErrNoApprovedProvisioningRule {
			return provErr
		}
	}

	if dpd > 0 {
		var overdue int64
		database.DB.Model(&models.DPDRecord{}).
			Where("loan_id = ? AND calculation_date = ?", loan.ID, asOf).
			Select("COALESCE(SUM(outstanding_paise), 0)").Scan(&overdue)
		if overdue > 0 {
			collections.TouchCase(loan.ID, loan.CustomerID.String(), dpd, overdue)
		}
		if overdue > 5000000 { // ₹50,000 — an illustrative threshold, not a validated policy figure; see report §17
			CreateAlert("LARGE_OVERDUE", models.AlertSeverityWarning, &loan.ID,
				"Loan #"+itoa(loan.ID)+" has overdue amount exceeding the configured large-overdue alert threshold")
		}
	} else {
		// DPD is back to 0 — auto-close any still-open case for this loan.
		// Without this, a case that TouchCase opened while the loan was
		// overdue had no path back to CLOSED/RESOLVED even after the
		// borrower paid in full; it just sat in OVERDUE/ASSIGNED/ESCALATED
		// forever (see the Phase 5 report's gaps).
		collections.AutoResolveIfCaughtUp(loan.ID)
	}

	return nil
}

// checkBucketChangeAlert's "did this actually change" check is backed by
// LoanBucketState — one row per loan in the database — rather than an
// in-process map. The map version reset to empty on every backend
// restart, so the next job run after any restart treated every
// still-overdue loan as a brand-new bucket change and re-fired its alert;
// a durable row doesn't have that problem. checkClassificationAlert needs
// no equivalent of its own: its caller (processOneLoan) already gates on
// a before/after comparison of the durable LoanClassification row.
func checkBucketChangeAlert(loanID uint, bucketName string, dpd int) {
	var state models.LoanBucketState
	hadState := database.DB.First(&state, "loan_id = ?", loanID).Error == nil
	if hadState && state.BucketName == bucketName {
		return
	}
	if hadState {
		state.BucketName = bucketName
		database.DB.Save(&state)
	} else {
		database.DB.Create(&models.LoanBucketState{LoanID: loanID, BucketName: bucketName})
	}

	severity := models.AlertSeverityInfo
	if bucketName == "NPA" || bucketName == "HIGH_RISK" {
		severity = models.AlertSeverityCritical
	}
	CreateAlert("BUCKET_CHANGE", severity, &loanID, "Loan #"+itoa(loanID)+" moved to bucket "+bucketName+" (DPD "+itoaInt(dpd)+")")
	if bucketName == "NPA" {
		CreateAlert("NPA_CLASSIFICATION", models.AlertSeverityCritical, &loanID, "Loan #"+itoa(loanID)+" has entered the NPA bucket")
	}
	if bucketName == "HIGH_RISK" {
		CreateAlert("HIGH_RISK_ACCOUNT", models.AlertSeverityWarning, &loanID, "Loan #"+itoa(loanID)+" is now HIGH_RISK")
	}
}

// checkClassificationAlert itself does no dedup — its caller already
// confirmed this is a genuine change against the durable LoanClassification
// row before calling it.
func checkClassificationAlert(loanID uint, classification string) {
	CreateAlert("CLASSIFICATION_CHANGE", models.AlertSeverityWarning, &loanID,
		"Loan #"+itoa(loanID)+" classification is now "+classification)
}
