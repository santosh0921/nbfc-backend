package config

import (
	"log"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

// Environment values (Phase 3 production-safety matrix). DEVELOPMENT and
// SANDBOX may run DEMO_MODE and sandbox-only providers (the deterministic
// bureau score, etc.); PRODUCTION must never be able to, enforced by
// ValidateProductionSafety below, called once from cmd/api/main.go right
// after Load().
const (
	EnvDevelopment = "development"
	EnvSandbox     = "sandbox"
	EnvProduction  = "production"
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

	// Environment is APP_ENV, normalized to one of Env* above. Defaults to
	// EnvDevelopment when unset — deliberately the LEAST trusting default:
	// an unset APP_ENV never accidentally grants production-only behavior,
	// it just means sandbox/demo paths stay available, same as today.
	Environment string

	// CORSAllowedOrigins is CORS_ALLOWED_ORIGINS, comma-separated — see
	// cmd/api/main.go's CORS setup (Phase 7). Empty in development permits
	// AllowAllOrigins for local tooling; ValidateProductionSafety below
	// refuses to start in production with this unset (item K: never "*"
	// for an authenticated production API).
	CORSAllowedOrigins []string
}

// DemoMode mirrors Config.DemoMode as a package-level flag so handlers that
// don't carry a *Config reference (pan_handler.go, aadhaar_handler.go) can
// check it directly. Set once by Load() at startup.
var DemoMode bool

// Environment mirrors Config.Environment as a package-level flag for the
// same reason — handlers/services throughout internal/kyc, internal/bureau,
// internal/ckyc, internal/screening gate sandbox-only behavior on this
// without needing a *Config reference threaded through every call.
var Environment string

// IsProduction reports whether the running deployment is APP_ENV=production
// — the single check every sandbox-only code path (fake/demo verification,
// the deterministic bureau score, etc.) must gate on before running.
func IsProduction() bool {
	return Environment == EnvProduction
}

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

		DemoMode:    os.Getenv("DEMO_MODE") == "true",
		Environment: normalizeEnvironment(os.Getenv("APP_ENV")),

		CORSAllowedOrigins: parseCommaList(os.Getenv("CORS_ALLOWED_ORIGINS")),
	}
}

// parseCommaList splits a comma-separated env value into a trimmed,
// empty-entry-free slice — "" (unset) becomes an empty (not nil-vs-empty-
// ambiguous) slice.
func parseCommaList(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if v := strings.TrimSpace(p); v != "" {
			out = append(out, v)
		}
	}
	return out
}

func normalizeEnvironment(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case EnvProduction, "prod":
		return EnvProduction
	case EnvSandbox:
		return EnvSandbox
	default:
		return EnvDevelopment
	}
}

// SetDemoMode is called once from main.go right after Load() so the
// package-level DemoMode flag matches Config.DemoMode.
func SetDemoMode(v bool) {
	DemoMode = v
}

// SetEnvironment is called once from main.go right after Load().
func SetEnvironment(v string) {
	Environment = v
}

// ValidateProductionSafety is the fail-closed gate required by Phase 3:
// called once at startup, after Load()/SetDemoMode()/SetEnvironment(). If
// APP_ENV=production, refuses to start at all when any sandbox-only
// bypass is still active — DEMO_MODE, or the required production secrets
// for encryption/verification are missing. This is deliberately a
// log.Fatal (process exit), not a warning: a production deployment that
// silently started with DEMO_MODE on would auto-verify every PAN with no
// real check, which is the exact defect this phase exists to close.
func (c *Config) ValidateProductionSafety() {
	if c.Environment != EnvProduction {
		return
	}
	var problems []string
	if c.DemoMode {
		problems = append(problems, "DEMO_MODE=true is set — DEMO_MODE must never be enabled in production (it bypasses real PAN/Aadhaar verification)")
	}
	if os.Getenv("PAN_ENCRYPTION_KEY") == "" {
		problems = append(problems, "PAN_ENCRYPTION_KEY is not set — required in production for PAN-at-rest encryption")
	}
	if os.Getenv("DOCUMENT_ENCRYPTION_KEY") == "" {
		problems = append(problems, "DOCUMENT_ENCRYPTION_KEY is not set — required in production for KYC document-at-rest encryption")
	}
	if c.SurepassBearerToken == "" {
		problems = append(problems, "SUREPASS_BEARER_TOKEN is not set — PAN verification cannot run without a real provider credential in production")
	}
	if len(c.CORSAllowedOrigins) == 0 {
		problems = append(problems, "CORS_ALLOWED_ORIGINS is not set — production must never fall back to allowing all origins for an authenticated API (Phase 7 item K)")
	}
	if len(problems) > 0 {
		log.Println("🛑 Refusing to start in production with unsafe configuration:")
		for _, p := range problems {
			log.Println("   - " + p)
		}
		log.Fatal("production safety check failed — see problems above")
	}
}