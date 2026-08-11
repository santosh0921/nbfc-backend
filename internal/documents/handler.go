package documents

import (
	"encoding/base64"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/models"
)

// maxDocumentBytes caps the DECODED size of an uploaded document. KYC scans
// and statements are realistically a few MB at most; with no cap at all a
// single request could inline an arbitrarily large blob into the database
// (documents are stored as base64 text, not object storage).
const maxDocumentBytes = 10 * 1024 * 1024 // 10MB

var allowedDocumentMimeTypes = map[string]bool{
	"image/jpeg":      true,
	"image/jpg":       true,
	"image/png":       true,
	"application/pdf": true,
}

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

// validateUpload enforces the checks that used to not exist at all: the
// MIME type must be one this app actually handles, the payload must
// actually be valid base64, and the DECODED size must stay under
// maxDocumentBytes — checking the base64 string's own length isn't
// equivalent (base64 inflates size by ~33%, and a crafted string could
// still decode to something larger than intended relative to its encoded
// length in edge cases), so it's decoded first and the real byte count is
// what's compared against the cap.
func validateUpload(body uploadRequest) (decodedSize int, err error) {
	if !allowedDocumentMimeTypes[strings.ToLower(strings.TrimSpace(body.MimeType))] {
		return 0, errUnsupportedMime
	}
	trimmed := strings.TrimSpace(body.DataBase64)
	if idx := strings.Index(trimmed, ","); idx != -1 && strings.HasPrefix(trimmed, "data:") {
		trimmed = trimmed[idx+1:]
	}
	decoded, decodeErr := base64.StdEncoding.DecodeString(trimmed)
	if decodeErr != nil {
		return 0, errInvalidBase64
	}
	if len(decoded) == 0 {
		return 0, errEmptyDocument
	}
	if len(decoded) > maxDocumentBytes {
		return 0, errDocumentTooLarge
	}
	return len(decoded), nil
}

type uploadValidationError string

func (e uploadValidationError) Error() string { return string(e) }

const (
	errUnsupportedMime  = uploadValidationError("Unsupported file type — only JPEG, PNG, and PDF are accepted")
	errInvalidBase64    = uploadValidationError("File data is not valid base64")
	errEmptyDocument    = uploadValidationError("File data is empty")
	errDocumentTooLarge = uploadValidationError("File is too large (max 10MB)")
)

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

	// loanId used to be taken straight from the request body with no
	// verification that the loan actually belongs to this customer — any
	// authenticated customer could attach a document to a stranger's loan
	// file, where an admin would review it as part of that stranger's
	// application. Now it's checked like every other loan-scoped endpoint
	// in this codebase already does (see loans/customer_handler.go).
	if body.LoanID != nil {
		var loan models.LoanApplication
		if err := database.DB.First(&loan, *body.LoanID).Error; err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": "Loan not found"})
			return
		}
		if loan.CustomerID != user.ID {
			c.JSON(http.StatusForbidden, gin.H{"message": "Not your loan"})
			return
		}
	}

	if _, err := validateUpload(body); err != nil {
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

	query := database.DB.Where("customer_id = ?", user.ID)
	// Optional filter so a customer with multiple loans can see just the
	// documents (e.g. verification photos captured by the assigned
	// employee) tied to one specific loan, instead of always getting
	// every document across every loan they've ever applied for.
	if loanID := c.Query("loanId"); loanID != "" {
		query = query.Where("loan_id = ?", loanID)
	}
	var docs []models.Document
	query.Order("created_at desc").Find(&docs)
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

	// Mirror-image of the customer-side check above: an employee used to
	// be able to attach a document to ANY loan, not just one assigned to
	// them.
	if loan.AssignedEmployeeID == nil || *loan.AssignedEmployeeID != empID {
		c.JSON(http.StatusForbidden, gin.H{"message": "This loan is not assigned to you"})
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

	if _, err := validateUpload(body); err != nil {
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
