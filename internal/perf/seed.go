package perf

import (
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/models"
)

// PerfMobilePrefix marks every row this harness creates. Real customer
// mobiles are 10 digits; these are deliberately longer and prefixed so
// they can never collide with, or be mistaken for, real data — and so
// Cleanup can delete exactly what it created and nothing else.
const PerfMobilePrefix = "99000"

// SeedCustomers inserts n isolated performance-test customers with the
// related rows the admin user-list handler touches (profile, address,
// PAN, KYC review), so a measurement of that handler reflects the real
// per-user query fan-out rather than an artificially empty one.
//
// Inserts in batches — a single 100k-row INSERT would itself be the
// bottleneck being measured.
func SeedCustomers(n int) error {
	const batch = 500
	now := time.Now()

	users := make([]models.User, 0, batch)
	profiles := make([]models.CustomerProfile, 0, batch)
	addresses := make([]models.CustomerAddress, 0, batch)

	flush := func() error {
		if len(users) == 0 {
			return nil
		}
		if err := database.DB.CreateInBatches(&users, batch).Error; err != nil {
			return err
		}
		if err := database.DB.CreateInBatches(&profiles, batch).Error; err != nil {
			return err
		}
		if err := database.DB.CreateInBatches(&addresses, batch).Error; err != nil {
			return err
		}
		users = users[:0]
		profiles = profiles[:0]
		addresses = addresses[:0]
		return nil
	}

	for i := 0; i < n; i++ {
		id := uuid.New()
		users = append(users, models.User{
			ID:               id,
			Mobile:           fmt.Sprintf("%s%09d", PerfMobilePrefix, i),
			IsMobileVerified: true,
			IsActive:         true,
			CreatedAt:        now.Add(-time.Duration(i) * time.Minute),
		})
		profiles = append(profiles, models.CustomerProfile{
			UserID: id, FirstName: "Perf", LastName: fmt.Sprintf("User%d", i),
		})
		addresses = append(addresses, models.CustomerAddress{
			UserID: id, CurrentCity: "PerfCity", CurrentState: "PerfState",
		})
		if len(users) >= batch {
			if err := flush(); err != nil {
				return err
			}
		}
	}
	return flush()
}

// CountSeeded reports how many perf customers currently exist.
func CountSeeded() int64 {
	var n int64
	database.DB.Model(&models.User{}).
		Where("mobile LIKE ?", PerfMobilePrefix+"%").Count(&n)
	return n
}

// Cleanup removes every row this harness created, matched by the
// prefix — never a blanket delete.
func Cleanup() error {
	var ids []uuid.UUID
	if err := database.DB.Model(&models.User{}).
		Where("mobile LIKE ?", PerfMobilePrefix+"%").
		Pluck("id", &ids).Error; err != nil {
		return err
	}
	if len(ids) == 0 {
		return nil
	}
	// Children first, then the users themselves.
	database.DB.Unscoped().Where("user_id IN ?", ids).Delete(&models.CustomerProfile{})
	database.DB.Unscoped().Where("user_id IN ?", ids).Delete(&models.CustomerAddress{})
	database.DB.Unscoped().Where("user_id IN ?", ids).Delete(&models.CustomerKYCReview{})
	return database.DB.Unscoped().Where("id IN ?", ids).Delete(&models.User{}).Error
}
