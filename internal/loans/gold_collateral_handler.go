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

// getOrCreateGoldCollateral returns the loan's single GoldCollateral row,
// creating the PENDING_APPRAISAL placeholder on first access — mirrors
// internal/kyc.GetOrCreate's own lazy-row pattern.
func getOrCreateGoldCollateral(loanID uint) (*models.GoldCollateral, error) {
	var gc models.GoldCollateral
	err := database.DB.Where("loan_id = ?", loanID).First(&gc).Error
	if err == nil {
		return &gc, nil
	}
	gc = models.GoldCollateral{LoanID: loanID, Status: models.GoldCollateralStatusPendingAppraisal}
	if err := database.DB.Create(&gc).Error; err != nil {
		return nil, err
	}
	return &gc, nil
}

// AdminGetGoldCollateralHandler handles GET /admin/loans/:id/gold-collateral
// — perm LOAN_VIEW.
func AdminGetGoldCollateralHandler(c *gin.Context) {
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
	if !isGoldLoanCategory(loan.Category) {
		c.JSON(http.StatusBadRequest, gin.H{"message": "This loan is not a Gold Loan"})
		return
	}
	gc, err := getOrCreateGoldCollateral(uint(loanID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Database error"})
		return
	}
	c.JSON(http.StatusOK, gc)
}

type appraiseGoldRequest struct {
	ActualWeightGrams float64 `json:"actualWeightGrams" binding:"required"`
	ActualPurityKarat int     `json:"actualPurityKarat" binding:"required"`
	GoldRatePerGram   float64 `json:"goldRatePerGram" binding:"required"`
	LTVPercent        float64 `json:"ltvPercent" binding:"required"`
	PacketTagNumber   string  `json:"packetTagNumber" binding:"required"`
	VaultBranch       string  `json:"vaultBranch" binding:"required"`
}

// AdminAppraiseGoldHandler handles
// POST /admin/loans/:id/gold-collateral/appraise — perm
// LOAN_DISBURSE_PREPARE (the branch/maker side). Records the branch's
// real physical test result and computes the amount it actually supports
// — 24-karat-equivalent weight × that day's rate × the LTV cap — which
// is what Operations uses at prepare-disbursement time, not the
// customer's own apply-time estimate.
func AdminAppraiseGoldHandler(c *gin.Context) {
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
	if !isGoldLoanCategory(loan.Category) {
		c.JSON(http.StatusBadRequest, gin.H{"message": "This loan is not a Gold Loan"})
		return
	}

	var body appraiseGoldRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	if body.LTVPercent <= 0 || body.LTVPercent > 90 {
		c.JSON(http.StatusBadRequest, gin.H{"message": "ltvPercent must be between 1 and 90"})
		return
	}

	gc, err := getOrCreateGoldCollateral(uint(loanID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Database error"})
		return
	}
	if gc.Status != models.GoldCollateralStatusPendingAppraisal {
		c.JSON(http.StatusConflict, gin.H{"message": "This gold has already been appraised (current status: " + gc.Status + ")"})
		return
	}

	// 24-karat-equivalent weight, same formula every real jeweller/bank
	// uses: purity fraction of 24K, since the quoted gold rate is always
	// per gram of 24K gold.
	pureGoldEquivalent := body.ActualWeightGrams * (float64(body.ActualPurityKarat) / 24.0)
	fullValue := pureGoldEquivalent * body.GoldRatePerGram
	approvedAmount := round2(fullValue * body.LTVPercent / 100)

	adminID, username, _ := currentAdminTriple(c)
	now := time.Now()
	gc.ActualWeightGrams = body.ActualWeightGrams
	gc.ActualPurityKarat = body.ActualPurityKarat
	gc.GoldRatePerGram = body.GoldRatePerGram
	gc.LTVPercent = body.LTVPercent
	gc.ApprovedLoanAmount = approvedAmount
	gc.PacketTagNumber = body.PacketTagNumber
	gc.VaultBranch = body.VaultBranch
	gc.AppraisedByAdminID = &adminID
	gc.AppraisedBy = username
	gc.AppraisedAt = &now
	gc.Status = models.GoldCollateralStatusAppraised
	if err := database.DB.Save(&gc).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Failed to save appraisal"})
		return
	}

	audit.RecordDetailed(c, audit.Entry{
		ActorType: "admin", ActorID: adminID, ActorRef: username,
		Action: "loan.gold_collateral.appraised", TargetType: "gold_collateral", TargetID: strconv.Itoa(int(gc.ID)),
		Result: audit.ResultSuccess, Details: "loan #" + strconv.Itoa(loanID) + ", approved amount " + strconv.FormatFloat(approvedAmount, 'f', 2, 64),
	})
	c.JSON(http.StatusOK, gc)
}

type approveGoldAppraisalRequest struct {
	Notes string `json:"notes"`
}

// AdminApproveGoldAppraisalHandler handles
// POST /admin/gold-collateral/:id/approve — perm LOAN_DISBURSE (the
// checker side). A different admin than whoever appraised it, same
// maker≠checker rule enforced everywhere else money is involved. This is
// also the moment the gold is considered sealed and in the vault.
func AdminApproveGoldAppraisalHandler(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid id"})
		return
	}
	var gc models.GoldCollateral
	if err := database.DB.First(&gc, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"message": "Not found"})
		return
	}
	if gc.Status != models.GoldCollateralStatusAppraised {
		c.JSON(http.StatusConflict, gin.H{"message": "This appraisal is not awaiting approval (current status: " + gc.Status + ")"})
		return
	}

	adminID, username, _ := currentAdminTriple(c)
	if gc.AppraisedByAdminID != nil && *gc.AppraisedByAdminID == adminID {
		c.JSON(http.StatusForbidden, gin.H{"message": "The admin who appraised this gold cannot also approve it (maker ≠ checker)."})
		return
	}

	now := time.Now()
	gc.ApprovedByAdminID = &adminID
	gc.ApprovedBy = username
	gc.ApprovedAt = &now
	gc.Status = models.GoldCollateralStatusApprovedInVault
	if err := database.DB.Save(&gc).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Failed to approve"})
		return
	}

	audit.RecordDetailed(c, audit.Entry{
		ActorType: "admin", ActorID: adminID, ActorRef: username,
		Action: "loan.gold_collateral.approved", TargetType: "gold_collateral", TargetID: strconv.Itoa(int(gc.ID)),
		Result: audit.ResultSuccess,
	})
	c.JSON(http.StatusOK, gc)
}

type releaseGoldRequest struct {
	Notes string `json:"notes"`
}

// AdminReleaseGoldHandler handles POST /admin/gold-collateral/:id/release
// — perm LOAN_DISBURSE. Manual, admin-triggered: the operator confirms
// the loan is fully repaid and hands the physical gold back — deliberately
// not wired to auto-fire off EMI completion the way NOC generation is,
// since handing back a physical object needs an explicit human act of
// custody transfer, not a background job.
func AdminReleaseGoldHandler(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid id"})
		return
	}
	var gc models.GoldCollateral
	if err := database.DB.First(&gc, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"message": "Not found"})
		return
	}
	if gc.Status != models.GoldCollateralStatusApprovedInVault {
		c.JSON(http.StatusConflict, gin.H{"message": "Gold is not currently in the vault (current status: " + gc.Status + ")"})
		return
	}
	var body releaseGoldRequest
	_ = c.ShouldBindJSON(&body)

	adminID, username, _ := currentAdminTriple(c)
	now := time.Now()
	gc.ReleasedByAdminID = &adminID
	gc.ReleasedBy = username
	gc.ReleasedAt = &now
	gc.ReleaseNotes = body.Notes
	gc.Status = models.GoldCollateralStatusReleased
	if err := database.DB.Save(&gc).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Failed to release"})
		return
	}

	audit.RecordDetailed(c, audit.Entry{
		ActorType: "admin", ActorID: adminID, ActorRef: username,
		Action: "loan.gold_collateral.released", TargetType: "gold_collateral", TargetID: strconv.Itoa(int(gc.ID)),
		Result: audit.ResultSuccess, Reason: body.Notes,
	})
	c.JSON(http.StatusOK, gc)
}

type auctionGoldRequest struct {
	Notes    string  `json:"notes" binding:"required"`
	Proceeds float64 `json:"proceeds"`
}

// AdminAuctionGoldHandler handles POST /admin/gold-collateral/:id/auction
// — perm LOAN_DISBURSE. The lender's real recovery path on a gold-loan
// default: no court process needed since the collateral is already
// physically held — just an explicit record of the auction and what it
// recovered.
func AdminAuctionGoldHandler(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid id"})
		return
	}
	var gc models.GoldCollateral
	if err := database.DB.First(&gc, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"message": "Not found"})
		return
	}
	if gc.Status != models.GoldCollateralStatusApprovedInVault {
		c.JSON(http.StatusConflict, gin.H{"message": "Gold is not currently in the vault (current status: " + gc.Status + ")"})
		return
	}
	var body auctionGoldRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "A reason is required to record an auction"})
		return
	}

	adminID, username, _ := currentAdminTriple(c)
	now := time.Now()
	gc.AuctionedByAdminID = &adminID
	gc.AuctionedBy = username
	gc.AuctionedAt = &now
	gc.AuctionNotes = body.Notes
	gc.AuctionProceeds = body.Proceeds
	gc.Status = models.GoldCollateralStatusAuctioned
	if err := database.DB.Save(&gc).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Failed to record auction"})
		return
	}

	audit.RecordDetailed(c, audit.Entry{
		ActorType: "admin", ActorID: adminID, ActorRef: username,
		Action: "loan.gold_collateral.auctioned", TargetType: "gold_collateral", TargetID: strconv.Itoa(int(gc.ID)),
		Result: audit.ResultSuccess, Reason: body.Notes,
	})
	c.JSON(http.StatusOK, gc)
}
