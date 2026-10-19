package admin

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"

	"github.com/santosh0921/nbfc-backend/internal/audit"
	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/models"
)

type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
	// ExpectedRole is optional, sent by portals that ask the user to pick
	// their role on the login screen (e.g. "COMPLIANCE_OFFICER" on the KYC
	// portal). When present it is enforced, not just displayed: a login
	// succeeds only if it matches adminUser.Role exactly, so an account
	// can never be presented as (or JWT-issued for) a role it doesn't
	// actually hold in RolePermissions. Omitted entirely, login behaves
	// exactly as before — this is additive.
	ExpectedRole string `json:"expectedRole"`
}

const (
	maxFailedLoginAttempts = 5
	lockoutDuration        = 15 * time.Minute
)

// genericInvalidCredentials is returned for every login-can't-succeed
// reason that shouldn't reveal WHICH reason it was (unknown username vs
// wrong password) — an attacker probing usernames gets identical
// responses either way. Lockout is deliberately the one exception: the
// account genuinely needs to know it's locked, same as the customer-side
// MPIN flow already does.
const genericInvalidCredentials = "Invalid username or password"

// LoginHandler handles POST /admin/login against the real admin_users
// table — replacing the single shared ADMIN_USERNAME/ADMIN_PASSWORD .env
// credential every admin used to share. Every admin now has their own
// bcrypt-hashed password, their own failed-attempt lockout, and their own
// audit-attributable identity.
func LoginHandler(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid request"})
		return
	}

	var adminUser models.AdminUser
	err := database.DB.Where("LOWER(username) = LOWER(?)", req.Username).First(&adminUser).Error
	if err != nil {
		// No account with this username — still run a dummy bcrypt
		// comparison so the response time doesn't leak whether the
		// username exists (a cheap, standard timing-attack mitigation).
		_ = bcrypt.CompareHashAndPassword([]byte("$2a$10$invalidinvalidinvalidinvalidinvalidinvalidinvalidinva"), []byte(req.Password))
		audit.RecordDetailed(c, audit.Entry{
			ActorType: "admin", ActorRef: req.Username, Action: "admin.login.failed",
			TargetType: "admin_user", TargetID: req.Username,
			Result: audit.ResultFailure, Reason: "unknown username",
		})
		c.JSON(http.StatusUnauthorized, gin.H{"message": genericInvalidCredentials})
		return
	}

	if !adminUser.Approved {
		audit.RecordDetailed(c, audit.Entry{
			ActorType: "admin", ActorID: adminUser.ID, ActorRef: adminUser.Username, ActorRole: adminUser.Role,
			Action: "admin.login.failed", TargetType: "admin_user", TargetID: itoa(adminUser.ID),
			Result: audit.ResultDenied, Reason: "pending super admin approval",
		})
		c.JSON(http.StatusForbidden, gin.H{"message": "Your account is pending approval by a Super Admin"})
		return
	}

	if adminUser.Status != models.AdminStatusActive {
		audit.RecordDetailed(c, audit.Entry{
			ActorType: "admin", ActorID: adminUser.ID, ActorRef: adminUser.Username, ActorRole: adminUser.Role,
			Action: "admin.login.failed", TargetType: "admin_user", TargetID: itoa(adminUser.ID),
			Result: audit.ResultDenied, Reason: "account disabled",
		})
		c.JSON(http.StatusForbidden, gin.H{"message": "This admin account has been disabled"})
		return
	}

	if adminUser.LockedUntil != nil {
		if adminUser.LockedUntil.After(time.Now()) {
			audit.RecordDetailed(c, audit.Entry{
				ActorType: "admin", ActorID: adminUser.ID, ActorRef: adminUser.Username, ActorRole: adminUser.Role,
				Action: "admin.login.failed", TargetType: "admin_user", TargetID: itoa(adminUser.ID),
				Result: audit.ResultDenied, Reason: "account locked",
			})
			c.JSON(http.StatusForbidden, gin.H{
				"message":     "Too many failed attempts. Try again after " + adminUser.LockedUntil.Format("15:04:05 MST"),
				"lockedUntil": adminUser.LockedUntil,
			})
			return
		}
		// Lock window has passed — clear it before evaluating the password.
		adminUser.LockedUntil = nil
		adminUser.FailedLoginAttempts = 0
	}

	if bcrypt.CompareHashAndPassword([]byte(adminUser.PasswordHash), []byte(req.Password)) != nil {
		adminUser.FailedLoginAttempts++
		locked := adminUser.FailedLoginAttempts >= maxFailedLoginAttempts
		if locked {
			lockUntil := time.Now().Add(lockoutDuration)
			adminUser.LockedUntil = &lockUntil
		}
		database.DB.Save(&adminUser)
		audit.RecordDetailed(c, audit.Entry{
			ActorType: "admin", ActorID: adminUser.ID, ActorRef: adminUser.Username, ActorRole: adminUser.Role,
			Action: "admin.login.failed", TargetType: "admin_user", TargetID: itoa(adminUser.ID),
			Result: audit.ResultFailure, Reason: "wrong password",
			Details: "failed_login_attempts=" + itoa(uint(adminUser.FailedLoginAttempts)),
		})
		resp := gin.H{"message": genericInvalidCredentials}
		// attemptsRemaining is UX-only (helps a genuine employee notice
		// they're about to get locked out before it happens) — omitted
		// once locked, since the lockout message itself takes over.
		if !locked {
			resp["attemptsRemaining"] = maxFailedLoginAttempts - adminUser.FailedLoginAttempts
		}
		c.JSON(http.StatusUnauthorized, resp)
		return
	}

	if req.ExpectedRole != "" && req.ExpectedRole != adminUser.Role {
		audit.RecordDetailed(c, audit.Entry{
			ActorType: "admin", ActorID: adminUser.ID, ActorRef: adminUser.Username, ActorRole: adminUser.Role,
			Action: "admin.login.failed", TargetType: "admin_user", TargetID: itoa(adminUser.ID),
			Result: audit.ResultDenied, Reason: "role mismatch",
			Details: "selected=" + req.ExpectedRole + " actual=" + adminUser.Role,
		})
		c.JSON(http.StatusForbidden, gin.H{"message": "This account is not registered under the selected role"})
		return
	}

	// Success — reset lockout state, record login time.
	adminUser.FailedLoginAttempts = 0
	adminUser.LockedUntil = nil
	now := time.Now()
	adminUser.LastLoginAt = &now
	database.DB.Save(&adminUser)

	session, rawRefreshToken, err := createSession(c, adminUser.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Failed to create session"})
		return
	}

	accessToken, err := GenerateAccessToken(adminUser.ID, adminUser.Username, adminUser.Role, session.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Failed to generate token"})
		return
	}

	audit.RecordDetailed(c, audit.Entry{
		ActorType: "admin", ActorID: adminUser.ID, ActorRef: adminUser.Username, ActorRole: adminUser.Role,
		Action: "admin.login.success", TargetType: "admin_user", TargetID: itoa(adminUser.ID),
		Result: audit.ResultSuccess,
	})

	c.JSON(http.StatusOK, gin.H{
		"message":      "Login successful",
		"token":        accessToken,
		"refreshToken": rawRefreshToken,
		"admin": gin.H{
			"id":       adminUser.ID,
			"username": adminUser.Username,
			"email":    adminUser.Email,
			"role":     adminUser.Role,
		},
	})
}

// createSession issues a new AdminSession row + refresh token, capturing
// the requesting IP/user-agent for later forensic review.
func createSession(c *gin.Context, adminID uint) (models.AdminSession, string, error) {
	rawToken, tokenHash, err := GenerateRefreshToken()
	if err != nil {
		return models.AdminSession{}, "", err
	}
	session := models.AdminSession{
		AdminUserID:      adminID,
		RefreshTokenHash: tokenHash,
		UserAgent:        c.Request.UserAgent(),
		IPAddress:        c.ClientIP(),
		ExpiresAt:        time.Now().Add(RefreshTokenTTL),
	}
	if err := database.DB.Create(&session).Error; err != nil {
		return models.AdminSession{}, "", err
	}
	return session, rawToken, nil
}
