// Package kyc implements the backend-authoritative KYC state machine
// (Phase 3). No customer-facing endpoint can set a customer's status to
// VERIFIED — Recompute derives it from the underlying verification rows,
// and only Approve (admin, KYC_APPROVE permission) can move a
// VERIFICATION_PENDING/REQUIRES_REVIEW record to VERIFIED.
package kyc

import (
	"log"
	"time"

	"github.com/google/uuid"

	"github.com/santosh0921/nbfc-backend/internal/audit"
	"github.com/santosh0921/nbfc-backend/internal/bureau"
	"github.com/santosh0921/nbfc-backend/internal/config"
	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/models"
)

// GetOrCreate returns the customer's CustomerKYC row, creating a fresh
// NOT_STARTED one if none exists yet.
func GetOrCreate(customerID uuid.UUID) (*models.CustomerKYC, error) {
	var record models.CustomerKYC
	err := database.DB.Where("customer_id = ?", customerID).First(&record).Error
	if err == nil {
		return &record, nil
	}
	record = models.CustomerKYC{
		CustomerID:        customerID,
		Status:            models.KYCStatusNotStarted,
		VerificationLevel: models.KYCLevelBasic,
	}
	if err := database.DB.Create(&record).Error; err != nil {
		return nil, err
	}
	return &record, nil
}

// Recompute derives the customer's KYC status from every underlying
// verification row and persists it — called after any event that could
// change completeness or verification outcome (profile/address save, PAN
// or Aadhaar or selfie submission, admin KYC review). This is the ONLY
// function that writes CustomerKYC.Status, and it never sets VERIFIED
// purely from automated checks passing — VERIFIED additionally requires
// review.Status == Approved (internal/admin/kyc_review_handler.go, an
// admin action gated by KYC_APPROVE), exactly preserving the existing
// "admin has the final say" behavior CustomerKYCReview already
// implemented, just formalized into this state machine.
func Recompute(customerID uuid.UUID, actorType, actorRef string) (*models.CustomerKYC, error) {
	record, err := GetOrCreate(customerID)
	if err != nil {
		return nil, err
	}
	before := audit.Snapshot(record)

	var profile models.CustomerProfile
	hasProfile := database.DB.Where("user_id = ?", customerID).First(&profile).Error == nil
	var address models.CustomerAddress
	hasAddress := database.DB.Where("user_id = ?", customerID).First(&address).Error == nil

	var pan models.CustomerPAN
	hasPAN := database.DB.Where("user_id = ?", customerID).First(&pan).Error == nil
	var aadhaar models.CustomerAadhaar
	hasAadhaar := database.DB.Where("user_id = ?", customerID).First(&aadhaar).Error == nil
	var selfie models.CustomerSelfie
	hasSelfie := database.DB.Where("user_id = ?", customerID).First(&selfie).Error == nil

	var review models.CustomerKYCReview
	hasReview := database.DB.Where("user_id = ?", customerID).First(&review).Error == nil

	documentsSubmitted := 0
	if hasPAN {
		documentsSubmitted++
	}
	if hasAadhaar {
		documentsSubmitted++
	}
	if hasSelfie {
		documentsSubmitted++
	}
	const requiredDocuments = 3

	allProviderVerified := hasPAN && pan.IsVerified && hasAadhaar && aadhaar.IsVerified && hasSelfie && selfie.IsVerified
	anyProviderFailed := (hasPAN && !pan.IsVerified && pan.VerificationStatus != "" && pan.VerificationStatus != "PENDING") ||
		(hasAadhaar && !aadhaar.IsVerified && aadhaar.VerificationStatus == "REJECTED")

	newStatus := models.KYCStatusNotStarted
	failureReason := ""
	reviewReason := ""

	switch {
	case hasReview && review.Status == "REJECTED":
		newStatus = models.KYCStatusRejected
		failureReason = review.RejectionReason
	case hasReview && review.Status == "APPROVED" && documentsSubmitted == requiredDocuments:
		// Admin approval is authoritative once all three documents are
		// actually on file — this no longer additionally requires each
		// provider's own IsVerified flag. Aadhaar and Selfie have no
		// automated provider configured in this deployment by design
		// (every submission already routes to manual review — see the
		// KYC portal's own case-review page copy), so allProviderVerified
		// could never become true for them and this branch was previously
		// dead code: an admin could click Approve and nothing would ever
		// happen. A KYC_APPROVE admin who has reviewed all three submitted
		// documents and explicitly approves IS the verification for the
		// providers that have none.
		newStatus = models.KYCStatusVerified
	case allProviderVerified && (!hasReview || review.Status == "PENDING"):
		// Every automated check passed but an admin hasn't signed off yet
		// — this is exactly the "documents pending" -> "verification
		// pending" -> only-an-admin-can-finalize boundary the brief asks
		// for: automated systems alone can never reach VERIFIED.
		newStatus = models.KYCStatusVerificationPending
	case anyProviderFailed:
		newStatus = models.KYCStatusRequiresReview
		reviewReason = "One or more provider verifications failed or returned a mismatch — needs manual compliance review"
	case documentsSubmitted > 0 && documentsSubmitted < requiredDocuments:
		newStatus = models.KYCStatusDocumentsPending
	case documentsSubmitted == requiredDocuments:
		newStatus = models.KYCStatusVerificationPending
	case hasProfile && hasAddress:
		newStatus = models.KYCStatusInitiated
	default:
		newStatus = models.KYCStatusNotStarted
	}

	justVerified := newStatus == models.KYCStatusVerified && record.Status != models.KYCStatusVerified

	record.Status = newStatus
	record.FailureReason = failureReason
	record.ReviewReason = reviewReason
	if newStatus == models.KYCStatusVerified {
		now := time.Now()
		record.VerifiedAt = &now
		record.VerifiedBy = review.ReviewedBy
		record.Provider = "ADMIN_APPROVAL"
	}

	if err := database.DB.Save(record).Error; err != nil {
		return nil, err
	}

	audit.RecordDetailed(nil, audit.Entry{
		ActorType: actorType, ActorRef: actorRef,
		Action: "kyc.status_recomputed", TargetType: "customer_kyc", TargetID: customerID.String(),
		Result: audit.ResultSuccess, Details: before, After: record,
	})

	// The first time (and only the first time — Recompute runs on every
	// KYC-adjacent event, not just approval) a customer crosses into
	// VERIFIED, hand them off to the credit bureau automatically instead
	// of waiting for a loan application to trigger the pull (previously
	// the ONLY trigger — internal/loans.getOrCreateCibilScore, called at
	// apply-time). This mirrors that exact same sandbox-consent pattern.
	// Best-effort: a bureau failure must never undo or block the KYC
	// approval that already succeeded above — in production, where no
	// customer-facing consent-capture screen exists yet (see that
	// function's comment), this will simply fail closed with no score,
	// exactly like the loan-apply path already does today.
	if justVerified {
		go triggerBureauCheckAfterApproval(customerID)
	}

	return record, nil
}

// triggerBureauCheckAfterApproval runs the bureau pull off the request
// goroutine — PullScore makes a real (sandboxed outside production)
// provider call and there's no reason the admin's "Approve" click should
// block on it. Database/bureau errors are logged, not surfaced anywhere,
// since by this point the KYC approval itself has already been
// committed; a failed bureau pull just leaves the customer without a
// score yet, the same recoverable state they'd be in if they applied for
// a loan and the pull failed there.
func triggerBureauCheckAfterApproval(customerID uuid.UUID) {
	bureauName := models.BureauCIBIL
	if !config.IsProduction() {
		bureauName = models.BureauSandbox
		if _, err := bureau.RecordConsent(customerID, bureauName, "KYC approval — credit bureau check", "v0-implicit-preconsent-flow", "", "", ""); err != nil {
			log.Printf("kyc: post-approval bureau consent record failed for customer %s: %v", customerID, err)
			return
		}
	}
	if _, err := bureau.PullScore(customerID, bureauName); err != nil {
		log.Printf("kyc: post-approval bureau pull failed for customer %s: %v", customerID, err)
	}
}

// IsSubmissionEligible implements the "KYC completeness" gate loan
// submission checks (internal/loans, KYCEligibilityService) — see that
// package's doc comment for why this is completeness, not full
// provider-VERIFIED status: requiring third-party-verified KYC before a
// customer can even submit an application would deadlock every
// application in any environment without a live Aadhaar verifier (see
// internal/verification/aadhaar.go), which is the current state of this
// deployment. What IS enforced: every required document must actually be
// on file and not in a rejected state — a customer who submitted nothing,
// or whose KYC was rejected, cannot apply.
func IsSubmissionEligible(status string) (eligible bool, reason string) {
	switch status {
	case models.KYCStatusVerificationPending, models.KYCStatusVerified:
		return true, ""
	case models.KYCStatusRequiresReview:
		return false, "Your KYC needs manual compliance review before you can apply — one of your submitted documents did not verify cleanly. Please contact support."
	case models.KYCStatusRejected:
		return false, "Your KYC was rejected. Please review the reason in your profile and resubmit your documents before applying."
	case models.KYCStatusDocumentsPending:
		return false, "Please complete your KYC — some required documents are still missing."
	case models.KYCStatusInitiated:
		return false, "Please submit your PAN, Aadhaar, and photo to complete KYC before applying for a loan."
	default:
		return false, "Please complete your profile and KYC before applying for a loan."
	}
}
