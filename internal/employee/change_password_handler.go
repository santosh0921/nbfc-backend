package employee

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"

	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/models"
)

// ChangePasswordHandler handles POST /employee/change-password — requires
// the current password (same as login) before setting a new one, so a
// stolen but still-logged-in session can't silently lock the real owner
// out.
func ChangePasswordHandler(c *gin.Context) {
	empID := c.GetUint("employee_id")

	var req struct {
		CurrentPassword string `json:"currentPassword" binding:"required"`
		NewPassword     string `json:"newPassword" binding:"required,min=6"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Enter your current password and a new password (min 6 characters)."})
		return
	}

	var emp models.Employee
	if err := database.DB.First(&emp, empID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"message": "Employee not found"})
		return
	}

	if bcrypt.CompareHashAndPassword([]byte(emp.PasswordHash), []byte(req.CurrentPassword)) != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"message": "Current password is incorrect"})
		return
	}

	newHash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Could not set new password"})
		return
	}

	emp.PasswordHash = string(newHash)
	database.DB.Save(&emp)
	c.JSON(http.StatusOK, gin.H{"message": "Password updated successfully"})
}
