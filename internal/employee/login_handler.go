package employee

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"

	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/models"
)

type LoginRequest struct {
	Code     string `json:"code" binding:"required"`
	Password string `json:"password" binding:"required"`
}

func LoginHandler(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid request"})
		return
	}

	var emp models.Employee
	if err := database.DB.Where("code = ?", req.Code).First(&emp).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"message": "Invalid credentials"})
		return
	}
	if !emp.Active {
		c.JSON(http.StatusForbidden, gin.H{"message": "Account deactivated"})
		return
	}
	if emp.AgencyID != 0 {
		var ag models.Agency
		if err := database.DB.First(&ag, emp.AgencyID).Error; err != nil || !ag.Active {
			c.JSON(http.StatusForbidden, gin.H{"message": "Agency deactivated"})
			return
		}
	}
	if bcrypt.CompareHashAndPassword([]byte(emp.PasswordHash), []byte(req.Password)) != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"message": "Invalid credentials"})
		return
	}

	token, err := GenerateToken(emp.ID, emp.Code, emp.Role)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Failed to generate token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":  "Login successful",
		"token":    token,
		"employee": emp,
	})
}
