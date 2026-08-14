package admin

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/santosh0921/nbfc-backend/internal/constants"
	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/models"
)

type UserListItem struct {
	ID                   string `json:"id"`
	Mobile               string `json:"mobile"`
	FirstName            string `json:"first_name"`
	LastName             string `json:"last_name"`
	IsMobileVerified     bool   `json:"is_mobile_verified"`
	IsActive             bool   `json:"is_active"`
	CompletionPercentage int    `json:"completion_percentage"`
	KYCReviewStatus      string `json:"kyc_review_status"`
	// City is the customer's current-residence city from their KYC
	// address record (models.CustomerAddress), the only place a
	// customer-level city is actually captured — neither User nor
	// CustomerProfile has one, and LoanApplication.City is per-loan, not
	// per-customer, so it can't answer "what city is this customer in"
	// for someone with zero or multiple loans. Empty until the customer
	// completes the address step of KYC.
	City      string `json:"city"`
	CreatedAt string `json:"created_at"`
}

func ListUsersHandler(c *gin.Context) {

	var users []models.User

	if err := database.DB.Order("created_at DESC").Find(&users).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": "Failed to fetch users",
		})
		return
	}

	items := make([]UserListItem, 0, len(users))

	for _, user := range users {

		item := UserListItem{
			ID:               user.ID.String(),
			Mobile:           user.Mobile,
			IsMobileVerified: user.IsMobileVerified,
			IsActive:         user.IsActive,
			CreatedAt:        user.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
			KYCReviewStatus:  constants.KYCPending,
		}

		var profile models.CustomerProfile
		if err := database.DB.Where("user_id = ?", user.ID).First(&profile).Error; err == nil {
			item.FirstName = profile.FirstName
			item.LastName = profile.LastName
		}

		var address models.CustomerAddress
		if err := database.DB.Where("user_id = ?", user.ID).First(&address).Error; err == nil {
			item.City = address.CurrentCity
		}

		item.CompletionPercentage = computeKYCCompletion(user.ID)

		var review models.CustomerKYCReview
		if err := database.DB.Where("user_id = ?", user.ID).First(&review).Error; err == nil {
			item.KYCReviewStatus = review.Status
		}

		items = append(items, item)
	}

	c.JSON(http.StatusOK, gin.H{
		"users": items,
		"total": len(items),
	})
}

// computeKYCCompletion mirrors the scoring in profile.GetKYCStatusHandler
// (profile + address + verified PAN + verified Aadhaar + verified selfie,
// each worth 20%) so the admin list and the customer app agree on what
// "100% complete" means.
func computeKYCCompletion(userID uuid.UUID) int {
	score := 0

	var profile models.CustomerProfile
	if err := database.DB.Where("user_id = ?", userID).First(&profile).Error; err == nil {
		score++
	}

	var address models.CustomerAddress
	if err := database.DB.Where("user_id = ?", userID).First(&address).Error; err == nil {
		score++
	}

	var pan models.CustomerPAN
	if err := database.DB.Where("user_id = ?", userID).First(&pan).Error; err == nil && pan.IsVerified {
		score++
	}

	var aadhaar models.CustomerAadhaar
	if err := database.DB.Where("user_id = ?", userID).First(&aadhaar).Error; err == nil && aadhaar.IsVerified {
		score++
	}

	var selfie models.CustomerSelfie
	if err := database.DB.Where("user_id = ?", userID).First(&selfie).Error; err == nil && selfie.IsVerified {
		score++
	}

	return score * 100 / 5
}
