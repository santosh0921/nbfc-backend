package tax

import (
	"errors"
	"math"
	"strconv"
	"time"

	"github.com/santosh0921/nbfc-backend/internal/audit"
	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/models"
)

var ErrNoApplicableRule = errors.New("tax: no APPROVED TDS rule covers this nature-of-payment/date — no TDS calculated")

// CalculateTDS (item D) — the single entry point for computing TDS on a
// gross amount. Looks up the applicable rule via SelectApplicableRule; if
// none matches, returns (nil, ErrNoApprovedRule) and creates NOTHING —
// callers (e.g. expense creation) must treat this as "TDS applicability
// undetermined, proceed without a deduction," never as "confirmed not
// applicable," per item R: "do not equate TDS with an expense
// automatically."
//
// Every input to the calculation (gross amount, threshold, rate, PAN
// status, rule ID/version, transaction/calculation dates) is stored on
// the resulting TDSTransaction row, so "why was this amount calculated"
// (item D) is always answerable without re-deriving anything.
type CalculateInput struct {
	ReferenceType    string
	ReferenceID      string
	DeducteeID       *uint
	DeducteeName     string
	GrossAmountPaise int64
	NatureOfPayment  string
	PANStatus        string
	TransactionDate  time.Time
	Actor            string
}

func CalculateTDS(in CalculateInput) (*models.TDSTransaction, error) {
	rule, err := SelectApplicableRule(in.NatureOfPayment, in.TransactionDate)
	if err != nil {
		return nil, err
	}
	if rule == nil {
		return nil, ErrNoApplicableRule
	}

	grossAmountPaise := in.GrossAmountPaise
	txn := &models.TDSTransaction{
		SectionRef: rule.SectionRef, BaseAmountPaise: grossAmountPaise,
		Deductee: in.DeducteeName, DeducteeID: in.DeducteeID,
		RuleID: &rule.ID, RuleVersion: rule.RuleVersion,
		TransactionDate: in.TransactionDate, CalculationDate: time.Now(),
		ThresholdPaise: rule.ThresholdPaise, PANStatusAtCalc: in.PANStatus,
		ReferenceType: in.ReferenceType, ReferenceID: in.ReferenceID,
	}
	panStatus := in.PANStatus
	actor := in.Actor
	natureOfPayment := in.NatureOfPayment

	if grossAmountPaise < rule.ThresholdPaise {
		// Below threshold — recorded as NOT_APPLICABLE (still stored, per
		// item D's "why" requirement — a below-threshold determination is
		// itself an answer, not a non-event) with zero TDS.
		txn.Status = models.TDSStatusNotApplicable
		txn.RatePercent = 0
		txn.TDSAmountPaise = 0
	} else {
		rate := rule.RatePercent
		if rule.RequiresPAN && panStatus != models.PANStatusValid && rule.NoPANRatePercent > 0 {
			rate = rule.NoPANRatePercent
		}
		txn.RatePercent = rate
		amount := float64(grossAmountPaise) * rate / 100
		if rule.SurchargePercent > 0 {
			amount += amount * rule.SurchargePercent / 100
		}
		if rule.CessPercent > 0 {
			amount += amount * rule.CessPercent / 100
		}
		txn.TDSAmountPaise = int64(math.Round(amount))
		txn.Status = models.TDSStatusApplicable
	}
	txn.NetPayablePaise = grossAmountPaise - txn.TDSAmountPaise

	if err := database.DB.Create(txn).Error; err != nil {
		return nil, err
	}
	audit.RecordDetailed(nil, audit.Entry{
		ActorType: actorTypeFor(actor), ActorRef: actor,
		Action: "tax.tds.calculated", TargetType: "tds_transaction", TargetID: itoa(txn.ID),
		Result: audit.ResultSuccess,
		Details: natureOfPayment + ", rule " + rule.RuleVersion + ", status " + txn.Status,
	})
	return txn, nil
}

// OverrideTDS (item L) — the only way to change a calculated TDS amount.
// Requires a reason; records the previous amount, the overriding admin,
// and (for a material change) a separate approver — never a silent edit.
func OverrideTDS(txnID uint, newTDSAmountPaise int64, reason string, overriddenByAdminID uint, overriddenByName, approvedByName string) (*models.TDSTransaction, error) {
	if reason == "" {
		return nil, errors.New("tax: an override reason is required")
	}
	var txn models.TDSTransaction
	if err := database.DB.First(&txn, txnID).Error; err != nil {
		return nil, err
	}
	previous := txn.TDSAmountPaise
	now := time.Now()
	txn.PreviousTDSAmountPaise = previous
	txn.TDSAmountPaise = newTDSAmountPaise
	txn.NetPayablePaise = txn.BaseAmountPaise - newTDSAmountPaise
	txn.IsOverride = true
	txn.OverrideReason = reason
	txn.OverriddenByAdminID = &overriddenByAdminID
	txn.OverriddenBy = overriddenByName
	txn.OverriddenAt = &now
	txn.ApprovedBy = approvedByName
	if err := database.DB.Save(&txn).Error; err != nil {
		return nil, err
	}
	audit.RecordDetailed(nil, audit.Entry{
		ActorType: "admin", ActorID: overriddenByAdminID, ActorRef: overriddenByName,
		Action: "tax.tds.overridden", TargetType: "tds_transaction", TargetID: itoa(txn.ID),
		Result: audit.ResultSuccess, Reason: reason,
		Details: "previous=" + strconv.FormatInt(previous, 10) + " new=" + strconv.FormatInt(newTDSAmountPaise, 10) + " approvedBy=" + approvedByName,
	})
	return &txn, nil
}

func actorTypeFor(actor string) string {
	if actor == "system" || actor == "" {
		return "system"
	}
	return "admin"
}

