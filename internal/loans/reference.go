package loans

import (
	"github.com/santosh0921/nbfc-backend/internal/bureau"
	"github.com/santosh0921/nbfc-backend/internal/config"
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

// getOrCreateCibilScore returns the customer's credit score via
// internal/bureau (Phase 3) — the deterministic mock generator that used
// to live directly in this function has moved to
// internal/bureau.sandboxProvider, reachable only through
// bureau.PullScore's consent-and-audit path, and only outside production
// (config.IsProduction()). In production with no real bureau configured,
// this now returns an error instead of silently fabricating a score —
// every caller must handle that by failing closed (see
// customer_handler.go), not by proceeding with a zero/default score.
func getOrCreateCibilScore(user *models.User) (int, error) {
	if user.CibilScore > 0 {
		return user.CibilScore, nil
	}

	bureauName := models.BureauCIBIL
	if !config.IsProduction() {
		bureauName = models.BureauSandbox
		// Outside production there is no customer-facing consent-capture
		// screen wired up yet (see the Phase 3 report) — record an
		// explicit, clearly-labeled implicit consent so the sandbox flow
		// keeps working without silently skipping the consent
		// requirement altogether, and so the audit trail is honest about
		// what actually happened.
		bureau.RecordConsent(user.ID, bureauName, "loan underwriting", "v0-implicit-preconsent-flow", "", "", "")
	}

	req, err := bureau.PullScore(user.ID, bureauName)
	if err != nil {
		return 0, err
	}

	user.CibilScore = req.Score
	database.DB.Model(user).Update("cibil_score", req.Score)
	return req.Score, nil
}
