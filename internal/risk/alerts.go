package risk

import "github.com/santosh0921/nbfc-backend/internal/database"
import "github.com/santosh0921/nbfc-backend/internal/models"

// CreateAlert (item O) writes an INTERNAL operational alert row — never
// sends a customer-facing message itself; that stays entirely in
// internal/notifications, a deliberately separate system, so a risk event
// can never accidentally spam a customer just because ops needed to know
// about it.
func CreateAlert(alertType, severity string, loanID *uint, message string) {
	database.DB.Create(&models.RiskAlert{
		AlertType: alertType, Severity: severity, LoanID: loanID, Message: message,
		Status: models.AlertStatusOpen,
	})
}
