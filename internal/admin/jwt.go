package admin

import (
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// JWTSecret is kept separate from the customer auth.JWTSecret so admin and
// customer tokens can never be interchanged. Falls back to a dev default if
// ADMIN_JWT_SECRET isn't set, mirroring the existing customer auth package.
var JWTSecret = []byte(envOrDefault("ADMIN_JWT_SECRET", "admin-super-secret-key"))

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

type Claims struct {
	Username string `json:"username"`
	jwt.RegisteredClaims
}

func GenerateToken(username string) (string, error) {

	claims := Claims{
		Username: username,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(12 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	return token.SignedString(JWTSecret)
}
