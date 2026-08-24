package admin

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"

	"github.com/santosh0921/nbfc-backend/internal/audit"
	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/models"
	"github.com/santosh0921/nbfc-backend/internal/rbac"
)

// AdminListAdminUsersHandler handles GET /admin/admin-users — requires
// ADMIN_MANAGE. Passwords never leave the DB (AdminUser.PasswordHash has
// json:"-").
func AdminListAdminUsersHandler(c *gin.Context) {
	var users []models.AdminUser
	database.DB.Order("created_at asc").Find(&users)
	c.JSON(http.StatusOK, users)
}

type createAdminUserRequest struct {
	Username string `json:"username" binding:"required"`
	Email    string `json:"email" binding:"required"`
	Password string `json:"password" binding:"required"`
	Role     string `json:"role" binding:"required"`
}

const minAdminPasswordLength = 12

// AdminCreateAdminUserHandler handles POST /admin/admin-users — requires
// ADMIN_MANAGE + ROLE_MANAGE (creating an account inherently assigns it a
// role). This is the only way to create an admin account now; there is no
// open self-registration path for admins (unlike the employee app).
func AdminCreateAdminUserHandler(c *gin.Context) {
	var req createAdminUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	req.Username = strings.TrimSpace(req.Username)
	req.Email = strings.TrimSpace(req.Email)
	if len(req.Password) < minAdminPasswordLength {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Password must be at least 12 characters"})
		return
	}
	if !rbac.IsValidRole(req.Role) {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid role"})
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Failed to hash password"})
		return
	}

	actingUsername := c.GetString("admin_username")
	adminUser := models.AdminUser{
		Username:     req.Username,
		Email:        req.Email,
		PasswordHash: string(hash),
		Role:         req.Role,
		Status:       models.AdminStatusActive,
		CreatedBy:    actingUsername,
	}
	if err := database.DB.Create(&adminUser).Error; err != nil {
		if isUniqueViolation(err) {
			c.JSON(http.StatusConflict, gin.H{"message": "Username or email already in use"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Failed to create admin user"})
		return
	}

	audit.RecordDetailed(c, audit.Entry{
		ActorType: "admin", ActorID: toAdminIDCtx(c), ActorRef: actingUsername, ActorRole: c.GetString("admin_role"),
		Action: "admin_user.create", TargetType: "admin_user", TargetID: itoa(adminUser.ID),
		Result: audit.ResultSuccess, After: adminUser,
	})

	c.JSON(http.StatusCreated, adminUser)
}

type updateAdminUserRoleRequest struct {
	Role string `json:"role" binding:"required"`
}

// AdminUpdateAdminUserRoleHandler handles PUT /admin/admin-users/:id/role
// — requires ROLE_MANAGE. A super-admin cannot demote themselves out of
// SUPER_ADMIN if they are the only remaining one, to avoid a full
// lockout of the admin-management function.
func AdminUpdateAdminUserRoleHandler(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var req updateAdminUserRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	if !rbac.IsValidRole(req.Role) {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid role"})
		return
	}

	var target models.AdminUser
	if err := database.DB.First(&target, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"message": "Not found"})
		return
	}

	if target.Role == models.AdminRoleSuperAdmin && req.Role != models.AdminRoleSuperAdmin {
		var superAdminCount int64
		database.DB.Model(&models.AdminUser{}).Where("role = ? AND status = ?", models.AdminRoleSuperAdmin, models.AdminStatusActive).Count(&superAdminCount)
		if superAdminCount <= 1 {
			c.JSON(http.StatusConflict, gin.H{"message": "Cannot change role: this is the only active SUPER_ADMIN account"})
			return
		}
	}

	before := audit.Snapshot(target)
	oldRole := target.Role
	target.Role = req.Role
	database.DB.Save(&target)

	audit.RecordDetailed(c, audit.Entry{
		ActorType: "admin", ActorID: toAdminIDCtx(c), ActorRef: c.GetString("admin_username"), ActorRole: c.GetString("admin_role"),
		Action: "admin_user.role_change", TargetType: "admin_user", TargetID: itoa(target.ID),
		Result: audit.ResultSuccess, Reason: "role changed from " + oldRole + " to " + req.Role,
		Details: before, After: target,
	})

	c.JSON(http.StatusOK, target)
}

// AdminSetAdminUserStatusHandler handles PUT /admin/admin-users/:id/status
// — requires ADMIN_MANAGE. Disabling an account takes effect immediately:
// AdminAuthMiddleware re-checks Status on every request, and every
// existing session for this admin is revoked here too so a live access
// token can't keep working until it naturally expires.
func AdminSetAdminUserStatusHandler(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var body struct {
		Status string `json:"status" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	if body.Status != models.AdminStatusActive && body.Status != models.AdminStatusDisabled {
		c.JSON(http.StatusBadRequest, gin.H{"message": "status must be 'active' or 'disabled'"})
		return
	}

	var target models.AdminUser
	if err := database.DB.First(&target, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"message": "Not found"})
		return
	}

	if target.ID == toAdminIDCtx(c) && body.Status == models.AdminStatusDisabled {
		c.JSON(http.StatusBadRequest, gin.H{"message": "You cannot disable your own account"})
		return
	}
	if target.Role == models.AdminRoleSuperAdmin && body.Status == models.AdminStatusDisabled {
		var activeSuperAdmins int64
		database.DB.Model(&models.AdminUser{}).Where("role = ? AND status = ?", models.AdminRoleSuperAdmin, models.AdminStatusActive).Count(&activeSuperAdmins)
		if activeSuperAdmins <= 1 {
			c.JSON(http.StatusConflict, gin.H{"message": "Cannot disable: this is the only active SUPER_ADMIN account"})
			return
		}
	}

	before := audit.Snapshot(target)
	target.Status = body.Status
	database.DB.Save(&target)

	if body.Status == models.AdminStatusDisabled {
		database.DB.Model(&models.AdminSession{}).
			Where("admin_user_id = ? AND revoked_at IS NULL", target.ID).
			Update("revoked_at", time.Now())
	}

	audit.RecordDetailed(c, audit.Entry{
		ActorType: "admin", ActorID: toAdminIDCtx(c), ActorRef: c.GetString("admin_username"), ActorRole: c.GetString("admin_role"),
		Action: "admin_user.status_change", TargetType: "admin_user", TargetID: itoa(target.ID),
		Result: audit.ResultSuccess, Reason: "status set to " + body.Status,
		Details: before, After: target,
	})

	c.JSON(http.StatusOK, target)
}

// AdminListRolesHandler handles GET /admin/roles — the "Roles &
// Permissions" screen's data source: every fixed role and the granular
// permissions it carries, so admins can see (not edit — permissions are
// fixed per role in this phase) what each role can do.
func AdminListRolesHandler(c *gin.Context) {
	out := make([]gin.H, 0, len(rbac.AllRoles))
	for _, role := range rbac.AllRoles {
		out = append(out, gin.H{
			"role":        role,
			"permissions": rbac.PermissionsFor(role),
		})
	}
	c.JSON(http.StatusOK, out)
}

func toAdminIDCtx(c *gin.Context) uint {
	if v, ok := c.Get("admin_id"); ok {
		if id, ok := v.(uint); ok {
			return id
		}
	}
	return 0
}
