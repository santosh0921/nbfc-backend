package audit

import (
	"log"

	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/models"
)

// Record inserts an audit log row. Fire-and-forget from the caller's
// perspective — it never returns an error — but a failed insert is logged
// rather than silently swallowed.
func Record(actorRef, action, targetType, targetID, details string) {
	entry := models.AuditLog{
		ActorType:  "admin",
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
