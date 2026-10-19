// Package corrections implements the follow-up queue a KYC rejection or a
// flagged bureau result lands in: a CorrectionCase, worked by the
// Correction Portal, resolved by re-opening the original review.
package corrections

import (
	"time"

	"github.com/google/uuid"

	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/models"
)

// RecomputeKYC is wired to kyc.Recompute by internal/routes at startup.
// A direct import would create an import cycle: internal/kyc needs to
// call corrections.Raise on reject, and this package needs to call back
// into kyc.Recompute on resolve — this injection breaks that cycle while
// keeping both sides real (never a second KYC state machine).
var RecomputeKYC func(customerID uuid.UUID, actorType, actorRef string) error

// documentCompleteness counts how many of the 3 required KYC documents
// (PAN/Aadhaar/Selfie) exist for a customer — presence only, same
// "documentsSubmitted" count kyc.Recompute uses, not verification status.
// Queried directly rather than via the kyc package to avoid the import
// cycle described above RecomputeKYC.
func documentCompleteness(customerID uuid.UUID) int {
	n := 0
	if database.DB.Where("user_id = ?", customerID).First(&models.CustomerPAN{}).Error == nil {
		n++
	}
	if database.DB.Where("user_id = ?", customerID).First(&models.CustomerAadhaar{}).Error == nil {
		n++
	}
	if database.DB.Where("user_id = ?", customerID).First(&models.CustomerSelfie{}).Error == nil {
		n++
	}
	return n
}

// Raise creates a new CorrectionCase. Best-effort on the loan-category
// lookup — a customer with no loan yet (rare for a KYC rejection, which
// implies they at least tried to apply) simply gets no category, never a
// fabricated one. Never returns an error for a lookup miss, only for a
// real DB write failure, since this always runs right after a KYC
// reject/bureau flag has already succeeded and must not undo that.
func Raise(domain string, customerID uuid.UUID, sourceRef, reasonCode, reasonDetail string, raisedByAdminID uint, raisedByUsername, raisedByRole string) (*models.CorrectionCase, error) {
	var loan models.LoanApplication
	var loanID *uint
	var category string
	if err := database.DB.Where("customer_id = ?", customerID).Order("created_at desc").First(&loan).Error; err == nil {
		id := loan.ID
		loanID = &id
		category = loan.Category
	}

	completeness := 0
	if domain == models.CorrectionDomainKYC {
		completeness = documentCompleteness(customerID)
	}

	cc := models.CorrectionCase{
		CustomerID:             customerID,
		LoanID:                 loanID,
		LoanCategory:           category,
		Domain:                 domain,
		ReasonCode:             reasonCode,
		ReasonDetail:           reasonDetail,
		SourceRef:              sourceRef,
		Status:                 models.CorrectionStatusOpen,
		RaisedByAdminID:        raisedByAdminID,
		RaisedByUsername:       raisedByUsername,
		RaisedByRole:           raisedByRole,
		DocCompletenessAtRaise: completeness,
	}
	if err := database.DB.Create(&cc).Error; err != nil {
		return nil, err
	}
	return &cc, nil
}

// Resolve marks a case resolved and re-opens the originating review so it
// naturally reappears in the KYC Queue or Credit Bureau Queue:
//   - KYC: resets the customer's CustomerKYCReview back to PENDING (there
//     is exactly one per customer — see models.CustomerKYCReview's
//     uniqueIndex) and recomputes, so the real document-completeness state
//     decides what status they land at, not a hardcoded guess.
//   - CREDIT: resets the specific BureauRequest's ReviewStatus (identified
//     by SourceRef, since a customer can have several pulls over time)
//     back to PENDING_REVIEW.
func Resolve(cc *models.CorrectionCase, resolvedByAdminID uint, resolvedByUsername, notes string) error {
	now := time.Now()
	cc.Status = models.CorrectionStatusResolved
	cc.ResolutionNotes = notes
	cc.ResolvedByAdminID = &resolvedByAdminID
	cc.ResolvedByUsername = resolvedByUsername
	cc.ResolvedAt = &now
	if err := database.DB.Save(cc).Error; err != nil {
		return err
	}

	switch cc.Domain {
	case models.CorrectionDomainKYC:
		var review models.CustomerKYCReview
		if err := database.DB.Where("user_id = ?", cc.CustomerID).First(&review).Error; err == nil {
			review.Status = "PENDING"
			database.DB.Save(&review)
		}
		if RecomputeKYC != nil {
			_ = RecomputeKYC(cc.CustomerID, "admin", resolvedByUsername)
		}
	case models.CorrectionDomainCredit:
		if cc.SourceRef != "" {
			if id, err := uuid.Parse(cc.SourceRef); err == nil {
				database.DB.Model(&models.BureauRequest{}).Where("id = ?", id).
					Update("review_status", models.BureauReviewStatusPending)
			}
		}
	}
	return nil
}
