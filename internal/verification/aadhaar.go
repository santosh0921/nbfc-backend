package verification

import (
	"strings"

	"github.com/santosh0921/nbfc-backend/internal/config"
	"github.com/santosh0921/nbfc-backend/internal/constants"
)

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

	// Temporary Sandbox

	if strings.ToUpper(name) == "SANJAY MAHATO" {

		return VerificationResult{
			IsVerified:  true,
			Status:      constants.VerificationVerified,
			Provider:    "INTERNAL_SANDBOX",
			Reference:   "AADHAAR-TEST-001",
			Remarks:     "Verified successfully in sandbox",
			NameMatched: true,
		}
	}

	return VerificationResult{
		IsVerified:    false,
		Status:        constants.VerificationNameMismatch,
		Provider:      "INTERNAL_SANDBOX",
		Reference:     "AADHAAR-TEST-001",
		FailureReason: "Name does not match Aadhaar",
		Remarks:       "Sandbox verification failed",
		NameMatched:   false,
	}
}