package models

import (
	"time"

	"github.com/google/uuid"
)

const (
	SupportThreadOpen   = "open"
	SupportThreadClosed = "closed"
)

const (
	SupportSenderCustomer = "customer"
	SupportSenderAdmin    = "admin"
)

type SupportThread struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	CustomerID uuid.UUID `gorm:"type:uuid;index" json:"customerId"`
	Status     string    `json:"status"` // "open" | "closed"
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

type SupportMessage struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	ThreadID   uint      `gorm:"index" json:"threadId"`
	SenderType string    `json:"senderType"` // "customer" | "admin"
	SenderRef  string    `json:"senderRef"`
	Body       string    `gorm:"type:text" json:"body"`
	CreatedAt  time.Time `json:"createdAt"`
}
