package risk

import (
	"errors"
	"math"
	"time"

	"github.com/santosh0921/nbfc-backend/internal/accounting"
	"github.com/santosh0921/nbfc-backend/internal/audit"
	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/models"
)

var ErrNoApprovedProvisioningRule = errors.New("risk: no APPROVED provisioning rule covers this classification — no provision was calculated")

// findApprovedProvisioningRule mirrors findApprovedClassificationRule's
// approval-gating — a DRAFT/RETIRED rule is never usable.
func findApprovedProvisioningRule(classification string, asOf time.Time) (*models.ProvisioningRule, error) {
	var rule models.ProvisioningRule
	err := database.DB.Where("classification = ? AND status = ? AND effective_date <= ?",
		classification, models.ClassificationStatusApproved, asOf).
		Order("effective_date DESC").First(&rule).Error
	if err != nil {
		return nil, nil
	}
	return &rule, nil
}

// outstandingPrincipalPaise sums the PrincipalComponent of every not-yet-
// paid installment for the loan — the exposure base for provisioning,
// same convention as Phase 4's interest-accrual outstanding-principal
// calculation (kept as a local duplicate rather than a cross-package
// export, since the two packages' notion of "outstanding" could
// legitimately diverge later, e.g. once Phase 5 folds in overdue
// interest/charges).
func outstandingPrincipalPaise(loanID uint) int64 {
	var total float64
	database.DB.Model(&models.EmiInstallment{}).
		Where("loan_id = ? AND status != ?", loanID, models.EmiStatusPaid).
		Select("COALESCE(SUM(principal_component), 0)").Scan(&total)
	if total < 0 {
		total = 0
	}
	return toPaise(total)
}

// CalculateProvision (item G/H) — computes and, if a rate is configured
// and approved, posts the provisioning journal entry via
// accounting.Service for one loan as of asOf. Idempotent per (loan,
// calculation date) via the DB's unique index; if classification changed
// since the last calculation, the previous ACTIVE provision for this loan
// is reversed (a new accounting reversal entry, never an edit to the old
// journal) before the new one posts.
//
// Returns (nil, ErrNoApprovedProvisioningRule) — not a fatal error to
// callers — when no APPROVED ProvisioningRule exists for this
// classification; this is the expected state until compliance configures
// and approves rates (see the Phase 5 report).
func CalculateProvision(loanID uint, classification string, asOf time.Time, actor string) (*models.LoanProvision, error) {
	asOf = time.Date(asOf.Year(), asOf.Month(), asOf.Day(), 0, 0, 0, 0, asOf.Location())

	var existing models.LoanProvision
	if err := database.DB.Where("loan_id = ? AND calculation_date = ?", loanID, asOf).First(&existing).Error; err == nil {
		return &existing, nil // already calculated today — idempotent no-op (item U #10)
	}

	rule, err := findApprovedProvisioningRule(classification, asOf)
	if err != nil {
		return nil, err
	}

	var previous models.LoanProvision
	hasPrevious := database.DB.Where("loan_id = ? AND status = ?", loanID, models.ProvisionStatusActive).
		Order("calculation_date DESC").First(&previous).Error == nil

	if rule == nil {
		// No approved rate for this classification — nothing to post. If a
		// previous ACTIVE provision exists (e.g. the loan improved from a
		// classification that HAD a rate to one that doesn't), reverse it
		// rather than leaving a stale provision on the books.
		if hasPrevious {
			reverseProvision(previous, "classification changed to "+classification+"; no approved provisioning rule for it", actor)
		}
		return nil, ErrNoApprovedProvisioningRule
	}

	exposure := outstandingPrincipalPaise(loanID)
	provisionAmount := int64(math.Round(float64(exposure) * rule.ProvisionRatePercent / 100))

	if hasPrevious && previous.Classification == classification && previous.ProvisionAmountPaise == provisionAmount {
		// Nothing has actually changed since the last calculation — still
		// record today's row (for the audit trail of "we checked and
		// nothing moved"), but skip re-posting an identical journal.
		record := models.LoanProvision{
			LoanID: loanID, Classification: classification, ExposureBasePaise: exposure,
			ProvisionRatePercent: rule.ProvisionRatePercent, ProvisionAmountPaise: provisionAmount,
			RuleVersion: rule.RuleVersion, CalculationDate: asOf, Status: models.ProvisionStatusActive,
			JournalReference: previous.JournalReference,
		}
		database.DB.Create(&record)
		return &record, nil
	}

	if hasPrevious {
		reverseProvision(previous, "reclassified/recalculated: "+previous.Classification+" -> "+classification, actor)
	}

	record := models.LoanProvision{
		LoanID: loanID, Classification: classification, ExposureBasePaise: exposure,
		ProvisionRatePercent: rule.ProvisionRatePercent, ProvisionAmountPaise: provisionAmount,
		RuleVersion: rule.RuleVersion, CalculationDate: asOf, Status: models.ProvisionStatusActive,
	}

	if provisionAmount > 0 {
		entry, postErr := accounting.Service.PostJournalEntry(accounting.PostingInput{
			TransactionType: "NPA_PROVISION",
			ReferenceType:   "loan_provision",
			ReferenceID:     itoa(loanID) + ":" + asOf.Format("2006-01-02"),
			TransactionDate: asOf,
			Description:     "NPA provision, loan #" + itoa(loanID) + ", classification " + classification,
			CreatedBy:       actor,
			Lines: []accounting.LineInput{
				{AccountCode: accounting.AcctProvisionExpenseNPA, DebitPaise: provisionAmount, Description: "Provision expense"},
				{AccountCode: accounting.AcctProvisionForNPA, CreditPaise: provisionAmount, Description: "Provision for NPA"},
			},
		})
		if postErr != nil {
			record.Status = models.ProvisionStatusFailed
			database.DB.Create(&record)
			return &record, postErr
		}
		record.JournalReference = entry.JournalNumber
	}

	database.DB.Create(&record)

	audit.RecordDetailed(nil, audit.Entry{
		ActorType: actorTypeFor(actor), ActorRef: actor,
		Action: "risk.provision.calculated", TargetType: "loan", TargetID: itoa(loanID),
		Result: audit.ResultSuccess,
		Details: classification + ", rate " + itoaInt(int(rule.ProvisionRatePercent)) + "%, amount paise " + itoa(uint(provisionAmount)),
	})

	return &record, nil
}

// reverseProvision marks a previous ACTIVE provision REVERSED and posts
// the offsetting accounting entry (item H: "Old Provision -> Reversal ->
// New Provision"), never editing the original posted journal.
func reverseProvision(previous models.LoanProvision, reason, actor string) {
	if previous.JournalReference != "" {
		var journal models.JournalEntry
		if err := database.DB.Where("journal_number = ?", previous.JournalReference).First(&journal).Error; err == nil {
			accounting.ReverseJournalEntry(journal.ID, reason, actor)
		}
	}
	database.DB.Model(&models.LoanProvision{}).Where("id = ?", previous.ID).
		Update("status", models.ProvisionStatusReversed)

	audit.RecordDetailed(nil, audit.Entry{
		ActorType: actorTypeFor(actor), ActorRef: actor,
		Action: "risk.provision.reversed", TargetType: "loan", TargetID: itoa(previous.LoanID),
		Result: audit.ResultSuccess, Reason: reason,
	})
}
