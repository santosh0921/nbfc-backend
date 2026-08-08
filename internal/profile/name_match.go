package profile

import (
	"regexp"
	"strings"
)

var multipleSpaces = regexp.MustCompile(`\s+`)

func normalizeName(name string) string {

	name = strings.ToUpper(name)

	// Remove common titles
	replacements := []string{
		"MR.",
		"MR",
		"MRS.",
		"MRS",
		"MS.",
		"MS",
		"DR.",
		"DR",
	}

	for _, r := range replacements {
		name = strings.ReplaceAll(name, r, "")
	}

	// Replace punctuation with spaces
	name = strings.ReplaceAll(name, ".", " ")
	name = strings.ReplaceAll(name, "-", " ")
	name = strings.ReplaceAll(name, "_", " ")

	// Remove extra spaces
	name = multipleSpaces.ReplaceAllString(name, " ")

	return strings.TrimSpace(name)
}

func IsNameMatched(name1, name2 string) bool {

	n1 := normalizeName(name1)
	n2 := normalizeName(name2)

	return n1 == n2
}