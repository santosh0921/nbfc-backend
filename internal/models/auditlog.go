package models

import "time"

// AuditLog is the compliance audit trail. Rows are intended to be
// append-only — see internal/admin/migration.go's audit-log trigger,
// which rejects UPDATE/DELETE at the database level so an ordinary admin
// (or a bug) can't quietly edit history.
//
// Older fields (ActorType/ActorRef/Action/TargetType/TargetID/Details) are
// kept unchanged so every pre-existing audit.Record/RecordAs call site
// keeps compiling and keeps writing rows in the same shape; the new
// fields below are additive and populated by the new
// audit.RecordDetailed path used for every sensitive action added in this
// phase.
type AuditLog struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	ActorType  string    `json:"actorType"` // "admin" | "customer" | "employee" | "system"
	ActorRef   string    `json:"actorRef"`  // human-readable actor identifier (admin username, customer mobile, employee code)
	Action     string    `json:"action"`    // e.g. "loan.sanction.recommend", "admin.login.failed"
	TargetType string    `json:"targetType"`
	TargetID   string    `json:"targetId"`
	Details    string    `gorm:"type:text" json:"details"`
	CreatedAt  time.Time `json:"createdAt"`

	// New, structured fields — populated via RecordDetailed. Left blank
	// ("", 0) on rows written via the older Record/RecordAs helpers.
	ActorID   uint   `gorm:"index" json:"actorId,omitempty"` // admin_users.id when ActorType=="admin"; 0 otherwise
	ActorRole string `json:"actorRole,omitempty"`
	IPAddress string `json:"ipAddress,omitempty"`
	UserAgent string `json:"userAgent,omitempty"`
	RequestID string `gorm:"index" json:"requestId,omitempty"`
	Result    string `json:"result,omitempty"` // "success" | "failure" | "denied"
	Reason    string `gorm:"type:text" json:"reason,omitempty"`

	// BeforeValue/AfterValue hold redacted JSON snapshots (see
	// internal/audit/redact.go) — never raw passwords/OTPs/JWT
	// secrets/unmasked Aadhaar-PAN, by construction of what callers pass in.
	BeforeValue string `gorm:"type:text" json:"beforeValue,omitempty"`
	AfterValue  string `gorm:"type:text" json:"afterValue,omitempty"`
}
