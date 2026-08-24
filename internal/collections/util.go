package collections

import (
	"strconv"

	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/models"
)

func itoa(v uint) string    { return strconv.FormatUint(uint64(v), 10) }
func itoaInt(v int) string  { return strconv.Itoa(v) }

// CreateEscalationAlert/CreateBrokenPTPAlert write directly to
// models.RiskAlert rather than depending on internal/risk (which itself
// depends on internal/collections for TouchCase) — avoids an import
// cycle while keeping every internal operational alert in the same
// table/admin screen regardless of which package raised it.
func CreateEscalationAlert(loanID, caseID uint) {
	database.DB.Create(&models.RiskAlert{
		AlertType: "COLLECTION_ESCALATION", Severity: models.AlertSeverityWarning,
		LoanID: &loanID, Message: "Collection case #" + itoa(caseID) + " for loan #" + itoa(loanID) + " was escalated",
		Status: models.AlertStatusOpen,
	})
}

func CreateBrokenPTPAlert(loanID, promiseID uint) {
	database.DB.Create(&models.RiskAlert{
		AlertType: "BROKEN_PTP", Severity: models.AlertSeverityWarning,
		LoanID: &loanID, Message: "Promise-to-pay #" + itoa(promiseID) + " for loan #" + itoa(loanID) + " was broken",
		Status: models.AlertStatusOpen,
	})
}
