package documents

import (
	"encoding/base64"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/models"
	"github.com/santosh0921/nbfc-backend/internal/storage"
)

// UploadVerificationReportHandler handles POST
// /employee/loans/:id/verification-report — stores the signed PDF the
// field officer generated on-device (see PdfGenerator.generateVisitReport
// in the Flutter app) so it's visible in the admin panel instead of
// staying only on the officer's phone.
func UploadVerificationReportHandler(c *gin.Context) {
	empID := c.GetUint("employee_id")
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
	if loan.AssignedEmployeeID == nil || *loan.AssignedEmployeeID != empID {
		c.JSON(http.StatusForbidden, gin.H{"message": "loan not assigned to you"})
		return
	}

	var body struct {
		CustomerName   string `json:"customerName"`
		LoanType       string `json:"loanType"`
		Recommendation string `json:"recommendation"`
		RiskAssessment string `json:"riskAssessment"`
		FileName       string `json:"fileName" binding:"required"`
		MimeType       string `json:"mimeType" binding:"required"`
		DataBase64     string `json:"dataBase64" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	decoded, err := decodeAndValidatePdf(body.MimeType, body.DataBase64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}

	report := models.VerificationReport{
		LoanID: uint(loanID), EmployeeID: empID,
		CustomerName: body.CustomerName, LoanType: body.LoanType,
		Recommendation: body.Recommendation, RiskAssessment: body.RiskAssessment,
		FileName: body.FileName, MimeType: body.MimeType,
	}
	if provider := storage.Active(); provider != nil {
		ref, err := provider.Store(loan.CustomerID, uuid.New(), decoded)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "Could not store report"})
			return
		}
		report.StorageReference = ref
	} else {
		report.DataBase64 = base64.StdEncoding.EncodeToString(decoded)
	}
	database.DB.Create(&report)
	c.JSON(http.StatusCreated, gin.H{"id": report.ID, "createdAt": report.CreatedAt})
}

// decodeAndValidatePdf mirrors validateUpload's checks (unexported in this
// same package, but scoped to images/PDF generically) — narrowed to just
// PDF here since a verification report is always a PDF generated
// on-device, never a raw photo.
func decodeAndValidatePdf(mimeType, dataBase64 string) ([]byte, error) {
	if mimeType != "application/pdf" {
		return nil, uploadValidationError("Verification reports must be a PDF")
	}
	decoded, err := base64.StdEncoding.DecodeString(dataBase64)
	if err != nil {
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

// AdminListVerificationReportsHandler handles GET /admin/verification-reports
func AdminListVerificationReportsHandler(c *gin.Context) {
	var reports []models.VerificationReport
	database.DB.Order("created_at desc").Find(&reports)

	out := make([]gin.H, 0, len(reports))
	for _, r := range reports {
		var emp models.Employee
		employeeName := ""
		if err := database.DB.First(&emp, r.EmployeeID).Error; err == nil {
			employeeName = emp.Name
		}
		dataBase64 := r.DataBase64
		if r.StorageReference != "" {
			if provider := storage.Active(); provider != nil {
				if raw, err := provider.Retrieve(r.StorageReference); err == nil {
					dataBase64 = base64.StdEncoding.EncodeToString(raw)
				}
			}
		}
		out = append(out, gin.H{
			"id": r.ID, "loanId": r.LoanID, "employeeId": r.EmployeeID, "employeeName": employeeName,
			"customerName": r.CustomerName, "loanType": r.LoanType,
			"recommendation": r.Recommendation, "riskAssessment": r.RiskAssessment,
			"fileName": r.FileName, "mimeType": r.MimeType, "createdAt": r.CreatedAt,
			"dataBase64": dataBase64,
		})
	}
	c.JSON(http.StatusOK, out)
}
