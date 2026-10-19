package kyc

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/santosh0921/nbfc-backend/internal/audit"
	"github.com/santosh0921/nbfc-backend/internal/corrections"
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

// queueRow adds two read-only, admin-UI-only conveniences to the raw
// CustomerKYC row: the customer's real name (so a queue/dashboard screen
// isn't stuck showing a bare UUID) and the distinct set of loan categories
// the customer has actually applied under (so cases can be grouped by loan
// type). Both are looked up from real rows — CustomerProfile and
// LoanApplication — never invented; a customer with no profile row or no
// loan applications yet simply gets an empty/omitted value, not a guess.
type queueRow struct {
	models.CustomerKYC
	CustomerName   string   `json:"customerName,omitempty"`
	LoanCategories []string `json:"loanCategories,omitempty"`
}

// AdminQueueHandler handles GET /admin/kyc/queue — requires KYC_VIEW.
// Every customer whose most-recently-computed status needs an admin's
// attention (VERIFICATION_PENDING or REQUIRES_REVIEW) — the "KYC
// Verification Queue" admin panel screen.
func AdminQueueHandler(c *gin.Context) {
	statusFilter := c.Query("status")
	q := database.DB.Model(&models.CustomerKYC{})
	switch {
	case statusFilter == "":
		q = q.Where("status IN ?", []string{models.KYCStatusVerificationPending, models.KYCStatusRequiresReview})
	case strings.Contains(statusFilter, ","):
		// Comma-separated list — lets the frontend ask for a broader
		// "everything not yet decided" view (e.g. Pending Verification
		// showing NOT_STARTED/INITIATED/DOCUMENTS_PENDING alongside
		// VERIFICATION_PENDING/REQUIRES_REVIEW) without a second endpoint.
		statuses := strings.Split(statusFilter, ",")
		for i := range statuses {
			statuses[i] = strings.TrimSpace(statuses[i])
		}
		q = q.Where("status IN ?", statuses)
	default:
		q = q.Where("status = ?", statusFilter)
	}
	var records []models.CustomerKYC
	q.Order("updated_at asc").Find(&records)

	if len(records) == 0 {
		c.JSON(http.StatusOK, []queueRow{})
		return
	}

	customerIDs := make([]uuid.UUID, 0, len(records))
	for _, r := range records {
		customerIDs = append(customerIDs, r.CustomerID)
	}

	// Batch-fetch names and loan categories for every customer in this page
	// in two queries total, not N+1 — this handler can be called on a
	// dashboard that renders on every page load.
	var profiles []models.CustomerProfile
	database.DB.Where("user_id IN ?", customerIDs).Find(&profiles)
	nameByCustomer := make(map[uuid.UUID]string, len(profiles))
	for _, p := range profiles {
		name := p.FirstName
		if p.LastName != "" {
			if name != "" {
				name += " "
			}
			name += p.LastName
		}
		if name != "" {
			nameByCustomer[p.UserID] = name
		}
	}

	var loans []models.LoanApplication
	database.DB.Select("customer_id", "category").Where("customer_id IN ?", customerIDs).Find(&loans)
	categoriesByCustomer := make(map[uuid.UUID]map[string]bool, len(customerIDs))
	for _, l := range loans {
		if l.Category == "" {
			continue
		}
		if categoriesByCustomer[l.CustomerID] == nil {
			categoriesByCustomer[l.CustomerID] = map[string]bool{}
		}
		categoriesByCustomer[l.CustomerID][l.Category] = true
	}

	out := make([]queueRow, 0, len(records))
	for _, r := range records {
		row := queueRow{CustomerKYC: r, CustomerName: nameByCustomer[r.CustomerID]}
		for cat := range categoriesByCustomer[r.CustomerID] {
			row.LoanCategories = append(row.LoanCategories, cat)
		}
		out = append(out, row)
	}
	c.JSON(http.StatusOK, out)
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
	// ReasonCode is the fixed dropdown selection the KYC portal shows on
	// reject (e.g. DOCUMENT_NOT_VISIBLE, CUSTOMER_UNCOOPERATIVE) — Reason
	// stays the free-text detail. Optional: a reject without a code still
	// works exactly as before, it just raises no CorrectionCase.
	ReasonCode string `json:"reasonCode"`
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
	if c.Writer.Status() != http.StatusOK {
		return // setReviewAndRecompute already wrote an error response
	}
	if body.ReasonCode != "" {
		if customerID, err := uuid.Parse(c.Param("customerId")); err == nil {
			adminID, username, role := toAdminIDCtx(c), c.GetString("admin_username"), c.GetString("admin_role")
			if _, err := corrections.Raise(models.CorrectionDomainKYC, customerID, "", body.ReasonCode, body.Reason, adminID, username, role); err != nil {
				// Best-effort: the KYC rejection itself already succeeded and
				// was already returned to the caller above — a failure to
				// raise the follow-up case must never look like the reject
				// failed.
				audit.RecordDetailed(c, audit.Entry{
					ActorType: "system", Action: "correction.raise_failed", TargetType: "customer_kyc", TargetID: customerID.String(),
					Result: audit.ResultFailure, Reason: err.Error(),
				})
			}
		}
	}
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

// AdminBackfillRecomputeHandler handles POST /admin/kyc/backfill-recompute
// — a one-time operational fix, not part of the normal request flow.
// Recompute previously only ran at loan-submission eligibility checks and
// admin decisions, so a customer who submitted PAN/Aadhaar/Selfie but
// never attempted a loan had a stale or missing CustomerKYC row — making
// them invisible to every admin-facing KYC queue even though they'd
// genuinely made progress. Document-submission handlers now call
// Recompute themselves going forward (see internal/profile), but that
// only fixes new activity; this walks every existing user once so
// already-submitted documents are reflected immediately instead of only
// on that customer's next action.
func AdminBackfillRecomputeHandler(c *gin.Context) {
	var userIDs []uuid.UUID
	if err := database.DB.Model(&models.User{}).Pluck("id", &userIDs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Database error"})
		return
	}

	updated, failed := 0, 0
	for _, id := range userIDs {
		if _, err := Recompute(id, "admin", adminActor(c)); err != nil {
			failed++
			continue
		}
		updated++
	}

	audit.RecordDetailed(c, audit.Entry{
		ActorType: "admin", ActorID: toAdminIDCtx(c), ActorRef: c.GetString("admin_username"), ActorRole: c.GetString("admin_role"),
		Action: "kyc.backfill_recompute", TargetType: "customer_kyc", TargetID: "all",
		Result: audit.ResultSuccess,
	})
	c.JSON(http.StatusOK, gin.H{"totalUsers": len(userIDs), "recomputed": updated, "failed": failed})
}
