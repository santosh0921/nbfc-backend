package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"

	"github.com/santosh0921/nbfc-backend/internal/admin"
	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/models"
)

// AdminAuthMiddleware validates the admin access-token JWT AND, on every
// request, re-checks the AdminSession it names is still unrevoked/
// unexpired and the AdminUser is still active — a signature-valid JWT
// alone is not enough to prove the session is still live, which is
// exactly what made the old logout endpoint a no-op (nothing ever
// re-checked anything server-side). Disabling an admin, or logging out,
// now takes effect on the very next request instead of only once the
// access token happens to expire on its own.
func AdminAuthMiddleware() gin.HandlerFunc {
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
		// jwt.WithValidMethods pins the accepted signing algorithm so a
		// token cannot switch algorithms out from under the keyfunc (the
		// keyfunc alone returns the same secret regardless of what
		// alg the token header claims, which is the vulnerable pattern).
		token, err := jwt.ParseWithClaims(
			tokenString,
			&admin.Claims{},
			func(token *jwt.Token) (interface{}, error) {
				return admin.JWTSecret, nil
			},
			jwt.WithValidMethods([]string{"HS256"}),
		)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"message": "Invalid Token"})
			c.Abort()
			return
		}

		claims, ok := token.Claims.(*admin.Claims)
		if !ok || !token.Valid {
			c.JSON(http.StatusUnauthorized, gin.H{"message": "Invalid Token"})
			c.Abort()
			return
		}

		var session models.AdminSession
		if err := database.DB.First(&session, claims.SessionID).Error; err != nil || session.RevokedAt != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"message": "Session has been revoked — please log in again"})
			c.Abort()
			return
		}

		var adminUser models.AdminUser
		if err := database.DB.First(&adminUser, claims.AdminID).Error; err != nil || adminUser.Status != models.AdminStatusActive {
			c.JSON(http.StatusForbidden, gin.H{"message": "This admin account has been disabled"})
			c.Abort()
			return
		}

		c.Set("admin_id", adminUser.ID)
		c.Set("admin_username", adminUser.Username)
		c.Set("admin_role", adminUser.Role)
		c.Set("admin_session_id", session.ID)

		c.Next()
	}
}
