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

// Two-stage KYC review (see models.LoanStatusPendingKYCDocReview /
// LoanStatusPendingKYCFinalReview). The first stage reviews the customer's
// uploaded documents before any field employee is ever notified; the second
// reviews the field employee's verification report before the loan is
// allowed to proceed to Credit Bureau/Underwriting. Both stages are
// loan-specific (unlike internal/kyc's per-customer approve/reject), because
// a single customer can have multiple loans sitting at different pipeline
// stages at once.

// AdminApproveLoanKYCDocumentsHandler handles
// POST /admin/loans/:id/kyc-documents/approve — the first-stage decision.
// Requires the customer's documents to actually be present (CustomerKYC at
// VERIFICATION_PENDING or VERIFIED) before releasing the loan to field
// employees; a loan can't skip straight to field verification just because
// an admin clicked approve on a case with no documents on file.
func AdminApproveLoanKYCDocumentsHandler(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid loan id"})
		return
	}
	adminID, adminUsername, adminRole := currentAdmin(c)

	var loan models.LoanApplication
	if err := database.DB.First(&loan, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"message": "Loan not found"})
		return
	}
	if loan.Status != models.LoanStatusPendingKYCDocReview {
		c.JSON(http.StatusConflict, gin.H{"message": "Loan in status '" + loan.Status + "' is not pending document review"})
		return
	}

	var kycRecord models.CustomerKYC
	if err := database.DB.Where("customer_id = ?", loan.CustomerID).First(&kycRecord).Error; err != nil ||
		(kycRecord.Status != models.KYCStatusVerificationPending && kycRecord.Status != models.KYCStatusVerified) {
		c.JSON(http.StatusBadRequest, gin.H{"message": "This customer's documents are not yet complete — cannot release to field verification"})
		return
	}

	before := audit.Snapshot(loan)
	now := time.Now()
	loan.KYCDocApprovedByAdminID = &adminID
	loan.KYCDocApprovedBy = adminUsername
	loan.KYCDocApprovedAt = &now
	database.DB.Save(&loan)

	NotifyEligibleEmployees(&loan)

	audit.RecordDetailed(c, audit.Entry{
		ActorType: "admin", ActorID: adminID, ActorRef: adminUsername, ActorRole: adminRole,
		Action: "loan.kyc_documents_approved", TargetType: "loan_application", TargetID: strconv.Itoa(int(loan.ID)),
		Result: audit.ResultSuccess, Details: before, After: loan,
	})

	c.JSON(http.StatusOK, withReferences(loan))
}

// AdminRejectLoanKYCDocumentsHandler handles
// POST /admin/loans/:id/kyc-documents/reject
func AdminRejectLoanKYCDocumentsHandler(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid loan id"})
		return
	}
	adminID, adminUsername, adminRole := currentAdmin(c)

	var body struct {
		Reason string `json:"reason" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "A rejection reason is required"})
		return
	}

	var loan models.LoanApplication
	if err := database.DB.First(&loan, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"message": "Loan not found"})
		return
	}
	if loan.Status != models.LoanStatusPendingKYCDocReview {
		c.JSON(http.StatusConflict, gin.H{"message": "Loan in status '" + loan.Status + "' is not pending document review"})
		return
	}

	before := audit.Snapshot(loan)
	loan.Status = models.LoanStatusRejected
	loan.DecisionRemarks = body.Reason
	loan.DecidedBy = adminUsername
	now := time.Now()
	loan.DecidedAt = &now
	database.DB.Save(&loan)

	audit.RecordDetailed(c, audit.Entry{
		ActorType: "admin", ActorID: adminID, ActorRef: adminUsername, ActorRole: adminRole,
		Action: "loan.kyc_documents_rejected", TargetType: "loan_application", TargetID: strconv.Itoa(int(loan.ID)),
		Result: audit.ResultSuccess, Reason: body.Reason, Details: before, After: loan,
	})

	c.JSON(http.StatusOK, withReferences(loan))
}

// AdminApproveLoanFinalKYCHandler handles
// POST /admin/loans/:id/kyc-final/approve — the second-stage decision,
// reviewing the field employee's verification report. Moves the loan to
// LoanStatusVerified, the exact state the rest of the pipeline (Credit
// Bureau, Underwriting, Disbursement) already expects — no downstream
// change needed.
func AdminApproveLoanFinalKYCHandler(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid loan id"})
		return
	}
	adminID, adminUsername, adminRole := currentAdmin(c)

	var loan models.LoanApplication
	if err := database.DB.First(&loan, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"message": "Loan not found"})
		return
	}
	if loan.Status != models.LoanStatusPendingKYCFinalReview {
		c.JSON(http.StatusConflict, gin.H{"message": "Loan in status '" + loan.Status + "' is not pending final KYC review"})
		return
	}

	before := audit.Snapshot(loan)
	now := time.Now()
	loan.KYCFinalApprovedByAdminID = &adminID
	loan.KYCFinalApprovedBy = adminUsername
	loan.KYCFinalApprovedAt = &now
	loan.Status = models.LoanStatusVerified
	database.DB.Save(&loan)

	audit.RecordDetailed(c, audit.Entry{
		ActorType: "admin", ActorID: adminID, ActorRef: adminUsername, ActorRole: adminRole,
		Action: "loan.kyc_final_approved", TargetType: "loan_application", TargetID: strconv.Itoa(int(loan.ID)),
		Result: audit.ResultSuccess, Details: before, After: loan,
	})

	c.JSON(http.StatusOK, withReferences(loan))
}

// AdminRejectLoanFinalKYCHandler handles
// POST /admin/loans/:id/kyc-final/reject
func AdminRejectLoanFinalKYCHandler(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid loan id"})
		return
	}
	adminID, adminUsername, adminRole := currentAdmin(c)

	var body struct {
		Reason string `json:"reason" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "A rejection reason is required"})
		return
	}

	var loan models.LoanApplication
	if err := database.DB.First(&loan, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"message": "Loan not found"})
		return
	}
	if loan.Status != models.LoanStatusPendingKYCFinalReview {
		c.JSON(http.StatusConflict, gin.H{"message": "Loan in status '" + loan.Status + "' is not pending final KYC review"})
		return
	}

	before := audit.Snapshot(loan)
	loan.Status = models.LoanStatusRejected
	loan.DecisionRemarks = body.Reason
	loan.DecidedBy = adminUsername
	now := time.Now()
	loan.DecidedAt = &now
	database.DB.Save(&loan)

	audit.RecordDetailed(c, audit.Entry{
		ActorType: "admin", ActorID: adminID, ActorRef: adminUsername, ActorRole: adminRole,
		Action: "loan.kyc_final_rejected", TargetType: "loan_application", TargetID: strconv.Itoa(int(loan.ID)),
		Result: audit.ResultSuccess, Reason: body.Reason, Details: before, After: loan,
	})

	c.JSON(http.StatusOK, withReferences(loan))
}
