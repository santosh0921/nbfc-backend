package auth

import "golang.org/x/crypto/bcrypt"

func HashMPIN(mpin string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(mpin), bcrypt.DefaultCost)
	return string(bytes), err
}

func CheckMPIN(hash, mpin string) bool {
	return bcrypt.CompareHashAndPassword(
		[]byte(hash),
		[]byte(mpin),
	) == nil
}