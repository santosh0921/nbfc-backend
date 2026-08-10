package audit

import (
	"log"

	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/models"
)

// Record inserts an admin-actor audit log row. Fire-and-forget from the
// caller's perspective — it never returns an error — but a failed insert
// is logged rather than silently swallowed. Kept as-is (actorType always
// "admin") so every existing call site is unaffected; non-admin actors
// (customers, employees) should use RecordAs instead.
func Record(actorRef, action, targetType, targetID, details string) {
	RecordAs("admin", actorRef, action, targetType, targetID, details)
}

// RecordAs is [Record] with an explicit actor type, for audit trails
// initiated by a customer or employee rather than an admin — mislabeling
// every actor as "admin" would undermine the audit trail's own purpose.
func RecordAs(actorType, actorRef, action, targetType, targetID, details string) {
	entry := models.AuditLog{
		ActorType:  actorType,
		ActorRef:   actorRef,
		Action:     action,
		TargetType: targetType,
		TargetID:   targetID,
		Details:    details,
	}
	if err := database.DB.Create(&entry).Error; err != nil {
		log.Println("audit.Record: failed to insert audit log entry:", err)
	}
}
