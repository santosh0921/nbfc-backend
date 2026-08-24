package models

import "time"

// Admin roles — fixed set, matches internal/rbac's RolePermissions map.
// Stored as a plain string column (not a DB enum) so adding a role later
// is a Go-level change, not a migration.
const (
	AdminRoleSuperAdmin          = "SUPER_ADMIN"
	AdminRoleCreditOfficer       = "CREDIT_OFFICER"
	AdminRoleCreditManager       = "CREDIT_MANAGER"
	AdminRoleOperations          = "OPERATIONS"
	AdminRoleDisbursementOfficer = "DISBURSEMENT_OFFICER"
	AdminRoleCollectionOfficer   = "COLLECTION_OFFICER"
	AdminRoleCollectionManager   = "COLLECTION_MANAGER"
	AdminRoleAccountant          = "ACCOUNTANT"
	AdminRoleComplianceOfficer   = "COMPLIANCE_OFFICER"
	AdminRoleAuditor             = "AUDITOR"
	AdminRoleReadOnly            = "READ_ONLY"
)

const (
	AdminStatusActive   = "active"
	AdminStatusDisabled = "disabled"
)

// AdminUser replaces the single shared ADMIN_USERNAME/ADMIN_PASSWORD .env
// credential with individually identifiable, individually revocable admin
// accounts — every admin action's audit trail can now be attributed to a
// real person instead of one shared login.
type AdminUser struct {
	ID           uint   `gorm:"primaryKey" json:"id"`
	Username     string `gorm:"uniqueIndex;not null" json:"username"`
	Email        string `gorm:"uniqueIndex;not null" json:"email"`
	PasswordHash string `gorm:"not null" json:"-"`
	Role         string `gorm:"index;not null" json:"role"`
	Status       string `gorm:"index;not null;default:active" json:"status"`

	FailedLoginAttempts int        `gorm:"default:0" json:"-"`
	LockedUntil         *time.Time `json:"-"`
	LastLoginAt         *time.Time `json:"lastLoginAt,omitempty"`

	CreatedBy string `json:"createdBy,omitempty"` // username of the admin who created this account ("bootstrap" for the seeded super-admin)

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// AdminSession backs refresh-token rotation and server-side session
// revocation — a plain JWT alone can't be invalidated before it expires,
// which is exactly the "logout doesn't really log out" gap this closes.
// Only a hash of the refresh token is stored, never the token itself.
type AdminSession struct {
	ID               uint       `gorm:"primaryKey" json:"id"`
	AdminUserID      uint       `gorm:"index;not null" json:"adminUserId"`
	RefreshTokenHash string     `gorm:"uniqueIndex;not null" json:"-"`
	UserAgent        string     `json:"userAgent"`
	IPAddress        string     `json:"ipAddress"`
	CreatedAt        time.Time  `json:"createdAt"`
	ExpiresAt        time.Time  `json:"expiresAt"`
	RevokedAt        *time.Time `json:"revokedAt,omitempty"`
}
