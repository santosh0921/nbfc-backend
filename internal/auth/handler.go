package auth

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"net/http"
	"time"

	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/models"
	"github.com/santosh0921/nbfc-backend/internal/otp"
)

func CreateMPINHandler(c *gin.Context) {

	var req CreateMPINRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"message": "Invalid Request",
		})
		return
	}

	if req.MPIN != req.ConfirmMPIN {
		c.JSON(http.StatusBadRequest, gin.H{
			"message": "MPIN and Confirm MPIN do not match",
		})
		return
	}

	var otpRecord otp.OTPVerification

	err := database.DB.
		Where("mobile = ? AND verified = ?", req.Mobile, true).
		First(&otpRecord).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusUnauthorized, gin.H{
				"message": "Please verify OTP first",
			})
			return
		}

		c.JSON(http.StatusInternalServerError, gin.H{
			"message": "Database error",
		})
		return
	}

	// VerifyOTP checks ExpiresAt, but this handler — which actually resets
	// the credential for an EXISTING user — used not to check it at all.
	// A user who verified an OTP and then abandoned the flow left a
	// permanent, still-usable MPIN-reset token behind with no time limit.
	if time.Now().After(otpRecord.ExpiresAt) {
		c.JSON(http.StatusUnauthorized, gin.H{
			"message": "OTP has expired. Please verify again.",
		})
		return
	}

	var user models.User

	err = database.DB.
		Where("mobile = ?", req.Mobile).
		First(&user).Error

	// ==========================
	// START DATABASE TRANSACTION
	// ==========================

	tx := database.DB.Begin()

	if tx.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": "Database error",
		})
		return
	}

	// ==========================
	// NEW USER
	// ==========================

	if err != nil {

		if err == gorm.ErrRecordNotFound {

			user.Mobile = req.Mobile
			user.IsMobileVerified = true
			user.IsActive = true
			user.BiometricEnabled = false
			user.FailedLoginAttempts = 0
			user.LockedUntil = nil

			hashed, err := HashMPIN(req.MPIN)
			if err != nil {
				tx.Rollback()
				c.JSON(http.StatusInternalServerError, gin.H{
					"message": "Unable to hash MPIN",
				})
				return
			}

			user.MPIN = hashed

			if err := tx.Create(&user).Error; err != nil {
				tx.Rollback()
				c.JSON(http.StatusInternalServerError, gin.H{
					"message": "Unable to create user",
				})
				return
			}

			if err := tx.Delete(&otpRecord).Error; err != nil {
				tx.Rollback()
				c.JSON(http.StatusInternalServerError, gin.H{
					"message": "Unable to delete OTP",
				})
				return
			}

			if err := tx.Commit().Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{
					"message": "Database error",
				})
				return
			}

			c.JSON(http.StatusOK, gin.H{
				"message": "MPIN Created Successfully",
			})
			return
		}

		tx.Rollback()

		c.JSON(http.StatusInternalServerError, gin.H{
			"message": "Database error",
		})
		return
	}

	// ==========================
	// EXISTING USER
	// ==========================

	hashed, err := HashMPIN(req.MPIN)
	if err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": "Unable to hash MPIN",
		})
		return
	}

	if err := tx.Model(&user).Updates(map[string]interface{}{
		"mpin":               hashed,
		"is_mobile_verified": true,
	}).Error; err != nil {

		tx.Rollback()

		c.JSON(http.StatusInternalServerError, gin.H{
			"message": "Unable to update MPIN",
		})
		return
	}

	if err := tx.Delete(&otpRecord).Error; err != nil {

		tx.Rollback()

		c.JSON(http.StatusInternalServerError, gin.H{
			"message": "Unable to delete OTP",
		})
		return
	}

	if err := tx.Commit().Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": "Database error",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "MPIN Updated Successfully",
	})
}

func LoginHandler(c *gin.Context) {

	var req LoginRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"message": "Invalid Request",
		})
		return
	}

	var user models.User

	err := database.DB.
		Where("mobile = ?", req.Mobile).
		First(&user).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusUnauthorized, gin.H{
				"message": "Invalid Mobile Number",
			})
			return
		}

		c.JSON(http.StatusInternalServerError, gin.H{
			"message": "Database error",
		})
		return
	}

	if !user.IsMobileVerified {
		c.JSON(http.StatusUnauthorized, gin.H{
			"message": "Mobile number not verified",
		})
		return
	}

	// Mobile exists but account is blocked
	if !user.IsActive {
		c.JSON(http.StatusForbidden, gin.H{
			"message": "Account is blocked",
		})
		return
	}

	// Check if account is temporarily locked.
	if user.LockedUntil != nil {
		if user.LockedUntil.After(time.Now()) {
			c.JSON(http.StatusForbidden, gin.H{
				"message": "Account is temporarily locked. Try again later.",
			})
			return
		}

		// The lock has expired. It used to be left in place (still
		// non-nil) until the NEXT successful login, and the re-lock
		// condition below required LockedUntil == nil — so once a lock
		// expired once, that guard could never be true again, and the
		// account could never be re-locked for the rest of its life
		// no matter how many more attempts followed. Clearing it here,
		// as soon as it's known to be expired, gives the attempt
		// counter a genuinely fresh window and lets a second lockout
		// actually trigger.
		user.LockedUntil = nil
		user.FailedLoginAttempts = 0
		if err := database.DB.Model(&user).Updates(map[string]interface{}{
			"failed_login_attempts": 0,
			"locked_until":          nil,
		}).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"message": "Database error",
			})
			return
		}
	}

	// Verify MPIN
	if !CheckMPIN(user.MPIN, req.MPIN) {

		user.FailedLoginAttempts++

		if user.FailedLoginAttempts >= 5 && user.LockedUntil == nil {
			lockTime := time.Now().Add(15 * time.Minute)
			user.LockedUntil = &lockTime
		}

		if err := database.DB.Model(&user).Updates(map[string]interface{}{
			"failed_login_attempts": user.FailedLoginAttempts,
			"locked_until":          user.LockedUntil,
		}).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"message": "Database error",
			})
			return
		}

		c.JSON(http.StatusUnauthorized, gin.H{
			"message": "Invalid MPIN",
		})
		return
	}

	token, err := GenerateToken(user.Mobile)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": "Unable to generate token",
		})
		return
	}

	// Reset failed login attempts after successful login
	user.FailedLoginAttempts = 0
	user.LockedUntil = nil

	if err := database.DB.Model(&user).Updates(map[string]interface{}{
		"failed_login_attempts": 0,
		"locked_until":          nil,
	}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": "Database error",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Login Successful",
		"token":   token,
		"user": gin.H{
			"id":                user.ID,
			"mobile":            user.Mobile,
			"biometric_enabled": user.BiometricEnabled,
		},
	})
}
