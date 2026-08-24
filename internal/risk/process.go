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

	classification, err := ClassifyLoan(loan.ID, loan.Category, "", dpd, asOf, "system")
	if err != nil && err != ErrNoApprovedRule {
		return err
	}
	if classification != "" {
		checkClassificationAlert(loan.ID, classification)
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
	}

	return nil
}

var lastKnownBucket = map[uint]string{}
var lastKnownClassification = map[uint]string{}

// checkBucketChangeAlert / checkClassificationAlert use a simple
// in-process map as a cheap same-run dedup guard against re-alerting for
// the same loan repeatedly within one job execution — not a durable
// "already alerted" record. A DB-backed alert-suppression window is a
// reasonable follow-up (see the Phase 5 report's gaps) but out of scope
// for this phase's job implementation.
func checkBucketChangeAlert(loanID uint, bucketName string, dpd int) {
	if lastKnownBucket[loanID] == bucketName {
		return
	}
	lastKnownBucket[loanID] = bucketName
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

func checkClassificationAlert(loanID uint, classification string) {
	if lastKnownClassification[loanID] == classification {
		return
	}
	lastKnownClassification[loanID] = classification
	CreateAlert("CLASSIFICATION_CHANGE", models.AlertSeverityWarning, &loanID,
		"Loan #"+itoa(loanID)+" classification is now "+classification)
}
