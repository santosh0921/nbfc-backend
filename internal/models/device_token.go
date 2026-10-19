package models

import "time"

// DeviceToken is one device's Firebase Cloud Messaging registration token
// for a specific customer or employee — the backend's push-notification
// counterpart to Notification (which is the in-app inbox row; DeviceToken
// is what internal/push.SendToUser actually sends a real device push to).
// A raw FCM token has no meaningful uniqueness beyond "this device, this
// install" — it's re-issued on reinstall/token-refresh and can briefly
// belong to a different account after a logout/login on the same device,
// so Token itself (not Role+UserID) is the real identity key.
type DeviceToken struct {
	ID     uint   `gorm:"primaryKey" json:"id"`
	Role   string `gorm:"index:idx_device_token_role_user" json:"role"` // customer | employee
	UserID string `gorm:"index:idx_device_token_role_user" json:"userId"`
	Token  string `gorm:"uniqueIndex;not null" json:"-"`
	// Platform is always "android" today — this app has no iOS/web build.
	Platform  string    `json:"platform"`
	CreatedAt time.Time `json:"createdAt"`
}
