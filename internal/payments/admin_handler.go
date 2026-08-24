package payments

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/models"
)

// AdminListPaymentsHandler handles GET /admin/payments — the "Payments"
// screen. Supports the filters the admin panel needs: status, loanId,
// customerId, gateway, and a date range on CreatedAt.
func AdminListPaymentsHandler(c *gin.Context) {
	q := database.DB.Model(&models.Payment{})
	if status := c.Query("status"); status != "" {
		q = q.Where("status = ?", status)
	}
	if loanID := c.Query("loanId"); loanID != "" {
		q = q.Where("loan_id = ?", loanID)
	}
	if customerID := c.Query("customerId"); customerID != "" {
		q = q.Where("customer_id = ?", customerID)
	}
	if gateway := c.Query("gateway"); gateway != "" {
		q = q.Where("gateway = ?", gateway)
	}
	if from := c.Query("from"); from != "" {
		q = q.Where("created_at >= ?", from)
	}
	if to := c.Query("to"); to != "" {
		q = q.Where("created_at <= ?", to)
	}

	limit := 100
	if v, err := strconv.Atoi(c.Query("limit")); err == nil && v > 0 && v <= 500 {
		limit = v
	}
	offset := 0
	if v, err := strconv.Atoi(c.Query("offset")); err == nil && v > 0 {
		offset = v
	}

	var total int64
	q.Count(&total)

	var payments []models.Payment
	q.Order("created_at desc").Limit(limit).Offset(offset).Find(&payments)
	c.JSON(http.StatusOK, gin.H{"payments": payments, "total": total})
}

// AdminGetPaymentHandler handles GET /admin/payments/:id — Payment
// Details + Payment Verification Status, including the full webhook
// event trail (including rejected/duplicate deliveries) for that payment.
func AdminGetPaymentHandler(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var payment models.Payment
	if err := database.DB.First(&payment, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"message": "Not found"})
		return
	}
	var events []models.PaymentEvent
	database.DB.Where("payment_id = ?", payment.ID).Order("created_at asc").Find(&events)
	var refunds []models.Refund
	database.DB.Where("payment_id = ?", payment.ID).Order("created_at asc").Find(&refunds)

	c.JSON(http.StatusOK, gin.H{
		"payment": payment,
		"events":  events,
		"refunds": refunds,
	})
}
