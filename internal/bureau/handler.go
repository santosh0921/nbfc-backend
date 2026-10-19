package bureau

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/santosh0921/nbfc-backend/internal/audit"
	"github.com/santosh0921/nbfc-backend/internal/corrections"
	"github.com/santosh0921/nbfc-backend/internal/customerinfo"
	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/models"
)

// bureauRequestRow adds the customer's real name (looked up from
// CustomerProfile, batched — never N+1, never fabricated) to the raw
// BureauRequest row, so an admin UI isn't stuck showing a bare customer
// UUID. Omitted when the customer has no profile row yet.
type bureauRequestRow struct {
	models.BureauRequest
	CustomerName string `json:"customerName,omitempty"`
}

// AdminListHandler handles GET /admin/bureau/requests — the "Credit Bureau
// Requests"/"Credit Bureau Results" admin panel screens (one list, the
// panel can filter client-side by status). Never returns RawResponse — the
// model doesn't even have that field persisted, only ResponseHash.
func AdminListHandler(c *gin.Context) {
	q := database.DB.Model(&models.BureauRequest{})
	if customerID := c.Query("customerId"); customerID != "" {
		q = q.Where("customer_id = ?", customerID)
	}
	var records []models.BureauRequest
	q.Order("requested_at desc").Limit(200).Find(&records)

	ids := make([]uuid.UUID, len(records))
	for i, r := range records {
		ids[i] = r.CustomerID
	}
	names := customerinfo.BatchNames(ids)

	out := make([]bureauRequestRow, len(records))
	for i, r := range records {
		out[i] = bureauRequestRow{BureauRequest: r, CustomerName: names[r.CustomerID]}
	}
	c.JSON(http.StatusOK, out)
}

type reviewRequestBody struct {
	Notes string `json:"notes"`
	// ReasonCode is only meaningful on a flag, not an approve — the fixed
	// dropdown the Credit portal shows (e.g. BUREAU_MISMATCH,
	// INCOME_PROOF_INSUFFICIENT). Optional: flagging without a code still
	// works exactly as before, it just raises no CorrectionCase.
	ReasonCode string `json:"reasonCode"`
}

// AdminApproveHandler handles POST /admin/bureau/requests/:id/approve —
// perm BUREAU_APPROVE. This is the gate internal/loans.
// requireApprovedBureauRequest checks before underwriting can recommend a
// sanction: a completed pull with a real score is not, on its own,
// enough for the loan to proceed — a credit officer holding this
// permission must explicitly sign off first.
func AdminApproveHandler(c *gin.Context) {
	approveOrFlag(c, models.BureauReviewStatusApproved, "bureau.request_approved")
}

// AdminFlagHandler handles POST /admin/bureau/requests/:id/flag — the
// other side of the same review: a credit officer who looked at the
// result and does NOT want underwriting to proceed records that
// explicitly too, rather than just leaving it silently unreviewed.
func AdminFlagHandler(c *gin.Context) {
	approveOrFlag(c, models.BureauReviewStatusFlagged, "bureau.request_flagged")
}

func approveOrFlag(c *gin.Context, newStatus, auditAction string) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid request id"})
		return
	}

	var req models.BureauRequest
	if err := database.DB.First(&req, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"message": "Bureau request not found"})
		return
	}
	if req.Status != models.BureauRequestStatusCompleted {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Only a completed bureau pull can be reviewed"})
		return
	}

	var body reviewRequestBody
	_ = c.ShouldBindJSON(&body)

	adminID, _ := c.Get("admin_id")
	username, _ := c.Get("admin_username")
	id32, _ := adminID.(uint)
	now := time.Now()

	before := audit.Snapshot(req)
	req.ReviewStatus = newStatus
	req.ReviewedByAdminID = id32
	req.ReviewedByUsername = toString(username)
	req.ReviewedAt = &now
	if err := database.DB.Save(&req).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Failed to save review"})
		return
	}

	audit.RecordDetailed(c, audit.Entry{
		ActorType: "admin", ActorID: id32, ActorRef: toString(username),
		Action: auditAction, TargetType: "bureau_request", TargetID: req.ID.String(),
		Result: audit.ResultSuccess, Details: before, After: req, Reason: body.Notes,
	})

	if newStatus == models.BureauReviewStatusFlagged && body.ReasonCode != "" {
		role, _ := c.Get("admin_role")
		if _, err := corrections.Raise(models.CorrectionDomainCredit, req.CustomerID, req.ID.String(), body.ReasonCode, body.Notes, id32, toString(username), toString(role)); err != nil {
			audit.RecordDetailed(c, audit.Entry{
				ActorType: "system", Action: "correction.raise_failed", TargetType: "bureau_request", TargetID: req.ID.String(),
				Result: audit.ResultFailure, Reason: err.Error(),
			})
		}
	}

	c.JSON(http.StatusOK, req)
}

func toString(v interface{}) string {
	s, _ := v.(string)
	return s
}
