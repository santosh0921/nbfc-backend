package main

import (
	"log"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"

	"github.com/santosh0921/nbfc-backend/internal/admin"
	"github.com/santosh0921/nbfc-backend/internal/config"
	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/users"
    "github.com/santosh0921/nbfc-backend/internal/routes"
	"github.com/santosh0921/nbfc-backend/internal/otp"
	"github.com/santosh0921/nbfc-backend/internal/profile"
	"github.com/santosh0921/nbfc-backend/internal/seed"

	"github.com/santosh0921/nbfc-backend/internal/surepass"



)

func main() {

	cfg := config.Load()
	config.SetDemoMode(cfg.DemoMode)
	if cfg.DemoMode {
		log.Println("⚠️  DEMO_MODE=true — PAN/Aadhaar verification is bypassed, do not use in production")
	}

	database.Connect(cfg)

	surepass.Initialize(cfg)

	users.Migrate()
	otp.Migrate()
	
	if err := profile.Migrate(); err != nil {
	    log.Fatal(err)
    }

	admin.Migrate()

	seed.Run()

	gin.SetMode(cfg.GinMode)

	router := gin.Default()

	router.Use(cors.New(cors.Config{
		AllowAllOrigins:  true,
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		AllowCredentials: false,
		MaxAge:           12 * time.Hour,
	}))

	routes.RegisterRoutes(router)

	router.GET("/", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"application": cfg.AppName,
			"status":      "Running",
		})
	})

	log.Println("🚀 Server starting on port", cfg.AppPort)

	if err := router.Run(":" + cfg.AppPort); err != nil {
	log.Fatal(err)
}
}