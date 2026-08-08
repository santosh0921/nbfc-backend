package verification

import (
	"strings"

	"github.com/santosh0921/nbfc-backend/internal/constants"
)

func VerifyPAN(panNumber string, name string) VerificationResult {

	// Temporary Internal Sandbox

	if strings.ToUpper(name) == "SANJAY MAHATO" {

		return VerificationResult{
			IsVerified:  true,
			Status:      constants.VerificationVerified,
			Provider:    "INTERNAL_SANDBOX",
			Reference:   "PAN-TEST-001",
			Remarks:     "Verified successfully in sandbox",
			NameMatched: true,
		}
	}

	return VerificationResult{
		IsVerified:    false,
		Status:        constants.VerificationNameMismatch,
		Provider:      "INTERNAL_SANDBOX",
		Reference:     "PAN-TEST-001",
		FailureReason: "Name does not match PAN",
		Remarks:       "Sandbox verification failed",
		NameMatched:   false,
	}
}