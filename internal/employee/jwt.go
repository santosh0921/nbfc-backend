package employee

import (
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// JWTSecret is kept separate from customer/admin secrets so tokens can never
// be interchanged across roles.
var JWTSecret = []byte(envOrDefault("EMPLOYEE_JWT_SECRET", "employee-super-secret-key"))

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
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
