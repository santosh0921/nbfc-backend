package kyc

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/santosh0921/nbfc-backend/internal/audit"
	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/models"
	"github.com/santosh0921/nbfc-backend/internal/notifications"
)

// AdminSendVerificationRequestHandler handles
// POST /admin/kyc/customers/:customerId/verification-requests — a KYC
// officer, looking at a specific customer's KYC case, asks that customer
// to schedule a video-verification call. Creates the request row plus a
// push + in-app notification; the customer app surfaces this and lets the
// customer say "I'm ready" with a preferred time (see
// CustomerRespondVerificationRequestHandler).
func AdminSendVerificationRequestHandler(c *gin.Context) {
	customerID, err := uuid.Parse(c.Param("customerId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid customer id"})
		return
	}

	var customer models.User
	if err := database.DB.First(&customer, "id = ?", customerID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"message": "Customer not found"})
		return
	}

	req := models.KycVerificationRequest{
		CustomerID:           customerID.String(),
		RequestedByAdminID:   toAdminIDCtx(c),
		RequestedByAdminName: c.GetString("admin_username"),
		Status:               "pending",
	}
	if err := database.DB.Create(&req).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Failed to create verification request"})
		return
	}

	notifications.Create(
		"customer",
		customerID.String(),
		"Video verification requested",
		"OneFinance needs a quick video call to verify your documents. Open the app to confirm you're ready.",
	)

	audit.RecordDetailed(c, audit.Entry{
		ActorType: "admin", ActorID: toAdminIDCtx(c), ActorRef: c.GetString("admin_username"), ActorRole: c.GetString("admin_role"),
		Action: "kyc.verification_request.send", TargetType: "customer_kyc", TargetID: customerID.String(),
		Result: audit.ResultSuccess, After: req,
	})

	c.JSON(http.StatusCreated, req)
}

// AdminListVerificationRequestsHandler handles
// GET /admin/kyc/customers/:customerId/verification-requests — the KYC
// officer's view of every verification-request/response for this
// customer, newest first, so a prior "I'm ready" response is visible on
// the customer's detail page before the officer starts the video call.
func AdminListVerificationRequestsHandler(c *gin.Context) {
	customerID, err := uuid.Parse(c.Param("customerId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid customer id"})
		return
	}
	reqs := []models.KycVerificationRequest{}
	database.DB.Where("customer_id = ?", customerID.String()).Order("created_at desc").Find(&reqs)
	c.JSON(http.StatusOK, reqs)
}

// CustomerListVerificationRequestsHandler handles
// GET /kyc/verification-requests — the customer app's own view of the
// video-verification requests sent to them, newest first.
func CustomerListVerificationRequestsHandler(c *gin.Context) {
	user, ok := currentUser(c)
	if !ok {
		return
	}
	reqs := []models.KycVerificationRequest{}
	database.DB.Where("customer_id = ?", user.ID.String()).Order("created_at desc").Find(&reqs)
	c.JSON(http.StatusOK, reqs)
}

type respondVerificationRequestBody struct {
	Ready         bool   `json:"ready"`
	PreferredTime string `json:"preferredTime"`
}

// CustomerRespondVerificationRequestHandler handles
// POST /kyc/verification-requests/:id/respond — the customer's "I'm
// ready" (or decline) reply to a specific request. A customer may only
// respond to their own request — verified against the authenticated
// customer's own ID, not merely trusted from the request body.
func CustomerRespondVerificationRequestHandler(c *gin.Context) {
	user, ok := currentUser(c)
	if !ok {
		return
	}

	var reqRow models.KycVerificationRequest
	if err := database.DB.First(&reqRow, "id = ?", c.Param("id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"message": "Verification request not found"})
		return
	}
	if reqRow.CustomerID != user.ID.String() {
		c.JSON(http.StatusForbidden, gin.H{"message": "Not your verification request"})
		return
	}

	var body respondVerificationRequestBody
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}

	now := time.Now()
	reqRow.RespondedAt = &now
	if body.Ready {
		reqRow.Status = "customer_ready"
		reqRow.PreferredTime = body.PreferredTime
	} else {
		reqRow.Status = "declined"
	}

	if err := database.DB.Save(&reqRow).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Failed to save response"})
		return
	}

	c.JSON(http.StatusOK, reqRow)
}
