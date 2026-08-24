// Package kyc implements the backend-authoritative KYC state machine
// (Phase 3). No customer-facing endpoint can set a customer's status to
// VERIFIED — Recompute derives it from the underlying verification rows,
// and only Approve (admin, KYC_APPROVE permission) can move a
// VERIFICATION_PENDING/REQUIRES_REVIEW record to VERIFIED.
package kyc

import (
	"time"

	"github.com/google/uuid"

	"github.com/santosh0921/nbfc-backend/internal/audit"
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
	case hasReview && review.Status == "APPROVED" && allProviderVerified:
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

	return record, nil
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
