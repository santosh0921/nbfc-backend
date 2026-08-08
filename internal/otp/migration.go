package otp

import (
	"log"

	"github.com/santosh0921/nbfc-backend/internal/database"
)

func Migrate() {

	err := database.DB.AutoMigrate(&OTPVerification{})

	if err != nil {
		log.Fatal("OTP migration failed:", err)
	}

	log.Println("✅ OTP table migrated successfully")
}