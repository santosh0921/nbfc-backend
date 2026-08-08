package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"

	"github.com/santosh0921/nbfc-backend/internal/auth"
	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/employee"
	"github.com/santosh0921/nbfc-backend/internal/models"
)

// CustomerOrEmployeeAuthMiddleware accepts either a customer JWT or an
// employee JWT (used by the /notifications endpoints, which serve both
// recipient types). It sets "mobile" for a customer token or
// "employee_id"/"employee_role" for an employee token, matching what each
// dedicated middleware sets, so downstream handlers can inspect either.
func CustomerOrEmployeeAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {

		authHeader := c.GetHeader("Authorization")
		parts := strings.Split(authHeader, " ")
		if authHeader == "" || len(parts) != 2 || parts[0] != "Bearer" {
			c.JSON(http.StatusUnauthorized, gin.H{"message": "Authorization header missing or invalid"})
			c.Abort()
			return
		}
		tokenString := parts[1]

		// Try employee token first.
		empClaims := &employee.Claims{}
		if tok, err := jwt.ParseWithClaims(tokenString, empClaims, func(t *jwt.Token) (interface{}, error) {
			return employee.JWTSecret, nil
		}); err == nil && tok.Valid {
			var emp models.Employee
			if err := database.DB.First(&emp, empClaims.EmployeeID).Error; err == nil && emp.Active {
				c.Set("employee_id", emp.ID)
				c.Set("employee_role", emp.Role)
				c.Next()
				return
			}
		}

		// Fall back to customer token.
		custClaims := &auth.Claims{}
		if tok, err := jwt.ParseWithClaims(tokenString, custClaims, func(t *jwt.Token) (interface{}, error) {
			return auth.JWTSecret, nil
		}); err == nil && tok.Valid {
			c.Set("mobile", custClaims.Mobile)
			c.Next()
			return
		}

		c.JSON(http.StatusUnauthorized, gin.H{"message": "Invalid Token"})
		c.Abort()
	}
}
