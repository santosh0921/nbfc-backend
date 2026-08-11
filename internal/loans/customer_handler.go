package loans

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

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

// SubmitLoanHandler handles POST /loans/apply
func SubmitLoanHandler(c *gin.Context) {
	user, ok := currentUser(c)
	if !ok {
		return
	}

	var body struct {
		Category         string  `json:"category" binding:"required"`
		AmountRequested  float64 `json:"amountRequested" binding:"required"`
		Purpose          string  `json:"purpose"`
		ApplicantName    string  `json:"applicantName" binding:"required"`
		ApplicantPhone   string  `json:"applicantPhone"`
		AddressLine      string  `json:"addressLine"`
		City             string  `json:"city" binding:"required"`
		TimingPreference string  `json:"timingPreference"`
		VisitDate        string  `json:"visitDate"` // "2006-01-02", optional
		VisitTimeSlot    string  `json:"visitTimeSlot"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}

	loan := models.LoanApplication{
		CustomerID:       user.ID,
		Category:         body.Category,
		AmountRequested:  body.AmountRequested,
		Purpose:          body.Purpose,
		ApplicantName:    body.ApplicantName,
		ApplicantPhone:   body.ApplicantPhone,
		AddressLine:      body.AddressLine,
		City:             body.City,
		CibilScore:       getOrCreateCibilScore(user),
		TimingPreference: body.TimingPreference,
		VisitTimeSlot:    body.VisitTimeSlot,
	}
	if body.VisitDate != "" {
		if parsed, err := time.Parse("2006-01-02", body.VisitDate); err == nil {
			loan.VisitDate = &parsed
		}
	}

	loan = submitLoanApplication(loan)

	c.JSON(http.StatusCreated, withReferences(loan))
}

// TopUpLoanHandler handles POST /auth/loans/:id/topup — customer-initiated
// top-up on an existing disbursed loan. Creates a new LoanApplication
// (ParentLoanID pointing at the original) and runs it through the exact
// same auto-approval/assignment pipeline as a normal application.
func TopUpLoanHandler(c *gin.Context) {
	user, ok := currentUser(c)
	if !ok {
		return
	}
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid loan id"})
		return
	}

	var parent models.LoanApplication
	if err := database.DB.First(&parent, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"message": "Loan not found"})
		return
	}
	if parent.CustomerID != user.ID {
		c.JSON(http.StatusForbidden, gin.H{"message": "Not your loan"})
		return
	}
	if parent.Status != models.LoanStatusDisbursed {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Top-up is only available on a disbursed loan"})
		return
	}

	var body struct {
		Amount  float64 `json:"amount" binding:"required"`
		Purpose string  `json:"purpose"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}

	parentID := parent.ID
	loan := models.LoanApplication{
		CustomerID:      user.ID,
		Category:        parent.Category,
		AmountRequested: body.Amount,
		Purpose:         "Top-up on Loan #" + itoa(parent.ID) + ": " + body.Purpose,
		ApplicantName:   parent.ApplicantName,
		ApplicantPhone:  parent.ApplicantPhone,
		AddressLine:     parent.AddressLine,
		City:            parent.City,
		ParentLoanID:    &parentID,
		CibilScore:      getOrCreateCibilScore(user),
	}

	loan = submitLoanApplication(loan)

	c.JSON(http.StatusCreated, withReferences(loan))
}

// ListMyLoansHandler handles GET /loans/mine
func ListMyLoansHandler(c *gin.Context) {
	user, ok := currentUser(c)
	if !ok {
		return
	}
	var loans []models.LoanApplication
	database.DB.Where("customer_id = ?", user.ID).Order("created_at desc").Find(&loans)
	c.JSON(http.StatusOK, loans)
}

// GetMyLoanHandler handles GET /loans/:id
func GetMyLoanHandler(c *gin.Context) {
	user, ok := currentUser(c)
	if !ok {
		return
	}
	var loan models.LoanApplication
	if err := database.DB.Where("id = ? AND customer_id = ?", c.Param("id"), user.ID).First(&loan).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"message": "Loan not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Database error"})
		return
	}
	c.JSON(http.StatusOK, withReferences(loan))
}
