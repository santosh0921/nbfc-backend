package accounting

import (
	"errors"
	"math"
	"strconv"
	"time"

	"gorm.io/gorm"

	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/models"
)

// AccrueInterestForLoan (item F) computes and posts one period's accrued
// interest for a single disbursed loan, independent of the EMI schedule
// generated at disbursement time (a customer's actual EMI due date can
// slip relative to accrual periods; this computes interest owed for the
// elapsed period on the loan's currently-outstanding principal, not off
// the fixed original schedule).
//
// Idempotent: the unique index on (loan_id, accrual_period) means a
// second call for the same loan+period returns the existing accrual
// unchanged rather than double-posting — callers (a scheduled job, or a
// manual admin trigger) do not need their own dedup logic.
//
// calculationBasis is intentionally simple (simple daily interest on the
// outstanding principal, actual/365) — this is a starting point for the
// NBFC's CA to confirm or correct, not a claim of the only correct method.
// NPA/IRAC income-recognition suspension rules (Phase 5) are NOT applied
// here; every accrual in this phase assumes standard/performing status.
func AccrueInterestForLoan(loanID uint, periodCode string, periodStart, periodEnd time.Time, actor string) (*models.InterestAccrual, error) {
	var existing models.InterestAccrual
	err := database.DB.Where("loan_id = ? AND accrual_period = ?", loanID, periodCode).First(&existing).Error
	if err == nil {
		return &existing, nil // already accrued — idempotent no-op
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	var loan models.LoanApplication
	if err := database.DB.First(&loan, loanID).Error; err != nil {
		return nil, err
	}
	if loan.Status != models.LoanStatusDisbursed || loan.InterestRatePercent == nil {
		return nil, errors.New("accounting: interest can only be accrued for a disbursed loan with a set interest rate")
	}

	outstanding := outstandingPrincipal(loan)
	if outstanding <= 0 {
		accrual := models.InterestAccrual{
			LoanID: loanID, AccrualPeriod: periodCode, CalculationBasis: "no outstanding principal",
			AmountPaise: 0, Status: models.AccrualStatusPosted,
		}
		database.DB.Create(&accrual)
		return &accrual, nil
	}

	days := periodEnd.Sub(periodStart).Hours()/24 + 1
	dailyRate := *loan.InterestRatePercent / 100 / 365
	interestRupees := outstanding * dailyRate * days
	amountPaise := toPaise(interestRupees)

	accrual := models.InterestAccrual{
		LoanID: loanID, AccrualPeriod: periodCode,
		CalculationBasis: "simple daily interest, actual/365, on outstanding principal — CA to confirm",
		AmountPaise:       amountPaise,
	}

	if amountPaise <= 0 {
		accrual.Status = models.AccrualStatusPosted
		database.DB.Create(&accrual)
		return &accrual, nil
	}

	entry, postErr := Service.PostJournalEntry(PostingInput{
		TransactionType: "INTEREST_ACCRUAL",
		ReferenceType:   "interest_accrual",
		ReferenceID:     strconv.Itoa(int(loanID)) + ":" + periodCode,
		TransactionDate: periodEnd,
		Description:     "Interest accrual, loan #" + strconv.Itoa(int(loanID)) + ", period " + periodCode,
		CreatedBy:       actor,
		Lines: []LineInput{
			{AccountCode: AcctInterestReceivable, DebitPaise: amountPaise, Description: "Interest accrued"},
			{AccountCode: AcctInterestIncome, CreditPaise: amountPaise, Description: "Interest income recognized"},
		},
	})
	if postErr != nil {
		accrual.Status = models.AccrualStatusFailed
		database.DB.Create(&accrual)
		return &accrual, postErr
	}

	accrual.Status = models.AccrualStatusPosted
	accrual.JournalReference = entry.JournalNumber
	if err := database.DB.Create(&accrual).Error; err != nil {
		return nil, err
	}
	return &accrual, nil
}

// outstandingPrincipal sums the PrincipalComponent of every not-yet-paid
// EmiInstallment for the loan — the actual basis for interest accrual,
// not the original DisbursedAmount, since that never decreases as EMIs
// are paid.
func outstandingPrincipal(loan models.LoanApplication) float64 {
	var total float64
	database.DB.Model(&models.EmiInstallment{}).
		Where("loan_id = ? AND status != ?", loan.ID, models.EmiStatusPaid).
		Select("COALESCE(SUM(principal_component), 0)").Scan(&total)
	return math.Max(total, 0)
}
