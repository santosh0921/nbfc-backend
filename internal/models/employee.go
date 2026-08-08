package models

import "time"

type Agency struct {
	ID      uint   `gorm:"primaryKey" json:"id"`
	Name    string `json:"name"`
	City    string `json:"city"`
	Address string `json:"address"`
	Active  bool   `gorm:"default:true" json:"active"`
}

type Employee struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	Code         string    `gorm:"uniqueIndex" json:"code"`
	Name         string    `json:"name"`
	PasswordHash string    `json:"-"`
	Role         string    `json:"role"` // verification | recovery | supervisor
	BranchCity   string    `json:"branchCity"`
	AgencyID     uint      `json:"agencyId"`
	Active       bool      `gorm:"default:true" json:"active"`
	CreatedAt    time.Time `json:"createdAt"`
}
