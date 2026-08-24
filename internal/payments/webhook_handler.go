package payments

import (
	"io"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm/clause"

	"github.com/santosh0921/nbfc-backend/internal/accounting"
	"github.com/santosh0921/nbfc-backend/internal/audit"
	"github.com/santosh0921/nbfc-backend/internal/collections"
	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/models"
	"github.com/santosh0921/nbfc-backend/internal/risk"
)

// RazorpayWebhookHandler handles POST /webhooks/razorpay. This is the ONLY
// code path in the entire application that can mark an EMI as paid — not
// the customer's own request, not an admin action. Every step here is
// deliberate:
//
//  1. Signature verified against the raw body BEFORE any JSON parsing —
//     an attacker who can't produce a valid HMAC never gets their payload
//     interpreted at all.
//  2. Idempotency check BEFORE any Payment/EMI mutation — a duplicate
//     delivery of an already-processed event is a 200 no-op, not a second
//     settlement.
//  3. The event's own amount/currency is compared against what THIS
//     backend recorded when the order was created — the gateway
//     confirming "some payment happened" is not enough; it must be a
//     payment for the exact amount this backend expects.
//  4. EMI settlement happens inside one DB transaction with row locks on
//     both the Payment and the EmiInstallment, so a duplicate/replayed
//     webhook racing a legitimate one can't double-settle.
func RazorpayWebhookHandler(c *gin.Context) {
	provider := Active()
	if provider == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"message": "Payment gateway not configured"})
		return
	}

	rawBody, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Could not read request body"})
		return
	}

	signature := c.GetHeader("X-Razorpay-Signature")
	if !provider.VerifyWebhookSignature(rawBody, signature) {
		logWebhookEvent(nil, provider.Name(), "", "unknown", false, string(rawBody), "rejected: invalid signature")
		// A generic 400 — never reveal WHY verification failed (e.g.
		// "signature valid but order unknown" vs "signature invalid")
		// to whoever is calling this endpoint, since it's unauthenticated
		// by design (a real gateway proves itself via the signature, not
		// a bearer token).
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid signature"})
		return
	}

	event, err := provider.ParseWebhookEvent(rawBody)
	if err != nil {
		logWebhookEvent(nil, provider.Name(), "", "unknown", true, string(rawBody), "rejected: unparseable payload")
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid payload"})
		return
	}

	// Prefer the gateway's own delivery-identity header when present
	// (Razorpay: X-Razorpay-Event-Id) over the payload-derived composite
	// key ParseWebhookEvent falls back to.
	if headerEventID := c.GetHeader("X-Razorpay-Event-Id"); headerEventID != "" {
		event.GatewayEventID = headerEventID
	}

	// --- Idempotency: has this exact event already been processed? ---
	var dup models.PaymentEvent
	if err := database.DB.Where("gateway_event_id = ?", event.GatewayEventID).First(&dup).Error; err == nil {
		// Not an error — this is the expected shape of a gateway retrying
		// a delivery it didn't get a prompt 200 for. Acknowledge with 200
		// so the gateway stops retrying, having done nothing further.
		c.JSON(http.StatusOK, gin.H{"message": "Already processed"})
		return
	}

	// Refund events don't carry an order_id in Razorpay's payload (the
	// refund entity only references the payment it was issued against) —
	// map those by GatewayPaymentID instead of GatewayOrderID.
	var payment models.Payment
	var lookupErr error
	if event.EventType == "refund.processed" || event.EventType == "refund.created" {
		lookupErr = database.DB.Where("gateway_payment_id = ?", event.GatewayPaymentID).First(&payment).Error
	} else {
		lookupErr = database.DB.Where("gateway_order_id = ?", event.GatewayOrderID).First(&payment).Error
	}
	if lookupErr != nil {
		logWebhookEvent(nil, provider.Name(), event.GatewayEventID, event.EventType, true, string(rawBody), "rejected: no matching payment order on this backend")
		// 200, not an error status — this is a valid, signed webhook, it
		// just doesn't correspond to an order this backend created (could
		// be a webhook misconfigured to point at the wrong environment).
		// Returning an error status here would make the gateway retry
		// forever for something that will never resolve.
		c.JSON(http.StatusOK, gin.H{"message": "No matching order"})
		return
	}

	switch event.EventType {
	case "payment.captured", "payment.authorized":
		processPaymentSuccess(c, provider.Name(), payment, event, rawBody)
	case "payment.failed":
		processPaymentFailure(provider.Name(), payment, event, rawBody)
	case "refund.processed", "refund.created":
		processRefund(provider.Name(), payment, event, rawBody)
	default:
		logWebhookEvent(&payment.ID, provider.Name(), event.GatewayEventID, event.EventType, true, string(rawBody), "ignored: unhandled event type")
	}

	c.JSON(http.StatusOK, gin.H{"message": "Processed"})
}

func processPaymentSuccess(c *gin.Context, gateway string, payment models.Payment, event WebhookEvent, rawBody []byte) {
	if event.AmountPaise != payment.AmountPaise || (event.Currency != "" && event.Currency != payment.Currency) {
		logWebhookEvent(&payment.ID, gateway, event.GatewayEventID, event.EventType, true, string(rawBody),
			"rejected: amount/currency mismatch — expected "+strconv.FormatInt(payment.AmountPaise, 10)+" "+payment.Currency+
				", got "+strconv.FormatInt(event.AmountPaise, 10)+" "+event.Currency)
		audit.RecordDetailed(c, audit.Entry{
			ActorType: "system", ActorRef: gateway,
			Action: "payment.webhook", TargetType: "payment", TargetID: strconv.Itoa(int(payment.ID)),
			Result: audit.ResultDenied, Reason: "amount/currency mismatch",
		})
		return
	}

	tx := database.DB.Begin()
	if tx.Error != nil {
		log.Println("payment webhook: failed to begin transaction:", tx.Error)
		return
	}

	var lockedPayment models.Payment
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&lockedPayment, payment.ID).Error; err != nil {
		tx.Rollback()
		return
	}
	if lockedPayment.Status == models.PaymentStatusSuccess {
		// Already settled by an earlier delivery of a differently-keyed
		// but equivalent event — safe no-op, still log the event row.
		tx.Rollback()
		logWebhookEvent(&payment.ID, gateway, event.GatewayEventID, event.EventType, true, string(rawBody), "duplicate: payment already SUCCESS")
		return
	}

	now := time.Now()
	lockedPayment.Status = models.PaymentStatusSuccess
	lockedPayment.GatewayPaymentID = event.GatewayPaymentID
	lockedPayment.Method = event.Method
	lockedPayment.VerifiedAt = &now
	if err := tx.Save(&lockedPayment).Error; err != nil {
		tx.Rollback()
		log.Println("payment webhook: failed to save payment:", err)
		return
	}

	var inst models.EmiInstallment
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&inst, lockedPayment.EmiID).Error; err != nil {
		tx.Rollback()
		log.Println("payment webhook: installment not found:", err)
		return
	}
	if inst.Status != models.EmiStatusPaid {
		inst.Status = models.EmiStatusPaid
		inst.PaidAt = &now
		inst.PaymentReference = event.GatewayPaymentID
		inst.PaymentMethod = event.Method
		if err := tx.Save(&inst).Error; err != nil {
			tx.Rollback()
			log.Println("payment webhook: failed to mark installment paid:", err)
			return
		}
	}

	if err := tx.Commit().Error; err != nil {
		log.Println("payment webhook: failed to commit:", err)
		return
	}

	logWebhookEvent(&payment.ID, gateway, event.GatewayEventID, event.EventType, true, string(rawBody), "applied: payment SUCCESS, EMI marked paid")

	// Accounting posting only happens now — after the webhook-verified
	// commit above (item E) — using the LMS's own principal/interest
	// split on the installment, so accounting and LMS numbers can never
	// disagree. A posting failure is logged, not fatal: the payment/EMI
	// state is already durably committed, and PostEmiPayment's
	// idempotency key on payment ID means a safe retry is possible later.
	if _, err := accounting.PostEmiPayment(lockedPayment, inst, gateway); err != nil {
		log.Println("payment webhook: accounting posting failed:", err)
	}

	// A promise-to-pay is only ever settled by a verified payment reaching
	// this point — never by the mere passage of time (item K).
	collections.CheckPromiseAfterPayment(lockedPayment.LoanID, lockedPayment.ID, lockedPayment.AmountPaise)

	audit.RecordDetailed(c, audit.Entry{
		ActorType: "system", ActorRef: gateway,
		Action: "payment.success", TargetType: "payment", TargetID: strconv.Itoa(int(payment.ID)),
		Result: audit.ResultSuccess,
		Details: "loan #" + strconv.Itoa(int(payment.LoanID)) + ", emi #" + strconv.Itoa(int(payment.EmiID)) +
			", gatewayPaymentId " + event.GatewayPaymentID,
	})
	audit.RecordDetailed(c, audit.Entry{
		ActorType: "system", ActorRef: gateway,
		Action: "emi.pay", TargetType: "emi_installment", TargetID: strconv.Itoa(int(inst.ID)),
		Result:  audit.ResultSuccess,
		Details: "Settled via verified gateway webhook, payment #" + strconv.Itoa(int(payment.ID)),
	})
}

func processPaymentFailure(gateway string, payment models.Payment, event WebhookEvent, rawBody []byte) {
	if payment.Status == models.PaymentStatusSuccess {
		// A failure event arriving after we already have a verified
		// success (out-of-order delivery) must never downgrade a settled
		// payment.
		logWebhookEvent(&payment.ID, gateway, event.GatewayEventID, event.EventType, true, string(rawBody), "ignored: payment already SUCCESS")
		return
	}
	database.DB.Model(&payment).Updates(map[string]interface{}{
		"status":         models.PaymentStatusFailed,
		"failure_reason": event.FailureReason,
	})
	logWebhookEvent(&payment.ID, gateway, event.GatewayEventID, event.EventType, true, string(rawBody), "applied: payment FAILED")
	audit.RecordDetailed(nil, audit.Entry{
		ActorType: "system", ActorRef: gateway,
		Action: "payment.failure", TargetType: "payment", TargetID: strconv.Itoa(int(payment.ID)),
		Result: audit.ResultSuccess, Reason: event.FailureReason,
	})
}

func processRefund(gateway string, payment models.Payment, event WebhookEvent, rawBody []byte) {
	if event.RefundID == "" {
		logWebhookEvent(&payment.ID, gateway, event.GatewayEventID, event.EventType, true, string(rawBody), "ignored: refund event with no refund id")
		return
	}
	var existing models.Refund
	if err := database.DB.Where("gateway_refund_id = ?", event.RefundID).First(&existing).Error; err == nil {
		logWebhookEvent(&payment.ID, gateway, event.GatewayEventID, event.EventType, true, string(rawBody), "duplicate: refund already recorded")
		return
	}

	// Cumulative refund validation (Phase 9 item K: "prevent refund >
	// original amount"). This is a gateway-initiated event (Razorpay
	// already processed the refund on its side before notifying us) —
	// this codebase cannot un-refund money already sent, so an over-
	// refund can't be "prevented" at this layer the way an admin-initiated
	// refund request could be; instead it's recorded (never silently
	// dropped — the money genuinely moved) but flagged loudly rather than
	// treated as a normal, in-range refund.
	var priorRefunded int64
	database.DB.Model(&models.Refund{}).Where("payment_id = ?", payment.ID).
		Select("COALESCE(SUM(amount_paise), 0)").Scan(&priorRefunded)
	cumulativeRefunded := priorRefunded + event.RefundAmountPaise
	overRefund := cumulativeRefunded > payment.AmountPaise

	refund := models.Refund{
		PaymentID:       payment.ID,
		GatewayRefundID: event.RefundID,
		AmountPaise:     event.RefundAmountPaise,
		Status:          "processed",
	}
	if overRefund {
		refund.Reason = "OVER_REFUND_FLAGGED: cumulative refunded " + strconv.FormatInt(cumulativeRefunded, 10) +
			" paise exceeds original payment " + strconv.FormatInt(payment.AmountPaise, 10) + " paise"
	}
	if err := database.DB.Create(&refund).Error; err != nil {
		log.Println("payment webhook: failed to record refund:", err)
		return
	}

	switch {
	case cumulativeRefunded >= payment.AmountPaise:
		database.DB.Model(&payment).Update("status", models.PaymentStatusRefunded)
	case cumulativeRefunded > 0:
		database.DB.Model(&payment).Update("status", models.PaymentStatusPartiallyRefunded)
	}

	// Accounting integration (Phase 8/9 gap closure — accounting.PostRefund
	// existed since Phase 4 but was never actually called from this path).
	// Capped at the original payment amount even for a flagged over-refund,
	// since the ledger's Loan Principal Receivable can never be reinstated
	// by more than what was originally collected against it — any excess
	// is a matter for manual investigation, not an automatic ledger entry.
	postAmount := event.RefundAmountPaise
	if overRefund {
		postAmount = payment.AmountPaise - priorRefunded
		if postAmount < 0 {
			postAmount = 0
		}
	}
	if postAmount > 0 {
		if _, err := accounting.PostRefund(payment, postAmount, gateway); err != nil {
			log.Println("payment webhook: accounting refund posting failed:", err)
		}
	}

	result := audit.ResultSuccess
	if overRefund {
		result = audit.ResultDenied // surfaced as a denied/flagged result even though the refund record itself was kept, per the "loudly flagged" note above
		risk.CreateAlert("REFUND_OVER_ORIGINAL_AMOUNT", models.AlertSeverityCritical, nil,
			"Payment #"+strconv.Itoa(int(payment.ID))+": cumulative refunds ("+strconv.FormatInt(cumulativeRefunded, 10)+
				" paise) exceed the original payment amount ("+strconv.FormatInt(payment.AmountPaise, 10)+" paise) — requires manual investigation")
	}
	logWebhookEvent(&payment.ID, gateway, event.GatewayEventID, event.EventType, true, string(rawBody), "applied: refund recorded")
	audit.RecordDetailed(nil, audit.Entry{
		ActorType: "system", ActorRef: gateway,
		Action: "payment.refund", TargetType: "payment", TargetID: strconv.Itoa(int(payment.ID)),
		Result:  result,
		Details: "refund " + event.RefundID + ", amount " + strconv.FormatInt(event.RefundAmountPaise, 10) + " paise, cumulative " + strconv.FormatInt(cumulativeRefunded, 10),
		Reason:  refund.Reason,
	})
}

func logWebhookEvent(paymentID *uint, gateway, gatewayEventID, eventType string, signatureValid bool, rawBody, note string) {
	entry := models.PaymentEvent{
		PaymentID:      paymentID,
		Gateway:        gateway,
		GatewayEventID: gatewayEventID,
		EventType:      eventType,
		SignatureValid: signatureValid,
		RawPayload:     rawBody,
		ProcessedNote:  note,
	}
	if err := database.DB.Create(&entry).Error; err != nil {
		log.Println("payment webhook: failed to log event:", err)
	}
}
