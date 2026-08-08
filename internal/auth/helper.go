package auth

import "unicode"

func IsNumeric(value string) bool {
	for _, ch := range value {
		if !unicode.IsDigit(ch) {
			return false
		}
	}
	return true
}

func IsValidMobile(mobile string) bool {
	return len(mobile) == 10 && IsNumeric(mobile)
}