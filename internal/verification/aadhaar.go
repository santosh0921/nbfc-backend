package verification

import (
	"github.com/santosh0921/nbfc-backend/internal/config"
	"github.com/santosh0921/nbfc-backend/internal/constants"
)

// VerifyAadhaar used to only ever pass for the literal name "SANJAY MAHATO"
// and NAME_MISMATCH-reject every other customer in non-demo mode — a
// leftover sandbox stub, not a real check (the name argument was compared
// against a hardcoded literal; the aadhaarNumber argument was never
// inspected at all).
//
// It has NOT been replaced with a real Surepass Aadhaar call: this
// project's configured Surepass token is authorized for PAN verification
// only — a live probe against
// /api/v1/aadhaar-validation/aadhaar-validation returned
// 401 "Your access token is not valid for this API/Scope" — so there is
// currently no real automated Aadhaar verification available to call.
// Fabricating one that always "succeeds" would just move the same
// demo-shortcut-in-production problem this defect report is about,
// dressed up to look like a real check.
//
// So, outside DEMO_MODE, every submission now goes to PENDING — an
// honest "not yet verified" state that queues for the existing admin KYC
// review workflow (internal/admin/kyc_review_handler.go) — instead of a
// name that happens to match a hardcoded literal (false accept) or a
// FAILED/NAME_MISMATCH status that wrongly implies the customer made a
// data-entry mistake in every other case (false rejection reason). Once
// a Surepass (or other) Aadhaar-verification scope is actually available
// on the account, replace the PENDING branch below with a real API call,
// mirroring VerifyPAN in internal/surepass/pan.go.
func VerifyAadhaar(aadhaarNumber string, name string) VerificationResult {

	if config.DemoMode {
		return VerificationResult{
			IsVerified:  true,
			Status:      constants.VerificationVerified,
			Provider:    "INTERNAL_SANDBOX",
			Reference:   "DEMO-MODE",
			Remarks:     "Auto-verified (DEMO_MODE)",
			NameMatched: true,
		}
	}

	return VerificationResult{
		IsVerified:  false,
		Status:      constants.VerificationPending,
		Provider:    "MANUAL_REVIEW",
		Reference:   "",
		Remarks:     "Automated Aadhaar verification is not currently available; queued for manual admin review",
		NameMatched: false,
	}
}