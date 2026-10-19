package models

import "time"

// KycVerificationRequest records a KYC officer's request for a customer to
// schedule a video-verification call, and the customer's response. This is
// deliberately a separate row from CustomerKYC/CustomerKYCReview: a
// customer can be asked to schedule more than once (a missed call, a
// rescheduled one), and each ask/response pair should stay visible in the
// case history rather than overwriting a single "scheduling" field.
type KycVerificationRequest struct {
	ID                   uint       `gorm:"primaryKey" json:"id"`
	CustomerID           string     `json:"customerId"` // uuid, matches Customer/User.ID.String()
	RequestedByAdminID   uint       `json:"requestedByAdminId"`
	RequestedByAdminName string     `json:"requestedByAdminName"`
	Status               string     `gorm:"default:'pending'" json:"status"` // "pending" | "customer_ready" | "declined"
	PreferredTime        string     `json:"preferredTime"`                   // free text customer enters, e.g. "today 5pm"
	RespondedAt          *time.Time `json:"respondedAt"`
	CreatedAt            time.Time  `json:"createdAt"`
	UpdatedAt            time.Time  `json:"updatedAt"`
}
