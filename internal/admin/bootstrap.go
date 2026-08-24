package admin

import (
	"crypto/rand"
	"encoding/base64"
	"log"
	"os"
	"strings"

	"golang.org/x/crypto/bcrypt"

	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/models"
)

// Bootstrap creates the first SUPER_ADMIN account if the admin_users
// table is empty — without which nobody could ever log in to create the
// second one. Must run once, after Migrate(), on every startup (it's a
// no-op once any admin_users row exists).
//
// Credential source, in priority order:
//  1. ADMIN_BOOTSTRAP_USERNAME / ADMIN_BOOTSTRAP_EMAIL / ADMIN_BOOTSTRAP_PASSWORD
//     env vars, if all three are set — lets an operator pin the first
//     account's credentials explicitly (e.g. from a secrets manager).
//  2. The legacy ADMIN_USERNAME / ADMIN_PASSWORD env vars, if set — a
//     one-time migration path so an existing deployment's already-
//     configured credential becomes the first real admin account instead
//     of being silently discarded. Read here and only here; nothing else
//     in this codebase consults these two vars anymore.
//  3. A freshly generated random username/password, printed ONCE to the
//     server's own stdout log at startup and never written to disk in
//     plaintext anywhere — the operator must copy it down immediately or
//     reset it via the database.
//
// In every case only the bcrypt hash is ever persisted.
func Bootstrap() {
	var count int64
	database.DB.Model(&models.AdminUser{}).Count(&count)
	if count > 0 {
		return
	}

	username := strings.TrimSpace(os.Getenv("ADMIN_BOOTSTRAP_USERNAME"))
	email := strings.TrimSpace(os.Getenv("ADMIN_BOOTSTRAP_EMAIL"))
	password := os.Getenv("ADMIN_BOOTSTRAP_PASSWORD")
	source := "ADMIN_BOOTSTRAP_* env vars"

	if username == "" || email == "" || password == "" {
		legacyUsername := strings.TrimSpace(os.Getenv("ADMIN_USERNAME"))
		legacyPassword := os.Getenv("ADMIN_PASSWORD")
		if legacyUsername != "" && legacyPassword != "" {
			username = legacyUsername
			password = legacyPassword
			if email == "" {
				email = legacyUsername + "@local.bootstrap"
			}
			source = "legacy ADMIN_USERNAME/ADMIN_PASSWORD env vars (one-time migration)"
		}
	}

	generatedPassword := ""
	if username == "" || email == "" || password == "" {
		username = "superadmin"
		email = "superadmin@local.bootstrap"
		generatedPassword = generateSecurePassword()
		password = generatedPassword
		source = "auto-generated (see below)"
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		log.Fatal("admin.Bootstrap: failed to hash bootstrap password:", err)
	}

	adminUser := models.AdminUser{
		Username:     username,
		Email:        email,
		PasswordHash: string(hash),
		Role:         models.AdminRoleSuperAdmin,
		Status:       models.AdminStatusActive,
		CreatedBy:    "bootstrap",
	}
	if err := database.DB.Create(&adminUser).Error; err != nil {
		log.Fatal("admin.Bootstrap: failed to create first SUPER_ADMIN:", err)
	}

	log.Println("✅ Bootstrapped first SUPER_ADMIN account — username:", username, "(credential source:", source+")")
	if generatedPassword != "" {
		log.Println("⚠️  GENERATED PASSWORD (shown once, not stored anywhere in plaintext):", generatedPassword)
		log.Println("⚠️  Log in now and note this down — it will not be shown again. Change it via the Admin Users screen if desired.")
	} else {
		log.Println("⚠️  Bootstrap admin created from configured credentials — remove ADMIN_BOOTSTRAP_PASSWORD / legacy ADMIN_PASSWORD from the environment now that the account exists.")
	}
}

func generateSecurePassword() string {
	buf := make([]byte, 18)
	if _, err := rand.Read(buf); err != nil {
		log.Fatal("admin.Bootstrap: failed to generate random password:", err)
	}
	return base64.RawURLEncoding.EncodeToString(buf)
}
