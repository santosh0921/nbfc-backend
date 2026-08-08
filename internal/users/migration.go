package users

import (
	"log"

	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/models"

)

func Migrate() {

	err := database.DB.AutoMigrate(&models.User{})

	if err != nil {
		log.Fatal("User migration failed:", err)
	}

	log.Println("✅ Users table migrated successfully")
}