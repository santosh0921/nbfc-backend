package admin

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/santosh0921/nbfc-backend/internal/constants"
	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/models"
	"github.com/santosh0921/nbfc-backend/internal/pagination"
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

// ListUsersHandler returns one page of customers.
//
// PHASE 15 REWRITE — this handler was the single worst scale bottleneck
// in the application. It previously loaded EVERY user with an unbounded
// Find, then issued 8 further queries per user (profile, address, KYC
// review, plus 5 more inside computeKYCCompletion). At 2,000 seeded
// customers that measured p50 = 12.97 SECONDS per request (0.8 req/sec)
// — and the cost is linear in customer count, so at 100,000 customers it
// would be roughly fifty times worse and would simply time out.
//
// Two changes, both necessary — pagination alone would not have fixed
// the per-row fan-out, and de-N+1-ing alone would still have loaded
// every row:
//
//  1. Pagination (item F): a bounded page, with a hard maximum, so the
//     response size no longer grows with the customer base.
//  2. Batch loading (item D): the related rows for the whole page are
//     fetched in a fixed SIX queries total, regardless of page size,
//     instead of eight per user.
//
// Total queries per request went from (1 + 8N) to (1 count + 1 page + 6
// batch) = 8, independent of N.
func ListUsersHandler(c *gin.Context) {
	p := pagination.FromQuery(c)

	var total int64
	if err := database.DB.Model(&models.User{}).Count(&total).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Failed to count users"})
		return
	}

	var users []models.User
	if err := database.DB.
		Order("created_at DESC").
		Limit(p.Limit).Offset(p.Offset()).
		Find(&users).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Failed to fetch users"})
		return
	}

	ids := make([]uuid.UUID, 0, len(users))
	for _, u := range users {
		ids = append(ids, u.ID)
	}

	// --- Batch-load every related table once for the whole page ---

	profiles := map[uuid.UUID]models.CustomerProfile{}
	addresses := map[uuid.UUID]models.CustomerAddress{}
	reviews := map[uuid.UUID]models.CustomerKYCReview{}
	// Verification flags feed the completion score; only the boolean
	// matters here, so these carry just the user id and its flag.
	panVerified := map[uuid.UUID]bool{}
	aadhaarVerified := map[uuid.UUID]bool{}
	selfieVerified := map[uuid.UUID]bool{}

	if len(ids) > 0 {
		var ps []models.CustomerProfile
		database.DB.Where("user_id IN ?", ids).Find(&ps)
		for _, x := range ps {
			profiles[x.UserID] = x
		}

		var as []models.CustomerAddress
		database.DB.Where("user_id IN ?", ids).Find(&as)
		for _, x := range as {
			addresses[x.UserID] = x
		}

		var rs []models.CustomerKYCReview
		database.DB.Where("user_id IN ?", ids).Find(&rs)
		for _, x := range rs {
			reviews[x.UserID] = x
		}

		var pans []models.CustomerPAN
		database.DB.Where("user_id IN ?", ids).Find(&pans)
		for _, x := range pans {
			panVerified[x.UserID] = x.IsVerified
		}

		var aadhaars []models.CustomerAadhaar
		database.DB.Where("user_id IN ?", ids).Find(&aadhaars)
		for _, x := range aadhaars {
			aadhaarVerified[x.UserID] = x.IsVerified
		}

		var selfies []models.CustomerSelfie
		database.DB.Where("user_id IN ?", ids).Find(&selfies)
		for _, x := range selfies {
			selfieVerified[x.UserID] = x.IsVerified
		}
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
		if pr, ok := profiles[user.ID]; ok {
			item.FirstName = pr.FirstName
			item.LastName = pr.LastName
		}
		if ad, ok := addresses[user.ID]; ok {
			item.City = ad.CurrentCity
		}
		if rv, ok := reviews[user.ID]; ok {
			item.KYCReviewStatus = rv.Status
		}

		// Same 5-part scoring as computeKYCCompletion below — kept
		// deliberately identical so the list and the detail view can
		// never disagree about what "100% complete" means.
		score := 0
		if _, ok := profiles[user.ID]; ok {
			score++
		}
		if _, ok := addresses[user.ID]; ok {
			score++
		}
		if panVerified[user.ID] {
			score++
		}
		if aadhaarVerified[user.ID] {
			score++
		}
		if selfieVerified[user.ID] {
			score++
		}
		item.CompletionPercentage = score * 100 / 5

		items = append(items, item)
	}

	c.JSON(http.StatusOK, gin.H{
		"users": items,
		// "total" previously meant "rows in this response", which was
		// also the whole table. It now means the full result-set size,
		// with the page's own size available as pagination.count — the
		// two are different numbers once paginated and conflating them
		// would misreport the customer base.
		"total":      total,
		"pagination": p.Meta(total, len(items)),
	})
}

// computeKYCCompletion mirrors the scoring in profile.GetKYCStatusHandler
// (profile + address + verified PAN + verified Aadhaar + verified selfie,
// each worth 20%) so the admin list and the customer app agree on what
// "100% complete" means.
//
// Retained for the SINGLE-user detail handler, where 5 queries for one
// user is appropriate. The list handler deliberately does not call this
// — see the batch-loading note above.
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
