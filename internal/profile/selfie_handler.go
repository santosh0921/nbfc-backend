package profile

import (
	"encoding/base64"
	"log"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/kyc"
	"github.com/santosh0921/nbfc-backend/internal/models"
	"github.com/santosh0921/nbfc-backend/internal/storage"
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
		ID:     uuid.New(),
		UserID: user.ID,

		VerificationStatus: verificationResult.Status,
		IsVerified: verificationResult.IsVerified,

		VerificationProvider:      verificationResult.Provider,
		VerificationReference:     verificationResult.Reference,
		VerificationFailureReason: verificationResult.FailureReason,
		VerificationRemarks:       verificationResult.Remarks,
	}

	trimmed := strings.TrimSpace(req.Image)
	if idx := strings.Index(trimmed, ","); idx != -1 && strings.HasPrefix(trimmed, "data:") {
		trimmed = trimmed[idx+1:]
	}
	decoded, decodeErr := base64.StdEncoding.DecodeString(trimmed)
	if decodeErr != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Image is not valid base64"})
		return
	}

	if provider := storage.Active(); provider != nil {
		ref, storeErr := provider.Store(user.ID, selfie.ID, decoded)
		if storeErr != nil {
			log.Println("selfie storage.Store failed:", storeErr)
			c.JSON(http.StatusInternalServerError, gin.H{"message": "Unable to save selfie"})
			return
		}
		selfie.StorageReference = ref
	} else {
		// Development-only fallback — no encrypted storage provider
		// configured. Never reachable in production (fail-closed at
		// startup, same guarantee as internal/documents).
		selfie.ImagePath = req.Image
	}

	if err := database.DB.Create(&selfie).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": "Unable to save selfie",
		})
		return
	}

	if _, err := kyc.Recompute(user.ID, "customer", mobile); err != nil {
		log.Println("kyc.Recompute after selfie upload failed:", err)
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Selfie uploaded successfully",
		"verification": gin.H{
			"status":      selfie.VerificationStatus,
			"is_verified": selfie.IsVerified,
		},
	})
}