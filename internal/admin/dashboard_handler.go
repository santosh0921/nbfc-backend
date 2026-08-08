package admin

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/santosh0921/nbfc-backend/internal/constants"
	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/models"
)

func DashboardHandler(c *gin.Context) {

	var totalCustomers int64
	database.DB.Model(&models.User{}).Count(&totalCustomers)

	var approvedCount int64
	database.DB.Model(&models.CustomerKYCReview{}).
		Where("status = ?", constants.KYCApproved).
		Count(&approvedCount)

	var rejectedCount int64
	database.DB.Model(&models.CustomerKYCReview{}).
		Where("status = ?", constants.KYCRejected).
		Count(&rejectedCount)

	pendingCount := totalCustomers - approvedCount - rejectedCount

	var recentUsers []models.User
	database.DB.Order("created_at DESC").Limit(10).Find(&recentUsers)

	recent := make([]gin.H, 0, len(recentUsers))
	for _, u := range recentUsers {
		recent = append(recent, gin.H{
			"id":         u.ID,
			"mobile":     u.Mobile,
			"created_at": u.CreatedAt,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"total_customers": totalCustomers,
		"kyc_breakdown": gin.H{
			"pending":  pendingCount,
			"approved": approvedCount,
			"rejected": rejectedCount,
		},
		"recent_signups": recent,
	})
}
