package admin

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/santosh0921/nbfc-backend/internal/audit"
	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/models"
)

// LogoutHandler handles POST /admin/logout — unlike the customer app's
// logout (a client-side-only token drop), this actually revokes the
// AdminSession behind the caller's access token server-side. The access
// token itself keeps parsing as "valid" JWT-wise until it expires (up to
// AccessTokenTTL later), but AdminAuthMiddleware re-checks the session's
// RevokedAt on every request, so it stops being usable immediately.
func LogoutHandler(c *gin.Context) {
	sessionID, _ := c.Get("admin_session_id")
	sid, ok := sessionID.(uint)
	if !ok || sid == 0 {
		c.JSON(http.StatusOK, gin.H{"message": "Logout successful"})
		return
	}

	now := time.Now()
	database.DB.Model(&models.AdminSession{}).Where("id = ? AND revoked_at IS NULL", sid).Update("revoked_at", now)

	adminID, _ := c.Get("admin_id")
	username, _ := c.Get("admin_username")
	role, _ := c.Get("admin_role")
	audit.RecordDetailed(c, audit.Entry{
		ActorType: "admin", ActorID: toUint(adminID), ActorRef: toString(username), ActorRole: toString(role),
		Action: "admin.logout", TargetType: "admin_session", TargetID: itoa(sid),
		Result: audit.ResultSuccess,
	})

	c.JSON(http.StatusOK, gin.H{"message": "Logout successful"})
}

func toUint(v interface{}) uint {
	if u, ok := v.(uint); ok {
		return u
	}
	return 0
}

func toString(v interface{}) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}
