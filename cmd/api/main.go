package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"

	"github.com/santosh0921/nbfc-backend/internal/accounting"
	"github.com/santosh0921/nbfc-backend/internal/admin"
	"github.com/santosh0921/nbfc-backend/internal/analytics"
	"github.com/santosh0921/nbfc-backend/internal/auth"
	"github.com/santosh0921/nbfc-backend/internal/bureau"
	"github.com/santosh0921/nbfc-backend/internal/ckyc"
	"github.com/santosh0921/nbfc-backend/internal/compliance"
	"github.com/santosh0921/nbfc-backend/internal/config"
	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/employee"
	"github.com/santosh0921/nbfc-backend/internal/health"
	"github.com/santosh0921/nbfc-backend/internal/jobs"
	"github.com/santosh0921/nbfc-backend/internal/middleware"
	"github.com/santosh0921/nbfc-backend/internal/otp"
	"github.com/santosh0921/nbfc-backend/internal/payments"
	"github.com/santosh0921/nbfc-backend/internal/profile"
	"github.com/santosh0921/nbfc-backend/internal/reporting"
	"github.com/santosh0921/nbfc-backend/internal/risk"
	"github.com/santosh0921/nbfc-backend/internal/routes"
	"github.com/santosh0921/nbfc-backend/internal/screening"
	"github.com/santosh0921/nbfc-backend/internal/seed"
	"github.com/santosh0921/nbfc-backend/internal/storage"
	"github.com/santosh0921/nbfc-backend/internal/tax"
	"github.com/santosh0921/nbfc-backend/internal/users"

	"github.com/santosh0921/nbfc-backend/internal/surepass"
)

func main() {

	cfg := config.Load()
	config.SetDemoMode(cfg.DemoMode)
	config.SetEnvironment(cfg.Environment)
	log.Println("Environment:", cfg.Environment)
	if cfg.DemoMode {
		log.Println("⚠️  DEMO_MODE=true — PAN/Aadhaar verification is bypassed, do not use in production")
	}
	// Fail closed: refuses to start at all if APP_ENV=production and any
	// sandbox-only bypass (DEMO_MODE, missing encryption/provider secrets)
	// is still active. No-op outside production.
	cfg.ValidateProductionSafety()

	// JWT signing secrets must be loaded here — after config.Load() has
	// read the real environment — and never as package-level var
	// initializers, which run before main() and therefore before .env is
	// loaded. A missing secret fails startup immediately rather than
	// silently falling back to a hardcoded default.
	if err := auth.InitJWTSecret(cfg.JWTSecret); err != nil {
		log.Fatal(err)
	}
	if err := admin.InitJWTSecret(cfg.AdminJWTSecret); err != nil {
		log.Fatal(err)
	}
	if err := employee.InitJWTSecret(cfg.EmployeeJWTSecret); err != nil {
		log.Fatal(err)
	}

	database.Connect(cfg)

	surepass.Initialize(cfg)
	payments.Initialize()
	if payments.Active() == nil {
		log.Println("⚠️  No payment gateway configured (RAZORPAY_KEY_ID/RAZORPAY_KEY_SECRET/RAZORPAY_WEBHOOK_SECRET) — EMI online payment is disabled until these are set")
	}
	storage.Initialize()
	if storage.Active() == nil {
		log.Println("⚠️  No document encryption key configured (DOCUMENT_ENCRYPTION_KEY) — KYC document uploads will fall back to legacy plaintext DB storage. Not permitted in production.")
	}
	bureau.Initialize()
	ckyc.Initialize()
	screening.Initialize()

	users.Migrate()
	otp.Migrate()

	if err := profile.Migrate(); err != nil {
		log.Fatal(err)
	}

	admin.Migrate()
	admin.CreatePerformanceIndexes()
	admin.Bootstrap()
	accounting.SeedChartOfAccounts()
	risk.SeedDefaultBuckets()
	reporting.SeedRetentionPolicyPlaceholders()
	tax.SeedNothing()
	compliance.SeedNothing()
	// Seeds descriptions of calculations this codebase performs — not
	// business policy. Thresholds and budgets are deliberately NOT seeded.
	analytics.SeedKPIDictionary()
	// Registers the schedulable job inventory. Registration only — this
	// starts NO timer and NO background loop; scheduling is external.
	jobs.RegisterAll()
	log.Println("✅ KPI dictionary seeded/verified — no management thresholds or budgets are seeded; configure them via the admin Analytics screens before any management alert can fire")

	seed.Run()

	gin.SetMode(cfg.GinMode)

	router := gin.Default()

	// TRUSTED_PROXIES (Phase 7): without this, Gin trusts every proxy by
	// default, which lets a client spoof X-Forwarded-For and defeat
	// per-IP rate limiting / audit-log IP attribution. Left unset ->
	// trust no proxies (direct connections only) — the safe default for
	// a deployment with no reverse proxy in front of it; a real
	// deployment behind a load balancer must set this to that LB's IP(s).
	if trustedProxies := strings.Split(os.Getenv("TRUSTED_PROXIES"), ","); os.Getenv("TRUSTED_PROXIES") != "" {
		router.SetTrustedProxies(trustedProxies)
	} else {
		router.SetTrustedProxies(nil)
	}

	router.Use(middleware.RequestID())
	// Phase 16 items T/U — per-request metrics and one structured log
	// line per request. Placed immediately after RequestID so every
	// logged line carries the correlation id.
	router.Use(middleware.Observability())

	// CORS (Phase 7 item K): AllowAllOrigins is only ever used in
	// development/sandbox, and only when CORS_ALLOWED_ORIGINS is unset —
	// ValidateProductionSafety above already refuses to start in
	// production without that variable set, so this fallback is
	// unreachable in a real production deployment, not a silent bypass.
	corsConfig := cors.Config{
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		AllowCredentials: false,
		MaxAge:           12 * time.Hour,
	}
	if len(cfg.CORSAllowedOrigins) > 0 {
		corsConfig.AllowOrigins = cfg.CORSAllowedOrigins
	} else {
		log.Println("⚠️  CORS_ALLOWED_ORIGINS not set — allowing all origins (development/sandbox only; refused at startup in production)")
		corsConfig.AllowAllOrigins = true
	}
	router.Use(cors.New(corsConfig))
	router.Use(middleware.SecurityHeaders())
	router.Use(middleware.MaxBodySize(15 << 20)) // 15MB — see internal/middleware/max_body_size.go

	routes.RegisterRoutes(router)

	router.GET("/", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"application": cfg.AppName,
			"status":      "Running",
		})
	})
	router.GET("/health", health.HealthHandler)
	router.GET("/readiness", health.ReadinessHandler)
	router.GET("/liveness", health.LivenessHandler)

	log.Println("🚀 Server starting on port", cfg.AppPort)

	srv := &http.Server{
		Addr:    ":" + cfg.AppPort,
		Handler: router,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()

	// Graceful shutdown: without this, router.Run's process exits the
	// instant SIGTERM/SIGINT arrives, killing every in-flight request mid-
	// transaction — including money-moving ones — on every deploy or
	// restart. This waits up to 15s for in-flight requests to finish
	// draining before the process actually exits.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("🛑 Shutdown signal received, draining in-flight requests...")

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Println("⚠️  Graceful shutdown did not complete cleanly:", err)
	} else {
		log.Println("✅ Server shut down cleanly")
	}
}
