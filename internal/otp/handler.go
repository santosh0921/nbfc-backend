package otp

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/santosh0921/nbfc-backend/internal/config"
	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/models"
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

	// accountExists tells the app whether this mobile already has an MPIN
	// set, so it can route to "enter your existing MPIN" instead of
	// "create a new MPIN" — without this, a returning user who logs out
	// and logs back in with the same number was silently sent through the
	// create-MPIN screen, which overwrites their real MPIN with whatever
	// they type next, rather than just asking them to enter it. Safe to
	// reveal only here, immediately after OTP verification already proved
	// possession of this phone number — not an unauthenticated enumeration
	// endpoint.
	accountExists := false
	var user models.User
	if err := database.DB.Where("mobile = ?", req.Mobile).First(&user).Error; err == nil {
		accountExists = user.MPIN != ""
	}

	c.JSON(http.StatusOK, gin.H{
		"success":       true,
		"message":       "OTP verified successfully",
		"accountExists": accountExists,
	})
}