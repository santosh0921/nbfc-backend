package kyc

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/santosh0921/nbfc-backend/internal/audit"
	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/models"
)

func currentUser(c *gin.Context) (*models.User, bool) {
	mobile := c.GetString("mobile")
	if mobile == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"message": "Unauthorized"})
		return nil, false
	}
	var user models.User
	if err := database.DB.Where("mobile = ?", mobile).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"message": "User not found"})
		return nil, false
	}
	return &user, true
}

// CustomerStatusHandler handles GET /auth/kyc/v2/status — the
// state-machine-backed status, additive alongside the existing
// GET /auth/kyc/status (internal/profile/kyc_status_handler.go, left
// unchanged so the shipped customer app's existing contract keeps
// working). Recomputes fresh from the underlying verification rows on
// every call — see Recompute's doc comment for why this is read-time
// computation rather than eagerly cached.
func CustomerStatusHandler(c *gin.Context) {
	user, ok := currentUser(c)
	if !ok {
		return
	}
	record, err := Recompute(user.ID, "customer", user.Mobile)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Failed to compute KYC status"})
		return
	}
	c.JSON(http.StatusOK, record)
}

// AdminQueueHandler handles GET /admin/kyc/queue — requires KYC_VIEW.
// Every customer whose most-recently-computed status needs an admin's
// attention (VERIFICATION_PENDING or REQUIRES_REVIEW) — the "KYC
// Verification Queue" admin panel screen.
func AdminQueueHandler(c *gin.Context) {
	statusFilter := c.Query("status")
	q := database.DB.Model(&models.CustomerKYC{})
	if statusFilter != "" {
		q = q.Where("status = ?", statusFilter)
	} else {
		q = q.Where("status IN ?", []string{models.KYCStatusVerificationPending, models.KYCStatusRequiresReview})
	}
	var records []models.CustomerKYC
	q.Order("updated_at asc").Find(&records)
	c.JSON(http.StatusOK, records)
}

// AdminDetailHandler handles GET /admin/kyc/:customerId — requires
// KYC_VIEW. Recomputes fresh (an admin reviewing a queue entry should see
// the current state, not a possibly-stale cached row) and also returns
// document access is via the existing
// GET /admin/customers/:id/documents endpoint (internal/documents),
// unchanged — this handler is status/decision only, not a document list,
// to avoid duplicating that endpoint's own audit-logged access record.
func AdminDetailHandler(c *gin.Context) {
	customerID, err := uuid.Parse(c.Param("customerId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid customer id"})
		return
	}
	record, err := Recompute(customerID, "admin", adminActor(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Failed to compute KYC status"})
		return
	}
	c.JSON(http.StatusOK, record)
}

type decisionRequest struct {
	Reason string `json:"reason"`
}

// AdminApproveHandler handles POST /admin/kyc/:customerId/approve —
// requires KYC_APPROVE. This is a thin wrapper over the existing
// admin.ApproveKYCHandler (internal/admin/kyc_review_handler.go, which
// writes CustomerKYCReview) followed by Recompute, so the new state
// machine and the pre-existing review table never drift out of sync —
// deliberately NOT a second, competing source of truth.
func AdminApproveHandler(c *gin.Context) {
	setReviewAndRecompute(c, "APPROVED", "")
}

func AdminRejectHandler(c *gin.Context) {
	var body decisionRequest
	c.ShouldBindJSON(&body)
	if body.Reason == "" {
		c.JSON(http.StatusBadRequest, gin.H{"message": "A rejection reason is required"})
		return
	}
	setReviewAndRecompute(c, "REJECTED", body.Reason)
}

// AdminRequireReviewHandler handles
// POST /admin/kyc/:customerId/require-review — flags a customer's KYC as
// needing a closer manual look without outright rejecting it (e.g. a
// borderline name-match, a suspicious document) — distinct from REJECTED
// so the customer isn't told to resubmit everything when what's actually
// needed is a compliance officer's judgment call.
func AdminRequireReviewHandler(c *gin.Context) {
	customerID, err := uuid.Parse(c.Param("customerId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid customer id"})
		return
	}
	var body decisionRequest
	c.ShouldBindJSON(&body)

	record, err := GetOrCreate(customerID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Database error"})
		return
	}
	before := audit.Snapshot(record)
	record.Status = models.KYCStatusRequiresReview
	record.ReviewReason = body.Reason
	database.DB.Save(record)

	audit.RecordDetailed(c, audit.Entry{
		ActorType: "admin", ActorID: toAdminIDCtx(c), ActorRef: c.GetString("admin_username"), ActorRole: c.GetString("admin_role"),
		Action: "kyc.require_review", TargetType: "customer_kyc", TargetID: customerID.String(),
		Result: audit.ResultSuccess, Reason: body.Reason, Details: before, After: record,
	})
	c.JSON(http.StatusOK, record)
}

func setReviewAndRecompute(c *gin.Context, status, reason string) {
	customerID, err := uuid.Parse(c.Param("customerId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid customer id"})
		return
	}

	var review models.CustomerKYCReview
	if err := database.DB.Where("user_id = ?", customerID).First(&review).Error; err != nil {
		review = models.CustomerKYCReview{UserID: customerID}
	}
	review.Status = status
	review.ReviewedBy = c.GetString("admin_username")
	review.RejectionReason = reason
	if err := database.DB.Save(&review).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Failed to save review"})
		return
	}

	record, err := Recompute(customerID, "admin", adminActor(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Failed to recompute KYC status"})
		return
	}

	audit.RecordDetailed(c, audit.Entry{
		ActorType: "admin", ActorID: toAdminIDCtx(c), ActorRef: c.GetString("admin_username"), ActorRole: c.GetString("admin_role"),
		Action: "kyc.decision." + status, TargetType: "customer_kyc", TargetID: customerID.String(),
		Result: audit.ResultSuccess, Reason: reason, After: record,
	})
	c.JSON(http.StatusOK, record)
}

func adminActor(c *gin.Context) string {
	if v := c.GetString("admin_username"); v != "" {
		return v
	}
	return "system"
}

func toAdminIDCtx(c *gin.Context) uint {
	if v, ok := c.Get("admin_id"); ok {
		if id, ok := v.(uint); ok {
			return id
		}
	}
	return 0
}
