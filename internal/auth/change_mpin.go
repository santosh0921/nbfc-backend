package auth

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"gorm.io/gorm"

	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/models"
)

func ChangeMPINHandler(c *gin.Context) {

	var req ChangeMPINRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"message": "Invalid Request",
		})
		return
	}

	mobile := c.GetString("mobile")

    if mobile == "" {
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

if !CheckMPIN(user.MPIN, req.CurrentMPIN) {
	c.JSON(http.StatusUnauthorized, gin.H{
		"message": "Current MPIN is incorrect",
	})
	return
}

if req.NewMPIN != req.ConfirmMPIN {
	c.JSON(http.StatusBadRequest, gin.H{
		"message": "New MPIN and Confirm MPIN do not match",
	})
	return
}

if req.CurrentMPIN == req.NewMPIN {
	c.JSON(http.StatusBadRequest, gin.H{
		"message": "New MPIN cannot be the same as the current MPIN",
	})
	return
}

hashedMPIN, err := HashMPIN(req.NewMPIN)
if err != nil {
	c.JSON(http.StatusInternalServerError, gin.H{
		"message": "Unable to hash MPIN",
	})
	return
}

if err := database.DB.Model(&user).Update("mpin", hashedMPIN).Error; err != nil {
	c.JSON(http.StatusInternalServerError, gin.H{
		"message": "Unable to update MPIN",
	})
	return
}
c.JSON(http.StatusOK, gin.H{
	"message": "MPIN changed successfully",
})

}