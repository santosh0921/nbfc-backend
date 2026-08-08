package loans

import (
	"errors"
	"math"
	"time"

	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/models"
)

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}

// generateEmiSchedule builds the full reducing-balance EMI amortization
// schedule for a just-disbursed loan and bulk-inserts the installments.
// Requires loan.TenureMonths, loan.InterestRatePercent and loan.DisbursedAt
// to already be set on the passed-in loan.
func generateEmiSchedule(loan *models.LoanApplication) error {
	if loan.TenureMonths == nil || *loan.TenureMonths <= 0 {
		return errors.New("loan is missing a valid tenure")
	}
	if loan.InterestRatePercent == nil || *loan.InterestRatePercent <= 0 {
		return errors.New("loan is missing a valid interest rate")
	}
	if loan.DisbursedAt == nil {
		return errors.New("loan has not been disbursed")
	}

	return generateEmiInstallments(loan.ID, loan.DisbursedAmount, *loan.TenureMonths, *loan.InterestRatePercent, *loan.DisbursedAt, 1)
}

// generateEmiInstallments builds a reducing-balance EMI amortization schedule
// for `principal` over `tenureMonths` months at `interestRatePercent` annual
// rate, with due dates starting one month after `baseDate`, and installment
// numbers starting at `startNumber`. Shared by the initial disbursement
// schedule (generateEmiSchedule, startNumber=1) and by restructuring, which
// regenerates the remaining schedule continuing the installment numbering
// after the loan's already-paid installments.
func generateEmiInstallments(loanID uint, principal float64, tenureMonths int, interestRatePercent float64, baseDate time.Time, startNumber int) error {
	if tenureMonths <= 0 {
		return errors.New("invalid tenure")
	}
	if interestRatePercent <= 0 {
		return errors.New("invalid interest rate")
	}

	n := tenureMonths
	r := interestRatePercent / 12 / 100

	pow := math.Pow(1+r, float64(n))
	emi := principal * r * pow / (pow - 1)

	balance := principal
	installments := make([]models.EmiInstallment, 0, n)

	for i := 1; i <= n; i++ {
		interest := round2(balance * r)
		var principalComp float64
		var amount float64
		if i == n {
			// last installment absorbs any rounding remainder so the
			// schedule sums exactly to principal
			principalComp = round2(balance)
			amount = round2(principalComp + interest)
		} else {
			principalComp = round2(emi - interest)
			amount = round2(principalComp + interest)
		}
		balance = round2(balance - principalComp)

		dueDate := baseDate.AddDate(0, i, 0)

		installments = append(installments, models.EmiInstallment{
			LoanID:             loanID,
			InstallmentNumber:  startNumber + i - 1,
			DueDate:            dueDate,
			Amount:             amount,
			PrincipalComponent: principalComp,
			InterestComponent:  interest,
			Status:             models.EmiStatusPending,
		})
	}

	return database.DB.Create(&installments).Error
}

// effectiveStatus derives "overdue" at read time without a background job.
func effectiveStatus(inst models.EmiInstallment) string {
	if inst.Status == models.EmiStatusPending && inst.DueDate.Before(time.Now()) {
		return models.EmiStatusOverdue
	}
	return inst.Status
}
