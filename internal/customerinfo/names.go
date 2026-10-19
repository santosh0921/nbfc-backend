// Package customerinfo provides small, read-only helpers for enriching
// admin-facing list responses with a customer's real display name, batched
// (never N+1), and never fabricated — a customer with no CustomerProfile
// row simply gets no entry in the returned map, and callers must treat
// that as "name unknown", not invent one.
package customerinfo

import (
	"github.com/google/uuid"

	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/models"
)

// BatchNames looks up display names ("First Last") for every customer ID
// in ids, in one query. IDs with no CustomerProfile row are simply absent
// from the returned map.
func BatchNames(ids []uuid.UUID) map[uuid.UUID]string {
	out := make(map[uuid.UUID]string, len(ids))
	if len(ids) == 0 {
		return out
	}
	var profiles []models.CustomerProfile
	database.DB.Where("user_id IN ?", ids).Find(&profiles)
	for _, p := range profiles {
		name := p.FirstName
		if p.LastName != "" {
			if name != "" {
				name += " "
			}
			name += p.LastName
		}
		if name != "" {
			out[p.UserID] = name
		}
	}
	return out
}

// BatchNamesByStringID is BatchNames for callers whose customer ID column
// is stored as a plain string (e.g. models.CollectionCase.CustomerID)
// rather than uuid.UUID. Values that don't parse as a UUID are silently
// skipped (never crash a list endpoint over one bad row) and simply won't
// have a name in the result.
func BatchNamesByStringID(ids []string) map[string]string {
	uuids := make([]uuid.UUID, 0, len(ids))
	for _, s := range ids {
		if id, err := uuid.Parse(s); err == nil {
			uuids = append(uuids, id)
		}
	}
	byUUID := BatchNames(uuids)
	out := make(map[string]string, len(byUUID))
	for id, name := range byUUID {
		out[id.String()] = name
	}
	return out
}

// LoanApplicantNames looks up ApplicantName (already stored directly on
// LoanApplication — no join to CustomerProfile needed) for a batch of loan
// IDs, e.g. for enriching RiskAlert rows that reference a LoanID but no
// customer ID directly.
func LoanApplicantNames(loanIDs []uint) map[uint]string {
	out := make(map[uint]string, len(loanIDs))
	if len(loanIDs) == 0 {
		return out
	}
	var loans []models.LoanApplication
	database.DB.Select("id", "applicant_name").Where("id IN ?", loanIDs).Find(&loans)
	for _, l := range loans {
		if l.ApplicantName != "" {
			out[l.ID] = l.ApplicantName
		}
	}
	return out
}
