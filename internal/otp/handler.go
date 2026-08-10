package otp

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/santosh0921/nbfc-backend/internal/config"
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

	resp := gin.H{
		"success": true,
		"message": "OTP generated successfully",
	}
	// The OTP itself is never returned outside DEMO_MODE — no SMS provider
	// is wired up yet, so this is the only way to test the OTP flow without
	// one, but it must never ship in a response the production app can see.
	// (See internal/config.DemoMode.)
	if config.DemoMode {
		resp["otp"] = otp
	}
	c.JSON(http.StatusOK, resp)
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