package models

import "time"

type AuditLog struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	ActorType  string    `json:"actorType"` // "admin" for now
	ActorRef   string    `json:"actorRef"`  // admin username from JWT claims
	Action     string    `json:"action"`    // e.g. "loan.sanction", "loan.reject", ...
	TargetType string    `json:"targetType"` // "loan" | "employee" | "agency" | "recovery_case"
	TargetID   string    `json:"targetId"`
	Details    string    `gorm:"type:text" json:"details"`
	CreatedAt  time.Time `json:"createdAt"`
}
