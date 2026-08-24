package ckyc

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/models"
)

// AdminListHandler handles GET /admin/ckyc — the "CKYC Status" admin panel
// screen. Optionally filtered by ?customerId=.
func AdminListHandler(c *gin.Context) {
	q := database.DB.Model(&models.CKYCRequest{})
	if customerID := c.Query("customerId"); customerID != "" {
		q = q.Where("customer_id = ?", customerID)
	}
	var records []models.CKYCRequest
	q.Order("requested_at desc").Limit(200).Find(&records)
	c.JSON(http.StatusOK, records)
}

// AdminSearchHandler handles POST /admin/ckyc/:customerId/search — triggers
// a CKYC search request for a customer. Always resolves to a real
// CKYCRequest row (BLOCKED today, since no provider is configured — see
// service.go) rather than a fake success.
func AdminSearchHandler(c *gin.Context) {
	customerID, err := uuid.Parse(c.Param("customerId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid customer id"})
		return
	}
	req, err := Request(customerID, models.CKYCRequestTypeSearch)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "CKYC search could not be completed", "request": req, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, req)
}
