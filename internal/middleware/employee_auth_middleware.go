package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"

	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/employee"
	"github.com/santosh0921/nbfc-backend/internal/models"
)

// EmployeeAuthMiddleware validates the employee JWT and re-checks the
// employee's (and their agency's) active flag on every request so a
// deactivation takes effect immediately (403 on next request).
func EmployeeAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {

		authHeader := c.GetHeader("Authorization")

		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"message": "Authorization header missing"})
			c.Abort()
			return
		}

		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.JSON(http.StatusUnauthorized, gin.H{"message": "Invalid Authorization Header"})
			c.Abort()
			return
		}

		tokenString := parts[1]

		token, err := jwt.ParseWithClaims(
			tokenString,
			&employee.Claims{},
			func(token *jwt.Token) (interface{}, error) {
				return employee.JWTSecret, nil
			},
		)

		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"message": "Invalid Token"})
			c.Abort()
			return
		}

		claims, ok := token.Claims.(*employee.Claims)
		if !ok || !token.Valid {
			c.JSON(http.StatusUnauthorized, gin.H{"message": "Invalid Token"})
			c.Abort()
			return
		}

		var emp models.Employee
		if err := database.DB.First(&emp, claims.EmployeeID).Error; err != nil || !emp.Active {
			c.JSON(http.StatusForbidden, gin.H{"message": "Account deactivated"})
			c.Abort()
			return
		}
		if emp.AgencyID != 0 {
			var ag models.Agency
			if err := database.DB.First(&ag, emp.AgencyID).Error; err != nil || !ag.Active {
				c.JSON(http.StatusForbidden, gin.H{"message": "Agency deactivated"})
				c.Abort()
				return
			}
		}

		c.Set("employee_id", emp.ID)
		c.Set("employee_role", emp.Role)
		c.Next()
	}
}
