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
	// Approved is false only for an employee who self-registered (POST
	// /employee/login self-registration is not a thing — this is set by
	// POST /employee/register) and hasn't yet been approved by an admin.
	// Admin-created employees (AdminCreateHandler) are pre-vetted and are
	// always created with Approved explicitly true. The `default:true`
	// tag exists purely so adding this column to an existing database
	// backfills every already-existing (and therefore already-vetted)
	// employee row as approved, rather than accidentally locking every
	// existing employee out of their own account the moment this column
	// is added.
	Approved  bool      `gorm:"default:true" json:"approved"`
	CreatedAt time.Time `json:"createdAt"`
}
