package admin

import (
	"log"

	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/models"
)

func Migrate() {

	err := database.DB.AutoMigrate(
		&models.CustomerKYCReview{},
		&models.Agency{},
		&models.Employee{},
		&models.LoanApplication{},
		&models.LoanReference{},
		&models.RecoveryCase{},
		&models.RecoveryReport{},
		&models.CaseNote{},
		&models.Notification{},
		&models.Document{},
		&models.EmiInstallment{},
		&models.AuditLog{},
		&models.SupportThread{},
		&models.SupportMessage{},
	)

	if err != nil {
		log.Fatal("Admin KYC review migration failed:", err)
	}

	log.Println("✅ Admin KYC review table migrated successfully")
}
