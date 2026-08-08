package auth

import (
	"net/http"

	"github.com/gin-gonic/gin"

    "gorm.io/gorm"
	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/models"
)

func GetProfile(c *gin.Context) {

	mobile, exists := c.Get("mobile")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"message": "Unauthorized",
		})
		return
	}

    var user models.User

	err := database.DB.
	Where("mobile = ?", mobile).
	First(&user).Error


	if err != nil {

	if err == gorm.ErrRecordNotFound {
		c.JSON(http.StatusNotFound, gin.H{
			"message": "User not found",
		})
		return
	}

	c.JSON(http.StatusInternalServerError, gin.H{
		"message": "Database error",
	})
	return
}

	c.JSON(http.StatusOK, gin.H{
	"message": "Profile fetched successfully",
	"user": gin.H{
		"id":                 user.ID,
		"mobile":             user.Mobile,
		"is_mobile_verified": user.IsMobileVerified,
		"biometric_enabled":  user.BiometricEnabled,
		"is_active":          user.IsActive,
	},
})
}
func ToggleBiometric(c *gin.Context) {

	var req BiometricRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{
			"message": "Invalid Request",
		})
		return
	}

	mobile := c.GetString("mobile")

	var user models.User

	if err := database.DB.Where("mobile = ?", mobile).First(&user).Error; err != nil {
		
		if err == gorm.ErrRecordNotFound {
	c.JSON(http.StatusNotFound, gin.H{
		"message": "User not found",
	})
	return
}

c.JSON(http.StatusInternalServerError, gin.H{
	"message": "Database error",
})
return
	}

	user.BiometricEnabled = req.Enabled

	if err := database.DB.Save(&user).Error; err != nil {
		c.JSON(500, gin.H{
			"message": "Unable to update biometric",
		})
		return
	}

	c.JSON(200, gin.H{
		"message": "Biometric updated successfully",
		"enabled": user.BiometricEnabled,
	})
}