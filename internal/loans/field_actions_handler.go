package loans

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/models"
)

// ownsLoan loads the loan by :id and checks it's assigned to the calling
// employee, writing an error response and returning false if not — the
// same ownership check EmployeeVerifyLoanHandler uses.
func ownsLoan(c *gin.Context, empID uint) (models.LoanApplication, bool) {
	id, _ := strconv.Atoi(c.Param("id"))
	var loan models.LoanApplication
	if err := database.DB.First(&loan, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"message": "loan not found"})
		return loan, false
	}
	if loan.AssignedEmployeeID == nil || *loan.AssignedEmployeeID != empID {
		c.JSON(http.StatusForbidden, gin.H{"message": "loan not assigned to you"})
		return loan, false
	}
	return loan, true
}

// EmployeeGoldAppraisalHandler handles POST /employee/loans/:id/gold-appraisal
func EmployeeGoldAppraisalHandler(c *gin.Context) {
	empID := c.GetUint("employee_id")
	loan, ok := ownsLoan(c, empID)
	if !ok {
		return
	}
	var body struct {
		WeightGrams float64 `json:"weightGrams"`
		PurityKarat float64 `json:"purityKarat"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	appraisal := models.GoldAppraisal{
		LoanID: loan.ID, EmployeeID: empID,
		WeightGrams: body.WeightGrams, PurityKarat: body.PurityKarat,
	}
	database.DB.Create(&appraisal)
	c.JSON(http.StatusCreated, appraisal)
}

// EmployeePropertyVerificationHandler handles POST /employee/loans/:id/property-verification
func EmployeePropertyVerificationHandler(c *gin.Context) {
	empID := c.GetUint("employee_id")
	loan, ok := ownsLoan(c, empID)
	if !ok {
		return
	}
	var body struct {
		Condition      string  `json:"condition"`
		EstimatedValue float64 `json:"estimatedValue"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	verification := models.PropertyVerification{
		LoanID: loan.ID, EmployeeID: empID,
		Condition: body.Condition, EstimatedValue: body.EstimatedValue,
	}
	database.DB.Create(&verification)
	c.JSON(http.StatusCreated, verification)
}

// EmployeeAttendanceHandler handles POST /employee/attendance — not tied
// to any loan, just a check-in/check-out record with device geolocation.
func EmployeeAttendanceHandler(c *gin.Context) {
	empID := c.GetUint("employee_id")
	var body struct {
		Action    string  `json:"action"`
		Latitude  float64 `json:"latitude"`
		Longitude float64 `json:"longitude"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	if body.Action != "checkin" && body.Action != "checkout" {
		c.JSON(http.StatusBadRequest, gin.H{"message": "action must be checkin or checkout"})
		return
	}
	record := models.EmployeeAttendance{
		EmployeeID: empID, Action: body.Action,
		Latitude: body.Latitude, Longitude: body.Longitude,
	}
	database.DB.Create(&record)
	c.JSON(http.StatusCreated, record)
}
