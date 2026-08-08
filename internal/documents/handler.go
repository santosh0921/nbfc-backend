package documents

import (
	"net/http"
	"strconv"

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

type uploadRequest struct {
	DocType    string `json:"docType" binding:"required"`
	FileName   string `json:"fileName" binding:"required"`
	MimeType   string `json:"mimeType" binding:"required"`
	DataBase64 string `json:"dataBase64" binding:"required"`
	LoanID     *uint  `json:"loanId"`
}

// UploadCustomerDocumentHandler handles POST /auth/documents
func UploadCustomerDocumentHandler(c *gin.Context) {
	user, ok := currentUser(c)
	if !ok {
		return
	}

	var body uploadRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}

	doc := models.Document{
		CustomerID:     user.ID,
		LoanID:         body.LoanID,
		DocType:        body.DocType,
		FileName:       body.FileName,
		MimeType:       body.MimeType,
		DataBase64:     body.DataBase64,
		UploadedByType: "customer",
		UploadedByRef:  user.ID.String(),
	}

	if err := database.DB.Create(&doc).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Unable to save document"})
		return
	}

	c.JSON(http.StatusCreated, doc)
}

// ListCustomerDocumentsHandler handles GET /auth/documents
func ListCustomerDocumentsHandler(c *gin.Context) {
	user, ok := currentUser(c)
	if !ok {
		return
	}

	var docs []models.Document
	database.DB.Where("customer_id = ?", user.ID).Order("created_at desc").Find(&docs)
	c.JSON(http.StatusOK, docs)
}

// UploadEmployeeLoanDocumentHandler handles POST /employee/loans/:id/documents
func UploadEmployeeLoanDocumentHandler(c *gin.Context) {
	empID := c.GetUint("employee_id")

	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid loan id"})
		return
	}

	var loan models.LoanApplication
	if err := database.DB.First(&loan, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"message": "Loan not found"})
		return
	}

	var emp models.Employee
	if err := database.DB.First(&emp, empID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Database error"})
		return
	}

	var body uploadRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}

	loanID := loan.ID
	doc := models.Document{
		CustomerID:     loan.CustomerID,
		LoanID:         &loanID,
		DocType:        body.DocType,
		FileName:       body.FileName,
		MimeType:       body.MimeType,
		DataBase64:     body.DataBase64,
		UploadedByType: "employee",
		UploadedByRef:  emp.Code,
	}

	if err := database.DB.Create(&doc).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Unable to save document"})
		return
	}

	c.JSON(http.StatusCreated, doc)
}

// AdminListLoanDocumentsHandler handles GET /admin/loans/:id/documents
func AdminListLoanDocumentsHandler(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid loan id"})
		return
	}

	var loan models.LoanApplication
	if err := database.DB.First(&loan, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"message": "Loan not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Database error"})
		return
	}

	var docs []models.Document
	loanID := uint(id)
	database.DB.
		Where("loan_id = ?", loanID).
		Order("created_at asc").
		Find(&docs)

	c.JSON(http.StatusOK, docs)
}

// AdminListCustomerDocumentsHandler handles GET /admin/customers/:id/documents
func AdminListCustomerDocumentsHandler(c *gin.Context) {
	customerID := c.Param("id")

	var docs []models.Document
	database.DB.Where("customer_id = ?", customerID).Order("created_at asc").Find(&docs)

	c.JSON(http.StatusOK, docs)
}
