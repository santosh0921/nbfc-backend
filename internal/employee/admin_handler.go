package employee

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"

	"github.com/santosh0921/nbfc-backend/internal/audit"
	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/models"
)

func hashPassword(pw string) (string, error) {
	h, err := bcrypt.GenerateFromPassword([]byte(pw), bcrypt.DefaultCost)
	return string(h), err
}

// AdminListHandler handles GET /admin/employees
func AdminListHandler(c *gin.Context) {
	var emps []models.Employee
	database.DB.Find(&emps)
	c.JSON(http.StatusOK, emps)
}

// AdminCreateHandler handles POST /admin/employees
func AdminCreateHandler(c *gin.Context) {
	var body struct {
		Code       string `json:"code" binding:"required"`
		Name       string `json:"name" binding:"required"`
		Password   string `json:"password" binding:"required"`
		Role       string `json:"role" binding:"required"`
		BranchCity string `json:"branchCity"`
		AgencyID   uint   `json:"agencyId"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	hash, err := hashPassword(body.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "failed to hash password"})
		return
	}
	emp := models.Employee{
		Code: body.Code, Name: body.Name, PasswordHash: hash,
		Role: body.Role, BranchCity: body.BranchCity, AgencyID: body.AgencyID, Active: true,
	}
	if err := database.DB.Create(&emp).Error; err != nil {
		c.JSON(http.StatusConflict, gin.H{"message": "employee code may already exist"})
		return
	}
	audit.Record(c.GetString("admin_username"), "employee.create", "employee", strconv.Itoa(int(emp.ID)),
		"Created employee "+emp.Code+" ("+emp.Name+"), role="+emp.Role)
	c.JSON(http.StatusCreated, emp)
}

// AdminDeleteHandler handles DELETE /admin/employees/:id — deactivates,
// revoking login/API access immediately (403 on next request).
func AdminDeleteHandler(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var emp models.Employee
	if err := database.DB.First(&emp, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"message": "Not found"})
		return
	}
	emp.Active = false
	database.DB.Save(&emp)
	audit.Record(c.GetString("admin_username"), "employee.delete", "employee", strconv.Itoa(int(emp.ID)),
		"Deactivated employee "+emp.Code+" ("+emp.Name+")")
	c.JSON(http.StatusOK, gin.H{"message": "employee deactivated, login access revoked"})
}
