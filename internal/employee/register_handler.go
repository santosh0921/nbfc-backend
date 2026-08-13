package employee

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/models"
)

type registerRequest struct {
	Name       string `json:"name" binding:"required"`
	Password   string `json:"password" binding:"required,min=6"`
	Role       string `json:"role" binding:"required"`
	BranchCity string `json:"branchCity"`
}

var validRegisterRoles = map[string]bool{"verification": true, "recovery": true, "supervisor": true}

// RegisterHandler handles POST /employee/register — genuine, public
// self-service sign-up for a new employee. Unlike the auto-registration
// this backend used to do silently at login time (removed as a critical
// security issue — see the doc comment on LoginHandler), this creates the
// account in a clearly pending state: Approved=false, no agency assigned.
// LoginHandler refuses to issue a token until an admin explicitly reviews
// and approves the account (AdminApproveHandler), which is what actually
// keeps this safe to expose publicly.
func RegisterHandler(c *gin.Context) {
	var req registerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid request"})
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Name is required"})
		return
	}
	if !validRegisterRoles[req.Role] {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Role must be verification, recovery, or supervisor"})
		return
	}

	var existing models.Employee
	err := database.DB.Where("LOWER(name) = LOWER(?)", name).First(&existing).Error
	if err == nil {
		c.JSON(http.StatusConflict, gin.H{"message": "An account with this name already exists. Try logging in, or contact your admin."})
		return
	}
	if err != gorm.ErrRecordNotFound {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Database error"})
		return
	}

	hash, err := hashPassword(req.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Failed to hash password"})
		return
	}

	emp := models.Employee{
		Code:         generateEmployeeCode(name),
		Name:         name,
		PasswordHash: hash,
		Role:         req.Role,
		BranchCity:   strings.TrimSpace(req.BranchCity),
		AgencyID:     0, // assigned by the admin at approval time
		Active:       true,
		Approved:     false,
	}
	if err := database.DB.Create(&emp).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Could not create account"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "Account created. An admin needs to approve it before you can log in.",
	})
}

// generateEmployeeCode derives a short, human-readable employee code from
// the registrant's name (e.g. "Priya Deshmukh" -> "PRIYAD"), falling back
// to appending a numeric suffix on collision — self-registration has no
// admin around to type one in by hand the way AdminCreateHandler expects.
func generateEmployeeCode(name string) string {
	letters := make([]rune, 0, 8)
	for _, r := range strings.ToUpper(name) {
		if r >= 'A' && r <= 'Z' {
			letters = append(letters, r)
		}
		if len(letters) >= 6 {
			break
		}
	}
	base := string(letters)
	if base == "" {
		base = "EMP"
	}
	code := base
	for suffix := 1; ; suffix++ {
		var count int64
		database.DB.Model(&models.Employee{}).Where("code = ?", code).Count(&count)
		if count == 0 {
			return code
		}
		code = base + strconv.Itoa(suffix)
	}
}
