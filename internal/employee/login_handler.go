package employee

import (
	"crypto/rand"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/models"
)

// LoginRequest is the employee-facing self-login contract: name + password,
// no more employee-typed codes. `module` tells us which team a genuinely
// first-time employee is joining ("verification" | "recovery") — it is only
// consulted on first-time auto-registration and is otherwise ignored.
type LoginRequest struct {
	Name     string `json:"name" binding:"required"`
	Password string `json:"password" binding:"required"`
	Module   string `json:"module"`
}

var moduleToRole = map[string]string{
	"verification": "verification",
	"recovery":     "recovery",
}

// generateEmployeeCode produces a unique internal identifier for a
// newly-registered employee, e.g. "EMP-9F3A2B". It is purely internal
// bookkeeping now — employees never see or type it, they log in by name.
func generateEmployeeCode() (string, error) {
	for i := 0; i < 20; i++ {
		buf := make([]byte, 3)
		if _, err := rand.Read(buf); err != nil {
			return "", err
		}
		code := fmt.Sprintf("EMP-%X", buf)
		var count int64
		database.DB.Model(&models.Employee{}).Where("code = ?", code).Count(&count)
		if count == 0 {
			return code, nil
		}
	}
	return "", fmt.Errorf("could not generate a unique employee code")
}

// LoginHandler handles POST /employee/login. Behavior:
//   - Name found: verify password. Correct -> JWT. Wrong -> 401 with a
//     collision-aware message (since a shared name is a plausible, honest
//     cause, not necessarily fraud).
//   - Name not found: first-time employee, auto-register (code, password
//     hash, role from `module`, best-effort agency) and log them in in the
//     same request/response.
func LoginHandler(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid request"})
		return
	}

	var emp models.Employee
	err := database.DB.Where("LOWER(name) = LOWER(?)", req.Name).First(&emp).Error

	if err == nil {
		// Existing employee (or someone else with the same name).
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
			c.JSON(http.StatusUnauthorized, gin.H{
				"message": fmt.Sprintf(
					"An employee named '%s' already exists. If this is you, enter your password. If you're a different person, please add a last initial or distinguishing detail to your name.",
					emp.Name,
				),
			})
			return
		}
	} else if err == gorm.ErrRecordNotFound {
		// First-time employee: auto-register.
		role, ok := moduleToRole[req.Module]
		if !ok {
			c.JSON(http.StatusBadRequest, gin.H{"message": "module must be 'verification' or 'recovery' for a first-time login"})
			return
		}

		code, genErr := generateEmployeeCode()
		if genErr != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "Failed to register employee"})
			return
		}

		hash, hashErr := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		if hashErr != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "Failed to register employee"})
			return
		}

		// Best-effort agency assignment: any active agency, else none.
		var agencyID uint
		var ag models.Agency
		if aerr := database.DB.Where("active = ?", true).First(&ag).Error; aerr == nil {
			agencyID = ag.ID
		}

		emp = models.Employee{
			Code:         code,
			Name:         req.Name,
			PasswordHash: string(hash),
			Role:         role,
			AgencyID:     agencyID,
			Active:       true,
		}
		if createErr := database.DB.Create(&emp).Error; createErr != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "Failed to register employee"})
			return
		}
	} else {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Database error"})
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
