package auth

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// JWTSecret is nil until InitJWTSecret is called from main(), after
// config.Load() has read JWT_SECRET from the real environment. It used to
// be a hardcoded literal set at package-init time (before main() ever ran),
// which meant JWT_SECRET was read into Config but never actually consumed —
// every customer token was signed with a secret sitting in source control,
// so rotating the deployed env var had no effect at all.
var JWTSecret []byte

// InitJWTSecret must be called exactly once at startup, after config.Load().
// Deliberately fails fast (returns an error the caller should log.Fatal on)
// rather than falling back to a default — a customer-facing signing secret
// with a known fallback is equivalent to no authentication at all.
func InitJWTSecret(secret string) error {
	if secret == "" {
		return errors.New("JWT_SECRET is not set — refusing to start with no customer JWT signing secret")
	}
	JWTSecret = []byte(secret)
	return nil
}

type Claims struct {
	Mobile string `json:"mobile"`
	jwt.RegisteredClaims
}

func GenerateToken(mobile string) (string, error) {

	claims := Claims{
		Mobile: mobile,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	return token.SignedString(JWTSecret)
}