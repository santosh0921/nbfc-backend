package recovery

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/santosh0921/nbfc-backend/internal/audit"
	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/models"
)

func createNotification(role string, userID string, title, body string) {
	n := models.Notification{Role: role, UserID: userID, Title: title, Body: body}
	database.DB.Create(&n)
}

// CasesTodayHandler handles GET /employee/recovery/cases/today
func CasesTodayHandler(c *gin.Context) {
	empID := c.GetUint("employee_id")
	var cases []models.RecoveryCase
	database.DB.Where("assigned_employee_id = ?", empID).Find(&cases)
	c.JSON(http.StatusOK, cases)
}

// SubmitReportHandler handles POST /employee/recovery/cases/:id/report
func SubmitReportHandler(c *gin.Context) {
	empID := c.GetUint("employee_id")
	id, _ := strconv.Atoi(c.Param("id"))
	var rc models.RecoveryCase
	if err := database.DB.First(&rc, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"message": "case not found"})
		return
	}
	if rc.AssignedEmployeeID != empID {
		c.JSON(http.StatusForbidden, gin.H{"message": "case not assigned to you"})
		return
	}
	var body struct {
		StatusUpdate   string `json:"statusUpdate"`
		Remarks        string `json:"remarks"`
		RiskAssessment string `json:"riskAssessment"`
		Recommendation string `json:"recommendation"`
		NextAction     string `json:"nextAction"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	report := models.RecoveryReport{
		CaseID: rc.ID, StatusUpdate: body.StatusUpdate, Remarks: body.Remarks,
		RiskAssessment: body.RiskAssessment, Recommendation: body.Recommendation, NextAction: body.NextAction,
	}
	database.DB.Create(&report)
	rc.Status = "reported"
	database.DB.Save(&rc)
	c.JSON(http.StatusCreated, report)
}

// AdminListReportsHandler handles GET /admin/recovery-reports
func AdminListReportsHandler(c *gin.Context) {
	var reports []models.RecoveryReport
	database.DB.Order("created_at desc").Find(&reports)
	c.JSON(http.StatusOK, reports)
}

// AdminActionHandler handles POST /admin/recovery-reports/:id/action (id = case id)
func AdminActionHandler(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var rc models.RecoveryCase
	if err := database.DB.First(&rc, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"message": "case not found"})
		return
	}
	var body struct {
		Action  string `json:"action" binding:"required"`
		Remarks string `json:"remarks"`
		Status  string `json:"status"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	note := models.CaseNote{CaseID: rc.ID, Author: "admin", Action: body.Action, Remarks: body.Remarks}
	database.DB.Create(&note)
	if body.Status != "" {
		rc.Status = body.Status
		database.DB.Save(&rc)
	}
	createNotification("employee", strconv.Itoa(int(rc.AssignedEmployeeID)), "Admin action on case #"+strconv.Itoa(int(rc.ID)),
		body.Action+": "+body.Remarks)
	audit.Record(c.GetString("admin_username"), "recovery.action", "recovery_case", strconv.Itoa(int(rc.ID)),
		body.Action+": "+body.Remarks)
	c.JSON(http.StatusCreated, note)
}
