package employee

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/models"
)

// LoginRequest is the employee-facing login contract: name + password.
// `module` is accepted for backward API compatibility with existing
// clients but is no longer consulted — see the removed auto-registration
// note below.
type LoginRequest struct {
	Name     string `json:"name" binding:"required"`
	Password string `json:"password" binding:"required"`
	Module   string `json:"module"`
}

// LoginHandler handles POST /employee/login. Employee accounts are
// admin-provisioned only (see AdminCreateHandler, POST /admin/employees) —
// this handler verifies name + password against an existing row and never
// creates one.
//
// It used to auto-register any unrecognized name on the spot, issuing a
// valid signed employee JWT with no invite, no admin approval, and no
// allowlist — anyone could self-grant access to real customer names,
// phone numbers, overdue amounts, and loan-verification actions. That
// path has been removed entirely; an unrecognized name is now a plain
// 401, identical in shape to a wrong password, so the response doesn't
// even leak whether an account exists.
func LoginHandler(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid request"})
		return
	}

	const invalidCredentialsMessage = "Invalid name or password. If you're a new employee, ask your admin to add your account."

	var emp models.Employee
	err := database.DB.Where("LOWER(name) = LOWER(?)", req.Name).First(&emp).Error

	if err == gorm.ErrRecordNotFound {
		c.JSON(http.StatusUnauthorized, gin.H{"message": invalidCredentialsMessage})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Database error"})
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
		c.JSON(http.StatusUnauthorized, gin.H{"message": invalidCredentialsMessage})
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
