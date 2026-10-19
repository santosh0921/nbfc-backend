package loans

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"

	"github.com/google/uuid"

	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/models"
)

// PolicyVersion identifies the exact underwriting-rule set that produced
// a CreditDecision row (Phase 3, item H) — every threshold in this file
// (minLoanAmount, maxFOIRPercent, the CIBIL bands in rateForCibil) is this
// version's actual policy. Bump this string whenever any of those values
// changes so historical decisions remain attributable to the rules that
// were actually in force when they were made. Configurable only by
// editing this file today (a DB-backed, admin-editable policy table is a
// natural next step, out of scope for this phase — see the Phase 3
// report).
const PolicyVersion = "underwriting-v1"

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

// Instant Loan — a distinct short-term product, not just a Category label
// on the regular EMI product. Single lump-sum repayment 35 days after
// disbursement, a flat one-time fee instead of reducing-balance monthly
// interest, and its own (smaller) amount band — none of the regular
// min/maxLoanAmount, min/maxTenureMonths, or FOIR rules apply to it. See
// evaluateInstantLoanEligibility and generateEmiSchedule's branch in
// emi_service.go for where this actually takes effect.
const (
	instantLoanCategory       = "Instant Loan"
	instantLoanMinAmount      = 1000.0
	instantLoanMaxAmount      = 50000.0
	instantLoanTenureDays     = 35
	instantLoanFlatFeePercent = 8.0
)

// isInstantLoanCategory matches case-insensitively since Category is a
// free-text field the client supplies, not a constrained enum.
func isInstantLoanCategory(category string) bool {
	return strings.EqualFold(category, instantLoanCategory)
}

const goldLoanCategory = "Gold Loan"

// isGoldLoanCategory — same case-insensitive matching as isInstantLoanCategory.
// Gold Loan uses the regular EMI eligibility rules (min/max amount, tenure,
// FOIR) at apply time — what differs is repayment shape, not eligibility —
// see generateGoldLoanInstallments in emi_service.go.
func isGoldLoanCategory(category string) bool {
	return strings.EqualFold(category, goldLoanCategory)
}

const farmerLoanCategory = "Farmer Loan"

// isFarmerLoanCategory — same case-insensitive matching. Farmer Loan also
// uses the regular EMI eligibility rules at apply time (see
// isGoldLoanCategory's comment) — only repayment shape differs, handled
// in generateFarmerLoanInstallment in emi_service.go.
func isFarmerLoanCategory(category string) bool {
	return strings.EqualFold(category, farmerLoanCategory)
}

// InstantLoanEligibilityResult mirrors EligibilityResult's shape closely
// enough to reuse the same JSON response handling in SubmitLoanHandler,
// but with fields that actually mean something for a flat-fee, fixed-term
// product (no monthly EMI, no annual rate).
type InstantLoanEligibilityResult struct {
	Eligible       bool
	Message        string
	FlatFeePercent float64
	TotalPayable   float64
	DueInDays      int
}

// evaluateInstantLoanEligibility is evaluateEligibility's counterpart for
// the Instant Loan product: same CIBIL-reject gate as every other loan
// (item G/underwriting policy still applies to who gets approved at all),
// but its own amount band and no FOIR/tenure checks — a 35-day, capped-at-
// ₹50,000 obligation is a fundamentally different affordability shape than
// a 5-year EMI, so reusing the monthly-FOIR math here would either reject
// good customers or approve genuinely unaffordable ones for the wrong
// reason.
func evaluateInstantLoanEligibility(amount float64, cibil int) InstantLoanEligibilityResult {
	if amount < instantLoanMinAmount || amount > instantLoanMaxAmount {
		return InstantLoanEligibilityResult{
			Eligible: false,
			Message:  fmt.Sprintf("Instant Loan amount must be between ₹%s and ₹%s.", formatAmount(instantLoanMinAmount), formatAmount(instantLoanMaxAmount)),
		}
	}
	if _, ok := rateForCibil(cibil); !ok {
		return InstantLoanEligibilityResult{
			Eligible: false,
			Message:  "Based on your credit profile, this application can't be processed right now.",
		}
	}

	totalPayable := round2(amount * (1 + instantLoanFlatFeePercent/100))
	return InstantLoanEligibilityResult{
		Eligible:       true,
		FlatFeePercent: instantLoanFlatFeePercent,
		TotalPayable:   totalPayable,
		DueInDays:      instantLoanTenureDays,
	}
}

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

// recordEligibilityDecision writes the CreditDecision audit trail row for
// an apply-time (or top-up-time) eligibility check — separate from the
// bureau pull that fed it (internal/bureau.PullScore already records its
// own BureauRequest) and separate from the sanction-time decision
// recorded in maker_checker_handler.go. loanID is nil when the
// application was rejected before a LoanApplication row ever existed.
// recordInstantLoanEligibilityDecision is recordEligibilityDecision's
// counterpart for the Instant Loan product, so its apply-time decisions
// show up in the same Credit Decision audit trail as every other loan's.
func recordInstantLoanEligibilityDecision(customerID uuid.UUID, loanID uint, result InstantLoanEligibilityResult, cibil int, amount float64) {
	decision := "ELIGIBLE"
	reasonCodes := ""
	if !result.Eligible {
		decision = "REJECTED"
		if amount < instantLoanMinAmount || amount > instantLoanMaxAmount {
			reasonCodes = "INSTANT_LOAN_AMOUNT_OUT_OF_RANGE"
		} else {
			reasonCodes = "CIBIL_BELOW_THRESHOLD"
		}
	}

	inputs, _ := json.Marshal(map[string]interface{}{
		"amountRequested": amount, "cibilScore": cibil,
		"flatFeePercent": result.FlatFeePercent, "totalPayable": result.TotalPayable, "dueInDays": result.DueInDays,
	})

	var loanIDPtr *uint
	if loanID != 0 {
		loanIDPtr = &loanID
	}

	database.DB.Create(&models.CreditDecision{
		LoanID: loanIDPtr, CustomerID: customerID, PolicyVersion: PolicyVersion + "-instant", InputsJSON: string(inputs),
		Decision: decision, ReasonCodes: reasonCodes, ReviewerOrSystem: "system",
	})
}

func recordEligibilityDecision(loanID uint, result EligibilityResult, cibil int, amount, monthlyIncome float64) {
	decision := "ELIGIBLE"
	reasonCodes := ""
	if !result.Eligible {
		decision = "REJECTED"
		reasonCodes = classifyRejectionReason(amount, cibil, result)
	}

	inputs, _ := json.Marshal(map[string]interface{}{
		"amountRequested": amount, "monthlyIncome": monthlyIncome, "cibilScore": cibil,
		"ratePercent": result.RatePercent, "monthlyEmi": result.MonthlyEmi,
	})

	var loanIDPtr *uint
	if loanID != 0 {
		loanIDPtr = &loanID
	}

	database.DB.Create(&models.CreditDecision{
		LoanID: loanIDPtr, PolicyVersion: PolicyVersion, InputsJSON: string(inputs),
		Decision: decision, ReasonCodes: reasonCodes, ReviewerOrSystem: "system",
	})
}

// recordSanctionDecision writes the CreditDecision audit trail row for the
// sanction-time decision (AdminApproveSanctionHandler) — distinct from the
// apply-time recordEligibilityDecision above: here the loan already exists
// (LoanID is real, never nil) and the reviewer is the approving admin, not
// "system".
func recordSanctionDecision(loan models.LoanApplication, reasonCodes, adminUsername string) {
	inputs, _ := json.Marshal(map[string]interface{}{
		"amountRequested": loan.AmountRequested,
		"cibilScore":      loan.CibilScore,
		"tenureMonths":    loan.TenureMonths,
		"ratePercent":     loan.InterestRatePercent,
	})
	loanID := loan.ID
	database.DB.Create(&models.CreditDecision{
		LoanID: &loanID, CustomerID: loan.CustomerID, PolicyVersion: PolicyVersion,
		InputsJSON: string(inputs), Decision: "SANCTIONED", ReasonCodes: reasonCodes,
		ReviewerOrSystem: adminUsername,
	})
}

func classifyRejectionReason(amount float64, cibil int, result EligibilityResult) string {
	if cibil > 0 && cibil < models.CibilRejectThreshold {
		return "CIBIL_BELOW_THRESHOLD"
	}
	if amount < minLoanAmount || amount > maxLoanAmount {
		return "AMOUNT_OUT_OF_RANGE"
	}
	if result.MaxEligibleAmount > 0 {
		return "FOIR_EXCEEDED"
	}
	return "OTHER"
}
