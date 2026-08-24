package admin

import (
	"strconv"
	"strings"
)

func itoa[T ~uint | ~uint64 | ~int](v T) string {
	return strconv.FormatUint(uint64(v), 10)
}

func isUniqueViolation(err error) bool {
	return err != nil && strings.Contains(strings.ToLower(err.Error()), "duplicate")
}
