package loans

import (
	"encoding/base64"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/models"
)

type EsignLetterRequest struct {
	// "Sanction Letter" or "Disbursement Letter" — must match the
	// original system-generated docType exactly.
	LetterType string `json:"letterType" binding:"required"`
	// Raw PNG bytes of the signature pad capture, base64-encoded
	// (no "data:image/png;base64," prefix).
	SignatureBase64 string `json:"signatureBase64" binding:"required"`
}

// CustomerEsignLetterHandler regenerates the requested letter with the
// borrower's signature embedded directly into the PDF (not a separately
// uploaded photo) and stores the result as a new Document — "Signed
// Sanction Letter" / "Signed Disbursement Letter" — so it's immediately
// downloadable and also visible in the admin's document history for this
// loan, same as every other document type.
func CustomerEsignLetterHandler(c *gin.Context) {
	user, ok := currentUser(c)
	if !ok {
		return
	}

	var req EsignLetterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}

	if req.LetterType != "Sanction Letter" && req.LetterType != "Disbursement Letter" {
		c.JSON(http.StatusBadRequest, gin.H{"message": "letterType must be 'Sanction Letter' or 'Disbursement Letter'"})
		return
	}

	var loan models.LoanApplication
	if err := database.DB.Where("id = ? AND customer_id = ?", c.Param("id"), user.ID).First(&loan).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"message": "Loan not found"})
		return
	}

	if req.LetterType == "Sanction Letter" && loan.Status != models.LoanStatusSanctioned && loan.Status != models.LoanStatusDisbursed {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Loan has not been sanctioned yet"})
		return
	}
	if req.LetterType == "Disbursement Letter" && loan.Status != models.LoanStatusDisbursed {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Loan has not been disbursed yet"})
		return
	}

	sigB64 := req.SignatureBase64
	if idx := strings.Index(sigB64, ","); strings.HasPrefix(sigB64, "data:") && idx != -1 {
		sigB64 = sigB64[idx+1:]
	}
	sigBytes, err := base64.StdEncoding.DecodeString(sigB64)
	if err != nil || len(sigBytes) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid signature image"})
		return
	}

	signedAt := time.Now()
	signedAtLabel := signedAt.Format("02-Jan-2006 15:04")

	pdfBytes, err := buildLetterPDF(loan, req.LetterType, sigBytes, signedAtLabel)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Unable to generate signed letter"})
		return
	}

	signedDocType := "Signed " + req.LetterType
	loanID := loan.ID

	// Replace any earlier signed copy of this exact letter (re-signing is
	// allowed; only the latest signed version should be considered current).
	database.DB.Where("loan_id = ? AND doc_type = ?", loanID, signedDocType).Delete(&models.Document{})

	doc := models.Document{
		CustomerID:     loan.CustomerID,
		LoanID:         &loanID,
		DocType:        signedDocType,
		FileName:       signedDocType + " - Loan " + itoa(loan.ID) + ".pdf",
		MimeType:       "application/pdf",
		DataBase64:     base64.StdEncoding.EncodeToString(pdfBytes),
		UploadedByType: "customer",
		UploadedByRef:  user.ID.String(),
	}
	if err := database.DB.Create(&doc).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Unable to save signed letter"})
		return
	}

	c.JSON(http.StatusOK, doc)
}
