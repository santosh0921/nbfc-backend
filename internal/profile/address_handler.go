package profile

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/models"

)

func CreateAddressHandler(c *gin.Context) {

	var req AddressRequest

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

var address models.CustomerAddress

err = database.DB.
    Where("user_id = ?", user.ID).
    First(&address).Error

isUpdate := err == nil

if err != nil && err != gorm.ErrRecordNotFound {
    c.JSON(http.StatusInternalServerError, gin.H{
        "message": "Database error",
    })
    return
}

if req.SameAsCurrent {

    req.PermanentAddress = req.CurrentAddress
    req.PermanentState = req.CurrentState
    req.PermanentDistrict = req.CurrentDistrict
    req.PermanentCity = req.CurrentCity
    req.PermanentPincode = req.CurrentPincode
}

address.UserID = user.ID

address.CurrentAddress = req.CurrentAddress
address.CurrentState = req.CurrentState
address.CurrentDistrict = req.CurrentDistrict
address.CurrentCity = req.CurrentCity
address.CurrentPincode = req.CurrentPincode

address.PermanentAddress = req.PermanentAddress
address.PermanentState = req.PermanentState
address.PermanentDistrict = req.PermanentDistrict
address.PermanentCity = req.PermanentCity
address.PermanentPincode = req.PermanentPincode



if isUpdate {

    if err := database.DB.Save(&address).Error; err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{
            "message": "Unable to update address",
        })
        return
    }

    c.JSON(http.StatusOK, gin.H{
        "message": "Address updated successfully",
    })

    return
}

if err := database.DB.Create(&address).Error; err != nil {
	c.JSON(http.StatusInternalServerError, gin.H{
		"message": "Unable to create address",
	})
	return
}

c.JSON(http.StatusOK, gin.H{
	"message": "Address created successfully",
})
}
