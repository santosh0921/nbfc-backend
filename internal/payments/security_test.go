package payments_test

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/joho/godotenv"

	"github.com/santosh0921/nbfc-backend/internal/auth"
	"github.com/santosh0921/nbfc-backend/internal/config"
	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/middleware"
	"github.com/santosh0921/nbfc-backend/internal/models"
	"github.com/santosh0921/nbfc-backend/internal/payments"
)

const testWebhookSecret = "test-webhook-secret-do-not-use-in-prod"

func setupPaymentsTestApp(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)

	if database.DB == nil {
		_ = godotenv.Load("../../.env")
		cfg := config.Load()
		if err := auth.InitJWTSecret(cfg.JWTSecret); err != nil {
			t.Fatalf("InitJWTSecret: %v", err)
		}
		database.Connect(cfg)
		database.DB.AutoMigrate(&models.Payment{}, &models.PaymentEvent{}, &models.Refund{})
	}

	// A real razorpayProvider, with a known test webhook secret so the
	// actual HMAC verification code runs end-to-end — not a stand-in mock.
	payments.SetActiveProviderForTesting(payments.NewRazorpayProviderForTesting("rzp_test_key", "rzp_test_secret", testWebhookSecret))

	r := gin.New()
	authorized := r.Group("/auth")
	authorized.Use(middleware.AuthMiddleware())
	authorized.GET("/payments/:paymentId", payments.CustomerGetPaymentStatusHandler)
	r.POST("/webhooks/razorpay", payments.RazorpayWebhookHandler)
	return r
}

func createTestCustomer(t *testing.T) (*models.User, string) {
	t.Helper()
	mobile := "9" + uuid.NewString()[0:9]
	user := models.User{ID: uuid.New(), Mobile: mobile, MPIN: "x", IsActive: true}
	if err := database.DB.Create(&user).Error; err != nil {
		t.Fatalf("create test user: %v", err)
	}
	t.Cleanup(func() { database.DB.Unscoped().Delete(&user) })

	token, err := auth.GenerateToken(mobile)
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}
	return &user, token
}

func createTestPayment(t *testing.T, customerID uuid.UUID, amountPaise int64) models.Payment {
	t.Helper()
	loan := models.LoanApplication{CustomerID: customerID, Status: models.LoanStatusDisbursed, AmountRequested: float64(amountPaise) / 100}
	database.DB.Create(&loan)
	t.Cleanup(func() { database.DB.Unscoped().Delete(&loan) })

	tenure := 12
	inst := models.EmiInstallment{LoanID: loan.ID, InstallmentNumber: 1, DueDate: time.Now(), Amount: float64(amountPaise) / 100, Status: models.EmiStatusPending}
	database.DB.Create(&inst)
	t.Cleanup(func() { database.DB.Unscoped().Delete(&inst) })
	_ = tenure

	payment := models.Payment{
		LoanID: loan.ID, EmiID: inst.ID, CustomerID: customerID.String(), Gateway: "razorpay",
		GatewayOrderID: "order_" + uuid.NewString(), AmountPaise: amountPaise, Currency: "INR",
		Status: models.PaymentStatusInitiated,
	}
	database.DB.Create(&payment)
	t.Cleanup(func() { database.DB.Unscoped().Delete(&payment) })
	return payment
}

func signedWebhookRequest(t *testing.T, r *gin.Engine, payload map[string]interface{}, secret string) *httptest.ResponseRecorder {
	t.Helper()
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	signature := hex.EncodeToString(mac.Sum(nil))

	req := httptest.NewRequest(http.MethodPost, "/webhooks/razorpay", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Razorpay-Signature", signature)
	req.Header.Set("X-Razorpay-Event-Id", "evt_"+uuid.NewString())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func capturedPayload(orderID string, amountPaise int64) map[string]interface{} {
	return capturedPayloadWithID(orderID, "pay_"+uuid.NewString(), amountPaise)
}

func capturedPayloadWithID(orderID, gatewayPaymentID string, amountPaise int64) map[string]interface{} {
	return map[string]interface{}{
		"event": "payment.captured",
		"payload": map[string]interface{}{
			"payment": map[string]interface{}{
				"entity": map[string]interface{}{
					"id": gatewayPaymentID, "order_id": orderID,
					"amount": amountPaise, "currency": "INR", "method": "upi",
				},
			},
		},
	}
}

// --- 2. Invalid gateway signature -> rejected ---

func TestWebhookInvalidSignatureRejected(t *testing.T) {
	r := setupPaymentsTestApp(t)
	user, _ := createTestCustomer(t)
	payment := createTestPayment(t, user.ID, 150000)

	body, _ := json.Marshal(capturedPayload(payment.GatewayOrderID, 150000))
	req := httptest.NewRequest(http.MethodPost, "/webhooks/razorpay", bytes.NewReader(body))
	req.Header.Set("X-Razorpay-Signature", "not-a-real-signature")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid signature, got %d: %s", w.Code, w.Body.String())
	}
	var reloaded models.Payment
	database.DB.First(&reloaded, payment.ID)
	if reloaded.Status != models.PaymentStatusInitiated {
		t.Fatalf("payment status must not change on an invalid-signature webhook, got %s", reloaded.Status)
	}
}

// --- 3. Valid payment -> EMI becomes PAID ---

func TestValidWebhookMarksEmiPaid(t *testing.T) {
	r := setupPaymentsTestApp(t)
	user, _ := createTestCustomer(t)
	payment := createTestPayment(t, user.ID, 150000)

	w := signedWebhookRequest(t, r, capturedPayload(payment.GatewayOrderID, 150000), testWebhookSecret)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var reloadedPayment models.Payment
	database.DB.First(&reloadedPayment, payment.ID)
	if reloadedPayment.Status != models.PaymentStatusSuccess {
		t.Fatalf("expected payment SUCCESS, got %s", reloadedPayment.Status)
	}

	var inst models.EmiInstallment
	database.DB.First(&inst, payment.EmiID)
	if inst.Status != models.EmiStatusPaid {
		t.Fatalf("expected EMI PAID, got %s", inst.Status)
	}
	if inst.PaymentReference == "" {
		t.Fatal("expected a gateway payment reference to be recorded")
	}
}

// --- 4. Duplicate webhook -> no duplicate settlement ---

func TestDuplicateWebhookIsIdempotent(t *testing.T) {
	r := setupPaymentsTestApp(t)
	user, _ := createTestCustomer(t)
	payment := createTestPayment(t, user.ID, 150000)

	payload := capturedPayload(payment.GatewayOrderID, 150000)
	body, _ := json.Marshal(payload)
	mac := hmac.New(sha256.New, []byte(testWebhookSecret))
	mac.Write(body)
	signature := hex.EncodeToString(mac.Sum(nil))
	eventID := "evt_fixed_" + uuid.NewString()

	send := func() *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/webhooks/razorpay", bytes.NewReader(body))
		req.Header.Set("X-Razorpay-Signature", signature)
		req.Header.Set("X-Razorpay-Event-Id", eventID)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w
	}

	w1 := send()
	w2 := send()
	if w1.Code != http.StatusOK || w2.Code != http.StatusOK {
		t.Fatalf("both deliveries should return 200, got %d and %d", w1.Code, w2.Code)
	}

	var count int64
	database.DB.Model(&models.PaymentEvent{}).Where("gateway_event_id = ?", eventID).Count(&count)
	if count != 1 {
		t.Fatalf("expected exactly one PaymentEvent row for a duplicated delivery, got %d", count)
	}

	var inst models.EmiInstallment
	database.DB.First(&inst, payment.EmiID)
	if inst.Status != models.EmiStatusPaid {
		t.Fatal("EMI should still be settled exactly once")
	}
}

// --- 5. Wrong amount -> rejected ---

func TestWrongAmountRejected(t *testing.T) {
	r := setupPaymentsTestApp(t)
	user, _ := createTestCustomer(t)
	payment := createTestPayment(t, user.ID, 150000)

	// Webhook reports a different (lower) amount than the order was for.
	w := signedWebhookRequest(t, r, capturedPayload(payment.GatewayOrderID, 100000), testWebhookSecret)
	if w.Code != http.StatusOK {
		t.Fatalf("webhook endpoint should still ack with 200, got %d", w.Code)
	}

	var reloaded models.Payment
	database.DB.First(&reloaded, payment.ID)
	if reloaded.Status == models.PaymentStatusSuccess {
		t.Fatal("a payment must not be settled when the webhook amount does not match the order amount")
	}
	var inst models.EmiInstallment
	database.DB.First(&inst, payment.EmiID)
	if inst.Status == models.EmiStatusPaid {
		t.Fatal("EMI must not be marked paid when the webhook amount is wrong")
	}
}

// --- 6. Wrong loan/payment mapping -> rejected ---

func TestUnknownOrderRejected(t *testing.T) {
	r := setupPaymentsTestApp(t)
	w := signedWebhookRequest(t, r, capturedPayload("order_does_not_exist_"+uuid.NewString(), 150000), testWebhookSecret)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 (acked, nothing to apply), got %d", w.Code)
	}
	var count int64
	database.DB.Model(&models.Payment{}).Where("status = ?", models.PaymentStatusSuccess).
		Where("gateway_order_id LIKE ?", "order_does_not_exist_%").Count(&count)
	if count != 0 {
		t.Fatal("no payment should have been settled for an unknown order id")
	}
}

// --- 7. Failed payment -> EMI remains unpaid ---

func TestFailedPaymentLeavesEmiUnpaid(t *testing.T) {
	r := setupPaymentsTestApp(t)
	user, _ := createTestCustomer(t)
	payment := createTestPayment(t, user.ID, 150000)

	payload := map[string]interface{}{
		"event": "payment.failed",
		"payload": map[string]interface{}{
			"payment": map[string]interface{}{
				"entity": map[string]interface{}{
					"id": "pay_" + uuid.NewString(), "order_id": payment.GatewayOrderID,
					"amount": 150000, "currency": "INR", "error_description": "insufficient funds",
				},
			},
		},
	}
	w := signedWebhookRequest(t, r, payload, testWebhookSecret)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var reloaded models.Payment
	database.DB.First(&reloaded, payment.ID)
	if reloaded.Status != models.PaymentStatusFailed {
		t.Fatalf("expected payment FAILED, got %s", reloaded.Status)
	}
	var inst models.EmiInstallment
	database.DB.First(&inst, payment.EmiID)
	if inst.Status == models.EmiStatusPaid {
		t.Fatal("EMI must remain unpaid after a failed payment")
	}
}

// --- 8. Refunded payment -> correct refund state ---

func TestRefundRecorded(t *testing.T) {
	r := setupPaymentsTestApp(t)
	user, _ := createTestCustomer(t)
	payment := createTestPayment(t, user.ID, 150000)

	// Settle it first, with a known gateway payment id.
	gatewayPaymentID := "pay_" + uuid.NewString()
	signedWebhookRequest(t, r, capturedPayloadWithID(payment.GatewayOrderID, gatewayPaymentID, 150000), testWebhookSecret)

	refundPayload := map[string]interface{}{
		"event": "refund.processed",
		"payload": map[string]interface{}{
			"refund": map[string]interface{}{
				"entity": map[string]interface{}{
					"id": "rfnd_" + uuid.NewString(), "payment_id": gatewayPaymentID, "amount": 150000,
				},
			},
		},
	}
	w := signedWebhookRequest(t, r, refundPayload, testWebhookSecret)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var reloaded models.Payment
	database.DB.First(&reloaded, payment.ID)
	if reloaded.Status != models.PaymentStatusRefunded {
		t.Fatalf("expected payment REFUNDED, got %s", reloaded.Status)
	}
	var refundCount int64
	database.DB.Model(&models.Refund{}).Where("payment_id = ?", payment.ID).Count(&refundCount)
	if refundCount != 1 {
		t.Fatalf("expected exactly one Refund row, got %d", refundCount)
	}
}

// --- Phase 9: a refund whose cumulative amount exceeds the original
// payment is recorded (the money already moved on the gateway's side)
// but flagged, never silently accepted as a normal in-range refund. ---

func TestOverRefundFlagged(t *testing.T) {
	r := setupPaymentsTestApp(t)
	user, _ := createTestCustomer(t)
	payment := createTestPayment(t, user.ID, 100000)

	gatewayPaymentID := "pay_" + uuid.NewString()
	signedWebhookRequest(t, r, capturedPayloadWithID(payment.GatewayOrderID, gatewayPaymentID, 100000), testWebhookSecret)

	// Refund for MORE than the original payment amount.
	refundPayload := map[string]interface{}{
		"event": "refund.processed",
		"payload": map[string]interface{}{
			"refund": map[string]interface{}{
				"entity": map[string]interface{}{
					"id": "rfnd_" + uuid.NewString(), "payment_id": gatewayPaymentID, "amount": 150000,
				},
			},
		},
	}
	w := signedWebhookRequest(t, r, refundPayload, testWebhookSecret)
	if w.Code != http.StatusOK {
		t.Fatalf("webhook itself should still return 200 (event was received and processed), got %d: %s", w.Code, w.Body.String())
	}

	var refund models.Refund
	database.DB.Where("payment_id = ?", payment.ID).First(&refund)
	if refund.Reason == "" {
		t.Fatal("an over-refund must be recorded with a non-empty Reason flagging it, never silently treated as normal")
	}
}

// --- 18a. Unauthorized user cannot access another customer's payment ---

func TestCustomerCannotViewAnotherCustomersPayment(t *testing.T) {
	r := setupPaymentsTestApp(t)
	owner, _ := createTestCustomer(t)
	payment := createTestPayment(t, owner.ID, 150000)

	_, otherToken := createTestCustomer(t)

	req := httptest.NewRequest(http.MethodGet, "/auth/payments/"+itoaP(payment.ID), nil)
	req.Header.Set("Authorization", "Bearer "+otherToken)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for a different customer's payment, got %d: %s", w.Code, w.Body.String())
	}
}

func TestNoTokenRejected(t *testing.T) {
	r := setupPaymentsTestApp(t)
	user, _ := createTestCustomer(t)
	payment := createTestPayment(t, user.ID, 150000)

	req := httptest.NewRequest(http.MethodGet, "/auth/payments/"+itoaP(payment.ID), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 with no token, got %d", w.Code)
	}
}

func itoaP(id uint) string {
	if id == 0 {
		return "0"
	}
	var b []byte
	for id > 0 {
		b = append([]byte{byte('0' + id%10)}, b...)
		id /= 10
	}
	return string(b)
}
