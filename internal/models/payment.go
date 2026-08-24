package models

import "time"

// Payment state machine. Only a verified gateway webhook can move a
// payment to Success — nothing in the customer-facing API can set this
// status directly (see internal/payments/webhook_handler.go). An EMI is
// only ever marked paid as a side effect of a Payment reaching Success,
// never from a customer-supplied claim.
const (
	PaymentStatusInitiated = "INITIATED" // order created, customer hasn't completed checkout yet
	PaymentStatusPending   = "PENDING"   // gateway has an attempt in flight (e.g. UPI collect awaiting customer action)
	PaymentStatusSuccess   = "SUCCESS"
	PaymentStatusFailed    = "FAILED"
	PaymentStatusRefunded  = "REFUNDED"
	PaymentStatusPartiallyRefunded = "PARTIALLY_REFUNDED" // Phase 9 — a refund event whose cumulative amount is less than the original payment
	PaymentStatusCancelled = "CANCELLED"
)

// Payment is one EMI payment attempt through a payment gateway — created
// at "create order" time (INITIATED) and only ever transitioned by
// server-side webhook verification (internal/payments/webhook_handler.go),
// never by a client request. One EmiInstallment can have multiple Payment
// rows over time (e.g. a FAILED attempt followed by a SUCCESS retry), but
// at most one SUCCESS.
type Payment struct {
	ID         uint   `gorm:"primaryKey" json:"id"`
	LoanID     uint   `gorm:"index;not null" json:"loanId"`
	EmiID      uint   `gorm:"index;not null" json:"emiId"`
	CustomerID string `gorm:"index;not null" json:"customerId"` // uuid string
	Gateway    string `gorm:"not null" json:"gateway"`          // "razorpay" today; abstraction allows more

	GatewayOrderID   string `gorm:"uniqueIndex;not null" json:"gatewayOrderId"`
	GatewayPaymentID string `gorm:"index" json:"gatewayPaymentId,omitempty"` // set once the gateway reports an attempt

	AmountPaise int64  `gorm:"not null" json:"amountPaise"` // integer paise — never float for money moved through a gateway
	Currency    string `gorm:"not null;default:INR" json:"currency"`
	Status      string `gorm:"index;not null" json:"status"`

	Method        string `json:"method,omitempty"` // upi | card | netbanking | wallet — from gateway, display only
	FailureReason string `json:"failureReason,omitempty"`

	CreatedAt  time.Time  `json:"createdAt"`
	UpdatedAt  time.Time  `json:"updatedAt"`
	VerifiedAt *time.Time `json:"verifiedAt,omitempty"` // when a webhook last moved this payment's status
}

// PaymentEvent is an immutable log of every webhook/event received for a
// payment — including ones that were rejected (bad signature, replay,
// amount mismatch) — kept distinct from Payment (current state) so the
// full history survives even across multiple retried webhook deliveries.
// GatewayEventID has a unique index specifically to make webhook
// processing idempotent: a duplicate delivery of the same event is
// detected and short-circuited before any Payment/EMI mutation happens.
type PaymentEvent struct {
	ID        uint   `gorm:"primaryKey" json:"id"`
	PaymentID *uint  `gorm:"index" json:"paymentId,omitempty"` // nullable: an event that doesn't map to a known order is still logged
	Gateway   string `gorm:"not null" json:"gateway"`

	GatewayEventID string `gorm:"uniqueIndex:idx_payment_event_gateway_event,where:gateway_event_id != ''" json:"gatewayEventId,omitempty"`
	EventType      string `json:"eventType"` // e.g. "payment.captured", "payment.failed"

	SignatureValid bool   `json:"signatureValid"`
	RawPayload     string `gorm:"type:text" json:"-"`      // never returned over the API — may contain gateway-internal fields
	ProcessedNote  string `json:"processedNote,omitempty"` // e.g. "duplicate, ignored" / "applied"

	CreatedAt time.Time `json:"createdAt"`
}

// Refund tracks a refund issued against a previously-SUCCESS payment.
// Kept as its own table (not just a Payment status flip) because a
// payment can be partially refunded and because the refund itself has its
// own gateway reference/lifecycle.
type Refund struct {
	ID              uint      `gorm:"primaryKey" json:"id"`
	PaymentID       uint      `gorm:"index;not null" json:"paymentId"`
	GatewayRefundID string    `gorm:"uniqueIndex;not null" json:"gatewayRefundId"`
	AmountPaise     int64     `gorm:"not null" json:"amountPaise"`
	Status          string    `gorm:"not null" json:"status"` // mirrors gateway refund status strings
	Reason          string    `json:"reason,omitempty"`
	CreatedAt       time.Time `json:"createdAt"`
}
