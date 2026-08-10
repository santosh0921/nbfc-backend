package admin

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// JWTSecret is kept separate from the customer auth.JWTSecret so admin and
// customer tokens can never be interchanged. It is nil until InitJWTSecret
// is called from main(), after config.Load().
//
// It used to be a package-level var reading os.Getenv at package-init
// time — which in Go runs BEFORE main(), and therefore before
// config.Load()'s godotenv.Load() had populated the environment from a
// local .env file. The result: in that setup ADMIN_JWT_SECRET was silently
// ignored and the hardcoded "admin-super-secret-key" fallback was always
// used instead — a forgeable admin token grants full /admin/* access to
// every customer record, every loan decision, every disbursement.
var JWTSecret []byte

// InitJWTSecret must be called exactly once at startup, after config.Load().
// Fails fast rather than falling back to a default, for the same reason as
// the customer/employee JWT secrets.
func InitJWTSecret(secret string) error {
	if secret == "" {
		return errors.New("ADMIN_JWT_SECRET is not set — refusing to start with no admin JWT signing secret")
	}
	JWTSecret = []byte(secret)
	return nil
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
