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

// generateEmiSchedule builds the repayment schedule for a just-disbursed
// loan and bulk-inserts the installment(s). For every regular loan this is
// the full reducing-balance EMI amortization schedule (requires
// loan.TenureMonths and loan.InterestRatePercent). An Instant Loan
// (isInstantLoanCategory) is a different product entirely — a single
// lump-sum installment due instantLoanTenureDays after disbursement, for a
// flat one-time fee instead of monthly reducing-balance interest — and
// needs neither of those fields.
func generateEmiSchedule(loan *models.LoanApplication) error {
	if loan.DisbursedAt == nil {
		return errors.New("loan has not been disbursed")
	}

	if isInstantLoanCategory(loan.Category) {
		return generateInstantLoanInstallment(loan.ID, loan.DisbursedAmount, *loan.DisbursedAt)
	}

	if isGoldLoanCategory(loan.Category) {
		if loan.TenureMonths == nil || *loan.TenureMonths <= 0 {
			return errors.New("loan is missing a valid tenure")
		}
		if loan.InterestRatePercent == nil || *loan.InterestRatePercent <= 0 {
			return errors.New("loan is missing a valid interest rate")
		}
		return generateGoldLoanInstallments(loan.ID, loan.DisbursedAmount, *loan.TenureMonths, *loan.InterestRatePercent, *loan.DisbursedAt)
	}

	if isFarmerLoanCategory(loan.Category) {
		if loan.TenureMonths == nil || *loan.TenureMonths <= 0 {
			return errors.New("loan is missing a valid tenure")
		}
		if loan.InterestRatePercent == nil || *loan.InterestRatePercent <= 0 {
			return errors.New("loan is missing a valid interest rate")
		}
		return generateFarmerLoanInstallment(loan.ID, loan.DisbursedAmount, *loan.TenureMonths, *loan.InterestRatePercent, *loan.DisbursedAt)
	}

	if loan.TenureMonths == nil || *loan.TenureMonths <= 0 {
		return errors.New("loan is missing a valid tenure")
	}
	if loan.InterestRatePercent == nil || *loan.InterestRatePercent <= 0 {
		return errors.New("loan is missing a valid interest rate")
	}

	return generateEmiInstallments(loan.ID, loan.DisbursedAmount, *loan.TenureMonths, *loan.InterestRatePercent, *loan.DisbursedAt, 1)
}

// generateInstantLoanInstallment creates the single due-in-35-days
// repayment row for an Instant Loan: principal + a flat one-time fee, no
// amortization. Once this is overdue it's picked up by the exact same
// collections/overdue machinery as a missed EMI on any other loan — no
// separate default-handling path.
func generateInstantLoanInstallment(loanID uint, principal float64, disbursedAt time.Time) error {
	fee := round2(principal * instantLoanFlatFeePercent / 100)
	dueDate := disbursedAt.AddDate(0, 0, instantLoanTenureDays)
	installment := models.EmiInstallment{
		LoanID:             loanID,
		InstallmentNumber:  1,
		DueDate:            dueDate,
		Amount:             round2(principal + fee),
		PrincipalComponent: principal,
		InterestComponent:  fee,
		Status:             models.EmiStatusPending,
	}
	return database.DB.Create(&installment).Error
}

// generateGoldLoanInstallments builds the real-world gold loan repayment
// shape: interest-only every month on the full principal (the principal
// never reduces mid-tenure, unlike a regular amortizing EMI), then one
// final installment that is the last month's interest PLUS the entire
// principal as a bullet repayment. This is why Gold Loan needed its own
// generator rather than reusing generateEmiInstallments — a reducing-
// balance schedule would be factually wrong for this product.
func generateGoldLoanInstallments(loanID uint, principal float64, tenureMonths int, interestRatePercent float64, baseDate time.Time) error {
	if tenureMonths <= 0 {
		return errors.New("invalid tenure")
	}
	if interestRatePercent <= 0 {
		return errors.New("invalid interest rate")
	}

	monthlyInterest := round2(principal * interestRatePercent / 12 / 100)
	installments := make([]models.EmiInstallment, 0, tenureMonths)

	for i := 1; i <= tenureMonths; i++ {
		dueDate := baseDate.AddDate(0, i, 0)
		isLast := i == tenureMonths

		principalComp := 0.0
		amount := monthlyInterest
		if isLast {
			principalComp = principal
			amount = round2(monthlyInterest + principal)
		}

		installments = append(installments, models.EmiInstallment{
			LoanID:             loanID,
			InstallmentNumber:  i,
			DueDate:            dueDate,
			Amount:             amount,
			PrincipalComponent: principalComp,
			InterestComponent:  monthlyInterest,
			Status:             models.EmiStatusPending,
		})
	}

	return database.DB.Create(&installments).Error
}

// generateFarmerLoanInstallment builds the real-world crop-loan repayment
// shape: a single bullet repayment due after harvest (tenureMonths out),
// not monthly installments — a farmer repays once the crop sells, not on
// a fixed monthly salary cycle. Simple (non-compounding) interest for the
// whole tenure, since there's no monthly reducing-balance concept when
// there's only one payment.
func generateFarmerLoanInstallment(loanID uint, principal float64, tenureMonths int, interestRatePercent float64, disbursedAt time.Time) error {
	interest := round2(principal * interestRatePercent / 100 * float64(tenureMonths) / 12)
	dueDate := disbursedAt.AddDate(0, tenureMonths, 0)
	installment := models.EmiInstallment{
		LoanID:             loanID,
		InstallmentNumber:  1,
		DueDate:            dueDate,
		Amount:             round2(principal + interest),
		PrincipalComponent: principal,
		InterestComponent:  interest,
		Status:             models.EmiStatusPending,
	}
	return database.DB.Create(&installment).Error
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
