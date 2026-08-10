package profile

import "testing"

// Reproduces the exact PROOF A cases from the security audit report
// (2026-08-08): normalizeName was stripping courtesy titles as bare
// substrings anywhere in the string, not as a leading whole word, causing
// both false accepts and false rejects in KYC name matching.
func TestNormalizeName_TitleStripping(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		// These four used to be mangled by the substring-strip bug: "MR"
		// inside "AMRITA"/"RAMRAJ", "DR" inside "DRISHTI", "MS" inside
		// "RAMSHARAN". None of these names carry a real title.
		{"AMRITA unaffected", "AMRITA SHARMA", "AMRITA SHARMA"},
		{"RAMRAJ unaffected", "RAMRAJ YADAV", "RAMRAJ YADAV"},
		{"DRISHTI unaffected", "DRISHTI MEHTA", "DRISHTI MEHTA"},
		{"RAMSHARAN unaffected", "RAMSHARAN LAL", "RAMSHARAN LAL"},

		// A real leading title must still be stripped.
		{"Mr. prefix stripped", "Mr. Rajesh Kumar", "RAJESH KUMAR"},
		{"Mrs. prefix stripped, no orphan letter", "Mrs. Sunita Rao", "SUNITA RAO"},
		{"Ms. prefix stripped", "Ms. Priya Singh", "PRIYA SINGH"},
		{"Dr prefix stripped, no period", "Dr Anil Gupta", "ANIL GUPTA"},
		{"MRS without period", "MRS SUNITA RAO", "SUNITA RAO"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := normalizeName(tc.in)
			if got != tc.want {
				t.Errorf("normalizeName(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestIsNameMatched(t *testing.T) {
	cases := []struct {
		name        string
		n1, n2      string
		wantMatched bool
	}{
		// The exact PROOF A false-accept: two DIFFERENT people whose names
		// both happened to lose "MR"/similar as a substring, colliding on
		// the same mangled output. Must NOT match now.
		{"different people, no longer collide", "AMRITA SHARMA", "AITA SHARMA", false},

		// The exact PROOF A false-reject: a real title-prefixed name vs
		// its plain form for the SAME person. Must match now.
		{"title vs no title, same person", "Mrs. Sunita Rao", "SUNITA RAO", true},

		// Exact match, case/spacing-insensitive.
		{"case and spacing insensitive", "  rajesh   kumar  ", "RAJESH KUMAR", true},

		// Legitimate middle-name discrepancy between PAN/Aadhaar records —
		// same first and last name, middle name present on only one side.
		{"middle name present on one side only", "Rajesh Kumar Sharma", "Rajesh Sharma", true},

		// Genuinely different people must still fail.
		{"different first name", "Amit Sharma", "Amrita Sharma", false},
		{"different last name", "Rajesh Sharma", "Rajesh Kumar", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := IsNameMatched(tc.n1, tc.n2)
			if got != tc.wantMatched {
				t.Errorf("IsNameMatched(%q, %q) = %v, want %v", tc.n1, tc.n2, got, tc.wantMatched)
			}
		})
	}
}
