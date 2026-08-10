package profile

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"strings"
	"regexp"
    "unicode"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"time"
	"log"
	"github.com/santosh0921/nbfc-backend/internal/config"
	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/models"
	"github.com/santosh0921/nbfc-backend/internal/security"
	"github.com/santosh0921/nbfc-backend/internal/surepass"

)

var panRegex = regexp.MustCompile(`^[A-Z]{5}[0-9]{4}[A-Z]{1}$`)


func CreatePANHandler(c *gin.Context) {

	var req PANRequest

	if err := c.ShouldBindJSON(&req); err != nil {
	    c.JSON(http.StatusBadRequest, gin.H{
		    "message": err.Error(),
	    })
	    return
    }

	req.PANNumber = strings.ToUpper(strings.TrimSpace(req.PANNumber))
	req.NameOnPAN = strings.ToUpper(strings.TrimSpace(req.NameOnPAN))


	if !isValidPANName(req.NameOnPAN) {
	c.JSON(http.StatusBadRequest, gin.H{
		"message": "Invalid name on PAN",
	})
	return
}

    if !panRegex.MatchString(req.PANNumber) {
	    c.JSON(http.StatusBadRequest, gin.H{
		    "message": "Invalid PAN format",
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

	var customerPAN models.CustomerPAN

// Check if this user already has a PAN
err = database.DB.
	Where("user_id = ?", user.ID).
	First(&customerPAN).Error

if err == nil {
	c.JSON(http.StatusConflict, gin.H{
		"message": "PAN already submitted",
	})
	return
}

if err != gorm.ErrRecordNotFound {
	c.JSON(http.StatusInternalServerError, gin.H{
		"message": "Database error",
	})
	return
}

// Check if this PAN is already linked to another customer (PAN is
// encrypted at rest, so dedup is done against a SHA-256 hash, not the
// ciphertext — the ciphertext isn't equality-comparable across rows
// because of AES-GCM's random nonce).
panHashBytes := sha256.Sum256([]byte(req.PANNumber))
panHash := hex.EncodeToString(panHashBytes[:])

err = database.DB.
	Where("pan_hash = ?", panHash).
	First(&customerPAN).Error

if err == nil {
	c.JSON(http.StatusConflict, gin.H{
		"message": "PAN already exists",
	})
	return
}

if err != gorm.ErrRecordNotFound {
	c.JSON(http.StatusInternalServerError, gin.H{
		"message": "Database error",
	})
	return
}


var (
	status         string
	verified       bool
	failureReason  string
	nameMatched    bool
	surepassName   string
	clientRef      string
	verifyRemarks  string
)

if config.DemoMode {
	log.Println("DEMO_MODE: skipping real Surepass PAN call")
	status = "VERIFIED"
	verified = true
	nameMatched = true
	surepassName = req.NameOnPAN
	clientRef = "DEMO-MODE"
	verifyRemarks = "Auto-verified (DEMO_MODE — Surepass call skipped)"
} else {
	log.Println("Calling Surepass PAN API")

	panResponse, err := surepass.API.VerifyPAN(req.PANNumber)

	if err != nil {
		var provErr *surepass.ProviderError
		if errors.As(err, &provErr) {
			log.Printf("Surepass PAN verification unavailable (status %d) — treating as a system error, not an invalid PAN", provErr.StatusCode)
		} else {
			log.Println("Surepass PAN verification request failed:", err)
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": "Unable to verify PAN",
		})
		return
	}

	if !panResponse.Success {
		c.JSON(http.StatusBadRequest, gin.H{
			"message": "PAN verification failed",
		})
		return
	}

	surepassName = panResponse.Data.FullName
	clientRef = panResponse.Data.ClientID
	verifyRemarks = "Verified using Surepass Sandbox"

	nameMatched = IsNameMatched(surepassName, req.NameOnPAN)

	status = "VERIFIED"
	verified = true

	if !nameMatched {
		status = "NAME_MISMATCH"
		verified = false
		failureReason = "PAN name does not match entered name"
	}
}

var verifiedAt *time.Time

if verified {
	now := time.Now()
	verifiedAt = &now
}

panEncrypted, err := security.Encrypt(req.PANNumber)
if err != nil {
	log.Println("PAN encryption failed:", err)
	c.JSON(http.StatusInternalServerError, gin.H{
		"message": "Unable to save PAN",
	})
	return
}
panLast4 := req.PANNumber[len(req.PANNumber)-4:]

customerPAN = models.CustomerPAN{
	UserID: user.ID,

	PANEncrypted: panEncrypted,
	PANLast4:     panLast4,
	PANHash:      panHash,
	NameOnPAN:    req.NameOnPAN,

	VerificationStatus:        status,
    IsVerified:                verified,
    VerificationProvider:      "SUREPASS",
    VerificationReference:     clientRef,
    VerificationRemarks:       verifyRemarks,
    VerificationFailureReason: failureReason,
    VerifiedAt:                verifiedAt,

}
if err := database.DB.Create(&customerPAN).Error; err != nil {
	c.JSON(http.StatusInternalServerError, gin.H{
		"message": "Unable to save PAN",
	})
	return
}

c.JSON(http.StatusOK, gin.H{
	"message": "PAN submitted successfully",

	"verification": gin.H{
		"status": customerPAN.VerificationStatus,
		"is_verified": customerPAN.IsVerified,
		"name_matched": nameMatched,
        "surepass_name": surepassName,
	},
})

}

func isValidPANName(name string) bool {
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

