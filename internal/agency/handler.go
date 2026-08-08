package agency

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/santosh0921/nbfc-backend/internal/audit"
	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/models"
)

// ListHandler handles GET /admin/agencies
func ListHandler(c *gin.Context) {
	var agencies []models.Agency
	database.DB.Find(&agencies)
	c.JSON(http.StatusOK, agencies)
}

// CreateHandler handles POST /admin/agencies
func CreateHandler(c *gin.Context) {
	var ag models.Agency
	if err := c.ShouldBindJSON(&ag); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	ag.Active = true
	database.DB.Create(&ag)
	audit.Record(c.GetString("admin_username"), "agency.create", "agency", strconv.Itoa(int(ag.ID)),
		"Created agency "+ag.Name+" ("+ag.City+")")
	c.JSON(http.StatusCreated, ag)
}

// DeleteHandler handles DELETE /admin/agencies/:id — deactivates the agency
// and cascades deactivation to its employees (revokes their access).
func DeleteHandler(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var ag models.Agency
	if err := database.DB.First(&ag, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"message": "Not found"})
		return
	}
	ag.Active = false
	database.DB.Save(&ag)
	database.DB.Model(&models.Employee{}).Where("agency_id = ?", ag.ID).Update("active", false)
	audit.Record(c.GetString("admin_username"), "agency.delete", "agency", strconv.Itoa(int(ag.ID)),
		"Deactivated agency "+ag.Name+" and its employees")
	c.JSON(http.StatusOK, gin.H{"message": "agency and its employees deactivated"})
}
