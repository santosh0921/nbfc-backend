package profile

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"regexp"
	"strings"
	"unicode"
	"log"


	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/models"
	"github.com/santosh0921/nbfc-backend/internal/verification"

)

var aadhaarRegex = regexp.MustCompile(`^[0-9]{12}$`)

func CreateAadhaarHandler(c *gin.Context) {


	var req AadhaarRequest

if err := c.ShouldBindJSON(&req); err != nil {
    log.Println("Bind Error:", err)

    c.JSON(http.StatusBadRequest, gin.H{
        "message": err.Error(),
    })
    return
}

	req.AadhaarNumber = strings.TrimSpace(req.AadhaarNumber)
	req.NameOnAadhaar = strings.ToUpper(strings.TrimSpace(req.NameOnAadhaar))

	if !req.Consent {
		c.JSON(http.StatusBadRequest, gin.H{
			"message": "Customer consent is required",
		})
		return
	}

	if !aadhaarRegex.MatchString(req.AadhaarNumber) {
		c.JSON(http.StatusBadRequest, gin.H{
			"message": "Invalid Aadhaar Number",
		})
		return
	}

	if !isValidAadhaarName(req.NameOnAadhaar) {
		c.JSON(http.StatusBadRequest, gin.H{
			"message": "Invalid name on Aadhaar",
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
var customerAadhaar models.CustomerAadhaar

// Check if this user already has an Aadhaar
err = database.DB.
	Where("user_id = ?", user.ID).
	First(&customerAadhaar).Error

if err == nil {
	c.JSON(http.StatusConflict, gin.H{
		"message": "Aadhaar already submitted",
	})
	return
}

if err != gorm.ErrRecordNotFound {
	c.JSON(http.StatusInternalServerError, gin.H{
		"message": "Database error",
	})
	return
}

// Generate SHA-256 hash of Aadhaar
hash := sha256.Sum256([]byte(req.AadhaarNumber))
aadhaarHash := hex.EncodeToString(hash[:])

// Check if Aadhaar already exists
err = database.DB.
	Where("aadhaar_hash = ?", aadhaarHash).
	First(&customerAadhaar).Error

if err == nil {
	c.JSON(http.StatusConflict, gin.H{
		"message": "Aadhaar already exists",
	})
	return
}

if err != gorm.ErrRecordNotFound {
	c.JSON(http.StatusInternalServerError, gin.H{
		"message": "Database error",
	})
	return
}

// Mask Aadhaar Number
last4 := req.AadhaarNumber[len(req.AadhaarNumber)-4:]
maskedAadhaar := "XXXX-XXXX-" + last4

verificationResult := verification.VerifyAadhaar(
	req.AadhaarNumber,
	req.NameOnAadhaar,
)
customerAadhaar = models.CustomerAadhaar{
	UserID: user.ID,

	MaskedAadhaar: maskedAadhaar,
	Last4Digits:   last4,
	AadhaarHash:   aadhaarHash,

	NameOnAadhaar: req.NameOnAadhaar,

	VerificationStatus:        verificationResult.Status,
    IsVerified:                verificationResult.IsVerified,
    VerificationProvider:      verificationResult.Provider,
    VerificationReference:     verificationResult.Reference,
    VerificationFailureReason: verificationResult.FailureReason,
    NameMatched:               verificationResult.NameMatched,
    VerificationRemarks:       verificationResult.Remarks,

    }

if err := database.DB.Create(&customerAadhaar).Error; err != nil {
	c.JSON(http.StatusInternalServerError, gin.H{
		"message": "Unable to save Aadhaar",
	})
	return
}

c.JSON(http.StatusOK, gin.H{
	"message": "Aadhaar submitted successfully",
	"status":  customerAadhaar.VerificationStatus,
})

}

func isValidAadhaarName(name string) bool {
	for _, r := range name {
		if unicode.IsLetter(r) ||
			unicode.IsSpace(r) ||
			r == '.' ||
			r == '-' {
			continue
		}
		return false
	}
	return true
}