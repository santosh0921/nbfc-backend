package otp

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"log"
	"math/big"
	"time"

	"github.com/santosh0921/nbfc-backend/internal/database"
)

// hashOTP returns the SHA-256 hex digest of an OTP code. A fast hash (not
// a slow password KDF like bcrypt) is the right tool here: OTPs are
// six-digit, five-minute-lived, attempt-limited secrets, not long-lived
// credentials — the threat this defends against is a database dump handing
// over live, unexpired OTP codes, not offline brute-force of the hash
// itself (VerifyOTP's 5-attempt cap already closes that door online, and a
// stolen hash is worthless after the 5-minute expiry regardless of how
// fast it can be attacked offline).
func hashOTP(otp string) string {
	sum := sha256.Sum256([]byte(otp))
	return hex.EncodeToString(sum[:])
}

// secureOTP generates a cryptographically random 6-digit code using
// crypto/rand — math/rand is a deterministic PRNG unsuitable for anything
// that gates account access.
func secureOTP() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(1000000))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%06d", n.Int64()), nil
}

// GenerateOTP creates and stores a new OTP for mobile, replacing any
// existing one, and returns the plaintext code to the caller. The caller
// (internal/otp/handler.go) is solely responsible for deciding whether
// that plaintext ever reaches a response body — outside DEMO_MODE, it
// must not. It must also never be logged; nothing in this function writes
// the code anywhere.
func GenerateOTP(mobile string) (string, error) {
	otpCode, err := secureOTP()
	if err != nil {
		return "", err
	}

	record := OTPVerification{
		Mobile:    mobile,
		OTPHash:   hashOTP(otpCode),
		Verified:  false,
		Attempts:  0,
		ExpiresAt: time.Now().Add(5 * time.Minute),
	}

	// Delete-then-create is wrapped in a transaction (rather than two
	// unchecked, unrelated statements) so a concurrent GenerateOTP call
	// for the same mobile can't interleave between the delete and the
	// create and leave two rows behind — the uniqueIndex on Mobile (see
	// model.go) would reject that at the database level anyway, but doing
	// it inside a transaction avoids surfacing that as an ugly unique-
	// constraint error to a caller who did nothing wrong.
	tx := database.DB.Begin()
	if tx.Error != nil {
		return "", tx.Error
	}
	if err := tx.Where("mobile = ?", mobile).Delete(&OTPVerification{}).Error; err != nil {
		tx.Rollback()
		return "", err
	}
	if err := tx.Create(&record).Error; err != nil {
		tx.Rollback()
		return "", err
	}
	if err := tx.Commit().Error; err != nil {
		return "", err
	}

	log.Println("OTP generated for mobile", maskMobile(mobile))

	return otpCode, nil
}

// maskMobile keeps only the last 2 digits visible, for log lines that need
// to reference which account without printing the full number.
func maskMobile(mobile string) string {
	if len(mobile) <= 2 {
		return "**"
	}
	return "********" + mobile[len(mobile)-2:]
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

	// Constant-time comparison of the hashes — otp itself is short-lived
	// and attempt-limited so this isn't defending against a realistic
	// timing attack, but there's no reason to use a comparison that leaks
	// timing information when a constant-time one is just as easy.
	if subtle.ConstantTimeCompare([]byte(hashOTP(otp)), []byte(record.OTPHash)) != 1 {

		record.Attempts++

		database.DB.Save(&record)

		return fmt.Errorf("invalid OTP")
	}

	record.Verified = true

	database.DB.Save(&record)

	return nil
}
