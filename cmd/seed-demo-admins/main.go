// One-off local tool: seeds 8 pre-approved demo admin accounts (one per
// role used across the 7 portals) directly into the production
// admin_users table, run against a temporarily-opened RDS security group
// rule (removed again immediately after). Safe to re-run — upserts by
// username. Not part of the deployed application.
package main

import (
	"fmt"
	"log"

	"github.com/joho/godotenv"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/santosh0921/nbfc-backend/internal/models"
)

const demoPassword = "DemoPass@12345"

func main() {
	env, err := godotenv.Read("seed-prod-db.env")
	if err != nil {
		log.Fatal("load seed-prod-db.env: ", err)
	}
	dsn := fmt.Sprintf(
		"host=%s user=%s password=%s dbname=%s port=%s sslmode=require",
		env["SEED_DB_HOST"], env["SEED_DB_USER"], env["SEED_DB_PASSWORD"], env["SEED_DB_NAME"], env["SEED_DB_PORT"],
	)
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatal(err)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(demoPassword), bcrypt.DefaultCost)
	if err != nil {
		log.Fatal(err)
	}

	demos := []struct {
		username string
		role     string
	}{
		{"demo_credit_officer", models.AdminRoleCreditOfficer},
		{"demo_credit_manager", models.AdminRoleCreditManager},
		{"demo_compliance_officer", models.AdminRoleComplianceOfficer},
		{"demo_operations", models.AdminRoleOperations},
		{"demo_disbursement_officer", models.AdminRoleDisbursementOfficer},
		{"demo_collection_officer", models.AdminRoleCollectionOfficer},
		{"demo_collection_manager", models.AdminRoleCollectionManager},
		{"demo_correction_officer", "CORRECTION_OFFICER"},
	}

	for _, d := range demos {
		var existing models.AdminUser
		err := db.Where("username = ?", d.username).First(&existing).Error
		if err == nil {
			existing.PasswordHash = string(hash)
			existing.Role = d.role
			existing.Status = models.AdminStatusActive
			existing.Approved = true
			existing.FailedLoginAttempts = 0
			existing.LockedUntil = nil
			if err := db.Save(&existing).Error; err != nil {
				log.Fatalf("update %s: %v", d.username, err)
			}
			fmt.Printf("updated  %-28s role=%s\n", d.username, d.role)
			continue
		}
		user := models.AdminUser{
			Username:     d.username,
			Email:        d.username + "@demo.crednova.local",
			PasswordHash: string(hash),
			Role:         d.role,
			Status:       models.AdminStatusActive,
			Approved:     true,
			CreatedBy:    "seed-demo-admins",
		}
		if err := db.Create(&user).Error; err != nil {
			log.Fatalf("create %s: %v", d.username, err)
		}
		fmt.Printf("created  %-28s role=%s\n", d.username, d.role)
	}

	fmt.Println("\ndemo password for all accounts:", demoPassword)
}
