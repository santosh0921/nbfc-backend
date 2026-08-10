package verification

import (
	"bytes"
	"encoding/base64"
	"image"
	_ "image/jpeg" // registers the JPEG decoder with the image package
	_ "image/png"  // registers the PNG decoder with the image package
	"strings"

	"github.com/santosh0921/nbfc-backend/internal/config"
	"github.com/santosh0921/nbfc-backend/internal/constants"
)

// minSelfieBytes is a coarse floor on decoded image size — real phone-camera
// selfies are, in practice, tens of KB or more even compressed; anything
// under this is almost certainly a placeholder/blank/corrupt image rather
// than a genuine photo.
const minSelfieBytes = 5 * 1024

// VerifySelfie used to ignore its input entirely and unconditionally
// return IsVerified=true — an empty string, a blank image, or somebody
// else's face all scored a full point toward KYC completion. There is no
// biometric liveness or face-match provider configured anywhere in this
// project (no API keys, no SDK), so a real face-match/liveness check
// cannot be fabricated here without lying about what was actually
// verified.
//
// What CAN be verified for real without a third-party API: that the
// submitted string actually decodes to a genuine, adequately-sized JPEG
// or PNG image, rather than empty/garbage/placeholder input. That much is
// enforced below. Passing that check does not mean the photo has been
// confirmed to be a live selfie of the account holder — it's marked
// PENDING for manual admin review, the same honest pattern used for
// Aadhaar (see aadhaar.go) until a real liveness/face-match provider is
// wired in.
func VerifySelfie(image64 string) VerificationResult {
	if config.DemoMode {
		return VerificationResult{
			IsVerified: true,
			Status:     constants.VerificationVerified,
			Provider:   "INTERNAL_SANDBOX",
			Reference:  "DEMO-MODE",
			Remarks:    "Auto-verified (DEMO_MODE)",
		}
	}

	trimmed := strings.TrimSpace(image64)
	// Tolerate a data: URL prefix (e.g. "data:image/jpeg;base64,...") —
	// the Flutter client may or may not include one depending on capture
	// path; only the payload after the comma is actual image data.
	if idx := strings.Index(trimmed, ","); idx != -1 && strings.HasPrefix(trimmed, "data:") {
		trimmed = trimmed[idx+1:]
	}

	decoded, err := base64.StdEncoding.DecodeString(trimmed)
	if err != nil {
		return invalidSelfie("Submitted image is not valid base64 data")
	}
	if len(decoded) < minSelfieBytes {
		return invalidSelfie("Submitted image is too small to be a genuine photo")
	}
	if _, format, err := image.Decode(bytes.NewReader(decoded)); err != nil || (format != "jpeg" && format != "png") {
		return invalidSelfie("Submitted data is not a valid JPEG or PNG image")
	}

	return VerificationResult{
		IsVerified: false,
		Status:     constants.VerificationPending,
		Provider:   "MANUAL_REVIEW",
		Reference:  "",
		Remarks:    "Image format validated; queued for manual admin review (no automated liveness/face-match provider configured)",
	}
}

func invalidSelfie(reason string) VerificationResult {
	return VerificationResult{
		IsVerified:    false,
		Status:        constants.VerificationFailed,
		Provider:      "MANUAL_REVIEW",
		FailureReason: reason,
		Remarks:       reason,
	}
}