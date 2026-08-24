package risk

import (
	"errors"
	"time"

	"gorm.io/gorm"

	"github.com/santosh0921/nbfc-backend/internal/audit"
	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/models"
)

var ErrNoApprovedRule = errors.New("risk: no APPROVED classification rule covers this loan/DPD — leaving classification unchanged rather than guessing")

// findApprovedClassificationRule picks the best-matching APPROVED rule for
// (product, loanType, dpd) as of asOf — a DRAFT or RETIRED rule is never
// eligible, satisfying "unapproved rule cannot be used in production"
// (item D / test #7). Product/loan-type-specific rules are preferred over
// a blanket (empty-field) rule; among ties, the most recent EffectiveDate
// wins, so a newer rule version supersedes an older one automatically.
func findApprovedClassificationRule(product, loanType string, dpd int, asOf time.Time) (*models.ClassificationRule, error) {
	var candidates []models.ClassificationRule
	err := database.DB.Where("status = ? AND min_dpd <= ? AND max_dpd >= ? AND effective_date <= ?",
		models.ClassificationStatusApproved, dpd, dpd, asOf).
		Where("product = ? OR product = ''", product).
		Where("loan_type = ? OR loan_type = ''", loanType).
		Order("effective_date DESC").
		Find(&candidates).Error
	if err != nil {
		return nil, err
	}
	if len(candidates) == 0 {
		return nil, nil
	}
	// Prefer the most specific match: product+loanType > product only >
	// loanType only > blanket.
	best := candidates[0]
	bestScore := specificityScore(best, product, loanType)
	for _, c := range candidates[1:] {
		if s := specificityScore(c, product, loanType); s > bestScore {
			best, bestScore = c, s
		}
	}
	return &best, nil
}

func specificityScore(r models.ClassificationRule, product, loanType string) int {
	score := 0
	if r.Product != "" && r.Product == product {
		score += 2
	}
	if r.LoanType != "" && r.LoanType == loanType {
		score += 1
	}
	return score
}

// ClassifyLoan (item D/E) determines and, if it differs from the current
// stored classification, records a new asset classification for a loan
// based on its current DPD. If no APPROVED rule covers this loan's
// DPD/product/loanType, the loan's classification is left exactly as it
// was — this function never invents a default (e.g. it does NOT assume
// "no rule = STANDARD"), per the instruction not to invent regulatory
// outcomes. Returns the resulting classification (possibly unchanged) and
// ErrNoApprovedRule if nothing matched (not treated as a fatal error by
// callers — see job.go).
func ClassifyLoan(loanID uint, product, loanType string, dpd int, asOf time.Time, actor string) (string, error) {
	rule, err := findApprovedClassificationRule(product, loanType, dpd, asOf)
	if err != nil {
		return "", err
	}

	var current models.LoanClassification
	hasCurrent := database.DB.First(&current, "loan_id = ?", loanID).Error == nil

	if rule == nil {
		if hasCurrent {
			return current.Classification, ErrNoApprovedRule
		}
		return "", ErrNoApprovedRule
	}

	if hasCurrent && current.Classification == rule.Classification {
		return current.Classification, nil // no change — no new history row needed
	}

	oldClassification := ""
	if hasCurrent {
		oldClassification = current.Classification
	}

	err = database.DB.Transaction(func(tx *gorm.DB) error {
		history := models.LoanClassificationHistory{
			LoanID: loanID, OldClassification: oldClassification, NewClassification: rule.Classification,
			EffectiveDate: asOf, Reason: "DPD " + itoaInt(dpd) + " matched rule " + rule.RuleVersion,
			RuleVersion: rule.RuleVersion, CalculatedBy: actor,
		}
		if err := tx.Create(&history).Error; err != nil {
			return err
		}

		result := models.LoanClassification{
			LoanID: loanID, Classification: rule.Classification, RuleVersion: rule.RuleVersion,
			CalculatedAt: time.Now(), CalculatedBy: actor,
		}
		return tx.Save(&result).Error
	})
	if err != nil {
		return "", err
	}

	audit.RecordDetailed(nil, audit.Entry{
		ActorType: actorTypeFor(actor), ActorRef: actor,
		Action: "risk.classification.changed", TargetType: "loan", TargetID: itoa(loanID),
		Result: audit.ResultSuccess,
		Details: oldClassification + " -> " + rule.Classification + " (rule " + rule.RuleVersion + ")",
	})

	return rule.Classification, nil
}

// ManualOverrideClassification (item D: "never allow a normal
// credit/operations user to manually change classification without
// appropriate authorization" — the caller/HTTP handler enforces the
// permission check; this function enforces that every override records a
// reason and an approver, which the automated path leaves blank).
func ManualOverrideClassification(loanID uint, newClassification, reason, actor, approvedBy string) error {
	if reason == "" || approvedBy == "" {
		return errors.New("risk: a manual classification override requires both a reason and an approver")
	}
	var current models.LoanClassification
	hasCurrent := database.DB.First(&current, "loan_id = ?", loanID).Error == nil
	oldClassification := ""
	if hasCurrent {
		oldClassification = current.Classification
	}

	err := database.DB.Transaction(func(tx *gorm.DB) error {
		history := models.LoanClassificationHistory{
			LoanID: loanID, OldClassification: oldClassification, NewClassification: newClassification,
			EffectiveDate: time.Now(), Reason: reason, CalculatedBy: actor, ApprovedBy: approvedBy,
		}
		if err := tx.Create(&history).Error; err != nil {
			return err
		}
		result := models.LoanClassification{
			LoanID: loanID, Classification: newClassification,
			CalculatedAt: time.Now(), CalculatedBy: actor,
		}
		return tx.Save(&result).Error
	})
	if err != nil {
		return err
	}

	audit.RecordDetailed(nil, audit.Entry{
		ActorType: "admin", ActorRef: actor,
		Action: "risk.classification.manual_override", TargetType: "loan", TargetID: itoa(loanID),
		Result: audit.ResultSuccess, Reason: reason,
		Details: oldClassification + " -> " + newClassification + ", approved by " + approvedBy,
	})
	return nil
}

func actorTypeFor(actor string) string {
	if actor == "system" || actor == "" {
		return "system"
	}
	return "admin"
}
