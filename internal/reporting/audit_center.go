// Package reporting implements the Phase 6 MIS / regulatory-reporting /
// compliance-monitoring / audit-readiness layer. It reads from every
// prior phase's authoritative tables (accounting, risk, kyc, bureau,
// collections, ...) and never recomputes a figure independently of them —
// an MIS number and the underlying engine's own number must never be able
// to drift apart.
package reporting

import (
	"sort"
	"time"

	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/models"
)

// TimelineEvent is one entry in a reconstructed audit timeline — sourced
// entirely from models.AuditLog (append-only, DB-trigger-enforced since
// Phase 1) plus a handful of typed history tables for detail AuditLog's
// free-text Details field doesn't carry structurally (classification/DPD
// history). Never built from a mutable free-text log.
type TimelineEvent struct {
	Timestamp  time.Time `json:"timestamp"`
	ActorType  string    `json:"actorType"`
	ActorRef   string    `json:"actorRef"`
	ActorRole  string    `json:"actorRole,omitempty"`
	Action     string    `json:"action"`
	TargetType string    `json:"targetType"`
	TargetID   string    `json:"targetId"`
	Result     string    `json:"result,omitempty"`
	Reason     string    `json:"reason,omitempty"`
	Source     string    `json:"source"` // "audit_log" | "classification_history" | "dpd_history"
}

// LoanTimeline (item G/H) — every audited event touching this loan or any
// entity keyed to it (disbursement, payments, collection case, write-off
// request), in chronological order, plus the structured classification
// history Phase 5 already keeps immutably. This is deliberately a read
// query with no write path — nothing in this package can alter any of the
// underlying records (item G: "do not allow auditors to modify any
// data").
func LoanTimeline(loanID uint) []TimelineEvent {
	var events []TimelineEvent

	var loanLogs []models.AuditLog
	database.DB.Where("target_type = ? AND target_id = ?", "loan", itoa(loanID)).Find(&loanLogs)
	events = append(events, fromAuditLogs(loanLogs)...)

	appendRelated := func(targetType, idQuery string, args ...interface{}) {
		var ids []string
		database.DB.Raw(idQuery, args...).Scan(&ids)
		if len(ids) == 0 {
			return
		}
		var logs []models.AuditLog
		database.DB.Where("target_type = ? AND target_id IN ?", targetType, ids).Find(&logs)
		events = append(events, fromAuditLogs(logs)...)
	}

	appendRelated("disbursement", "SELECT id::text FROM disbursements WHERE loan_id = ?", loanID)
	appendRelated("payment", "SELECT id::text FROM payments WHERE loan_id = ?", loanID)
	appendRelated("collection_case", "SELECT id::text FROM collection_cases WHERE loan_id = ?", loanID)
	appendRelated("write_off_request", "SELECT id::text FROM write_off_requests WHERE loan_id = ?", loanID)
	appendRelated("collection_promise", "SELECT id::text FROM collection_promises WHERE loan_id = ?", loanID)

	var history []models.LoanClassificationHistory
	database.DB.Where("loan_id = ?", loanID).Find(&history)
	for _, h := range history {
		events = append(events, TimelineEvent{
			Timestamp: h.CreatedAt, ActorType: actorTypeFor(h.CalculatedBy), ActorRef: h.CalculatedBy,
			Action: "risk.classification." + h.NewClassification, TargetType: "loan", TargetID: itoa(loanID),
			Reason: h.Reason, Source: "classification_history",
		})
	}

	sortByTimestamp(events)
	return events
}

// CustomerTimeline (item G) — the customer-level equivalent, covering
// KYC/screening/bureau events not tied to any single loan.
func CustomerTimeline(customerID string) []TimelineEvent {
	var events []TimelineEvent

	for _, targetType := range []string{"customer", "customer_kyc", "bureau_request", "bureau_consent", "screening_request", "ckyc_request"} {
		var logs []models.AuditLog
		database.DB.Where("target_type = ? AND actor_ref = ?", targetType, customerID).Find(&logs)
		events = append(events, fromAuditLogs(logs)...)
	}

	var kycLogs []models.AuditLog
	database.DB.Where("target_type = ? AND target_id = ?", "customer_kyc", customerID).Find(&kycLogs)
	events = append(events, fromAuditLogs(kycLogs)...)

	sortByTimestamp(events)
	return events
}

func fromAuditLogs(logs []models.AuditLog) []TimelineEvent {
	out := make([]TimelineEvent, 0, len(logs))
	for _, l := range logs {
		out = append(out, TimelineEvent{
			Timestamp: l.CreatedAt, ActorType: l.ActorType, ActorRef: l.ActorRef, ActorRole: l.ActorRole,
			Action: l.Action, TargetType: l.TargetType, TargetID: l.TargetID,
			Result: l.Result, Reason: l.Reason, Source: "audit_log",
		})
	}
	return out
}

func sortByTimestamp(events []TimelineEvent) {
	sort.Slice(events, func(i, j int) bool { return events[i].Timestamp.Before(events[j].Timestamp) })
}

func actorTypeFor(actor string) string {
	if actor == "system" || actor == "" {
		return "system"
	}
	return "admin"
}
