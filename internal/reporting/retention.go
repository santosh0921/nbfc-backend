// Retention policy configuration (item M). This package NEVER deletes
// anything — RetentionPolicy is metadata only, and even a VALIDATED
// policy is not wired to any automated purge job. Deleting KYC documents,
// loan records, payment records, accounting records, audit logs,
// regulatory reports, bureau records, or customer communications is
// explicitly out of scope for this phase — item M says "do not
// automatically delete historical records," and no code path here does.
package reporting

import (
	"time"

	"github.com/santosh0921/nbfc-backend/internal/audit"
	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/models"
)

func ListRetentionPolicies() []models.RetentionPolicy {
	var policies []models.RetentionPolicy
	database.DB.Order("record_category").Find(&policies)
	return policies
}

// ConfigureRetentionPolicy — creates or updates the policy row for a
// record category. Starts (or is reset to) UNVALIDATED on every change;
// only a separate, explicit ValidateRetentionPolicy call (a distinct
// compliance action, never bundled into this one) can mark it VALIDATED —
// so a routine metadata edit can never accidentally imply legal sign-off.
func ConfigureRetentionPolicy(recordCategory string, retentionYears int, legalBasisNote, configuredBy string) (*models.RetentionPolicy, error) {
	var policy models.RetentionPolicy
	err := database.DB.Where("record_category = ?", recordCategory).First(&policy).Error
	if err != nil {
		policy = models.RetentionPolicy{RecordCategory: recordCategory}
	}
	policy.RetentionYears = retentionYears
	policy.LegalBasisNote = legalBasisNote
	policy.Status = models.RetentionStatusUnvalidated
	policy.ConfiguredBy = configuredBy
	policy.ConfiguredAt = time.Now()
	if err := database.DB.Save(&policy).Error; err != nil {
		return nil, err
	}
	audit.RecordDetailed(nil, audit.Entry{
		ActorType: "admin", ActorRef: configuredBy,
		Action: "reporting.retention.configured", TargetType: "retention_policy", TargetID: recordCategory,
		Result: audit.ResultSuccess, Details: itoaInt(retentionYears) + " years",
	})
	return &policy, nil
}

// ValidateRetentionPolicy — a compliance officer's explicit confirmation
// that a configured retention period has been checked against the
// applicable legal/regulatory requirement. This still does not enable any
// deletion — it only changes Status for reporting/audit purposes (item T:
// distinguishing CONFIGURED_NOT_VALIDATED from VALIDATED).
func ValidateRetentionPolicy(recordCategory, validatedBy string) error {
	err := database.DB.Model(&models.RetentionPolicy{}).Where("record_category = ?", recordCategory).
		Update("status", models.RetentionStatusValidated).Error
	if err != nil {
		return err
	}
	audit.RecordDetailed(nil, audit.Entry{
		ActorType: "admin", ActorRef: validatedBy,
		Action: "reporting.retention.validated", TargetType: "retention_policy", TargetID: recordCategory,
		Result: audit.ResultSuccess,
	})
	return nil
}
