package complaints

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/models"
)

func currentUserID(c *gin.Context) (string, bool) {
	mobile := c.GetString("mobile")
	if mobile == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"message": "Unauthorized"})
		return "", false
	}
	var user models.User
	if err := database.DB.Where("mobile = ?", mobile).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"message": "User not found"})
		return "", false
	}
	return user.ID.String(), true
}

// CreateComplaintHandler handles POST /auth/complaints — the real backend
// the customer app's "Raise a Complaint" form should have been calling
// all along.
func CreateComplaintHandler(c *gin.Context) {
	customerID, ok := currentUserID(c)
	if !ok {
		return
	}
	var body struct {
		Category    string `json:"category" binding:"required"`
		Description string `json:"description" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}

	customerUUID, err := uuid.Parse(customerID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Invalid customer id"})
		return
	}

	complaint := models.Complaint{
		CustomerID:  customerUUID,
		Category:    body.Category,
		Description: body.Description,
		Status:      models.ComplaintStatusOpen,
	}
	if err := database.DB.Create(&complaint).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Failed to submit complaint"})
		return
	}
	c.JSON(http.StatusCreated, complaint)
}

// ListMyComplaintsHandler handles GET /auth/complaints — the customer's
// own complaint history, newest first, backing the "My Complaints" tab.
func ListMyComplaintsHandler(c *gin.Context) {
	customerID, ok := currentUserID(c)
	if !ok {
		return
	}
	// Non-nil start — see loans.ListMyLoansHandler's fix note: a nil slice
	// marshals to JSON `null`, which the client's list decoder throws on.
	complaints := []models.Complaint{}
	database.DB.Where("customer_id = ?", customerID).Order("created_at desc").Find(&complaints)
	c.JSON(http.StatusOK, complaints)
}

// AdminListComplaintsHandler handles GET /admin/complaints. Enriched with
// the customer's name/mobile — same reasoning as
// internal/support.AdminListThreadsHandler: an admin scanning a list of
// complaints needs to know who filed each one, not just a raw customer ID.
func AdminListComplaintsHandler(c *gin.Context) {
	var complaints []models.Complaint
	database.DB.Order("created_at desc").Find(&complaints)

	type complaintSummary struct {
		models.Complaint
		CustomerName   string `json:"customerName"`
		CustomerMobile string `json:"customerMobile"`
	}
	out := make([]complaintSummary, 0, len(complaints))
	for _, cp := range complaints {
		var user models.User
		customerName := ""
		customerMobile := ""
		if err := database.DB.First(&user, "id = ?", cp.CustomerID).Error; err == nil {
			customerMobile = user.Mobile
			var profile models.CustomerProfile
			if err := database.DB.Where("user_id = ?", cp.CustomerID).First(&profile).Error; err == nil {
				customerName = strings.TrimSpace(profile.FirstName + " " + profile.LastName)
			}
		}
		out = append(out, complaintSummary{
			Complaint:      cp,
			CustomerName:   customerName,
			CustomerMobile: customerMobile,
		})
	}
	c.JSON(http.StatusOK, out)
}

// AdminUpdateComplaintHandler handles PUT /admin/complaints/:id — sets the
// status and/or an admin-facing resolution note. ResolvedAt is stamped
// exactly once, the first time a complaint transitions into
// resolved/closed — a later status edit back to open never clears it,
// preserving "when was this first resolved" even if it gets reopened.
func AdminUpdateComplaintHandler(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid complaint id"})
		return
	}
	var complaint models.Complaint
	if err := database.DB.First(&complaint, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"message": "Complaint not found"})
		return
	}

	var body struct {
		Status    string `json:"status" binding:"required"`
		AdminNote string `json:"adminNote"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	switch body.Status {
	case models.ComplaintStatusOpen, models.ComplaintStatusInProgress, models.ComplaintStatusResolved, models.ComplaintStatusClosed:
	default:
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid status"})
		return
	}

	updates := map[string]interface{}{
		"status":     body.Status,
		"admin_note": body.AdminNote,
	}
	isResolving := (body.Status == models.ComplaintStatusResolved || body.Status == models.ComplaintStatusClosed) && complaint.ResolvedAt == nil
	if isResolving {
		now := time.Now()
		updates["resolved_at"] = now
	}
	if err := database.DB.Model(&complaint).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Failed to update complaint"})
		return
	}
	database.DB.First(&complaint, id)
	c.JSON(http.StatusOK, complaint)
}
