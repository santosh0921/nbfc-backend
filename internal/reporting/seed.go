package reporting

import (
	"log"

	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/models"
)

// SeedRetentionPolicyPlaceholders creates an empty, UNVALIDATED
// RetentionPolicy row (RetentionYears: 0, no legal basis note) for each
// record category item M names — purely so the admin Retention screen has
// something to configure rather than an empty table with no guidance on
// what categories need attention. Idempotent (skips a category that
// already has a row). NO retention period is invented here — every row's
// RetentionYears stays 0 and Status stays UNVALIDATED until compliance
// enters and validates a real figure.
//
// This function deliberately does NOT seed any RegulatoryReportDefinition
// — unlike retention categories (which are a known, fixed list item M
// names), a specific regulatory RETURN is something only the NBFC's
// compliance officer/CA can identify as applicable; inventing even a
// placeholder row for one would risk being mistaken for a real
// requirement.
func SeedRetentionPolicyPlaceholders() {
	categories := []string{
		"kyc_documents", "loan_documents", "payment_records", "accounting_records",
		"audit_logs", "regulatory_reports", "credit_bureau_records", "customer_communications",
	}
	for _, category := range categories {
		var existing models.RetentionPolicy
		if err := database.DB.Where("record_category = ?", category).First(&existing).Error; err == nil {
			continue
		}
		database.DB.Create(&models.RetentionPolicy{
			RecordCategory: category, RetentionYears: 0, Status: models.RetentionStatusUnvalidated,
		})
	}
	log.Println("✅ Retention policy placeholders seeded/verified (all UNVALIDATED, 0 years — compliance must configure real periods)")
}
