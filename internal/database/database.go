package database

import (
	"fmt"
	"log"
	"os"

	"github.com/santosh0921/nbfc-backend/internal/config"


	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var DB *gorm.DB

func Connect(cfg *config.Config) {

	// Local Postgres (native install, no Docker) doesn't have SSL set up,
	// so "disable" stays the default — but managed Postgres (Render, RDS,
	// etc.) requires SSL, so this is overridable via DB_SSLMODE without
	// touching code for each deploy target.
	sslMode := os.Getenv("DB_SSLMODE")
	if sslMode == "" {
		sslMode = "disable"
	}

	dsn := fmt.Sprintf(
		"host=%s user=%s password=%s dbname=%s port=%s sslmode=%s",
		cfg.DBHost,
		cfg.DBUser,
		cfg.DBPassword,
		cfg.DBName,
		cfg.DBPort,
		sslMode,
	)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})

	if err != nil {
		log.Fatal("Database connection failed:", err)
	}

	DB = db

	log.Println("✅ Database connected successfully")
}