package loans

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/models"
)

// EmployeeTasksTodayHandler handles GET /employee/tasks/today
func EmployeeTasksTodayHandler(c *gin.Context) {
	empID := c.GetUint("employee_id")
	var loans []models.LoanApplication
	database.DB.Where("assigned_employee_id = ? AND status = ?", empID, models.LoanStatusAssigned).Find(&loans)
	resp := make([]LoanResponse, 0, len(loans))
	for _, loan := range loans {
		resp = append(resp, withReferences(loan))
	}
	c.JSON(http.StatusOK, resp)
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
	loan.Status = models.LoanStatusVerified
	loan.VerificationFindings = body.Findings
	loan.VerificationReport = body.Remarks
	loan.VerificationRisk = body.RiskAssessment
	loan.VerificationRecommendation = body.Recommendation
	database.DB.Save(&loan)
	c.JSON(http.StatusOK, withReferences(loan))
}
