package compliance

import (
	"encoding/base64"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/santosh0921/nbfc-backend/internal/audit"
	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/models"
)

func currentAdmin(c *gin.Context) (uint, string) {
	var id uint
	if v, ok := c.Get("admin_id"); ok {
		if cast, ok := v.(uint); ok {
			id = cast
		}
	}
	return id, c.GetString("admin_username")
}

// --- Profile ---

func AdminGetProfileHandler(c *gin.Context) {
	p, err := GetOrCreateProfile()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, p)
}

func AdminUpdateProfileHandler(c *gin.Context) {
	var updates map[string]interface{}
	if err := c.ShouldBindJSON(&updates); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	delete(updates, "id")
	delete(updates, "confirmationStatus")
	delete(updates, "confirmedBy")
	delete(updates, "confirmedAt")
	_, actor := currentAdmin(c)
	p, err := UpdateProfile(updates, actor)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, p)
}

func AdminConfirmProfileHandler(c *gin.Context) {
	_, actor := currentAdmin(c)
	p, err := ConfirmProfile(actor)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, p)
}

// --- Requirements ---

func AdminListRequirementsHandler(c *gin.Context) {
	var rows []models.ComplianceRequirement
	database.DB.Order("id desc").Find(&rows)
	c.JSON(http.StatusOK, rows)
}

func AdminCreateRequirementHandler(c *gin.Context) {
	var req models.ComplianceRequirement
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	_, actor := currentAdmin(c)
	created, err := CreateRequirement(&req, actor)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, created)
}

func AdminActivateRequirementHandler(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	_, actor := currentAdmin(c)
	req, err := ActivateRequirement(uint(id), actor)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, req)
}

func AdminSetApplicabilityHandler(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var body struct {
		Applicability string `json:"applicability" binding:"required"`
		Note          string `json:"note"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	_, actor := currentAdmin(c)
	if err := SetApplicability(uint(id), body.Applicability, body.Note, actor); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "updated"})
}

// --- Calendar ---

func AdminListCalendarHandler(c *gin.Context) {
	q := database.DB.Model(&models.ComplianceCalendarItem{})
	if status := c.Query("status"); status != "" {
		q = q.Where("status = ?", status)
	}
	var rows []models.ComplianceCalendarItem
	q.Order("due_date").Find(&rows)
	c.JSON(http.StatusOK, rows)
}

func AdminCreateCalendarItemHandler(c *gin.Context) {
	var item models.ComplianceCalendarItem
	if err := c.ShouldBindJSON(&item); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	_, actor := currentAdmin(c)
	created, err := CreateCalendarItem(&item, actor)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, created)
}

func AdminUpdateCalendarStatusHandler(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var body struct {
		Status  string `json:"status" binding:"required"`
		Remarks string `json:"remarks"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	_, actor := currentAdmin(c)
	if err := UpdateCalendarItemStatus(uint(id), body.Status, body.Remarks, actor); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "updated"})
}

// --- Regulatory returns ---

func AdminListReturnsHandler(c *gin.Context) {
	var rows []models.RegulatoryReturnTracking
	database.DB.Order("id desc").Find(&rows)
	c.JSON(http.StatusOK, rows)
}

func AdminCreateReturnHandler(c *gin.Context) {
	var ret models.RegulatoryReturnTracking
	if err := c.ShouldBindJSON(&ret); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	adminID, actor := currentAdmin(c)
	created, err := CreateReturn(&ret, adminID, actor)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, created)
}

func AdminApproveReturnHandler(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var body struct {
		Comments string `json:"comments"`
	}
	c.ShouldBindJSON(&body)
	adminID, actor := currentAdmin(c)
	ret, err := ReviewAndApproveReturn(uint(id), adminID, actor, body.Comments)
	if err != nil {
		status := http.StatusBadRequest
		if err == ErrSameActor {
			status = http.StatusForbidden
		}
		c.JSON(status, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, ret)
}

func AdminSubmitReturnHandler(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var body struct {
		SubmissionRef string `json:"submissionRef" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	_, actor := currentAdmin(c)
	ret, err := SubmitReturn(uint(id), body.SubmissionRef, actor)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, ret)
}

func AdminCorrectReturnHandler(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var body struct {
		Reason string `json:"reason" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	adminID, actor := currentAdmin(c)
	correction, err := CreateCorrection(uint(id), body.Reason, adminID, actor)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, correction)
}

// --- Evidence ---

func AdminUploadEvidenceHandler(c *gin.Context) {
	var body struct {
		RequirementID  *uint  `json:"requirementId"`
		CalendarItemID *uint  `json:"calendarItemId"`
		Title          string `json:"title" binding:"required"`
		Category       string `json:"category"`
		DataBase64     string `json:"dataBase64" binding:"required"`
		MimeType       string `json:"mimeType"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	decoded, err := base64.StdEncoding.DecodeString(body.DataBase64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "dataBase64 is not valid base64"})
		return
	}
	adminID, actor := currentAdmin(c)
	ev, err := UploadEvidence(body.RequirementID, body.CalendarItemID, body.Title, body.Category, decoded, body.MimeType, adminID, actor)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, ev)
}

func AdminListEvidenceHandler(c *gin.Context) {
	var rows []models.ComplianceEvidence
	database.DB.Order("id desc").Limit(500).Find(&rows)
	c.JSON(http.StatusOK, rows)
}

// AdminDownloadEvidenceHandler — every access is audit-logged (item H/W),
// same pattern as Phase 3 document downloads.
func AdminDownloadEvidenceHandler(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	ev, data, err := RetrieveEvidence(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"message": "Evidence not found or unavailable"})
		return
	}
	_, actor := currentAdmin(c)
	audit.RecordDetailed(c, audit.Entry{
		ActorType: "admin", ActorRef: actor,
		Action: "compliance.evidence.downloaded", TargetType: "compliance_evidence", TargetID: strconv.Itoa(id),
		Result: audit.ResultSuccess, Details: ev.Title,
	})
	c.JSON(http.StatusOK, gin.H{"title": ev.Title, "mimeType": ev.MimeType, "dataBase64": base64.StdEncoding.EncodeToString(data)})
}

// --- Policies ---

func AdminListPoliciesHandler(c *gin.Context) {
	var rows []models.Policy
	database.DB.Order("id desc").Find(&rows)
	c.JSON(http.StatusOK, rows)
}

func AdminCreatePolicyHandler(c *gin.Context) {
	var p models.Policy
	if err := c.ShouldBindJSON(&p); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	_, actor := currentAdmin(c)
	created, err := CreatePolicy(&p, actor)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, created)
}

func AdminAdvancePolicyStatusHandler(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var body struct {
		Status string `json:"status" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	_, actor := currentAdmin(c)
	if err := AdvancePolicyStatus(uint(id), body.Status, actor); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "updated"})
}

// --- Internal audit / findings / CAPA ---

func AdminListAuditsHandler(c *gin.Context) {
	var rows []models.InternalAudit
	database.DB.Order("id desc").Find(&rows)
	c.JSON(http.StatusOK, rows)
}

func AdminCreateAuditHandler(c *gin.Context) {
	var a models.InternalAudit
	if err := c.ShouldBindJSON(&a); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	_, actor := currentAdmin(c)
	created, err := CreateAudit(&a, actor)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, created)
}

func AdminListFindingsHandler(c *gin.Context) {
	q := database.DB.Model(&models.AuditFinding{})
	if severity := c.Query("severity"); severity != "" {
		q = q.Where("severity = ?", severity)
	}
	if status := c.Query("status"); status != "" {
		q = q.Where("status = ?", status)
	}
	var rows []models.AuditFinding
	q.Order("id desc").Find(&rows)
	c.JSON(http.StatusOK, rows)
}

func AdminCreateFindingHandler(c *gin.Context) {
	var f models.AuditFinding
	if err := c.ShouldBindJSON(&f); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	_, actor := currentAdmin(c)
	created, err := CreateFinding(&f, actor)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, created)
}

func AdminManagementResponseHandler(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var body struct {
		Response string `json:"response" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	_, actor := currentAdmin(c)
	if err := RecordManagementResponse(uint(id), body.Response, actor); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "recorded"})
}

func AdminCloseFindingHandler(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var body struct {
		VerificationEvidence string `json:"verificationEvidence" binding:"required"`
		VerifiedBy           string `json:"verifiedBy" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	_, actor := currentAdmin(c)
	if err := CloseFinding(uint(id), body.VerificationEvidence, body.VerifiedBy, actor); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "closed"})
}

func AdminListCAPAsHandler(c *gin.Context) {
	var rows []models.CAPA
	database.DB.Order("id desc").Find(&rows)
	c.JSON(http.StatusOK, rows)
}

func AdminCreateCAPAHandler(c *gin.Context) {
	var capa models.CAPA
	if err := c.ShouldBindJSON(&capa); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	_, actor := currentAdmin(c)
	created, err := CreateCAPA(&capa, actor)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, created)
}

func AdminUpdateCAPAStatusHandler(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var body struct {
		Status string `json:"status" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	_, actor := currentAdmin(c)
	if err := UpdateCAPAStatus(uint(id), body.Status, actor); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "updated"})
}

// --- Alerts / dashboard / sweep ---

func AdminListAlertsHandler(c *gin.Context) {
	q := database.DB.Model(&models.ComplianceAlert{})
	if c.Query("all") != "1" {
		q = q.Where("status = ?", models.ComplianceAlertStatusOpen)
	}
	var rows []models.ComplianceAlert
	q.Order("id desc").Limit(300).Find(&rows)
	c.JSON(http.StatusOK, rows)
}

func AdminResolveAlertHandler(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var body struct {
		Resolution string `json:"resolution"`
	}
	c.ShouldBindJSON(&body)
	if err := ResolveAlert(uint(id), body.Resolution); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "resolved"})
}

func AdminRunSweepHandler(c *gin.Context) {
	created := RunDailySweep()
	c.JSON(http.StatusOK, gin.H{"alertsCreated": created})
}

func AdminDashboardHandler(c *gin.Context) {
	c.JSON(http.StatusOK, GenerateDashboard())
}
