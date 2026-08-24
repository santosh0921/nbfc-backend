package database

import (
	"fmt"
	"log"
	"os"
	"time"

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

	// statement_timeout (Phase 7 item H): a single runaway/misbehaving
	// query (e.g. an unbounded report scan) should never be able to hold
	// a connection — and therefore a slot in the pool below — forever.
	// 30s is a conservative ceiling for this application's normal
	// request-scoped queries; a long-running background job (the daily
	// risk job, report generation) should use its own explicit context
	// timeout rather than relying on raising this global default.
	dsn := fmt.Sprintf(
		"host=%s user=%s password=%s dbname=%s port=%s sslmode=%s options='-c statement_timeout=30000'",
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

	// Connection pool limits (Phase 7 item H/R) — without these, a
	// traffic spike (or a leak) can open unbounded connections against
	// Postgres until it refuses new ones for every other client too.
	// Values here are conservative defaults suitable for a single-
	// instance deployment; a multi-instance production deployment should
	// size these against Postgres's own max_connections divided by
	// instance count, not just raise them independently.
	sqlDB, err := db.DB()
	if err != nil {
		log.Fatal("Failed to access underlying sql.DB:", err)
	}
	sqlDB.SetMaxOpenConns(25)
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetConnMaxLifetime(30 * time.Minute)
	sqlDB.SetConnMaxIdleTime(5 * time.Minute)

	DB = db

	log.Println("✅ Database connected successfully")
}