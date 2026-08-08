package profile

import (
	"net/http"

	"github.com/gin-gonic/gin"
   	"time"

	"gorm.io/gorm"

    "github.com/santosh0921/nbfc-backend/internal/database"
    "github.com/santosh0921/nbfc-backend/internal/models"
)

func CreateProfileHandler(c *gin.Context) {

	var req CreateProfileRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"message": "Invalid Request",
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


var profile models.CustomerProfile

err = database.DB.
	Where("user_id = ?", user.ID).
	First(&profile).Error

if err == nil {
	c.JSON(http.StatusConflict, gin.H{
		"message": "Profile already exists",
	})
	return
}

if err != gorm.ErrRecordNotFound {
	c.JSON(http.StatusInternalServerError, gin.H{
		"message": "Database error",
	})
	return
}
	var dob time.Time

dob, err = time.Parse("2006-01-02", req.DateOfBirth)
if err != nil {
	c.JSON(http.StatusBadRequest, gin.H{
		"message": "Invalid date format. Use YYYY-MM-DD",
	})
	return
}

profile = models.CustomerProfile{
	UserID:           user.ID,
	FirstName:        req.FirstName,
	LastName:         req.LastName,
	Email:            req.Email,
	DateOfBirth:      &dob,
	Gender:           req.Gender,
	MaritalStatus:    req.MaritalStatus,
	Occupation:       req.Occupation,
	ProfileCompleted: true,
}

if err := database.DB.Create(&profile).Error; err != nil {
	c.JSON(http.StatusInternalServerError, gin.H{
		"message": "Unable to create profile",
	})
	return
}

c.JSON(http.StatusOK, gin.H{
	"message": "Profile created successfully",
})
}