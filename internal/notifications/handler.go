package notifications

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/models"
)

// resolveRecipient inspects context values set by whichever auth middleware
// ran (customer or employee JWT) to infer the notification recipient.
func resolveRecipient(c *gin.Context) (role string, userID string, ok bool) {
	if empID := c.GetUint("employee_id"); empID != 0 {
		return "employee", strconv.Itoa(int(empID)), true
	}
	if mobile := c.GetString("mobile"); mobile != "" {
		var user models.User
		if err := database.DB.Where("mobile = ?", mobile).First(&user).Error; err == nil {
			return "customer", user.ID.String(), true
		}
	}
	return "", "", false
}

// ListHandler handles GET /notifications
func ListHandler(c *gin.Context) {
	role, userID, ok := resolveRecipient(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"message": "Unauthorized"})
		return
	}
	var notes []models.Notification
	database.DB.Where("role = ? AND user_id = ?", role, userID).Order("created_at desc").Find(&notes)
	c.JSON(http.StatusOK, notes)
}

// MarkReadHandler handles POST /notifications/:id/read
func MarkReadHandler(c *gin.Context) {
	role, userID, ok := resolveRecipient(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"message": "Unauthorized"})
		return
	}
	id, _ := strconv.Atoi(c.Param("id"))
	var n models.Notification
	if err := database.DB.Where("id = ? AND role = ? AND user_id = ?", id, role, userID).First(&n).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"message": "Not found"})
		return
	}
	n.Read = true
	database.DB.Save(&n)
	c.JSON(http.StatusOK, n)
}
