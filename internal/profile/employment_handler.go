package profile

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/models"
)

func CreateEmploymentHandler(c *gin.Context) {

	var req EmploymentRequest

	if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{
            "message": err.Error(),
        })
        return
    }

	mobile := c.GetString("mobile")

	if mobile == "" {
		c.JSON(http.StatusUnauthorized, gin.H{
			"message": "Unauthorized",
		})
		return
	}

	var user models.User

	err := database.DB.
		Where("mobile = ?", mobile).
		First(&user).Error

	if err != nil {

		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{
				"message": "User not found",
			})
			return
		}

		c.JSON(http.StatusInternalServerError, gin.H{
			"message": "Database error",
		})
		return
	}

	var employment models.CustomerEmployment

	err = database.DB.
		Where("user_id = ?", user.ID).
		First(&employment).Error

	isUpdate := err == nil

	if err != nil && err != gorm.ErrRecordNotFound {
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": "Database error",
		})
		return
	}

	employment.UserID = user.ID
	employment.EmploymentType = req.EmploymentType
	employment.CompanyName = req.CompanyName
	employment.Occupation = req.Occupation
	employment.MonthlyIncome = req.MonthlyIncome
	employment.WorkEmail = req.WorkEmail
	employment.ExperienceYears = req.ExperienceYears
	employment.SalaryDay = req.SalaryDay

	if isUpdate {

		if err := database.DB.Save(&employment).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"message": "Unable to update employment",
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"message": "Employment updated successfully",
		})
		return
	}

	if err := database.DB.Create(&employment).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": "Unable to create employment",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Employment created successfully",
	})
}