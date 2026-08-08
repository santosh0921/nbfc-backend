package otp

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/santosh0921/nbfc-backend/internal/database"
)

func GenerateOTP(mobile string) (string, error) {

	// Generate random 6-digit OTP
	rand.New(rand.NewSource(time.Now().UnixNano()))

	otp := fmt.Sprintf("%06d", rand.Intn(1000000))

	record := OTPVerification{
		Mobile:    mobile,
		OTP:       otp,
		Verified:  false,
		Attempts:  0,
		ExpiresAt: time.Now().Add(5 * time.Minute),
	}

	// Delete old OTPs for same mobile
	database.DB.Where("mobile = ?", mobile).Delete(&OTPVerification{})

	// Save new OTP
	err := database.DB.Create(&record).Error

	if err != nil {
		return "", err
	}

	fmt.Println("===================================")
	fmt.Println("📱 Mobile :", mobile)
	fmt.Println("🔐 OTP    :", otp)
	fmt.Println("===================================")

	return otp, nil
}

func VerifyOTP(mobile string, otp string) error {

	var record OTPVerification

	err := database.DB.
		Where("mobile = ?", mobile).
		First(&record).Error

	if err != nil {
		return fmt.Errorf("OTP not found")
	}

	if time.Now().After(record.ExpiresAt) {
		return fmt.Errorf("OTP expired")
	}

	if record.Verified {
		return fmt.Errorf("OTP already verified")
	}

	if record.Attempts >= 5 {
		return fmt.Errorf("too many invalid attempts")
	}

	if record.OTP != otp {

		record.Attempts++

		database.DB.Save(&record)

		return fmt.Errorf("invalid OTP")
	}

	record.Verified = true

	database.DB.Save(&record)

	return nil
}