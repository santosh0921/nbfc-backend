// Package tax implements the Phase 10 TDS/tax-rule engine.
//
// NOTHING in this package hard-codes a tax rate, threshold, section
// number, form name, or due date. internal/tax/seed.go seeds ZERO
// TDSRule rows — every rule is entered by an authorized admin, requires
// a non-empty SourceReference (item Y: verified against an official
// source) before it can be drafted, and requires a different admin's
// approval (maker≠checker, same pattern as every prior phase's rule
// engines — Phase 5 classification/provisioning, Phase 6 regulatory
// report definitions) before CalculateTDS will ever select it. Until an
// admin configures and approves at least one rule, this engine correctly
// produces NO TDS calculations for anything — that is the intended,
// safe default, not a bug.
package tax

import (
	"errors"
	"time"

	"github.com/santosh0921/nbfc-backend/internal/audit"
	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/models"
)

var (
	ErrSourceRequired = errors.New("tax: a sourceReference (item Y — the official source this rule was verified against) is required before a rule can be drafted")
	ErrSameActor      = errors.New("tax: the admin who drafted this rule cannot also approve it")
	ErrNotDraft       = errors.New("tax: only a DRAFT rule can be approved/retired from draft")
)

// DraftRule — requires SourceReference (item Y). Never itself usable by
// CalculateTDS until a different admin approves it.
func DraftRule(rule *models.TDSRule, createdBy string) (*models.TDSRule, error) {
	if rule.SourceReference == "" {
		return nil, ErrSourceRequired
	}
	if rule.TaxAct == "" || rule.SectionRef == "" || rule.NatureOfPayment == "" || rule.RuleVersion == "" {
		return nil, errors.New("tax: taxAct, sectionRef, natureOfPayment, and ruleVersion are required")
	}
	if rule.EffectiveFrom.IsZero() {
		return nil, errors.New("tax: effectiveFrom is required")
	}
	rule.Status = models.TDSRuleStatusDraft
	rule.CreatedBy = createdBy
	rule.ApprovedBy = ""
	rule.ApprovedAt = nil
	if err := database.DB.Create(rule).Error; err != nil {
		return nil, err
	}
	audit.RecordDetailed(nil, audit.Entry{
		ActorType: "admin", ActorRef: createdBy,
		Action: "tax.rule.drafted", TargetType: "tds_rule", TargetID: itoa(rule.ID),
		Result: audit.ResultSuccess, Details: rule.NatureOfPayment + " / " + rule.SectionRef + " / " + rule.RuleVersion,
	})
	return rule, nil
}

// ApproveRule — maker != checker.
func ApproveRule(ruleID uint, approvedByAdminID uint, approvedByName string) (*models.TDSRule, error) {
	var rule models.TDSRule
	if err := database.DB.First(&rule, ruleID).Error; err != nil {
		return nil, err
	}
	if rule.Status != models.TDSRuleStatusDraft {
		return nil, ErrNotDraft
	}
	if rule.CreatedBy == approvedByName {
		return nil, ErrSameActor
	}
	now := time.Now()
	rule.Status = models.TDSRuleStatusApproved
	rule.ApprovedBy = approvedByName
	rule.ApprovedAt = &now
	if err := database.DB.Save(&rule).Error; err != nil {
		return nil, err
	}
	audit.RecordDetailed(nil, audit.Entry{
		ActorType: "admin", ActorID: approvedByAdminID, ActorRef: approvedByName,
		Action: "tax.rule.approved", TargetType: "tds_rule", TargetID: itoa(rule.ID),
		Result: audit.ResultSuccess,
	})
	return &rule, nil
}

func RetireRule(ruleID uint, actor string) error {
	if err := database.DB.Model(&models.TDSRule{}).Where("id = ?", ruleID).Update("status", models.TDSRuleStatusRetired).Error; err != nil {
		return err
	}
	audit.RecordDetailed(nil, audit.Entry{
		ActorType: "admin", ActorRef: actor,
		Action: "tax.rule.retired", TargetType: "tds_rule", TargetID: itoa(ruleID),
		Result: audit.ResultSuccess,
	})
	return nil
}

// SelectApplicableRule (item B/O) — the single lookup every TDS
// calculation goes through. Selects the APPROVED rule for
// natureOfPayment whose [EffectiveFrom, EffectiveTo) window contains
// transactionDate. Never falls back to "the newest rule regardless of
// date" — a rule whose window doesn't cover transactionDate is not a
// match, full stop, even if it's the only rule configured. Returns nil
// (not an error) when no rule matches — the caller must treat that as
// "cannot determine applicability," never as "not applicable."
func SelectApplicableRule(natureOfPayment string, transactionDate time.Time) (*models.TDSRule, error) {
	var rules []models.TDSRule
	q := database.DB.Where("status = ? AND nature_of_payment = ? AND effective_from <= ?",
		models.TDSRuleStatusApproved, natureOfPayment, transactionDate).
		Where("effective_to IS NULL OR effective_to > ?", transactionDate).
		Order("effective_from DESC")
	if err := q.Find(&rules).Error; err != nil {
		return nil, err
	}
	if len(rules) == 0 {
		return nil, nil
	}
	return &rules[0], nil
}
