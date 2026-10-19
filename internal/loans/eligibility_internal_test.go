package loans

import "testing"

// Internal-package test (not loans_test) since evaluateInstantLoanEligibility
// and isInstantLoanCategory are unexported — pure logic, no DB needed.

func TestIsInstantLoanCategoryIsCaseInsensitive(t *testing.T) {
	cases := map[string]bool{
		"Instant Loan": true,
		"instant loan": true,
		"INSTANT LOAN": true,
		"Personal Loan": false,
		"":              false,
	}
	for input, want := range cases {
		if got := isInstantLoanCategory(input); got != want {
			t.Errorf("isInstantLoanCategory(%q) = %v, want %v", input, got, want)
		}
	}
}

func TestEvaluateInstantLoanEligibilityAmountBounds(t *testing.T) {
	// Below the ₹1,000 minimum.
	if r := evaluateInstantLoanEligibility(500, 780); r.Eligible {
		t.Fatal("₹500 is below the ₹1,000 Instant Loan minimum and must be rejected")
	}
	// Above the ₹50,000 maximum.
	if r := evaluateInstantLoanEligibility(60000, 780); r.Eligible {
		t.Fatal("₹60,000 is above the ₹50,000 Instant Loan maximum and must be rejected")
	}
	// Within range, good CIBIL.
	r := evaluateInstantLoanEligibility(10000, 780)
	if !r.Eligible {
		t.Fatalf("₹10,000 with a prime CIBIL score should be eligible, got: %s", r.Message)
	}
	if r.FlatFeePercent != instantLoanFlatFeePercent {
		t.Fatalf("expected flat fee %v%%, got %v%%", instantLoanFlatFeePercent, r.FlatFeePercent)
	}
	if r.TotalPayable != 10800 {
		t.Fatalf("expected total payable 10800 (10000 + 8%%), got %v", r.TotalPayable)
	}
	if r.DueInDays != 35 {
		t.Fatalf("expected due in 35 days, got %d", r.DueInDays)
	}
}

func TestEvaluateInstantLoanEligibilityRejectsBadCibil(t *testing.T) {
	// Below CibilRejectThreshold must be rejected regardless of amount —
	// the Instant Loan product still goes through the same underwriting
	// CIBIL gate as every other loan, only the amount/tenure/fee mechanics
	// differ.
	r := evaluateInstantLoanEligibility(5000, 500)
	if r.Eligible {
		t.Fatal("a disqualifying CIBIL score must reject an Instant Loan application too")
	}
}
