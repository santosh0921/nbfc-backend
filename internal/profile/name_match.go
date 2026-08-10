package profile

import (
	"regexp"
	"strings"
)

var multipleSpaces = regexp.MustCompile(`\s+`)

// leadingTitle matches a courtesy title ONLY as a whole leading word,
// followed by a mandatory word boundary (whitespace, after an optional
// trailing period) — never as a bare substring. The old implementation
// used strings.ReplaceAll per title, which strips the title from ANYWHERE
// in the string: "AMRITA" contains "MR", "DRISHTI" contains "DR",
// "RAMSHARAN" contains "MS", mangling ordinary first names.
//
// "MRS?" (not two separate "MR"/"MRS" alternatives) also fixes the
// original's title ORDERING bug: "MR" was tried before "MRS." in the old
// replacement list, so "MRS. SUNITA RAO" had its "MR" eaten first,
// leaving an orphaned "S. SUNITA RAO". A single pattern with an optional
// trailing "S" matches the longest applicable title first by construction
// (greedy `?`), independent of any list ordering.
var leadingTitle = regexp.MustCompile(`^\s*(MRS?|MS|DR)\.?\s+`)

func normalizeName(name string) string {

	name = strings.ToUpper(name)

	name = leadingTitle.ReplaceAllString(name, "")

	// Replace punctuation with spaces
	name = strings.ReplaceAll(name, ".", " ")
	name = strings.ReplaceAll(name, "-", " ")
	name = strings.ReplaceAll(name, "_", " ")

	// Remove extra spaces
	name = multipleSpaces.ReplaceAllString(name, " ")

	return strings.TrimSpace(name)
}

// IsNameMatched compares two names after normalization. An exact match
// after normalization is always accepted. Failing that, it falls back to
// comparing only the first and last name tokens — real PAN/Aadhaar
// records routinely differ in middle name/initial (present on one
// document, abbreviated or absent on the other), which used to fail this
// check outright even for the same person. This fallback still requires
// BOTH the first and last tokens to match exactly, so it doesn't turn into
// a broad fuzzy match — "Amrita Sharma" and "Amit Sharma" still fail.
func IsNameMatched(name1, name2 string) bool {

	n1 := normalizeName(name1)
	n2 := normalizeName(name2)

	if n1 == n2 {
		return true
	}

	t1 := strings.Fields(n1)
	t2 := strings.Fields(n2)
	if len(t1) == 0 || len(t2) == 0 {
		return false
	}

	return t1[0] == t2[0] && t1[len(t1)-1] == t2[len(t2)-1]
}
