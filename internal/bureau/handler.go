package bureau

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/models"
)

// AdminListHandler handles GET /admin/bureau/requests — the "Credit Bureau
// Requests"/"Credit Bureau Results" admin panel screens (one list, the
// panel can filter client-side by status). Never returns RawResponse — the
// model doesn't even have that field persisted, only ResponseHash.
func AdminListHandler(c *gin.Context) {
	q := database.DB.Model(&models.BureauRequest{})
	if customerID := c.Query("customerId"); customerID != "" {
		q = q.Where("customer_id = ?", customerID)
	}
	var records []models.BureauRequest
	q.Order("requested_at desc").Limit(200).Find(&records)
	c.JSON(http.StatusOK, records)
}
