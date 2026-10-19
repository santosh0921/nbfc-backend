package admin

import (
	"net/http"
	"strings"

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
	// KYCStatus is the real pipeline position (internal/models.KYCStatus*
	// state machine) — distinct from KYCReviewStatus above, which only
	// reflects an admin decision and defaults to PENDING regardless of
	// how far along the customer actually is.
	KYCStatus string `json:"kyc_status"`
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
	search := strings.TrimSpace(c.Query("search"))

	var total int64
	var users []models.User

	if search != "" {
		// No single column covers "search" — mobile lives on User, name
		// lives on CustomerProfile (only created once a customer reaches
		// the profile step of KYC, so a customer who's only applied for a
		// loan may have no profile row at all and is findable by mobile
		// only). Resolve the matching id set from both, dedupe, then
		// paginate over that in-memory set — the customer base is small
		// enough (low thousands, per the Phase 15 note above) that this
		// is simple and fast without needing a real search index or a
		// SQL-level UNION.
		like := "%" + search + "%"

		var mobileIDs []uuid.UUID
		database.DB.Model(&models.User{}).Where("mobile ILIKE ?", like).Pluck("id", &mobileIDs)

		var nameIDs []uuid.UUID
		database.DB.Model(&models.CustomerProfile{}).
			Where("first_name ILIKE ? OR last_name ILIKE ? OR (first_name || ' ' || last_name) ILIKE ?", like, like, like).
			Pluck("user_id", &nameIDs)

		// LoanApplication.ApplicantName/ApplicantPhone are free-text fields
		// typed on the loan form — entirely separate from User.Mobile (the
		// authenticated login identity) and CustomerProfile's name. A
		// customer can type a different contact number or a name spelling
		// on their application than what they actually signed up/logged in
		// with (confirmed real-world case: an admin searching the exact
		// phone number shown on a customer's own loan detail page got zero
		// matches, because that number was never the customer's User.Mobile
		// to begin with). Searching loans too, and resolving to their
		// CustomerID, closes that gap.
		var loanCustomerIDs []uuid.UUID
		database.DB.Model(&models.LoanApplication{}).
			Where("applicant_name ILIKE ? OR applicant_phone ILIKE ?", like, like).
			Pluck("customer_id", &loanCustomerIDs)

		seen := make(map[uuid.UUID]bool, len(mobileIDs)+len(nameIDs)+len(loanCustomerIDs))
		matchingIDs := make([]uuid.UUID, 0, len(mobileIDs)+len(nameIDs)+len(loanCustomerIDs))
		for _, id := range mobileIDs {
			if !seen[id] {
				seen[id] = true
				matchingIDs = append(matchingIDs, id)
			}
		}
		for _, id := range nameIDs {
			if !seen[id] {
				seen[id] = true
				matchingIDs = append(matchingIDs, id)
			}
		}
		for _, id := range loanCustomerIDs {
			if !seen[id] {
				seen[id] = true
				matchingIDs = append(matchingIDs, id)
			}
		}

		total = int64(len(matchingIDs))
		start := p.Offset()
		if start > len(matchingIDs) {
			start = len(matchingIDs)
		}
		end := start + p.Limit
		if end > len(matchingIDs) {
			end = len(matchingIDs)
		}
		pageIDs := matchingIDs[start:end]

		if len(pageIDs) > 0 {
			if err := database.DB.Where("id IN ?", pageIDs).Order("created_at DESC").Find(&users).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"message": "Failed to search users"})
				return
			}
		}
	} else {
		if err := database.DB.Model(&models.User{}).Count(&total).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "Failed to count users"})
			return
		}

		if err := database.DB.
			Order("created_at DESC").
			Limit(p.Limit).Offset(p.Offset()).
			Find(&users).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "Failed to fetch users"})
			return
		}
	}

	ids := make([]uuid.UUID, 0, len(users))
	for _, u := range users {
		ids = append(ids, u.ID)
	}

	// --- Batch-load every related table once for the whole page ---

	profiles := map[uuid.UUID]models.CustomerProfile{}
	addresses := map[uuid.UUID]models.CustomerAddress{}
	reviews := map[uuid.UUID]models.CustomerKYCReview{}
	kycRecords := map[uuid.UUID]models.CustomerKYC{}
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

		// kyc_review_status (above) only reflects an admin's approve/reject
		// decision and defaults to PENDING for every customer who hasn't
		// been reviewed yet — including someone at 20% completion with no
		// documents submitted. That reads as "ready for verification" to
		// anyone glancing at the badge, which it is not. kycStatus is the
		// real CustomerKYC state machine position (NOT_STARTED through
		// VERIFIED) and is what actually answers "where is this customer
		// in the pipeline".
		var kycs []models.CustomerKYC
		database.DB.Where("customer_id IN ?", ids).Find(&kycs)
		for _, x := range kycs {
			kycRecords[x.CustomerID] = x
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
			KYCStatus:        models.KYCStatusNotStarted,
		}
		if k, ok := kycRecords[user.ID]; ok {
			item.KYCStatus = k.Status
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
