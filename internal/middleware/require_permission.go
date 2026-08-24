package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/santosh0921/nbfc-backend/internal/audit"
	"github.com/santosh0921/nbfc-backend/internal/rbac"
)

// RequirePermission enforces RBAC in the backend — the actual security
// boundary, not the admin panel choosing which buttons to render. Must be
// chained after AdminAuthMiddleware (reads admin_role from context, which
// only that middleware sets). Fails closed: any missing/unrecognized role
// is denied, never allowed.
func RequirePermission(perm rbac.Permission) gin.HandlerFunc {
	return func(c *gin.Context) {
		role, _ := c.Get("admin_role")
		roleStr, _ := role.(string)

		if !rbac.Has(roleStr, perm) {
			adminID, _ := c.Get("admin_id")
			username, _ := c.Get("admin_username")
			audit.RecordDetailed(c, audit.Entry{
				ActorType: "admin", ActorID: toUint(adminID), ActorRef: toStr(username), ActorRole: roleStr,
				Action: "authz.denied", TargetType: "route", TargetID: c.Request.Method + " " + c.FullPath(),
				Result: audit.ResultDenied, Reason: "missing permission " + string(perm),
			})
			c.JSON(http.StatusForbidden, gin.H{"message": "You do not have permission to perform this action"})
			c.Abort()
			return
		}
		c.Next()
	}
}

func toUint(v interface{}) uint {
	if u, ok := v.(uint); ok {
		return u
	}
	return 0
}

func toStr(v interface{}) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}
