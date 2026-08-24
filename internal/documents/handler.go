package documents

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/santosh0921/nbfc-backend/internal/audit"
	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/models"
	"github.com/santosh0921/nbfc-backend/internal/storage"
)

// maxDocumentBytes caps the DECODED size of an uploaded document. KYC scans
// and statements are realistically a few MB at most.
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

// validateUpload enforces MIME allowlist + valid base64 + decoded size
// cap, and returns the decoded bytes so the caller never has to decode
// twice.
func validateUpload(body uploadRequest) (decoded []byte, err error) {
	if !allowedDocumentMimeTypes[strings.ToLower(strings.TrimSpace(body.MimeType))] {
		return nil, errUnsupportedMime
	}
	trimmed := strings.TrimSpace(body.DataBase64)
	if idx := strings.Index(trimmed, ","); idx != -1 && strings.HasPrefix(trimmed, "data:") {
		trimmed = trimmed[idx+1:]
	}
	decoded, decodeErr := base64.StdEncoding.DecodeString(trimmed)
	if decodeErr != nil {
		return nil, errInvalidBase64
	}
	if len(decoded) == 0 {
		return nil, errEmptyDocument
	}
	if len(decoded) > maxDocumentBytes {
		return nil, errDocumentTooLarge
	}
	return decoded, nil
}

type uploadValidationError string

func (e uploadValidationError) Error() string { return string(e) }

const (
	errUnsupportedMime  = uploadValidationError("Unsupported file type — only JPEG, PNG, and PDF are accepted")
	errInvalidBase64    = uploadValidationError("File data is not valid base64")
	errEmptyDocument    = uploadValidationError("File data is empty")
	errDocumentTooLarge = uploadValidationError("File is too large (max 10MB)")
)

// persistDocument encrypts+stores decoded via internal/storage when a
// provider is configured (the normal, production-required path — see
// config.ValidateProductionSafety), and only falls back to the legacy
// inline DataBase64 column when no provider is configured, which is
// itself only reachable in development (production refuses to start
// without DOCUMENT_ENCRYPTION_KEY).
func persistDocument(doc *models.Document, decoded []byte) error {
	sum := sha256.Sum256(decoded)
	doc.Hash = hex.EncodeToString(sum[:])
	doc.SizeBytes = len(decoded)
	doc.Status = models.DocumentStatusActive

	if provider := storage.Active(); provider != nil {
		ref, err := provider.Store(doc.CustomerID, doc.ID, decoded)
		if err != nil {
			return err
		}
		doc.StorageReference = ref
		return nil
	}
	// Development-only fallback — no encrypted storage provider
	// configured. Never reachable in production (fail-closed at startup).
	doc.DataBase64 = base64.StdEncoding.EncodeToString(decoded)
	return nil
}

// hydrateDocument builds the API response shape: every Document field
// plus a decrypted-on-read dataBase64 value, so existing clients (the
// admin panel and Flutter apps, which render this field directly) keep
// working unchanged even though the database itself no longer holds
// plaintext bytes for new uploads.
func hydrateDocument(doc models.Document) gin.H {
	dataBase64 := doc.DataBase64
	if doc.StorageReference != "" {
		if provider := storage.Active(); provider != nil {
			if raw, err := provider.Retrieve(doc.StorageReference); err == nil {
				dataBase64 = base64.StdEncoding.EncodeToString(raw)
			}
		}
	}
	return gin.H{
		"id": doc.ID, "customerId": doc.CustomerID, "loanId": doc.LoanID,
		"docType": doc.DocType, "fileName": doc.FileName, "mimeType": doc.MimeType,
		"hash": doc.Hash, "sizeBytes": doc.SizeBytes, "status": doc.Status,
		"storageReference": maskReference(doc.StorageReference),
		"uploadedByType":   doc.UploadedByType, "uploadedByRef": doc.UploadedByRef,
		"createdAt": doc.CreatedAt, "verifiedAt": doc.VerifiedAt,
		"dataBase64": dataBase64,
	}
}

// maskReference never returns the raw storage reference to a customer-
// facing caller context isn't checked here (both admin and customer
// handlers use hydrateDocument) — kept as a no-op passthrough today since
// the reference is already an opaque random UUID with no direct-access
// meaning outside the server process, but named/kept separate from the
// raw field so a future real object-storage key isn't accidentally
// exposed by reusing this same helper without revisiting it.
func maskReference(ref string) string {
	return ref
}

func hydrateDocuments(docs []models.Document) []gin.H {
	out := make([]gin.H, 0, len(docs))
	for _, d := range docs {
		out = append(out, hydrateDocument(d))
	}
	return out
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

	decoded, err := validateUpload(body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}

	doc := models.Document{
		ID:             uuid.New(),
		CustomerID:     user.ID,
		LoanID:         body.LoanID,
		DocType:        body.DocType,
		FileName:       body.FileName,
		MimeType:       body.MimeType,
		UploadedByType: "customer",
		UploadedByRef:  user.ID.String(),
		CreatedBy:      user.Mobile,
	}
	if err := persistDocument(&doc, decoded); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Unable to store document"})
		return
	}

	if err := database.DB.Create(&doc).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Unable to save document"})
		return
	}

	audit.RecordDetailed(c, audit.Entry{
		ActorType: "customer", ActorRef: user.Mobile,
		Action: "document.upload", TargetType: "document", TargetID: doc.ID.String(),
		Result: audit.ResultSuccess, Details: doc.DocType + " (" + fmt.Sprint(doc.SizeBytes) + " bytes)",
	})

	c.JSON(http.StatusCreated, hydrateDocument(doc))
}

// ListCustomerDocumentsHandler handles GET /auth/documents
func ListCustomerDocumentsHandler(c *gin.Context) {
	user, ok := currentUser(c)
	if !ok {
		return
	}

	query := database.DB.Where("customer_id = ?", user.ID)
	if loanID := c.Query("loanId"); loanID != "" {
		query = query.Where("loan_id = ?", loanID)
	}
	var docs []models.Document
	query.Order("created_at desc").Find(&docs)
	c.JSON(http.StatusOK, hydrateDocuments(docs))
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

	decoded, err := validateUpload(body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}

	loanID := loan.ID
	doc := models.Document{
		ID:             uuid.New(),
		CustomerID:     loan.CustomerID,
		LoanID:         &loanID,
		DocType:        body.DocType,
		FileName:       body.FileName,
		MimeType:       body.MimeType,
		UploadedByType: "employee",
		UploadedByRef:  emp.Code,
		CreatedBy:      emp.Code,
	}
	if err := persistDocument(&doc, decoded); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Unable to store document"})
		return
	}

	if err := database.DB.Create(&doc).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Unable to save document"})
		return
	}

	audit.RecordDetailed(c, audit.Entry{
		ActorType: "employee", ActorRef: emp.Code,
		Action: "document.upload", TargetType: "document", TargetID: doc.ID.String(),
		Result: audit.ResultSuccess, Details: doc.DocType + " for loan #" + strconv.Itoa(int(loan.ID)),
	})

	c.JSON(http.StatusCreated, hydrateDocument(doc))
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

	recordDocumentAccess(c, "loan", strconv.Itoa(id), len(docs))
	c.JSON(http.StatusOK, hydrateDocuments(docs))
}

// AdminListCustomerDocumentsHandler handles GET /admin/customers/:id/documents
func AdminListCustomerDocumentsHandler(c *gin.Context) {
	customerID := c.Param("id")

	var docs []models.Document
	database.DB.Where("customer_id = ?", customerID).Order("created_at asc").Find(&docs)

	recordDocumentAccess(c, "customer", customerID, len(docs))
	c.JSON(http.StatusOK, hydrateDocuments(docs))
}

// recordDocumentAccess audits admin access to KYC documents — these
// responses embed the actual file content (decrypted-on-read base64), so
// listing IS downloading here, not just metadata browsing.
func recordDocumentAccess(c *gin.Context, targetType, targetID string, count int) {
	adminID, _ := c.Get("admin_id")
	username, _ := c.Get("admin_username")
	role, _ := c.Get("admin_role")
	id, _ := adminID.(uint)
	audit.RecordDetailed(c, audit.Entry{
		ActorType: "admin", ActorID: id, ActorRef: fmt.Sprint(username), ActorRole: fmt.Sprint(role),
		Action: "document.download", TargetType: targetType, TargetID: targetID,
		Result: audit.ResultSuccess, Details: fmt.Sprintf("%d document(s) returned", count),
	})
}
