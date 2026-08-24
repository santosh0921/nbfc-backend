package audit

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/models"
)

// AdminListHandler handles GET /admin/audit-log — requires AUDIT_VIEW.
// Supports filtering by action, targetType, actor (actorRef, exact
// match), and a createdAt date range (RFC3339 "from"/"to" query params) —
// previously the admin panel could only filter by action/targetType, and
// only client-side against whatever page it had already fetched.
func AdminListHandler(c *gin.Context) {
	action := c.Query("action")
	targetType := c.Query("targetType")
	actor := c.Query("actor")
	from := c.Query("from")
	to := c.Query("to")

	limit := 100
	if v, err := strconv.Atoi(c.Query("limit")); err == nil && v > 0 && v <= 500 {
		limit = v
	}
	offset := 0
	if v, err := strconv.Atoi(c.Query("offset")); err == nil && v > 0 {
		offset = v
	}

	q := database.DB.Model(&models.AuditLog{})
	if action != "" {
		q = q.Where("action = ?", action)
	}
	if targetType != "" {
		q = q.Where("target_type = ?", targetType)
	}
	if actor != "" {
		q = q.Where("actor_ref = ?", actor)
	}
	if from != "" {
		if t, err := time.Parse(time.RFC3339, from); err == nil {
			q = q.Where("created_at >= ?", t)
		}
	}
	if to != "" {
		if t, err := time.Parse(time.RFC3339, to); err == nil {
			q = q.Where("created_at <= ?", t)
		}
	}

	var total int64
	q.Count(&total)

	var entries []models.AuditLog
	q.Order("created_at desc").Limit(limit).Offset(offset).Find(&entries)
	c.JSON(http.StatusOK, gin.H{"entries": entries, "total": total})
}
