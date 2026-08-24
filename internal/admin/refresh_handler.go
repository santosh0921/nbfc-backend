package admin

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/santosh0921/nbfc-backend/internal/audit"
	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/models"
)

type refreshRequest struct {
	RefreshToken string `json:"refreshToken" binding:"required"`
}

// RefreshHandler handles POST /admin/refresh — exchanges a valid,
// unrevoked refresh token for a new short-lived access token, rotating
// the refresh token itself in the process (the old one is revoked the
// moment a new one is issued, so a leaked-and-then-legitimately-used
// refresh token can't be replayed by an attacker after the real client
// has rotated past it).
func RefreshHandler(c *gin.Context) {
	var req refreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid request"})
		return
	}

	tokenHash := HashRefreshToken(req.RefreshToken)
	var session models.AdminSession
	if err := database.DB.Where("refresh_token_hash = ?", tokenHash).First(&session).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"message": "Invalid or expired session"})
		return
	}
	if session.RevokedAt != nil || session.ExpiresAt.Before(time.Now()) {
		c.JSON(http.StatusUnauthorized, gin.H{"message": "Invalid or expired session"})
		return
	}

	var adminUser models.AdminUser
	if err := database.DB.First(&adminUser, session.AdminUserID).Error; err != nil || adminUser.Status != models.AdminStatusActive {
		c.JSON(http.StatusForbidden, gin.H{"message": "Account no longer active"})
		return
	}

	now := time.Now()
	database.DB.Model(&session).Update("revoked_at", now)

	newSession, rawRefreshToken, err := createSession(c, adminUser.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Failed to create session"})
		return
	}

	accessToken, err := GenerateAccessToken(adminUser.ID, adminUser.Username, adminUser.Role, newSession.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Failed to generate token"})
		return
	}

	audit.RecordDetailed(c, audit.Entry{
		ActorType: "admin", ActorID: adminUser.ID, ActorRef: adminUser.Username, ActorRole: adminUser.Role,
		Action: "admin.session.refresh", TargetType: "admin_session", TargetID: itoa(newSession.ID),
		Result: audit.ResultSuccess,
	})

	c.JSON(http.StatusOK, gin.H{
		"token":        accessToken,
		"refreshToken": rawRefreshToken,
	})
}
