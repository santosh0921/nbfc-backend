package admin

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/santosh0921/nbfc-backend/internal/constants"
	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/models"
)

type RejectRequest struct {
	Reason string `json:"reason" binding:"required"`
}

func ApproveKYCHandler(c *gin.Context) {
	setKYCReviewStatus(c, constants.KYCApproved, "")
}

func RejectKYCHandler(c *gin.Context) {

	var req RejectRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"message": "Rejection reason is required",
		})
		return
	}

	setKYCReviewStatus(c, constants.KYCRejected, req.Reason)
}

func setKYCReviewStatus(c *gin.Context, status string, reason string) {

	idParam := c.Param("id")

	userID, err := uuid.Parse(idParam)

	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"message": "Invalid user id",
		})
		return
	}

	var user models.User

	if err := database.DB.First(&user, "id = ?", userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"message": "User not found",
		})
		return
	}

	adminUsername := c.GetString("admin_username")
	now := time.Now()

	var review models.CustomerKYCReview

	err = database.DB.Where("user_id = ?", userID).First(&review).Error

	if err != nil {
		review = models.CustomerKYCReview{
			UserID: userID,
		}
	}

	review.Status = status
	review.ReviewedBy = adminUsername
	review.ReviewedAt = &now
	review.RejectionReason = reason

	if saveErr := database.DB.Save(&review).Error; saveErr != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": "Failed to update KYC review",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":     "KYC review updated",
		"kyc_review":  review,
	})
}
