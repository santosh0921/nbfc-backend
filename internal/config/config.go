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

	JWTSecret  string

	SurepassBaseURL    string
    SurepassBearerToken string

	DemoMode bool
}

// DemoMode mirrors Config.DemoMode as a package-level flag so handlers that
// don't carry a *Config reference (pan_handler.go, aadhaar_handler.go) can
// check it directly. Set once by Load() at startup.
var DemoMode bool

func Load() *Config {

	err := godotenv.Load()

	if err != nil {
		log.Fatal(".env file not found")
	}

	return &Config{
		AppName: os.Getenv("APP_NAME"),
		AppPort:    os.Getenv("APP_PORT"),
		GinMode: os.Getenv("GIN_MODE"),

		DBHost:     os.Getenv("DB_HOST"),
		DBPort:     os.Getenv("DB_PORT"),
		DBUser:     os.Getenv("DB_USER"),
		DBPassword: os.Getenv("DB_PASSWORD"),
		DBName: os.Getenv("DB_NAME"),

		JWTSecret:  os.Getenv("JWT_SECRET"),
		
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