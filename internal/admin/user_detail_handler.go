package admin

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/santosh0921/nbfc-backend/internal/constants"
	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/models"
	"github.com/santosh0921/nbfc-backend/internal/security"
)

func GetUserDetailHandler(c *gin.Context) {

	idParam := c.Param("id")

	userID, err := uuid.Parse(idParam)

	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"message": "Invalid user id",
		})
		return
	}

	var user models.User

	if err := database.DB.First(&user, "id = ?", userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"message": "User not found",
		})
		return
	}

	response := gin.H{
		"id":                 user.ID,
		"mobile":             user.Mobile,
		"is_mobile_verified": user.IsMobileVerified,
		"biometric_enabled":  user.BiometricEnabled,
		"is_active":          user.IsActive,
		"created_at":         user.CreatedAt,
	}

	var profile models.CustomerProfile
	if err := database.DB.Where("user_id = ?", userID).First(&profile).Error; err == nil {
		response["profile"] = gin.H{
			"first_name":        profile.FirstName,
			"last_name":         profile.LastName,
			"email":             profile.Email,
			"date_of_birth":     profile.DateOfBirth,
			"gender":            profile.Gender,
			"marital_status":    profile.MaritalStatus,
			"occupation":        profile.Occupation,
			"profile_completed": profile.ProfileCompleted,
		}
	}

	var address models.CustomerAddress
	if err := database.DB.Where("user_id = ?", userID).First(&address).Error; err == nil {
		response["address"] = gin.H{
			"current_address":    address.CurrentAddress,
			"current_state":      address.CurrentState,
			"current_district":   address.CurrentDistrict,
			"current_city":       address.CurrentCity,
			"current_pincode":    address.CurrentPincode,
			"permanent_address":  address.PermanentAddress,
			"permanent_state":    address.PermanentState,
			"permanent_district": address.PermanentDistrict,
			"permanent_city":     address.PermanentCity,
			"permanent_pincode":  address.PermanentPincode,
		}
	}

	var pan models.CustomerPAN
	if err := database.DB.Where("user_id = ?", userID).First(&pan).Error; err == nil {
		response["pan"] = gin.H{
			"pan_number":                  security.MaskPAN(pan.PANLast4),
			"name_on_pan":                 pan.NameOnPAN,
			"verification_status":         pan.VerificationStatus,
			"is_verified":                 pan.IsVerified,
			"verification_failure_reason": pan.VerificationFailureReason,
			"verified_at":                 pan.VerifiedAt,
		}
	}

	var aadhaar models.CustomerAadhaar
	if err := database.DB.Where("user_id = ?", userID).First(&aadhaar).Error; err == nil {
		response["aadhaar"] = gin.H{
			"masked_aadhaar":              aadhaar.MaskedAadhaar,
			"name_on_aadhaar":             aadhaar.NameOnAadhaar,
			"name_matched":                aadhaar.NameMatched,
			"verification_status":         aadhaar.VerificationStatus,
			"is_verified":                 aadhaar.IsVerified,
			"verification_failure_reason": aadhaar.VerificationFailureReason,
			"verified_at":                 aadhaar.VerifiedAt,
		}
	}

	var selfie models.CustomerSelfie
	if err := database.DB.Where("user_id = ?", userID).First(&selfie).Error; err == nil {
		response["selfie"] = gin.H{
			"image_path":                  selfie.ImagePath,
			"verification_status":         selfie.VerificationStatus,
			"is_verified":                 selfie.IsVerified,
			"verification_failure_reason": selfie.VerificationFailureReason,
			"verified_at":                 selfie.VerifiedAt,
		}
	}

	var employment models.CustomerEmployment
	if err := database.DB.Where("user_id = ?", userID).First(&employment).Error; err == nil {
		response["employment"] = gin.H{
			"employment_type":  employment.EmploymentType,
			"company_name":     employment.CompanyName,
			"occupation":       employment.Occupation,
			"monthly_income":   employment.MonthlyIncome,
			"work_email":       employment.WorkEmail,
			"experience_years": employment.ExperienceYears,
			"salary_day":       employment.SalaryDay,
		}
	}

	response["completion_percentage"] = computeKYCCompletion(userID)

	var review models.CustomerKYCReview
	if err := database.DB.Where("user_id = ?", userID).First(&review).Error; err == nil {
		response["kyc_review"] = gin.H{
			"status":            review.Status,
			"reviewed_by":       review.ReviewedBy,
			"reviewed_at":       review.ReviewedAt,
			"rejection_reason":  review.RejectionReason,
		}
	} else {
		response["kyc_review"] = gin.H{
			"status": constants.KYCPending,
		}
	}

	c.JSON(http.StatusOK, response)
}
