package otp

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func SendOTPHandler(c *gin.Context) {

	var req SendOTPRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid mobile number",
		})
		return
	}

	otp, err := GenerateOTP(req.Mobile)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to generate OTP",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "OTP generated successfully",
		"otp": otp, // Remove this later when SMS service is integrated
	})
}

func VerifyOTPHandler(c *gin.Context) {

	var req VerifyOTPRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid request",
		})
		return
	}

	err := VerifyOTP(req.Mobile, req.OTP)

	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "OTP verified successfully",
	})
}