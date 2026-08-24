package audit

import (
	"log"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/models"
)

const (
	ResultSuccess = "success"
	ResultFailure = "failure"
	ResultDenied  = "denied"
)

// Entry is the structured payload for RecordDetailed — every sensitive
// action added in this phase (admin login, maker-checker steps, admin
// user/role changes, KYC decisions, document/export access) should build
// one of these rather than calling the older free-text Record/RecordAs.
type Entry struct {
	ActorType  string // "admin" | "customer" | "employee" | "system"
	ActorID    uint
	ActorRef   string
	ActorRole  string
	Action     string
	TargetType string
	TargetID   string
	Result     string // ResultSuccess | ResultFailure | ResultDenied
	Reason     string
	Details    string
	Before     interface{} // passed through audit.Snapshot (redacted) — pass nil if not applicable
	After      interface{}
}

// RequestID returns the X-Request-Id set by middleware.RequestID for this
// request, or "" if that middleware isn't in the chain (e.g. a call with
// no gin.Context, such as a background job).
func RequestID(c *gin.Context) string {
	if c == nil {
		return ""
	}
	if v, ok := c.Get("request_id"); ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// RecordDetailed writes a fully structured audit row: actor identity, IP,
// user-agent, request-id, result, reason, and redacted before/after
// snapshots. c may be nil for actions with no HTTP request (none exist
// yet in this phase, but the signature allows for it).
func RecordDetailed(c *gin.Context, e Entry) {
	entry := models.AuditLog{
		ActorType:   e.ActorType,
		ActorID:     e.ActorID,
		ActorRef:    e.ActorRef,
		ActorRole:   e.ActorRole,
		Action:      e.Action,
		TargetType:  e.TargetType,
		TargetID:    e.TargetID,
		Details:     e.Details,
		Result:      e.Result,
		Reason:      e.Reason,
		BeforeValue: Snapshot(e.Before),
		AfterValue:  Snapshot(e.After),
	}
	if c != nil {
		entry.IPAddress = c.ClientIP()
		entry.UserAgent = c.Request.UserAgent()
		entry.RequestID = RequestID(c)
	}
	if entry.RequestID == "" {
		entry.RequestID = uuid.NewString()
	}
	if err := database.DB.Create(&entry).Error; err != nil {
		log.Println("audit.RecordDetailed: failed to insert audit log entry:", err)
	}
}
