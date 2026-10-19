package loans

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/santosh0921/nbfc-backend/internal/audit"
	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/models"
)

func currentAdminTriple(c *gin.Context) (uint, string, string) {
	id, _ := c.Get("admin_id")
	idVal, _ := id.(uint)
	return idVal, c.GetString("admin_username"), c.GetString("admin_role")
}

// AdminListTranchesHandler handles GET /admin/loans/:id/tranches — perm
// LOAN_VIEW. Every demand-letter cycle raised against this Home Loan so
// far, oldest first.
func AdminListTranchesHandler(c *gin.Context) {
	loanID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid loan id"})
		return
	}
	var tranches []models.LoanTranche
	database.DB.Where("loan_id = ?", loanID).Order("tranche_number asc").Find(&tranches)
	c.JSON(http.StatusOK, tranches)
}

type raiseTrancheRequest struct {
	ConstructionStageClaimed string  `json:"constructionStageClaimed" binding:"required"`
	RequestedAmount          float64 `json:"requestedAmount" binding:"required"`
	DemandLetterDocumentID   string  `json:"demandLetterDocumentId"`
	IsFinalTranche           bool    `json:"isFinalTranche"`
}

// AdminRaiseTrancheHandler handles POST /admin/loans/:id/tranches — perm
// LOAN_DISBURSE_PREPARE. Logs the builder's demand letter against this
// loan as a new tranche, in PENDING_SITE_VISIT — raising a tranche is not
// itself a money action, so it only needs the same permission the regular
// disbursement-prepare step needs, not a checker-level one.
func AdminRaiseTrancheHandler(c *gin.Context) {
	loanID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid loan id"})
		return
	}
	var loan models.LoanApplication
	if err := database.DB.First(&loan, loanID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"message": "Loan not found"})
		return
	}
	if loan.Category != "Home Loan" && loan.Category != "Housing Loan" {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Tranches only apply to Home Loan"})
		return
	}
	if loan.Status != models.LoanStatusSanctioned && loan.Status != models.LoanStatusDisbursementProcessing {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Loan must be sanctioned before tranches can be raised (current status: " + loan.Status + ")"})
		return
	}

	var body raiseTrancheRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	if body.RequestedAmount <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"message": "requestedAmount must be positive"})
		return
	}

	var count int64
	database.DB.Model(&models.LoanTranche{}).Where("loan_id = ?", loanID).Count(&count)

	adminID, username, _ := currentAdminTriple(c)
	tr := models.LoanTranche{
		LoanID:                   uint(loanID),
		TrancheNumber:            int(count) + 1,
		IsFinalTranche:           body.IsFinalTranche,
		ConstructionStageClaimed: body.ConstructionStageClaimed,
		RequestedAmount:          body.RequestedAmount,
		RaisedByAdminID:          adminID,
		RaisedByUsername:         username,
		Status:                   models.TrancheStatusPendingSiteVisit,
	}
	if body.DemandLetterDocumentID != "" {
		tr.DemandLetterDocumentID = &body.DemandLetterDocumentID
	}
	if err := database.DB.Create(&tr).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Failed to raise tranche"})
		return
	}

	// First tranche ever raised moves the loan into the same
	// disbursement_processing status the regular path uses, so it shows
	// up alongside everything else mid-disbursement rather than still
	// looking merely "sanctioned".
	if loan.Status == models.LoanStatusSanctioned {
		loan.Status = models.LoanStatusDisbursementProcessing
		database.DB.Save(&loan)
	}

	audit.RecordDetailed(c, audit.Entry{
		ActorType: "admin", ActorID: adminID, ActorRef: username,
		Action: "loan.tranche.raised", TargetType: "loan_tranche", TargetID: strconv.Itoa(int(tr.ID)),
		Result: audit.ResultSuccess, Details: "loan #" + strconv.Itoa(loanID) + ", tranche #" + strconv.Itoa(tr.TrancheNumber),
	})
	c.JSON(http.StatusOK, tr)
}

type siteVisitRequest struct {
	Approved bool   `json:"approved"`
	Notes    string `json:"notes"`
	Reason   string `json:"reason"`
}

// AdminTrancheSiteVisitHandler handles POST /admin/tranches/:id/site-visit
// — perm LOAN_VIEW (any field-capable admin can record a visit; the money
// decision that follows still needs LOAN_DISBURSE_PREPARE/LOAN_DISBURSE).
// Confirms (or disputes) that the claimed construction stage actually
// matches reality before a single rupee moves — the fraud control real
// construction-linked lending relies on.
func AdminTrancheSiteVisitHandler(c *gin.Context) {
	trancheID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid tranche id"})
		return
	}
	var tr models.LoanTranche
	if err := database.DB.First(&tr, trancheID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"message": "Tranche not found"})
		return
	}
	if tr.Status != models.TrancheStatusPendingSiteVisit {
		c.JSON(http.StatusConflict, gin.H{"message": "This tranche is not awaiting a site visit (current status: " + tr.Status + ")"})
		return
	}

	var body siteVisitRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	if !body.Approved && body.Reason == "" {
		c.JSON(http.StatusBadRequest, gin.H{"message": "A reason is required when rejecting a site visit"})
		return
	}

	adminID, username, _ := currentAdminTriple(c)
	now := time.Now()
	before := audit.Snapshot(tr)

	tr.SiteVisitByAdminID = &adminID
	tr.SiteVisitBy = username
	tr.SiteVisitAt = &now
	tr.SiteVisitNotes = body.Notes
	if body.Approved {
		tr.Status = models.TrancheStatusVerified
	} else {
		tr.Status = models.TrancheStatusSiteRejected
		tr.SiteRejectReason = body.Reason
	}
	if err := database.DB.Save(&tr).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Failed to save site visit"})
		return
	}

	audit.RecordDetailed(c, audit.Entry{
		ActorType: "admin", ActorID: adminID, ActorRef: username,
		Action: "loan.tranche.site_visit", TargetType: "loan_tranche", TargetID: strconv.Itoa(int(tr.ID)),
		Result: audit.ResultSuccess, Details: before, After: tr,
	})
	c.JSON(http.StatusOK, tr)
}

type prepareTrancheRequest struct {
	Amount float64 `json:"amount" binding:"required"`
}

// AdminPrepareTrancheHandler handles POST /admin/tranches/:id/prepare —
// perm LOAN_DISBURSE_PREPARE. The maker side of the same release
// separation the regular disbursement path already enforces.
func AdminPrepareTrancheHandler(c *gin.Context) {
	trancheID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid tranche id"})
		return
	}
	var tr models.LoanTranche
	if err := database.DB.First(&tr, trancheID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"message": "Tranche not found"})
		return
	}
	if tr.Status != models.TrancheStatusVerified {
		c.JSON(http.StatusConflict, gin.H{"message": "Tranche must pass a site visit before it can be prepared (current status: " + tr.Status + ")"})
		return
	}
	var body prepareTrancheRequest
	if err := c.ShouldBindJSON(&body); err != nil || body.Amount <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"message": "A positive amount is required"})
		return
	}

	adminID, username, _ := currentAdminTriple(c)
	now := time.Now()
	tr.PreparedByAdminID = &adminID
	tr.PreparedBy = username
	tr.PreparedAt = &now
	tr.PreparedAmount = body.Amount
	tr.Status = models.TrancheStatusPrepared
	if err := database.DB.Save(&tr).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Failed to prepare tranche"})
		return
	}
	audit.RecordDetailed(c, audit.Entry{
		ActorType: "admin", ActorID: adminID, ActorRef: username,
		Action: "loan.tranche.prepared", TargetType: "loan_tranche", TargetID: strconv.Itoa(int(tr.ID)),
		Result: audit.ResultSuccess,
	})
	c.JSON(http.StatusOK, tr)
}

// AdminApproveTrancheHandler handles POST /admin/tranches/:id/approve —
// perm LOAN_DISBURSE. The checker side — a different admin than whoever
// prepared it, enforced here exactly like
// AdminApproveDisbursementHandler enforces it for a regular loan.
// Approving IS disbursing for a tranche (no separate confirm/reconcile
// step — tranches are a demo-scope simplification of the regular
// disbursement path's prepare→approve→confirm, same shortcut
// AdminDemoConfirmDisbursementHandler already represents for ordinary
// loans). If this is the final tranche, this is also the moment the loan
// itself becomes fully disbursed: EMI schedule and disbursement letter
// generate exactly as they would for any other loan.
func AdminApproveTrancheHandler(c *gin.Context) {
	trancheID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid tranche id"})
		return
	}
	var tr models.LoanTranche
	if err := database.DB.First(&tr, trancheID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"message": "Tranche not found"})
		return
	}
	if tr.Status != models.TrancheStatusPrepared {
		c.JSON(http.StatusConflict, gin.H{"message": "Tranche is not prepared (current status: " + tr.Status + ")"})
		return
	}

	adminID, username, _ := currentAdminTriple(c)
	if tr.PreparedByAdminID != nil && *tr.PreparedByAdminID == adminID {
		c.JSON(http.StatusForbidden, gin.H{"message": "The admin who prepared this tranche cannot also approve it (maker ≠ checker)."})
		return
	}

	var loan models.LoanApplication
	if err := database.DB.First(&loan, tr.LoanID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"message": "Loan not found"})
		return
	}

	now := time.Now()
	tr.ApprovedByAdminID = &adminID
	tr.ApprovedBy = username
	tr.ApprovedAt = &now
	tr.DisbursedAmount = tr.PreparedAmount
	tr.DisbursedAt = &now
	tr.UTR = "DEMO-TRANCHE-" + strconv.Itoa(int(tr.ID)) + "-" + strconv.FormatInt(now.Unix(), 10)
	tr.Status = models.TrancheStatusDisbursed
	if err := database.DB.Save(&tr).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Failed to approve tranche"})
		return
	}

	// Running total across every disbursed tranche so far — shown back on
	// the loan, same meaning DisbursedAmount already has for every other
	// category, just accumulated instead of set once.
	var totalDisbursed float64
	database.DB.Model(&models.LoanTranche{}).Where("loan_id = ? AND status = ?", loan.ID, models.TrancheStatusDisbursed).
		Select("COALESCE(SUM(disbursed_amount),0)").Scan(&totalDisbursed)
	loan.DisbursedAmount = totalDisbursed
	if loan.DisbursedAt == nil {
		loan.DisbursedAt = &now
	}

	finalized := false
	if tr.IsFinalTranche {
		loan.Status = models.LoanStatusDisbursed
		finalized = true
	}
	if err := database.DB.Save(&loan).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Failed to update loan"})
		return
	}

	if finalized {
		if err := generateEmiSchedule(&loan); err != nil {
			audit.RecordDetailed(c, audit.Entry{
				ActorType: "system", Action: "loan.tranche.emi_generation_failed", TargetType: "loan", TargetID: strconv.Itoa(int(loan.ID)),
				Result: audit.ResultFailure, Reason: err.Error(),
			})
		}
		notifyDisbursement(loan, totalDisbursed, now)
		if err := generateLoanLetter(loan, "Disbursement Letter"); err != nil {
			audit.RecordDetailed(c, audit.Entry{
				ActorType: "system", Action: "loan.tranche.letter_failed", TargetType: "loan", TargetID: strconv.Itoa(int(loan.ID)),
				Result: audit.ResultFailure, Reason: err.Error(),
			})
		}
	}

	audit.RecordDetailed(c, audit.Entry{
		ActorType: "admin", ActorID: adminID, ActorRef: username,
		Action: "loan.tranche.approved", TargetType: "loan_tranche", TargetID: strconv.Itoa(int(tr.ID)),
		Result: audit.ResultSuccess, Details: "disbursed " + strconv.FormatFloat(tr.DisbursedAmount, 'f', 2, 64) + (func() string {
			if finalized {
				return " (final tranche — loan now disbursed)"
			}
			return ""
		})(),
	})
	c.JSON(http.StatusOK, gin.H{"tranche": tr, "loanStatus": loan.Status, "loanDisbursedAmount": loan.DisbursedAmount})
}
