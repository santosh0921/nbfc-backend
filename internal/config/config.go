package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	AppName    string
	AppPort    string
	GinMode    string
	DBHost     string
	DBPort     string
	DBUser     string
	DBPassword string
	DBName     string

	JWTSecret         string
	AdminJWTSecret    string
	EmployeeJWTSecret string

	SurepassBaseURL    string
    SurepassBearerToken string

	DemoMode bool
}

// DemoMode mirrors Config.DemoMode as a package-level flag so handlers that
// don't carry a *Config reference (pan_handler.go, aadhaar_handler.go) can
// check it directly. Set once by Load() at startup.
var DemoMode bool

func Load() *Config {

	// godotenv.Load() only applies when a .env file is actually present
	// (local dev). On a deploy platform (Render, etc.) there is no .env
	// file — secrets are injected directly as real environment variables
	// — so a missing file here is expected, not fatal.
	if err := godotenv.Load(); err != nil {
		log.Println(".env file not found — reading configuration from real environment variables instead")
	}

	appPort := os.Getenv("APP_PORT")
	if appPort == "" {
		// Render (and most PaaS platforms) inject the port to bind to via
		// $PORT rather than a custom var — fall back to it so the app is
		// deployable with zero platform-specific config.
		appPort = os.Getenv("PORT")
	}
	if appPort == "" {
		appPort = "8080"
	}

	return &Config{
		AppName: os.Getenv("APP_NAME"),
		AppPort:    appPort,
		GinMode: os.Getenv("GIN_MODE"),

		DBHost:     os.Getenv("DB_HOST"),
		DBPort:     os.Getenv("DB_PORT"),
		DBUser:     os.Getenv("DB_USER"),
		DBPassword: os.Getenv("DB_PASSWORD"),
		DBName: os.Getenv("DB_NAME"),

		JWTSecret:         os.Getenv("JWT_SECRET"),
		AdminJWTSecret:    os.Getenv("ADMIN_JWT_SECRET"),
		EmployeeJWTSecret: os.Getenv("EMPLOYEE_JWT_SECRET"),

		SurepassBaseURL: os.Getenv("SUREPASS_BASE_URL"),
        SurepassBearerToken:   os.Getenv("SUREPASS_BEARER_TOKEN"),

		DemoMode: os.Getenv("DEMO_MODE") == "true",
	}
}

// SetDemoMode is called once from main.go right after Load() so the
// package-level DemoMode flag matches Config.DemoMode.
func SetDemoMode(v bool) {
	DemoMode = v
}