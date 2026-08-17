package loans

import (
	"os"
	"strings"
)

// ProcessingFeeRatePercent is applied to the sanctioned amount to derive
// the base (pre-GST) processing fee. No product/tier-specific rate exists
// in this codebase yet, so a single flat rate is used everywhere a
// processing fee is quoted or charged.
const ProcessingFeeRatePercent = 2.0

// GSTRatePercent is the total GST rate on NBFC processing fees (financial
// services), split either as CGST 9% + SGST 9% (intra-state) or IGST 18%
// (inter-state) — never both, and never added on top of each other.
const GSTRatePercent = 18.0
const halfGSTRatePercent = GSTRatePercent / 2 // 9% each for CGST/SGST

const (
	GSTTypeCGSTSGST = "CGST_SGST"
	GSTTypeIGST     = "IGST"
)

// companyState returns the NBFC's own registered-office state — the
// "place of supply" reference point for the intra-state vs inter-state
// comparison. Configurable via COMPANY_STATE for deployments in a
// different state; defaults to Maharashtra, matching the registered
// office address already hardcoded in letter.go's lenderOfficeLine.
func companyState() string {
	if v := strings.TrimSpace(os.Getenv("COMPANY_STATE")); v != "" {
		return v
	}
	return "Maharashtra"
}

// GSTBreakdown holds every figure needed to show GST as its own line
// items rather than folding it into the processing fee or principal.
type GSTBreakdown struct {
	ProcessingFeeBase float64
	CGST              float64
	SGST              float64
	IGST              float64
	GSTTotal          float64
	Total             float64 // ProcessingFeeBase + GSTTotal
	GSTType           string  // GSTTypeCGSTSGST | GSTTypeIGST
	CustomerState     string
	IsIntraState      bool
}

// calculateProcessingFeeGST computes the processing fee on sanctionedAmount
// and its GST split, comparing customerState against the NBFC's own
// registered-office state (case/whitespace-insensitive — state names come
// from free-text KYC address fields, not a fixed enum). An empty
// customerState can't be classified as same-state or different-state, so
// it defaults to inter-state (IGST) — the safer assumption, since charging
// CGST+SGST for a customer outside the home state would be a genuine
// compliance error, while IGST is always valid regardless of which state
// the customer is actually in.
func calculateProcessingFeeGST(sanctionedAmount float64, customerState string) GSTBreakdown {
	base := roundToPaise(sanctionedAmount * ProcessingFeeRatePercent / 100)

	isIntraState := customerState != "" && strings.EqualFold(strings.TrimSpace(customerState), companyState())

	b := GSTBreakdown{
		ProcessingFeeBase: base,
		CustomerState:     customerState,
		IsIntraState:      isIntraState,
	}

	if isIntraState {
		b.GSTType = GSTTypeCGSTSGST
		b.CGST = roundToPaise(base * halfGSTRatePercent / 100)
		b.SGST = roundToPaise(base * halfGSTRatePercent / 100)
	} else {
		b.GSTType = GSTTypeIGST
		b.IGST = roundToPaise(base * GSTRatePercent / 100)
	}

	b.GSTTotal = roundToPaise(b.CGST + b.SGST + b.IGST)
	b.Total = roundToPaise(b.ProcessingFeeBase + b.GSTTotal)
	return b
}

// roundToPaise rounds to 2 decimal places (nearest paisa) — currency
// values shouldn't carry raw floating-point noise into stored fields or
// printed documents.
func roundToPaise(v float64) float64 {
	return float64(int64(v*100+0.5)) / 100
}
