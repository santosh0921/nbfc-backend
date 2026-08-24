package writeoff

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/models"
)

func currentAdmin(c *gin.Context) (uint, string) {
	var id uint
	if v, ok := c.Get("admin_id"); ok {
		if cast, ok := v.(uint); ok {
			id = cast
		}
	}
	return id, c.GetString("admin_username")
}

func AdminListWriteOffsHandler(c *gin.Context) {
	q := database.DB.Model(&models.WriteOffRequest{})
	if status := c.Query("status"); status != "" {
		q = q.Where("status = ?", status)
	}
	var requests []models.WriteOffRequest
	q.Order("id desc").Find(&requests)
	c.JSON(http.StatusOK, requests)
}

// AdminRequestWriteOffHandler — requires WRITE_OFF_REQUEST.
func AdminRequestWriteOffHandler(c *gin.Context) {
	var body struct {
		LoanID      uint   `json:"loanId" binding:"required"`
		AmountPaise int64  `json:"amountPaise" binding:"required"`
		Reason      string `json:"reason" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	adminID, adminName := currentAdmin(c)
	req, err := RequestWriteOff(body.LoanID, body.AmountPaise, body.Reason, adminID, adminName)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, req)
}

// AdminApproveWriteOffHandler — requires WRITE_OFF_APPROVE.
func AdminApproveWriteOffHandler(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	adminID, adminName := currentAdmin(c)
	req, err := ApproveWriteOff(uint(id), adminID, adminName)
	if err != nil {
		status := http.StatusBadRequest
		if err == ErrSameActor {
			status = http.StatusForbidden
		}
		c.JSON(status, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, req)
}

func AdminRejectWriteOffHandler(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var body struct {
		Reason string `json:"reason"`
	}
	c.ShouldBindJSON(&body)
	adminID, adminName := currentAdmin(c)
	req, err := RejectWriteOff(uint(id), adminID, adminName, body.Reason)
	if err != nil {
		status := http.StatusBadRequest
		if err == ErrSameActor {
			status = http.StatusForbidden
		}
		c.JSON(status, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, req)
}

// AdminRecordRecoveryHandler — requires WRITE_OFF_APPROVE (same tier that
// approves write-offs handles recovery bookings against them).
func AdminRecordRecoveryHandler(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var body struct {
		AmountPaise int64  `json:"amountPaise" binding:"required"`
		Reference   string `json:"reference"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	adminID, adminName := currentAdmin(c)
	recovery, err := RecordRecovery(uint(id), body.AmountPaise, body.Reference, adminID, adminName)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, recovery)
}
