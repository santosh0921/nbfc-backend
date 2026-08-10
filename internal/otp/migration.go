package otp

import (
	"log"

	"github.com/santosh0921/nbfc-backend/internal/database"
)

func Migrate() {

	// The OTPVerification struct used to have a plaintext `OTP` column,
	// renamed to `OTPHash` when OTP storage was switched to a SHA-256
	// hash. GORM's AutoMigrate only ever ADDS columns it doesn't
	// recognize — it never drops or renames one for a field that no
	// longer exists in the struct. On any database that already had the
	// old `otp` column (still NOT NULL, with no default), every new
	// insert would otherwise violate that constraint, since the current
	// code has no field to populate it with — this must run before
	// AutoMigrate creates/touches anything else so it can't race a
	// concurrent insert into an inconsistent mid-migration state.
	if database.DB.Migrator().HasTable(&OTPVerification{}) && database.DB.Migrator().HasColumn(&OTPVerification{}, "otp") {
		if err := database.DB.Migrator().DropColumn(&OTPVerification{}, "otp"); err != nil {
			log.Fatal("OTP migration failed (dropping legacy otp column):", err)
		}
	}

	err := database.DB.AutoMigrate(&OTPVerification{})

	if err != nil {
		log.Fatal("OTP migration failed:", err)
	}

	log.Println("✅ OTP table migrated successfully")
}