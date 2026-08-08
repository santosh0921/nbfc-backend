package loans

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"hash/fnv"
	"regexp"
	"strings"

	"github.com/google/uuid"

	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/models"
	"github.com/santosh0921/nbfc-backend/internal/security"
)

var (
	refAadhaarRegex = regexp.MustCompile(`^[0-9]{12}$`)
	refPANRegex     = regexp.MustCompile(`^[A-Z]{5}[0-9]{4}[A-Z]{1}$`)
)

// ReferenceInput is the shape a caller submits per reference in the loan
// application/top-up request body.
type ReferenceInput struct {
	Name          string `json:"name"`
	Relation      string `json:"relation"`
	Phone         string `json:"phone"`
	AadhaarNumber string `json:"aadhaarNumber"`
	PANNumber     string `json:"panNumber"`
}

// MaskedReference is the shape returned in any API response — raw Aadhaar/
// PAN values are never included, only masked forms.
type MaskedReference struct {
	ID              uint   `json:"id"`
	Name            string `json:"name"`
	Relation        string `json:"relation"`
	Phone           string `json:"phone"`
	AadhaarMasked   string `json:"aadhaarMasked"`
	PANMasked       string `json:"panMasked"`
}

// validateReferences enforces exactly 2 references, each with all
// required fields present and Aadhaar/PAN in valid format.
func validateReferences(refs []ReferenceInput) error {
	if len(refs) != 2 {
		return errors.New("exactly 2 references are required")
	}
	for i, r := range refs {
		name := strings.TrimSpace(r.Name)
		relation := strings.TrimSpace(r.Relation)
		phone := strings.TrimSpace(r.Phone)
		aadhaar := strings.TrimSpace(r.AadhaarNumber)
		pan := strings.ToUpper(strings.TrimSpace(r.PANNumber))

		if name == "" {
			return errors.New("reference " + itoa(uint(i+1)) + ": name is required")
		}
		if relation == "" {
			return errors.New("reference " + itoa(uint(i+1)) + ": relation is required")
		}
		if phone == "" {
			return errors.New("reference " + itoa(uint(i+1)) + ": phone is required")
		}
		if !refAadhaarRegex.MatchString(aadhaar) {
			return errors.New("reference " + itoa(uint(i+1)) + ": aadhaarNumber must be exactly 12 digits")
		}
		if !refPANRegex.MatchString(pan) {
			return errors.New("reference " + itoa(uint(i+1)) + ": panNumber must be a valid PAN (e.g. ABCDE1234F)")
		}
	}
	return nil
}

// createReferences persists the (already-validated) references for a loan,
// encrypting/hashing PAN and Aadhaar per the same discipline used for the
// customer's own KYC documents.
func createReferences(loanID uint, refs []ReferenceInput) error {
	for _, r := range refs {
		aadhaar := strings.TrimSpace(r.AadhaarNumber)
		pan := strings.ToUpper(strings.TrimSpace(r.PANNumber))

		aHash := sha256.Sum256([]byte(aadhaar))
		aadhaarHash := hex.EncodeToString(aHash[:])
		last4 := aadhaar[len(aadhaar)-4:]
		aadhaarMasked := "XXXX-XXXX-" + last4

		panEncrypted, err := security.Encrypt(pan)
		if err != nil {
			return err
		}
		panLast4 := pan[len(pan)-4:]

		ref := models.LoanReference{
			LoanID:        loanID,
			Name:          strings.TrimSpace(r.Name),
			Relation:      strings.TrimSpace(r.Relation),
			Phone:         strings.TrimSpace(r.Phone),
			AadhaarMasked: aadhaarMasked,
			AadhaarHash:   aadhaarHash,
			PANEncrypted:  panEncrypted,
			PANLast4:      panLast4,
		}
		if err := database.DB.Create(&ref).Error; err != nil {
			return err
		}
	}
	return nil
}

// LoanResponse wraps a LoanApplication with its masked references for any
// endpoint that returns full loan detail (customer/admin/employee).
type LoanResponse struct {
	models.LoanApplication
	References []MaskedReference `json:"references"`
}

func withReferences(loan models.LoanApplication) LoanResponse {
	return LoanResponse{LoanApplication: loan, References: loadMaskedReferences(loan.ID)}
}

// loadMaskedReferences fetches and masks the references for a loan, for
// inclusion in customer/admin/employee loan-detail responses.
func loadMaskedReferences(loanID uint) []MaskedReference {
	var refs []models.LoanReference
	database.DB.Where("loan_id = ?", loanID).Order("id asc").Find(&refs)
	out := make([]MaskedReference, 0, len(refs))
	for _, r := range refs {
		out = append(out, MaskedReference{
			ID:            r.ID,
			Name:          r.Name,
			Relation:      r.Relation,
			Phone:         r.Phone,
			AadhaarMasked: r.AadhaarMasked,
			PANMasked:     security.MaskPAN(r.PANLast4),
		})
	}
	return out
}

// getOrCreateCibilScore returns the customer's stored CIBIL score,
// generating a deterministic mock score (550-850) the first time — seeded
// from the customer's UUID so it's stable across every application they
// ever submit, not re-randomized each time.
func getOrCreateCibilScore(user *models.User) int {
	if user.CibilScore > 0 {
		return user.CibilScore
	}
	score := deterministicCibil(user.ID)
	user.CibilScore = score
	database.DB.Model(user).Update("cibil_score", score)
	return score
}

// deterministicCibil hashes the customer's UUID (FNV-1a, fast + stable)
// into the 550-850 CIBIL range.
func deterministicCibil(id uuid.UUID) int {
	h := fnv.New32a()
	h.Write([]byte(id.String()))
	sum := h.Sum32()
	return 550 + int(sum%301) // 550..850 inclusive
}
