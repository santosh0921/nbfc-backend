// Package screening implements the PEP/sanctions screening abstraction
// (Phase 3, item K). No screening vendor (e.g. a watchlist API) is
// configured in this environment, so Screen always fails safe to
// REVIEW_REQUIRED rather than CLEAR — an unconfigured environment must
// never let a customer through screening unchecked. A POTENTIAL_MATCH from
// a real provider is likewise never auto-treated as confirmed: both
// outcomes land in the same compliance review queue, and only
// Decide (gated to rbac.ScreeningDecide, i.e. compliance-tier roles) can
// resolve one.
package screening

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/santosh0921/nbfc-backend/internal/audit"
	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/models"
)

// Result is what a real Provider implementation would return.
type Result struct {
	Outcome     string // CLEAR | POTENTIAL_MATCH
	Reference   string
}

// Provider is the seam a real PEP/sanctions vendor plugs into.
type Provider interface {
	Name() string
	Screen(customerID uuid.UUID, fullName string) (Result, error)
}

var activeProvider Provider

func Initialize() {
	// No screening vendor configured in this environment. activeProvider
	// stays nil — Screen always produces REVIEW_REQUIRED, never CLEAR, in
	// that case. See package doc comment.
}

// Screen runs (or fails-safe-defaults) a screening check and always writes
// a ScreeningRequest row plus an audit event.
func Screen(customerID uuid.UUID, fullName string) (*models.ScreeningRequest, error) {
	req := models.ScreeningRequest{
		CustomerID: customerID,
		ScreenedAt: time.Now(),
		Decision:   models.ScreeningDecisionPending,
	}

	if activeProvider == nil {
		req.Provider = "NONE_CONFIGURED"
		req.Result = models.ScreeningResultReviewRequired
	} else {
		result, err := activeProvider.Screen(customerID, fullName)
		req.Provider = activeProvider.Name()
		if err != nil {
			// A provider error is not a clean bill of health — fail safe
			// to REVIEW_REQUIRED, same as no provider configured at all.
			req.Result = models.ScreeningResultReviewRequired
			req.Reference = err.Error()
		} else if result.Outcome == models.ScreeningResultClear {
			req.Result = models.ScreeningResultClear
			req.Reference = result.Reference
		} else {
			// POTENTIAL_MATCH (or anything else) never becomes CLEAR here —
			// it always routes to compliance review.
			req.Result = models.ScreeningResultReviewRequired
			req.Reference = result.Reference
		}
	}

	database.DB.Create(&req)
	audit.RecordDetailed(nil, audit.Entry{
		ActorType: "system", ActorRef: req.Provider,
		Action: "screening.request", TargetType: "screening_request", TargetID: req.ID.String(),
		Result: audit.ResultSuccess, Details: "customer " + customerID.String() + " -> " + req.Result,
	})

	return &req, nil
}

// Decide records a compliance officer's resolution of a screening result.
// The caller (HTTP handler) is responsible for enforcing
// rbac.ScreeningDecide before calling this — this function does not
// re-check RBAC itself, matching the pattern of every other admin
// write path in this codebase (the permission gate lives in the route/
// middleware layer, not duplicated in the service).
func Decide(c *gin.Context, requestID uuid.UUID, adminID uint, adminRef, decision, reason string) (*models.ScreeningRequest, error) {
	var req models.ScreeningRequest
	if err := database.DB.First(&req, "id = ?", requestID).Error; err != nil {
		return nil, err
	}
	before := audit.Snapshot(req)
	now := time.Now()
	req.ReviewerAdminID = adminID
	req.ReviewerRef = adminRef
	req.Decision = decision
	req.DecisionReason = reason
	req.DecidedAt = &now
	if err := database.DB.Save(&req).Error; err != nil {
		return nil, err
	}

	audit.RecordDetailed(c, audit.Entry{
		ActorType: "admin", ActorID: adminID, ActorRef: adminRef,
		Action: "screening.decision", TargetType: "screening_request", TargetID: req.ID.String(),
		Result: audit.ResultSuccess, Reason: reason, Details: before, After: req,
	})
	return &req, nil
}
