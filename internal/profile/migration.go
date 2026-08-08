package profile

import (
	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/models"
)

func Migrate() error {
	return database.DB.AutoMigrate(
		&models.CustomerProfile{},
		&models.CustomerAddress{},
		&models.CustomerPAN{},
		&models.CustomerAadhaar{},
		&models.CustomerSelfie{},
		&models.CustomerEmployment{}, 


	)
}