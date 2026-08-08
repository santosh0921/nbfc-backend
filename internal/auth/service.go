package auth

import (
	"errors"

	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/models"
	"github.com/santosh0921/nbfc-backend/internal/otp"
)

func CreateMPIN(req CreateMPINRequest) error {

	var otpRecord otp.OTPVerification

	err := database.DB.
		Where("mobile=? AND verified=?", req.Mobile, true).
		Last(&otpRecord).Error

	if err != nil {
		return errors.New("mobile number is not verified")
	}

	hash, err := HashMPIN(req.MPIN)

	if err != nil {
		return err
	}

	user := models.User{
		Mobile:           req.Mobile,
		MPIN:             hash,
		IsMobileVerified: true,
	}

	return CreateUser(&user)
}