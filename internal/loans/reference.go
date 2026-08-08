package loans

import (
	"hash/fnv"

	"github.com/google/uuid"

	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/models"
	"github.com/santosh0921/nbfc-backend/internal/security"
)

// MaskedReference is the shape returned in any API response — raw Aadhaar/
// PAN values are never included, only masked forms.
type MaskedReference struct {
	ID            uint   `json:"id"`
	Name          string `json:"name"`
	Relation      string `json:"relation"`
	Phone         string `json:"phone"`
	AadhaarMasked string `json:"aadhaarMasked"`
	PANMasked     string `json:"panMasked"`
}

// LoanResponse wraps a LoanApplication with its masked references for any
// endpoint that returns full loan detail (customer/admin/employee).
//
// References are no longer collected on application/top-up — this stays
// in the response shape only so existing clients that read `references`
// keep getting a (now always empty) array instead of a missing field.
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
