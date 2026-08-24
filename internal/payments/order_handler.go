package payments

import (
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/santosh0921/nbfc-backend/internal/audit"
	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/models"
)

func currentUser(c *gin.Context) (*models.User, bool) {
	mobile := c.GetString("mobile")
	if mobile == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"message": "Unauthorized"})
		return nil, false
	}
	var user models.User
	if err := database.DB.Where("mobile = ?", mobile).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"message": "User not found"})
		return nil, false
	}
	return &user, true
}

func toPaise(rupees float64) int64 {
	return int64(math.Round(rupees * 100))
}

// CustomerCreatePaymentOrderHandler handles
// POST /auth/loans/:id/emi/:installmentId/create-order — replaces the old
// POST .../pay endpoint's role. The customer never gets to declare a
// payment successful here: this only asks the gateway for an order to
// check out against. The EMI is marked paid later, exclusively by
// webhookHandler after independently verifying the gateway's own
// confirmation (see webhook_handler.go).
func CustomerCreatePaymentOrderHandler(c *gin.Context) {
	user, ok := currentUser(c)
	if !ok {
		return
	}
	provider := Active()
	if provider == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"message": "Online payment is not available right now — please try again later or contact support."})
		return
	}

	loanID, _ := strconv.Atoi(c.Param("id"))
	var loan models.LoanApplication
	if err := database.DB.First(&loan, loanID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"message": "Loan not found"})
		return
	}
	if loan.CustomerID != user.ID {
		c.JSON(http.StatusForbidden, gin.H{"message": "Not your loan"})
		return
	}
	if loan.Status != models.LoanStatusDisbursed {
		c.JSON(http.StatusBadRequest, gin.H{"message": "This loan has no active EMI schedule to pay against"})
		return
	}

	installmentID, _ := strconv.Atoi(c.Param("installmentId"))
	var inst models.EmiInstallment
	if err := database.DB.Where("id = ? AND loan_id = ?", installmentID, loan.ID).First(&inst).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"message": "Installment not found"})
		return
	}
	if inst.Status == models.EmiStatusPaid {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Installment already paid"})
		return
	}

	// A stale INITIATED/PENDING order for this exact installment is
	// reused rather than creating a new gateway order every time the
	// customer re-opens the pay screen — avoids accumulating abandoned
	// orders, and means a customer who backgrounds the app mid-checkout
	// and returns gets the same order back.
	var existing models.Payment
	err := database.DB.Where("emi_id = ? AND status IN ?", inst.ID,
		[]string{models.PaymentStatusInitiated, models.PaymentStatusPending}).
		Order("created_at desc").First(&existing).Error
	if err == nil {
		c.JSON(http.StatusOK, gin.H{
			"paymentId":      existing.ID,
			"gatewayOrderId": existing.GatewayOrderID,
			"amountPaise":    existing.AmountPaise,
			"currency":       existing.Currency,
			"gateway":        existing.Gateway,
		})
		return
	}

	amountPaise := toPaise(inst.Amount)
	receipt := "loan" + strconv.Itoa(int(loan.ID)) + "-emi" + strconv.Itoa(int(inst.ID)) + "-" + strconv.FormatInt(time.Now().Unix(), 10)

	order, err := provider.CreateOrder(amountPaise, "INR", receipt)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"message": "Could not start the payment: " + err.Error()})
		return
	}

	payment := models.Payment{
		LoanID:         loan.ID,
		EmiID:          inst.ID,
		CustomerID:     user.ID.String(),
		Gateway:        provider.Name(),
		GatewayOrderID: order.GatewayOrderID,
		AmountPaise:    amountPaise,
		Currency:       "INR",
		Status:         models.PaymentStatusInitiated,
	}
	if err := database.DB.Create(&payment).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Failed to record payment order"})
		return
	}

	audit.RecordDetailed(c, audit.Entry{
		ActorType: "customer", ActorRef: user.Mobile,
		Action: "payment.initiated", TargetType: "payment", TargetID: strconv.Itoa(int(payment.ID)),
		Result: audit.ResultSuccess,
		Details: "Loan #" + strconv.Itoa(int(loan.ID)) + ", installment #" + strconv.Itoa(inst.InstallmentNumber) +
			", amount " + strconv.FormatInt(amountPaise, 10) + " paise",
	})

	c.JSON(http.StatusCreated, gin.H{
		"paymentId":      payment.ID,
		"gatewayOrderId": order.GatewayOrderID,
		"gatewayKeyId":   order.KeyID, // public key, safe for the client checkout SDK
		"amountPaise":    amountPaise,
		"currency":       "INR",
		"gateway":        provider.Name(),
	})
}

// CustomerGetPaymentStatusHandler handles GET /auth/payments/:paymentId —
// the customer app polls this to render Processing/Success/Failed/Pending;
// it never sets status itself, only reads whatever webhookHandler has
// already verified and written.
func CustomerGetPaymentStatusHandler(c *gin.Context) {
	user, ok := currentUser(c)
	if !ok {
		return
	}
	id, _ := strconv.Atoi(c.Param("paymentId"))
	var payment models.Payment
	if err := database.DB.First(&payment, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"message": "Payment not found"})
		return
	}
	if payment.CustomerID != user.ID.String() {
		c.JSON(http.StatusForbidden, gin.H{"message": "Not your payment"})
		return
	}
	c.JSON(http.StatusOK, payment)
}
