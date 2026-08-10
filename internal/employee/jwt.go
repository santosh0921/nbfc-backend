package employee

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// JWTSecret is kept separate from customer/admin secrets so tokens can never
// be interchanged across roles. It is nil until InitJWTSecret is called
// from main(), after config.Load().
//
// It used to be a package-level var reading os.Getenv at package-init
// time — before main() runs, and therefore before config.Load()'s
// godotenv.Load() had populated the environment from a local .env file —
// so EMPLOYEE_JWT_SECRET was always ignored in that setup. Worse, no .env
// in this project ever defined EMPLOYEE_JWT_SECRET at all, so the
// hardcoded "employee-super-secret-key" fallback was in effect always in
// use, in every environment, including production.
var JWTSecret []byte

// InitJWTSecret must be called exactly once at startup, after config.Load().
// Fails fast rather than falling back to a default, for the same reason as
// the customer/admin JWT secrets.
func InitJWTSecret(secret string) error {
	if secret == "" {
		return errors.New("EMPLOYEE_JWT_SECRET is not set — refusing to start with no employee JWT signing secret")
	}
	JWTSecret = []byte(secret)
	return nil
}

type Claims struct {
	EmployeeID uint   `json:"employeeId"`
	Code       string `json:"code"`
	Role       string `json:"role"`
	jwt.RegisteredClaims
}

func GenerateToken(id uint, code, role string) (string, error) {
	claims := Claims{
		EmployeeID: id,
		Code:       code,
		Role:       role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(12 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(JWTSecret)
}
