package loans

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm/clause"

	"github.com/santosh0921/nbfc-backend/internal/audit"
	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/models"
)

// EmployeeTasksTodayHandler handles GET /employee/tasks/today — returns
// three kinds of loans together: this employee's own already-claimed,
// still-open assignments (status=assigned), every unclaimed pending_claim
// loan broadcast to this employee's city (see submitLoanApplication)
// that nobody has claimed yet, and this employee's own recently-completed
// work (verified/sanctioned/disbursed/rejected — the Completed count on
// the dashboard, and the "Completed" filter in the cases list, come from
// this same feed). Previously the query only ever matched status=assigned,
// so a loan disappeared entirely the instant EmployeeVerifyLoanHandler
// moved it to "verified" — it never showed as done anywhere, the
// Completed count stayed permanently at whatever it started at. Scoped
// to the last 30 days by UpdatedAt so a long-tenured employee's feed
// doesn't grow unbounded forever.
//
// The client tells these apart by `status` — a pending_claim entry needs
// POST /employee/loans/:id/claim before this employee can act on it, an
// assigned one is already theirs and still open, anything else here is
// theirs and done.
func EmployeeTasksTodayHandler(c *gin.Context) {
	empID := c.GetUint("employee_id")

	var mine []models.LoanApplication
	database.DB.Where("assigned_employee_id = ? AND status = ?", empID, models.LoanStatusAssigned).Find(&mine)

	var completed []models.LoanApplication
	database.DB.Where(
		"assigned_employee_id = ? AND status IN ? AND updated_at > ?",
		empID,
		[]string{models.LoanStatusPendingKYCFinalReview, models.LoanStatusVerified, models.LoanStatusSanctioned, models.LoanStatusDisbursed, models.LoanStatusRejected},
		time.Now().AddDate(0, 0, -30),
	).Find(&completed)

	var claimable []models.LoanApplication
	var emp models.Employee
	if err := database.DB.First(&emp, empID).Error; err == nil && emp.BranchCity != "" {
		database.DB.Where("status = ? AND assigned_employee_id IS NULL AND LOWER(city) = LOWER(?)",
			models.LoanStatusPendingClaim, emp.BranchCity).Find(&claimable)
	}

	all := append(mine, completed...)
	all = append(all, claimable...)
	resp := make([]LoanResponse, 0, len(all))
	for _, loan := range all {
		resp = append(resp, withReferences(loan))
	}
	c.JSON(http.StatusOK, resp)
}

// EmployeeClaimLoanHandler handles POST /employee/loans/:id/claim — the
// first-come-first-served resolution for a loan broadcast to every
// verification employee in its city (LoanStatusPendingClaim). Row-locked
// so two employees tapping "Claim" on the same loan within the same
// instant can never both succeed — whichever transaction's UPDATE lands
// first wins, and the loser's stale local copy just gets a 409 they can
// use to remove it from their own list.
func EmployeeClaimLoanHandler(c *gin.Context) {
	empID := c.GetUint("employee_id")
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid loan id"})
		return
	}

	var emp models.Employee
	if err := database.DB.First(&emp, empID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Database error"})
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
		c.JSON(http.StatusNotFound, gin.H{"message": "Loan not found"})
		return
	}
	if loan.Status != models.LoanStatusPendingClaim || loan.AssignedEmployeeID != nil {
		tx.Rollback()
		c.JSON(http.StatusConflict, gin.H{"message": "This loan has already been claimed by another employee"})
		return
	}
	if !strings.EqualFold(loan.City, emp.BranchCity) {
		tx.Rollback()
		c.JSON(http.StatusForbidden, gin.H{"message": "This loan is not in your assigned city"})
		return
	}

	before := audit.Snapshot(loan)
	loan.AssignedEmployeeID = &empID
	loan.Status = models.LoanStatusAssigned
	if err := tx.Save(&loan).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Failed to claim loan"})
		return
	}
	if err := tx.Commit().Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Database error"})
		return
	}

	audit.RecordDetailed(c, audit.Entry{
		ActorType: "employee", ActorID: empID, ActorRef: emp.Name, ActorRole: emp.Role,
		Action: "loan.claimed", TargetType: "loan_application", TargetID: strconv.Itoa(int(loan.ID)),
		Result: audit.ResultSuccess, Details: before, After: loan,
	})

	c.JSON(http.StatusOK, withReferences(loan))
}

// EmployeeVerifyLoanHandler handles POST /employee/loans/:id/verify
func EmployeeVerifyLoanHandler(c *gin.Context) {
	empID := c.GetUint("employee_id")
	id, _ := strconv.Atoi(c.Param("id"))
	var loan models.LoanApplication
	if err := database.DB.First(&loan, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"message": "Loan not found"})
		return
	}
	if loan.AssignedEmployeeID == nil || *loan.AssignedEmployeeID != empID {
		c.JSON(http.StatusForbidden, gin.H{"message": "Loan not assigned to you"})
		return
	}
	// State whitelist: verification is only meaningful from the states
	// where a field-verification report is actually pending — an initial
	// assignment (assigned) or an employee re-verification loop
	// (pending_claim). Anything already verified/sanctioned/rejected/
	// disbursed must not be silently overwritten back to "verified".
	if loan.Status != models.LoanStatusAssigned && loan.Status != models.LoanStatusPendingClaim {
		c.JSON(http.StatusConflict, gin.H{"message": "Loan in status '" + loan.Status + "' is not pending verification"})
		return
	}
	var body struct {
		Findings       string `json:"findings"`
		Remarks        string `json:"remarks"`
		RiskAssessment string `json:"riskAssessment"`
		Recommendation string `json:"recommendation"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	before := audit.Snapshot(loan)
	loan.Status = models.LoanStatusPendingKYCFinalReview
	loan.VerificationFindings = body.Findings
	loan.VerificationReport = body.Remarks
	loan.VerificationRisk = body.RiskAssessment
	loan.VerificationRecommendation = body.Recommendation
	database.DB.Save(&loan)

	var emp models.Employee
	database.DB.First(&emp, empID)
	audit.RecordDetailed(c, audit.Entry{
		ActorType: "employee", ActorID: empID, ActorRef: emp.Name, ActorRole: emp.Role,
		Action: "loan.field_verification_submitted", TargetType: "loan_application", TargetID: strconv.Itoa(int(loan.ID)),
		Result: audit.ResultSuccess, Details: before, After: loan,
	})

	c.JSON(http.StatusOK, withReferences(loan))
}
