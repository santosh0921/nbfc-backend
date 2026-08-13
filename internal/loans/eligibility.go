package loans

import (
	"fmt"
	"math"

	"github.com/santosh0921/nbfc-backend/internal/models"
)

// Loan eligibility engine — before this existed, SubmitLoanHandler accepted
// literally any positive amountRequested with no relationship at all to the
// customer's income, tenure, or the interest rate they'd actually be
// offered. This ties amount + tenure + income + CIBIL-derived rate
// together into one affordability check at application time, using the
// exact same reducing-balance EMI formula (internal/loans/emi_service.go
// generateEmiInstallments) and the exact same CIBIL rate/reject rules
// already enforced at sanction time (AdminDecisionHandler) — so the
// number a customer sees in the apply flow is the same number the loan
// will actually carry if approved, not a decorative preview.
const (
	minLoanAmount   = 50000.0
	maxLoanAmount   = 5000000.0
	minTenureMonths = 12
	maxTenureMonths = 60

	// FOIR (Fixed Obligation to Income Ratio) cap — this EMI, on top of
	// whatever else the customer already owes, can't exceed this fraction
	// of declared monthly income. 50% is a standard conservative NBFC
	// retail-lending ceiling.
	maxFOIRPercent = 50.0
)

// EligibilityResult is what the apply flow needs to either proceed (with
// the real EMI/rate the customer will be offered) or show a specific,
// actionable rejection reason.
type EligibilityResult struct {
	Eligible          bool
	Message           string
	MonthlyEmi        float64
	RatePercent       float64
	TotalPayable      float64
	MaxEligibleAmount float64
}

// calculateEmi is the same standard reducing-balance formula as
// generateEmiInstallments in emi_service.go, factored out so the apply-time
// estimate and the actual post-sanction schedule are never allowed to
// drift apart into two different formulas.
func calculateEmi(principal, ratePercent float64, tenureMonths int) float64 {
	r := ratePercent / 12 / 100
	n := float64(tenureMonths)
	pow := math.Pow(1+r, n)
	return round2(principal * r * pow / (pow - 1))
}

// maxAffordablePrincipal inverts calculateEmi: given the largest EMI a
// customer can afford, what's the largest principal that produces it at
// this rate/tenure — used to tell a rejected customer a concrete number
// ("reduce to about ₹X") instead of just "no".
func maxAffordablePrincipal(maxEmi, ratePercent float64, tenureMonths int) float64 {
	r := ratePercent / 12 / 100
	n := float64(tenureMonths)
	pow := math.Pow(1+r, n)
	return round2(maxEmi * (pow - 1) / (r * pow))
}

// rateForCibil mirrors AdminDecisionHandler's sanction-time rule exactly
// (internal/loans/admin_handler.go) so the rate quoted at apply time is
// the rate that would actually be offered at sanction, not a guess. ok is
// false when the score is outright disqualifying.
func rateForCibil(cibil int) (rate float64, ok bool) {
	if cibil > 0 && cibil < models.CibilRejectThreshold {
		return 0, false
	}
	if cibil > models.CibilPrimeThreshold {
		return models.CibilMinRatePercent, true
	}
	// 650-750 inclusive, or no score on file yet — the conservative mid
	// rate is used for the affordability estimate either way.
	return models.CibilMidRatePercent, true
}

// evaluateEligibility is the single gate every loan amount/tenure combo
// must pass before a LoanApplication is created — called from both
// SubmitLoanHandler and TopUpLoanHandler.
func evaluateEligibility(amount float64, tenureMonths int, monthlyIncome float64, cibil int) EligibilityResult {
	if amount < minLoanAmount || amount > maxLoanAmount {
		return EligibilityResult{
			Eligible: false,
			Message:  fmt.Sprintf("Loan amount must be between ₹%s and ₹%s.", formatAmount(minLoanAmount), formatAmount(maxLoanAmount)),
		}
	}
	if tenureMonths < minTenureMonths || tenureMonths > maxTenureMonths {
		return EligibilityResult{
			Eligible: false,
			Message:  fmt.Sprintf("Tenure must be between %d and %d months.", minTenureMonths, maxTenureMonths),
		}
	}
	if monthlyIncome <= 0 {
		return EligibilityResult{
			Eligible: false,
			Message:  "Please enter your monthly income to calculate eligibility.",
		}
	}

	rate, ok := rateForCibil(cibil)
	if !ok {
		return EligibilityResult{
			Eligible: false,
			Message:  "Based on your credit profile, this application can't be processed right now.",
		}
	}

	emi := calculateEmi(amount, rate, tenureMonths)
	maxAffordableEmi := round2(monthlyIncome * maxFOIRPercent / 100)

	if emi > maxAffordableEmi {
		maxAmount := maxAffordablePrincipal(maxAffordableEmi, rate, tenureMonths)
		return EligibilityResult{
			Eligible:          false,
			MonthlyEmi:        emi,
			RatePercent:       rate,
			MaxEligibleAmount: maxAmount,
			Message: fmt.Sprintf(
				"This EMI (₹%s/month) would be more than %d%% of your declared monthly income. "+
					"Reduce the amount to about ₹%s, or choose a longer tenure, and try again.",
				formatAmount(emi), int(maxFOIRPercent), formatAmount(maxAmount),
			),
		}
	}

	return EligibilityResult{
		Eligible:     true,
		MonthlyEmi:   emi,
		RatePercent:  rate,
		TotalPayable: round2(emi * float64(tenureMonths)),
	}
}

func formatAmount(v float64) string {
	return fmt.Sprintf("%.0f", v)
}
