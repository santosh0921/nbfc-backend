package verification

import "github.com/santosh0921/nbfc-backend/internal/constants"

func VerifySelfie(image string) VerificationResult {

	return VerificationResult{
		IsVerified: true,

		Status: constants.VerificationVerified,

		Provider: "INTERNAL_SANDBOX",

		Reference: "SELFIE-TEST-001",

		Remarks: "Selfie verified successfully",
	}
}