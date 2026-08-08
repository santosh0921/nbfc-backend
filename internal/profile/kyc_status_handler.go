package profile

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/santosh0921/nbfc-backend/internal/constants"
	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/models"
)

func GetKYCStatusHandler(c *gin.Context) {

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
		c.JSON(http.StatusNotFound, gin.H{
			"message": "User not found",
		})
		return
	}

	response := KYCStatusResponse{}

	score := 0


	// ---------------- Profile ----------------

var profile models.CustomerProfile

if err := database.DB.
	Where("user_id = ?", user.ID).
	First(&profile).Error; err == nil {

	response.Profile.Completed = true
	response.Profile.Status = constants.VerificationVerified
	score++
} else {
	response.Profile.Completed = false
	response.Profile.Status = constants.VerificationPending
}

	// ---------------- Address ----------------

	
var address models.CustomerAddress

if err := database.DB.
	Where("user_id = ?", user.ID).
	First(&address).Error; err == nil {

	response.Address.Completed = true
	response.Address.Status = constants.VerificationVerified
	score++
} else {
	response.Address.Completed = false
	response.Address.Status = constants.VerificationPending
}

	// ---------------- PAN ----------------

var pan models.CustomerPAN

if err := database.DB.
	Where("user_id = ?", user.ID).
	First(&pan).Error; err == nil {

	response.PAN.Completed = true
	response.PAN.Status = pan.VerificationStatus

	if pan.IsVerified {
		score++
	} else {
		response.PAN.Reason = pan.VerificationFailureReason
	}

} else {

	response.PAN.Completed = false
	response.PAN.Status = constants.VerificationPending
}

// ---------------- Aadhaar ----------------

var aadhaar models.CustomerAadhaar

if err := database.DB.
	Where("user_id = ?", user.ID).
	First(&aadhaar).Error; err == nil {

	response.Aadhaar.Completed = true
	response.Aadhaar.Status = aadhaar.VerificationStatus

	if aadhaar.IsVerified {
		score++
	} else {
		response.Aadhaar.Reason = aadhaar.VerificationFailureReason
	}

} else {

	response.Aadhaar.Completed = false
	response.Aadhaar.Status = constants.VerificationPending
}	

// ---------------- Selfie ----------------

var selfie models.CustomerSelfie

if err := database.DB.
	Where("user_id = ?", user.ID).
	First(&selfie).Error; err == nil {

	response.Selfie.Completed = true
	response.Selfie.Status = selfie.VerificationStatus

	if selfie.IsVerified {
		score++
	} else {
		response.Selfie.Reason = selfie.VerificationFailureReason
	}

} else {

	response.Selfie.Completed = false
	response.Selfie.Status = constants.VerificationPending
}

	response.CompletionPercentage = score * 100 / 5

if response.CompletionPercentage == 100 {
	response.OverallKYC = constants.KYCApproved
} else {
	response.OverallKYC = constants.KYCPending
}

// Decide which screen user should complete next

switch {

case !response.Profile.Completed:
	response.NextStep = "PROFILE"

case !response.Address.Completed:
	response.NextStep = "ADDRESS"

case response.PAN.Status != constants.VerificationVerified:
	response.NextStep = "PAN"

case response.Aadhaar.Status != constants.VerificationVerified:
	response.NextStep = "AADHAAR"

case response.Selfie.Status != constants.VerificationVerified:
	response.NextStep = "SELFIE"

default:
	response.NextStep = "COMPLETED"
}

c.JSON(http.StatusOK, response)
}