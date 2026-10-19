package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"

	"github.com/santosh0921/nbfc-backend/internal/admin"
	"github.com/santosh0921/nbfc-backend/internal/auth"
	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/employee"
	"github.com/santosh0921/nbfc-backend/internal/models"
)

// CallSignalingAuthMiddleware guards GET /ws/call/:loanId ONLY — it is
// deliberately its own middleware rather than an extension of
// CustomerOrEmployeeAuthMiddleware, for two reasons specific to this one
// WebSocket route:
//
//  1. It accepts an admin JWT (a KYC-portal officer starting a video
//     verification call), in addition to employee/customer, validated the
//     exact same way AdminAuthMiddleware does (session not revoked, admin
//     user active).
//  2. It accepts the bearer token via a `?token=` query parameter as a
//     fallback when no Authorization header is present — native browser
//     WebSocket clients (`new WebSocket(url)`) cannot set custom headers
//     on the handshake request, unlike this app's own Flutter client. A
//     token in a URL query string is a real exposure (server/CDN access
//     logs, browser history, Referer leakage), so this fallback is scoped
//     to exactly this one low-sensitivity, short-lived-token, signaling-
//     only route and must never be folded into a widely-shared middleware
//     that also guards data-returning routes like /notifications or
//     /device-token.
func CallSignalingAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenString := ""
		authHeader := c.GetHeader("Authorization")
		parts := strings.Split(authHeader, " ")
		if authHeader != "" && len(parts) == 2 && parts[0] == "Bearer" {
			tokenString = parts[1]
		} else {
			tokenString = c.Query("token")
		}
		if tokenString == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"message": "Authorization header missing or invalid"})
			c.Abort()
			return
		}

		// Try employee token first.
		empClaims := &employee.Claims{}
		if tok, err := jwt.ParseWithClaims(tokenString, empClaims, func(t *jwt.Token) (interface{}, error) {
			return employee.JWTSecret, nil
		}, jwt.WithValidMethods([]string{"HS256"})); err == nil && tok.Valid {
			var emp models.Employee
			if err := database.DB.First(&emp, empClaims.EmployeeID).Error; err == nil && emp.Active {
				c.Set("employee_id", emp.ID)
				c.Set("employee_role", emp.Role)
				c.Next()
				return
			}
		}

		// Fall back to admin token — same validation AdminAuthMiddleware
		// performs: signature-valid JWT, session not revoked, admin user
		// still active.
		adminClaims := &admin.Claims{}
		if tok, err := jwt.ParseWithClaims(tokenString, adminClaims, func(t *jwt.Token) (interface{}, error) {
			return admin.JWTSecret, nil
		}, jwt.WithValidMethods([]string{"HS256"})); err == nil && tok.Valid {
			var session models.AdminSession
			if err := database.DB.First(&session, adminClaims.SessionID).Error; err == nil && session.RevokedAt == nil {
				var adminUser models.AdminUser
				if err := database.DB.First(&adminUser, adminClaims.AdminID).Error; err == nil && adminUser.Status == models.AdminStatusActive {
					c.Set("admin_id", adminUser.ID)
					c.Set("admin_username", adminUser.Username)
					c.Set("admin_role", adminUser.Role)
					c.Set("admin_session_id", session.ID)
					c.Next()
					return
				}
			}
		}

		// Fall back to customer token.
		custClaims := &auth.Claims{}
		if tok, err := jwt.ParseWithClaims(tokenString, custClaims, func(t *jwt.Token) (interface{}, error) {
			return auth.JWTSecret, nil
		}, jwt.WithValidMethods([]string{"HS256"})); err == nil && tok.Valid {
			c.Set("mobile", custClaims.Mobile)
			c.Next()
			return
		}

		c.JSON(http.StatusUnauthorized, gin.H{"message": "Invalid Token"})
		c.Abort()
	}
}
