// Package payments implements EMI collection through a real payment
// gateway's server-side API and webhook mechanism — replacing the Phase 1
// stub (internal/loans/emi_handler.go's VerifyGatewaySignature) that only
// checked a customer-supplied string was non-empty. An EMI is now only
// ever marked paid from webhookHandler after independent, cryptographic
// verification of a gateway event — never from a customer request.
package payments

import "errors"

// Order is what CreateOrder returns to hand to the client's checkout SDK.
type Order struct {
	GatewayOrderID string
	// KeyID is the gateway's PUBLIC key (safe to send to the client — it
	// identifies the merchant account, it is not a secret). The
	// corresponding key SECRET never leaves the backend.
	KeyID string
}

// WebhookEvent is the provider-agnostic shape webhook_handler.go operates
// on, after a concrete Provider has parsed and verified a raw delivery.
type WebhookEvent struct {
	GatewayEventID   string // for idempotency — see models.PaymentEvent.GatewayEventID
	EventType        string // e.g. "payment.captured", "payment.failed", "refund.processed"
	GatewayOrderID   string
	GatewayPaymentID string
	AmountPaise      int64
	Currency         string
	Method           string // upi | card | netbanking | wallet
	FailureReason    string
	// RefundID/RefundAmountPaise are populated only for refund.* events.
	RefundID          string
	RefundAmountPaise int64
}

// Provider is the seam a real payment gateway plugs into. Only one
// concrete implementation exists today (Razorpay, razorpay.go) — the
// interface exists so the rest of this package (and internal/loans) never
// depends on a specific vendor's request/response shapes directly.
type Provider interface {
	// Name identifies this provider for Payment.Gateway / audit logs.
	Name() string

	// CreateOrder asks the gateway to create a payment order for
	// amountPaise (integer paise, never a float) against receipt (an
	// opaque merchant reference — this project uses the Payment row's
	// eventual ID). Returns the gateway order ID the client's checkout
	// flow needs.
	CreateOrder(amountPaise int64, currency, receipt string) (Order, error)

	// VerifyWebhookSignature checks rawBody against signatureHeader using
	// the gateway's documented HMAC scheme and the webhook secret from
	// configuration. Returns false (never an error) for "signature does
	// not match" — that is an expected, common outcome (replay, forged
	// request, wrong secret), not an exceptional one.
	VerifyWebhookSignature(rawBody []byte, signatureHeader string) bool

	// ParseWebhookEvent extracts a WebhookEvent from an already
	// signature-verified rawBody. Call only after VerifyWebhookSignature
	// returned true.
	ParseWebhookEvent(rawBody []byte) (WebhookEvent, error)
}

var ErrProviderNotConfigured = errors.New("payment gateway is not configured — set RAZORPAY_KEY_ID/RAZORPAY_KEY_SECRET/RAZORPAY_WEBHOOK_SECRET")

// activeProvider is set once at startup by Initialize (cmd/api/main.go),
// mirroring the pattern already used for surepass.Initialize.
var activeProvider Provider

// Initialize wires up the configured gateway. Safe to call with empty
// credentials — activeProvider stays nil and every payment-initiating
// endpoint reports ErrProviderNotConfigured rather than silently no-op
// "succeeding," which is what a fake/mocked provider would do.
func Initialize() {
	p := newRazorpayProvider()
	if p != nil {
		activeProvider = p
	}
}

// Active returns the configured provider, or nil if none is configured
// (no real credentials present in this environment/deployment).
func Active() Provider {
	return activeProvider
}

// SetActiveProviderForTesting overrides the active provider — used only
// by this package's own integration tests (security_test.go) to inject a
// real razorpayProvider instance with a known, test-controlled webhook
// secret, so the actual signature-verification code path can be exercised
// without live Razorpay credentials. Never called from production code.
func SetActiveProviderForTesting(p Provider) {
	activeProvider = p
}

// NewRazorpayProviderForTesting constructs a real razorpayProvider with
// explicit credentials, bypassing environment-variable lookup — for tests
// only (see SetActiveProviderForTesting).
func NewRazorpayProviderForTesting(keyID, keySecret, webhookSecret string) Provider {
	return &razorpayProvider{keyID: keyID, keySecret: keySecret, webhookSecret: webhookSecret}
}
