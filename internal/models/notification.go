package models

import "time"

type Notification struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Role      string    `json:"role"` // customer | employee
	UserID    string    `json:"userId"`
	Title     string    `json:"title"`
	Body      string    `json:"body"`
	Read      bool      `gorm:"default:false" json:"read"`
	CreatedAt time.Time `json:"createdAt"`
}
