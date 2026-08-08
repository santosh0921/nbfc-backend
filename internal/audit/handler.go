package audit

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/models"
)

// AdminListHandler handles GET /admin/audit-log
func AdminListHandler(c *gin.Context) {
	action := c.Query("action")
	targetType := c.Query("targetType")

	limit := 100
	if v, err := strconv.Atoi(c.Query("limit")); err == nil && v > 0 {
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

	var entries []models.AuditLog
	q.Order("created_at desc").Limit(limit).Offset(offset).Find(&entries)
	c.JSON(http.StatusOK, entries)
}
