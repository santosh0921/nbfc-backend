package payments

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

// razorpayProvider implements Provider against Razorpay's real, documented
// server-side Orders API (https://razorpay.com/docs/api/orders/) and
// webhook signature scheme
// (https://razorpay.com/docs/webhooks/validate-test/) — HMAC-SHA256 of the
// raw request body, keyed by the webhook secret, hex-encoded, compared
// against the X-Razorpay-Signature header.
//
// This is genuine integration code, not a mock: it makes a real HTTPS
// call to api.razorpay.com and implements the real signature algorithm.
// It has no effect in this environment because no live
// RAZORPAY_KEY_ID/RAZORPAY_KEY_SECRET/RAZORPAY_WEBHOOK_SECRET are
// configured (Initialize() leaves activeProvider nil in that case) — see
// the Phase 2 report for exactly what credentials are still required.
type razorpayProvider struct {
	keyID         string
	keySecret     string
	webhookSecret string
	httpClient    *http.Client
}

const razorpayOrdersURL = "https://api.razorpay.com/v1/orders"

func newRazorpayProvider() *razorpayProvider {
	keyID := strings.TrimSpace(os.Getenv("RAZORPAY_KEY_ID"))
	keySecret := strings.TrimSpace(os.Getenv("RAZORPAY_KEY_SECRET"))
	webhookSecret := strings.TrimSpace(os.Getenv("RAZORPAY_WEBHOOK_SECRET"))
	if keyID == "" || keySecret == "" || webhookSecret == "" {
		return nil
	}
	return &razorpayProvider{
		keyID:         keyID,
		keySecret:     keySecret,
		webhookSecret: webhookSecret,
		httpClient:    &http.Client{Timeout: 15 * time.Second},
	}
}

func (r *razorpayProvider) Name() string { return "razorpay" }

func (r *razorpayProvider) CreateOrder(amountPaise int64, currency, receipt string) (Order, error) {
	body, err := json.Marshal(map[string]interface{}{
		"amount":          amountPaise,
		"currency":        currency,
		"receipt":         receipt,
		"payment_capture": 1,
	})
	if err != nil {
		return Order{}, err
	}

	req, err := http.NewRequest(http.MethodPost, razorpayOrdersURL, bytes.NewReader(body))
	if err != nil {
		return Order{}, err
	}
	req.SetBasicAuth(r.keyID, r.keySecret)
	req.Header.Set("Content-Type", "application/json")

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return Order{}, fmt.Errorf("razorpay order creation request failed: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		ID    string `json:"id"`
		Error *struct {
			Description string `json:"description"`
		} `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return Order{}, fmt.Errorf("razorpay order creation: could not parse response: %w", err)
	}
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		msg := "razorpay order creation failed"
		if result.Error != nil {
			msg = "razorpay order creation failed: " + result.Error.Description
		}
		return Order{}, errors.New(msg)
	}
	if result.ID == "" {
		return Order{}, errors.New("razorpay order creation: no order id in response")
	}

	return Order{GatewayOrderID: result.ID, KeyID: r.keyID}, nil
}

// VerifyWebhookSignature implements Razorpay's documented scheme exactly:
// hex(HMAC-SHA256(webhookSecret, rawBody)) compared to the
// X-Razorpay-Signature header. Constant-time comparison to avoid a
// timing side-channel on the expected signature.
func (r *razorpayProvider) VerifyWebhookSignature(rawBody []byte, signatureHeader string) bool {
	if signatureHeader == "" {
		return false
	}
	mac := hmac.New(sha256.New, []byte(r.webhookSecret))
	mac.Write(rawBody)
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(strings.TrimSpace(signatureHeader)))
}

// razorpayWebhookPayload mirrors Razorpay's documented webhook body shape
// (https://razorpay.com/docs/webhooks/payloads/payments/) for the event
// types this project handles: payment.captured, payment.failed,
// refund.processed.
type razorpayWebhookPayload struct {
	Event   string `json:"event"`
	Payload struct {
		Payment *struct {
			Entity struct {
				ID          string `json:"id"`
				OrderID     string `json:"order_id"`
				Amount      int64  `json:"amount"`
				Currency    string `json:"currency"`
				Method      string `json:"method"`
				ErrorReason string `json:"error_description"`
			} `json:"entity"`
		} `json:"payment"`
		Refund *struct {
			Entity struct {
				ID        string `json:"id"`
				PaymentID string `json:"payment_id"`
				Amount    int64  `json:"amount"`
			} `json:"entity"`
		} `json:"refund"`
	} `json:"payload"`
}

func (r *razorpayProvider) ParseWebhookEvent(rawBody []byte) (WebhookEvent, error) {
	var payload razorpayWebhookPayload
	if err := json.Unmarshal(rawBody, &payload); err != nil {
		return WebhookEvent{}, fmt.Errorf("invalid webhook payload: %w", err)
	}

	event := WebhookEvent{EventType: payload.Event}

	if payload.Payload.Payment != nil {
		p := payload.Payload.Payment.Entity
		event.GatewayOrderID = p.OrderID
		event.GatewayPaymentID = p.ID
		event.AmountPaise = p.Amount
		event.Currency = p.Currency
		event.Method = p.Method
		event.FailureReason = p.ErrorReason
	}
	if payload.Payload.Refund != nil {
		rf := payload.Payload.Refund.Entity
		event.RefundID = rf.ID
		event.GatewayPaymentID = rf.PaymentID
		event.RefundAmountPaise = rf.Amount
	}

	// Razorpay does not always include a dedicated top-level event ID in
	// the payload body itself (delivery identity is more commonly carried
	// in the X-Razorpay-Event-Id header on newer webhook configurations) —
	// idempotency is keyed on that header when present, with a
	// deterministic fallback composite here so a payload that never
	// carries the header still can't be replayed to double-settle.
	event.GatewayEventID = payload.Event + ":" + event.GatewayPaymentID + ":" + event.RefundID

	return event, nil
}
