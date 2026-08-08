package auth

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/models"
	"github.com/santosh0921/nbfc-backend/internal/otp"
)

func ForgotMPINHandler(c *gin.Context) {

	var req ForgotMPINRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"message": "Invalid Request",
		})
		return
	}

	if req.NewMPIN != req.ConfirmMPIN {
	    c.JSON(http.StatusBadRequest, gin.H{
		    "message": "New MPIN and Confirm MPIN do not match",
	    })
	    return
    }

	var otpRecord otp.OTPVerification

err := database.DB.
	Where("mobile = ? AND verified = ?", req.Mobile, true).
	First(&otpRecord).Error

if err != nil {

	if err == gorm.ErrRecordNotFound {
		c.JSON(http.StatusUnauthorized, gin.H{
			"message": "Please verify OTP first",
		})
		return
	}

	c.JSON(http.StatusInternalServerError, gin.H{
		"message": "Database error",
	})	
	return
}

var user models.User

err = database.DB.
	Where("mobile = ?", req.Mobile).
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

if err := database.DB.Delete(&otpRecord).Error; err != nil {
	c.JSON(http.StatusInternalServerError, gin.H{
		"message": "Unable to delete OTP",
	})
	return
}

c.JSON(http.StatusOK, gin.H{
	"message": "MPIN reset successfully",
})

}