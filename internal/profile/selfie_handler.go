package profile

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/models"
	"github.com/santosh0921/nbfc-backend/internal/verification"
)

func CreateSelfieHandler(c *gin.Context) {

	var req SelfieRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"message": err.Error(),
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

	var selfie models.CustomerSelfie

	err = database.DB.
		Where("user_id = ?", user.ID).
		First(&selfie).Error

	if err == nil {
		c.JSON(http.StatusConflict, gin.H{
			"message": "Selfie already uploaded",
		})
		return
	}

	if err != gorm.ErrRecordNotFound {
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": "Database error",
		})
		return
	}

	verificationResult := verification.VerifySelfie(req.Image)

	selfie = models.CustomerSelfie{
		UserID: user.ID,

		ImagePath: req.Image,

		VerificationStatus: verificationResult.Status,
		IsVerified: verificationResult.IsVerified,

		VerificationProvider:      verificationResult.Provider,
		VerificationReference:     verificationResult.Reference,
		VerificationFailureReason: verificationResult.FailureReason,
		VerificationRemarks:       verificationResult.Remarks,
	}

	if err := database.DB.Create(&selfie).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": "Unable to save selfie",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Selfie uploaded successfully",
		"verification": gin.H{
			"status":      selfie.VerificationStatus,
			"is_verified": selfie.IsVerified,
		},
	})
}