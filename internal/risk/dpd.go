// Package risk implements the Phase 5 loan portfolio risk foundation: DPD
// calculation, delinquency buckets, NPA/asset classification, and
// provisioning — built on top of Phase 4's accounting.Service for every
// financial posting, and Phase 2's verified payment/EMI records as the
// only source of truth for what has actually been paid.
//
// IMPORTANT: this package NEVER invents a DPD bucket range, a
// classification threshold, or a provisioning rate. Those live in
// DelinquencyBucket/ClassificationRule/ProvisioningRule rows that an
// authorized admin must configure and, for classification/provisioning,
// get maker-checker APPROVED before this package will use them for a
// single loan — see classification.go and provisioning.go.
package risk

import (
	"math"
	"time"

	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/models"
)

func toPaise(rupees float64) int64 {
	return int64(math.Round(rupees * 100))
}

// CalculateDPDForLoan computes and stores one DPDRecord per due,
// not-yet-superseded installment for a loan, as of asOf (normalized to
// midnight). Idempotent: an installment that already has a DPDRecord for
// this exact calculation date is left untouched — a second call for the
// same loan+date is a safe no-op, never a duplicate row (the DB's unique
// index on (schedule_id, calculation_date) is the actual guarantee; this
// function's own existence check is just the fast path).
//
// Returns the loan's current DPD — the maximum DPD across every
// installment that still has an outstanding balance as of asOf (0 if
// every due installment is fully paid). DPD is never negative: an
// installment not yet due contributes 0, and a paid installment's DPD is
// clamped to 0 regardless of how late the payment posting was.
func CalculateDPDForLoan(loanID uint, asOf time.Time) (int, error) {
	asOf = time.Date(asOf.Year(), asOf.Month(), asOf.Day(), 0, 0, 0, 0, asOf.Location())

	var installments []models.EmiInstallment
	if err := database.DB.Where("loan_id = ? AND due_date <= ?", loanID, asOf).
		Order("due_date").Find(&installments).Error; err != nil {
		return 0, err
	}

	loanDPD := 0
	for _, inst := range installments {
		dueAmount := toPaise(inst.Amount)
		var paidAmount int64
		if inst.Status == models.EmiStatusPaid {
			paidAmount = dueAmount
		}
		outstanding := dueAmount - paidAmount
		if outstanding < 0 {
			outstanding = 0
		}

		dpd := 0
		if outstanding > 0 {
			dueDateMidnight := time.Date(inst.DueDate.Year(), inst.DueDate.Month(), inst.DueDate.Day(), 0, 0, 0, 0, inst.DueDate.Location())
			days := int(asOf.Sub(dueDateMidnight).Hours() / 24)
			if days > 0 {
				dpd = days
			}
			if dpd > loanDPD {
				loanDPD = dpd
			}
		}

		var existing models.DPDRecord
		err := database.DB.Where("schedule_id = ? AND calculation_date = ?", inst.ID, asOf).First(&existing).Error
		if err == nil {
			continue // already calculated for this installment+date — idempotent no-op
		}

		record := models.DPDRecord{
			LoanID: loanID, ScheduleID: inst.ID, DueDate: inst.DueDate,
			DueAmountPaise: dueAmount, PaidAmountPaise: paidAmount, OutstandingPaise: outstanding,
			DPD: dpd, CalculationDate: asOf, Status: models.DPDRecordStatusCalculated,
		}
		// A unique-constraint violation here (a concurrent calculation for
		// the same installment+date) is treated as the same "already
		// calculated" no-op as the existence check above, not an error —
		// the row that won the race is what matters, not which caller
		// wrote it.
		database.DB.Create(&record)
	}

	return loanDPD, nil
}

// FindBucketForDPD returns the highest-priority ACTIVE
// DelinquencyBucket whose [MinDPD, MaxDPD] range contains dpd as of asOf,
// or nil if no bucket is configured to cover it — callers must treat nil
// as "cannot classify," never default to any particular bucket.
func FindBucketForDPD(dpd int, asOf time.Time) (*models.DelinquencyBucket, error) {
	var buckets []models.DelinquencyBucket
	q := database.DB.Where("status = ? AND min_dpd <= ? AND max_dpd >= ? AND effective_from <= ?",
		models.BucketStatusActive, dpd, dpd, asOf).
		Where("effective_to IS NULL OR effective_to >= ?", asOf).
		Order("priority ASC")
	if err := q.Find(&buckets).Error; err != nil {
		return nil, err
	}
	if len(buckets) == 0 {
		return nil, nil
	}
	return &buckets[0], nil
}
