package loans

import (
	"log"
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm/clause"

	"github.com/santosh0921/nbfc-backend/internal/audit"
	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/disbursement"
	"github.com/santosh0921/nbfc-backend/internal/models"
)

// currentAdmin pulls the acting admin's identity out of context, set by
// middleware.AdminAuthMiddleware. Every handler in this file uses this
// instead of trusting anything client-supplied for "who is doing this."
func currentAdmin(c *gin.Context) (id uint, username, role string) {
	if v, ok := c.Get("admin_id"); ok {
		id, _ = v.(uint)
	}
	if v, ok := c.Get("admin_username"); ok {
		username, _ = v.(string)
	}
	if v, ok := c.Get("admin_role"); ok {
		role, _ = v.(string)
	}
	return
}

func toPaiseLoans(rupees float64) int64 {
	return int64(math.Round(rupees * 100))
}

// ===================== SANCTION: recommend (maker) =====================

// AdminRecommendSanctionHandler handles POST /admin/loans/:id/recommend —
// the maker half of the sanction maker-checker workflow. Requires
// LOAN_REVIEW. Records the recommended terms but does NOT sanction the
// loan — it moves to PENDING_SANCTION_APPROVAL, where a different admin
// with LOAN_SANCTION must independently approve or reject it
// (AdminApproveSanctionHandler below). This replaces the old single-call
// AdminDecisionHandler("sanction") path, which let one admin sanction a
// loan unilaterally.
func AdminRecommendSanctionHandler(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	adminID, adminUsername, adminRole := currentAdmin(c)

	var body struct {
		TenureMonths        *int     `json:"tenureMonths" binding:"required"`
		InterestRatePercent *float64 `json:"interestRatePercent" binding:"required"`
		Remarks             string   `json:"remarks"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	if body.TenureMonths == nil || *body.TenureMonths <= 0 || body.InterestRatePercent == nil || *body.InterestRatePercent <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"message": "tenureMonths and interestRatePercent are required and must be positive"})
		return
	}

	var loan models.LoanApplication
	if err := database.DB.First(&loan, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"message": "Not found"})
		return
	}
	if !canRecommend(loan.Status) {
		c.JSON(http.StatusConflict, gin.H{"message": "Loan is not in a state that can be recommended for sanction (current status: " + loan.Status + ")"})
		return
	}

	// Same CIBIL hard-block as before — a sub-650 score can never be
	// recommended for sanction, checker or no checker.
	if loan.CibilScore > 0 && loan.CibilScore < models.CibilRejectThreshold {
		c.JSON(http.StatusBadRequest, gin.H{
			"message": "Cannot recommend: applicant's CIBIL score (" + strconv.Itoa(loan.CibilScore) +
				") is below the minimum threshold of " + strconv.Itoa(models.CibilRejectThreshold) + ". This application must be rejected.",
		})
		return
	}

	before := audit.Snapshot(loan)

	now := time.Now()
	loan.Status = models.LoanStatusPendingSanctionApproval
	loan.RecommendedByAdminID = &adminID
	loan.RecommendedBy = adminUsername
	loan.RecommendedAt = &now
	loan.RecommendedTenureMonths = body.TenureMonths
	loan.RecommendedInterestRatePercent = body.InterestRatePercent
	loan.RecommendationRemarks = body.Remarks

	if err := database.DB.Save(&loan).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Failed to save recommendation"})
		return
	}

	audit.RecordDetailed(c, audit.Entry{
		ActorType: "admin", ActorID: adminID, ActorRef: adminUsername, ActorRole: adminRole,
		Action: "loan.sanction.recommend", TargetType: "loan", TargetID: itoa(loan.ID),
		Result: audit.ResultSuccess, Reason: body.Remarks,
		Before: nil, After: nil, Details: before,
	})

	c.JSON(http.StatusOK, withReferences(loan))
}

func canRecommend(status string) bool {
	switch status {
	case models.LoanStatusSubmitted, models.LoanStatusAutoVerified, models.LoanStatusAssigned, models.LoanStatusVerified:
		return true
	default:
		return false
	}
}

// ===================== SANCTION: approve/reject (checker) =====================

// AdminApproveSanctionHandler handles POST /admin/loans/:id/approve-sanction
// — the checker half. Requires LOAN_SANCTION. The acting admin MUST be a
// different person than whoever recommended it (maker != checker,
// enforced here, not just by convention). Row-locks the loan for the
// duration of the check-and-update so two concurrent approval requests
// can't both succeed.
func AdminApproveSanctionHandler(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	adminID, adminUsername, adminRole := currentAdmin(c)

	var body struct {
		Approve bool   `json:"approve"`
		Remarks string `json:"remarks"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}

	tx := database.DB.Begin()
	if tx.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Database error"})
		return
	}

	var loan models.LoanApplication
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&loan, id).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusNotFound, gin.H{"message": "Not found"})
		return
	}
	if loan.Status != models.LoanStatusPendingSanctionApproval {
		tx.Rollback()
		c.JSON(http.StatusConflict, gin.H{"message": "Loan is not pending sanction approval (current status: " + loan.Status + ")"})
		return
	}
	if loan.RecommendedByAdminID != nil && *loan.RecommendedByAdminID == adminID {
		tx.Rollback()
		audit.RecordDetailed(c, audit.Entry{
			ActorType: "admin", ActorID: adminID, ActorRef: adminUsername, ActorRole: adminRole,
			Action: "loan.sanction.approve", TargetType: "loan", TargetID: itoa(loan.ID),
			Result: audit.ResultDenied, Reason: "maker cannot approve their own recommendation",
		})
		c.JSON(http.StatusForbidden, gin.H{"message": "You recommended this sanction — a different authorized admin must approve or reject it"})
		return
	}

	before := audit.Snapshot(loan)
	now := time.Now()

	if !body.Approve {
		loan.Status = models.LoanStatusRejected
		loan.DecisionRemarks = body.Remarks
		loan.DecidedBy = adminUsername
		loan.DecidedByAdminID = &adminID
		loan.DecidedAt = &now
		if err := tx.Save(&loan).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"message": "Failed to save decision"})
			return
		}
		if err := tx.Commit().Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "Database error"})
			return
		}
		audit.RecordDetailed(c, audit.Entry{
			ActorType: "admin", ActorID: adminID, ActorRef: adminUsername, ActorRole: adminRole,
			Action: "loan.sanction.reject", TargetType: "loan", TargetID: itoa(loan.ID),
			Result: audit.ResultSuccess, Reason: body.Remarks, Details: before,
		})
		createNotification("customer", loan.CustomerID.String(), "Loan rejected",
			"Your "+loan.Category+" application #"+itoa(loan.ID)+" has been rejected. Remarks: "+body.Remarks)
		c.JSON(http.StatusOK, withReferences(loan))
		return
	}

	// Approve — same CIBIL-driven rate rule as the old single-admin path,
	// applied now to the recommended rate rather than an admin-typed one.
	tenure := loan.RecommendedTenureMonths
	rate := loan.RecommendedInterestRatePercent
	if tenure == nil || rate == nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Loan has no recommended terms to approve"})
		return
	}
	var cibilRateNote string
	effectiveRate := *rate
	switch {
	case loan.CibilScore > models.CibilPrimeThreshold:
		effectiveRate = models.CibilMinRatePercent
		cibilRateNote = "CIBIL " + strconv.Itoa(loan.CibilScore) + " > " + strconv.Itoa(models.CibilPrimeThreshold) + " — rate forced to minimum"
	case loan.CibilScore >= models.CibilRejectThreshold:
		effectiveRate = models.CibilMidRatePercent
		cibilRateNote = "CIBIL " + strconv.Itoa(loan.CibilScore) + " in range — rate forced"
	default:
		cibilRateNote = "CIBIL score not on file — recommended rate used as-is"
	}

	loan.TenureMonths = tenure
	loan.InterestRatePercent = &effectiveRate
	loan.Status = models.LoanStatusSanctioned
	loan.DecisionRemarks = body.Remarks
	loan.DecidedBy = adminUsername
	loan.DecidedByAdminID = &adminID
	loan.DecidedAt = &now

	var custAddr models.CustomerAddress
	database.DB.Where("user_id = ?", loan.CustomerID).First(&custAddr)
	gst := calculateProcessingFeeGST(loan.AmountRequested, custAddr.CurrentState)
	loan.ProcessingFeeBase = gst.ProcessingFeeBase
	loan.ProcessingFeeCGST = gst.CGST
	loan.ProcessingFeeSGST = gst.SGST
	loan.ProcessingFeeIGST = gst.IGST
	loan.ProcessingFeeGSTTotal = gst.GSTTotal
	loan.ProcessingFeeTotal = gst.Total
	loan.ProcessingFeeGSTType = gst.GSTType
	loan.ProcessingFeeCustomerState = gst.CustomerState

	if err := tx.Save(&loan).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Failed to save decision"})
		return
	}
	if err := tx.Commit().Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Database error"})
		return
	}

	audit.RecordDetailed(c, audit.Entry{
		ActorType: "admin", ActorID: adminID, ActorRef: adminUsername, ActorRole: adminRole,
		Action: "loan.sanction.approve", TargetType: "loan", TargetID: itoa(loan.ID),
		Result:  audit.ResultSuccess,
		Reason:  body.Remarks + ". " + cibilRateNote + ". Recommended by " + loan.RecommendedBy + ".",
		Details: before,
	})

	recordSanctionDecision(loan, cibilRateNote, adminUsername)

	createNotification("customer", loan.CustomerID.String(), "Loan sanctioned",
		"Your "+loan.Category+" application #"+itoa(loan.ID)+" has been sanctioned. Please review, sign, and re-upload your Sanction Letter.")

	if err := generateLoanLetter(loan, "Sanction Letter"); err != nil {
		log.Println("failed to generate sanction letter:", err)
	}

	c.JSON(http.StatusOK, withReferences(loan))
}

// ===================== DISBURSEMENT: prepare (maker) =====================

// AdminPrepareDisbursementHandler handles POST /admin/loans/:id/prepare-disbursement
// — the maker half of the disbursement maker-checker workflow. Requires
// LOAN_DISBURSE_PREPARE. Moves a sanctioned loan to READY_FOR_DISBURSEMENT;
// no funds move and no EMI schedule is generated until a different admin
// with LOAN_DISBURSE approves it.
func AdminPrepareDisbursementHandler(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	adminID, adminUsername, adminRole := currentAdmin(c)

	var body struct {
		Amount               float64 `json:"amount"`
		BankAccountReference string  `json:"bankAccountReference"`
	}
	c.ShouldBindJSON(&body)

	var loan models.LoanApplication
	if err := database.DB.First(&loan, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"message": "Not found"})
		return
	}
	// Explicitly guards against every disallowed source state named in
	// the brief: a rejected or cancelled loan can never reach here
	// (LoanStatusSanctioned is the only accepted prior state), and a loan
	// already mid-disbursement can't be re-prepared on top of itself.
	if loan.Status != models.LoanStatusSanctioned {
		c.JSON(http.StatusBadRequest, gin.H{"message": "loan must be sanctioned before disbursement can be prepared"})
		return
	}
	if loan.TenureMonths == nil || *loan.TenureMonths <= 0 || loan.InterestRatePercent == nil || *loan.InterestRatePercent <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"message": "loan is missing tenureMonths/interestRatePercent from sanction"})
		return
	}

	amt := body.Amount
	if amt == 0 {
		amt = loan.AmountRequested
	}

	// Disbursement.LoanID is uniquely indexed — a stale row from a
	// previously-rejected disbursement attempt on this same loan is
	// replaced rather than left to collide with a fresh insert.
	database.DB.Where("loan_id = ? AND status = ?", loan.ID, models.DisbursementStatusPendingApproval).
		Delete(&models.Disbursement{})

	disb := models.Disbursement{
		LoanID:               loan.ID,
		CustomerID:           loan.CustomerID.String(),
		AmountPaise:          toPaiseLoans(amt),
		Currency:             "INR",
		Status:               models.DisbursementStatusPendingApproval,
		BankAccountReference: body.BankAccountReference,
		Provider:             disbursement.Active().Name(),
		PreparedByAdminID:    &adminID,
		PreparedBy:           adminUsername,
	}
	if err := database.DB.Create(&disb).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Failed to create disbursement record"})
		return
	}

	before := audit.Snapshot(loan)
	now := time.Now()
	loan.Status = models.LoanStatusReadyForDisbursement
	loan.DisbursementPreparedByAdminID = &adminID
	loan.DisbursementPreparedBy = adminUsername
	loan.DisbursementPreparedAt = &now
	loan.DisbursementPreparedAmount = &amt

	if err := database.DB.Save(&loan).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Failed to save"})
		return
	}

	audit.RecordDetailed(c, audit.Entry{
		ActorType: "admin", ActorID: adminID, ActorRef: adminUsername, ActorRole: adminRole,
		Action: "loan.disbursement.prepare", TargetType: "loan", TargetID: itoa(loan.ID),
		Result: audit.ResultSuccess, Details: before,
	})

	c.JSON(http.StatusOK, withReferences(loan))
}

// ===================== DISBURSEMENT: approve/release (checker) =====================

// AdminApproveDisbursementHandler handles POST /admin/loans/:id/approve-disbursement
// — the checker half. Requires LOAN_DISBURSE. The acting admin MUST be a
// different person than whoever prepared it. This is the ONLY point at
// which a loan's status actually becomes DISBURSED, DisbursedAmount is
// set, and the EMI schedule + Disbursement Letter are generated — a
// prepared disbursement is not final and cannot become so merely because
// the prepare endpoint was called.
func AdminApproveDisbursementHandler(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	adminID, adminUsername, adminRole := currentAdmin(c)

	var body struct {
		Approve bool   `json:"approve"`
		Remarks string `json:"remarks"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}

	tx := database.DB.Begin()
	if tx.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Database error"})
		return
	}

	var loan models.LoanApplication
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&loan, id).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusNotFound, gin.H{"message": "Not found"})
		return
	}
	if loan.Status != models.LoanStatusReadyForDisbursement {
		tx.Rollback()
		c.JSON(http.StatusConflict, gin.H{"message": "Loan is not ready for disbursement approval (current status: " + loan.Status + ")"})
		return
	}
	if loan.DisbursementPreparedByAdminID != nil && *loan.DisbursementPreparedByAdminID == adminID {
		tx.Rollback()
		audit.RecordDetailed(c, audit.Entry{
			ActorType: "admin", ActorID: adminID, ActorRef: adminUsername, ActorRole: adminRole,
			Action: "loan.disbursement.approve", TargetType: "loan", TargetID: itoa(loan.ID),
			Result: audit.ResultDenied, Reason: "maker cannot approve their own disbursement preparation",
		})
		c.JSON(http.StatusForbidden, gin.H{"message": "You prepared this disbursement — a different authorized admin must approve or reject it"})
		return
	}

	var disb models.Disbursement
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("loan_id = ? AND status = ?", loan.ID, models.DisbursementStatusPendingApproval).
		First(&disb).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"message": "No pending disbursement record found for this loan"})
		return
	}

	before := audit.Snapshot(loan)
	now := time.Now()

	if !body.Approve {
		// Reject: return the loan to "sanctioned" so it can be
		// re-prepared, and remove the stale Disbursement row (its
		// LoanID unique index would otherwise block a fresh prepare).
		loan.Status = models.LoanStatusSanctioned
		loan.DisbursementPreparedByAdminID = nil
		loan.DisbursementPreparedBy = ""
		loan.DisbursementPreparedAt = nil
		loan.DisbursementPreparedAmount = nil
		if err := tx.Save(&loan).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"message": "Failed to save"})
			return
		}
		if err := tx.Delete(&disb).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"message": "Failed to save"})
			return
		}
		if err := tx.Commit().Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "Database error"})
			return
		}
		audit.RecordDetailed(c, audit.Entry{
			ActorType: "admin", ActorID: adminID, ActorRef: adminUsername, ActorRole: adminRole,
			Action: "loan.disbursement.reject", TargetType: "loan", TargetID: itoa(loan.ID),
			Result: audit.ResultSuccess, Reason: body.Remarks, Details: before,
		})
		c.JSON(http.StatusOK, withReferences(loan))
		return
	}

	// Approval only authorizes the transfer — it does not, by itself,
	// make the loan DISBURSED. The provider call below either records the
	// transfer as INITIATED (no external system to fail against, in the
	// no-live-payout-API-configured default) or reports a real failure
	// from an actual provider, in which case nothing here is finalized.
	transfer, transferErr := disbursement.Active().CreateTransfer(disb.AmountPaise, disb.BankAccountReference, "loan-"+itoa(loan.ID)+"-disbursement-"+itoa(disb.ID))
	if transferErr != nil {
		tx.Rollback()
		c.JSON(http.StatusBadGateway, gin.H{"message": "Failed to initiate transfer: " + transferErr.Error()})
		return
	}

	loan.Status = models.LoanStatusDisbursementProcessing
	loan.DisbursementApprovedByAdminID = &adminID
	loan.DisbursementApprovedBy = adminUsername
	loan.DisbursementApprovedAt = &now

	disb.Status = transfer.Status
	disb.ProviderReference = transfer.ProviderReference
	disb.ApprovedByAdminID = &adminID
	disb.ApprovedBy = adminUsername
	disb.InitiatedAt = &now

	if err := tx.Save(&loan).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Failed to save"})
		return
	}
	if err := tx.Save(&disb).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Failed to save"})
		return
	}
	if err := tx.Commit().Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Database error"})
		return
	}

	audit.RecordDetailed(c, audit.Entry{
		ActorType: "admin", ActorID: adminID, ActorRef: adminUsername, ActorRole: adminRole,
		Action: "loan.disbursement.initiated", TargetType: "loan", TargetID: itoa(loan.ID),
		Result:  audit.ResultSuccess,
		Reason:  "Prepared by " + loan.DisbursementPreparedBy + ". " + body.Remarks,
		Details: before,
	})

	c.JSON(http.StatusOK, withReferences(loan))
}

// AdminListPendingApprovalsHandler handles GET /admin/loans/pending-approvals
// — the admin panel's "Pending Approvals" queue: every loan currently
// waiting on a checker (sanction or disbursement), across all admins.
func AdminListPendingApprovalsHandler(c *gin.Context) {
	var loans []models.LoanApplication
	database.DB.
		Where("status IN ?", []string{models.LoanStatusPendingSanctionApproval, models.LoanStatusReadyForDisbursement}).
		Order("created_at asc").
		Find(&loans)
	c.JSON(http.StatusOK, loans)
}
