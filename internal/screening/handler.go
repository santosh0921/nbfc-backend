package screening

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/models"
)

// AdminListHandler handles GET /admin/screening — the "Screening Results"
// admin panel screen. Defaults to only PENDING-decision rows (the
// compliance review queue) unless ?all=1 is passed.
func AdminListHandler(c *gin.Context) {
	q := database.DB.Model(&models.ScreeningRequest{})
	if c.Query("all") != "1" {
		q = q.Where("decision = ?", models.ScreeningDecisionPending)
	}
	var records []models.ScreeningRequest
	q.Order("screened_at desc").Limit(200).Find(&records)
	c.JSON(http.StatusOK, records)
}

// AdminRunHandler handles POST /admin/screening/:customerId/run — triggers
// (or re-runs) a screening check for a customer.
func AdminRunHandler(c *gin.Context) {
	customerID, err := uuid.Parse(c.Param("customerId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid customer id"})
		return
	}
	var body struct {
		FullName string `json:"fullName"`
	}
	c.ShouldBindJSON(&body)
	req, err := Screen(customerID, body.FullName)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Screening failed"})
		return
	}
	c.JSON(http.StatusOK, req)
}

// AdminDecideHandler handles POST /admin/screening/:requestId/decide —
// requires SCREENING_DECIDE (compliance-tier roles only). An ordinary
// credit/loan officer holding only SCREENING_VIEW cannot reach this route
// at all — enforced by the RequirePermission gate in routes.go, not here.
func AdminDecideHandler(c *gin.Context) {
	requestID, err := uuid.Parse(c.Param("requestId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid request id"})
		return
	}
	var body struct {
		Decision string `json:"decision" binding:"required"` // CLEARED | REJECTED
		Reason   string `json:"reason"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	if body.Decision != models.ScreeningDecisionCleared && body.Decision != models.ScreeningDecisionRejected {
		c.JSON(http.StatusBadRequest, gin.H{"message": "decision must be CLEARED or REJECTED"})
		return
	}

	adminID, _ := c.Get("admin_id")
	id, _ := adminID.(uint)
	req, err := Decide(c, requestID, id, c.GetString("admin_username"), body.Decision, body.Reason)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Failed to record decision"})
		return
	}
	c.JSON(http.StatusOK, req)
}
