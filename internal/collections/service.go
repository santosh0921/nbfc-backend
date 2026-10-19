// Package collections implements the Phase 5 delinquency-driven
// collections workflow (item I/J/K) — operational states layered on top
// of the risk package's DPD/classification facts, never a substitute for
// them. A CollectionCase's Status (CURRENT/REMINDER/OVERDUE/...) is purely
// operational; it is not, and must never be read as, a regulatory asset
// classification (that's models.LoanClassification, owned by
// internal/risk).
package collections

import (
	"errors"
	"time"

	"github.com/santosh0921/nbfc-backend/internal/audit"
	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/models"
)

// TouchCase opens a new CollectionCase for a delinquent loan, or updates
// the DPD/overdue-amount snapshot on its existing OPEN case — called by
// internal/risk's daily job after DPD is calculated, never by collection
// staff directly (they can only act on an existing case, not conjure one
// for a current loan — see handler.go's permission gating).
func TouchCase(loanID uint, customerID string, dpd int, overdueAmountPaise int64) (*models.CollectionCase, error) {
	var existing models.CollectionCase
	err := database.DB.Where("loan_id = ? AND status NOT IN ?", loanID,
		[]string{models.CollectionStatusClosed, models.CollectionStatusResolved}).
		Order("id desc").First(&existing).Error
	if err == nil {
		existing.DPD = dpd
		existing.OverdueAmountPaise = overdueAmountPaise
		if existing.Status == models.CollectionStatusCurrent && dpd > 0 {
			existing.Status = statusForDPD(dpd)
		}
		database.DB.Save(&existing)
		return &existing, nil
	}

	newCase := models.CollectionCase{
		LoanID: loanID, CustomerID: customerID, DPD: dpd, OverdueAmountPaise: overdueAmountPaise,
		Status: statusForDPD(dpd),
	}
	if err := database.DB.Create(&newCase).Error; err != nil {
		return nil, err
	}
	audit.RecordDetailed(nil, audit.Entry{
		ActorType: "system", ActorRef: systemActorRiskJob,
		Action: "collection.case.opened", TargetType: "collection_case", TargetID: itoa(newCase.ID),
		Result: audit.ResultSuccess, Details: "loan #" + itoa(loanID) + ", DPD " + itoaInt(dpd),
	})
	return &newCase, nil
}

// AutoResolveIfCaughtUp closes a loan's open case when the daily risk job
// finds its DPD has returned to 0 — the counterpart to TouchCase, which
// only ever opens/updates a case while DPD > 0 and never runs once the
// borrower catches up. Silent no-op when there's no open case to resolve
// (the common case — most loans never have one).
func AutoResolveIfCaughtUp(loanID uint) {
	var existing models.CollectionCase
	err := database.DB.Where("loan_id = ? AND status NOT IN ?", loanID,
		[]string{models.CollectionStatusClosed, models.CollectionStatusResolved}).
		Order("id desc").First(&existing).Error
	if err != nil {
		return
	}
	ResolveCase(existing.ID, 0, "", "Account caught up — DPD returned to 0")
}

// statusForDPD is a simple, explicitly operational (not regulatory)
// mapping from DPD to an initial collection-workflow stage — distinct
// from, and much coarser than, the DelinquencyBucket/ClassificationRule
// configuration in internal/risk. Ops can move a case through the rest of
// the workflow manually regardless of what this starting point picks.
func statusForDPD(dpd int) string {
	switch {
	case dpd <= 0:
		return models.CollectionStatusCurrent
	case dpd <= 7:
		return models.CollectionStatusReminder
	case dpd <= 30:
		return models.CollectionStatusOverdue
	default:
		return models.CollectionStatusAssigned
	}
}

var ErrUnauthorizedAction = errors.New("collections: this action requires collection-manager authorization")

// AssignCase (item I/L) — collection managers assign/reassign; the caller
// (HTTP handler) enforces the rbac.CollectionManage permission before
// calling this.
func AssignCase(caseID, assignToAdminID uint, assignToName string, assignedByAdminID uint, assignedByName, reason string) (*models.CollectionCase, error) {
	var c models.CollectionCase
	if err := database.DB.First(&c, caseID).Error; err != nil {
		return nil, err
	}
	c.AssignedOfficerAdminID = &assignToAdminID
	c.AssignedOfficerName = assignToName
	if c.Status == models.CollectionStatusOverdue || c.Status == models.CollectionStatusReminder {
		c.Status = models.CollectionStatusAssigned
	}
	if err := database.DB.Save(&c).Error; err != nil {
		return nil, err
	}
	database.DB.Create(&models.CollectionAssignment{
		CaseID: caseID, AssignedToAdminID: assignToAdminID, AssignedToName: assignToName,
		AssignedByAdminID: assignedByAdminID, AssignedByName: assignedByName, Reason: reason,
	})
	audit.RecordDetailed(nil, audit.Entry{
		ActorType: "admin", ActorID: assignedByAdminID, ActorRef: assignedByName,
		Action: "collection.assigned", TargetType: "collection_case", TargetID: itoa(caseID),
		Result: audit.ResultSuccess, Reason: reason, Details: "assigned to " + assignToName,
	})
	return &c, nil
}

// EscalateCase — item I. Any authorized collection actor (officer or
// manager) can escalate; only a manager can assign, per item L.
func EscalateCase(caseID uint, actorAdminID uint, actorName, reason string) (*models.CollectionCase, error) {
	var c models.CollectionCase
	if err := database.DB.First(&c, caseID).Error; err != nil {
		return nil, err
	}
	c.Status = models.CollectionStatusEscalated
	if err := database.DB.Save(&c).Error; err != nil {
		return nil, err
	}
	database.DB.Create(&models.CollectionAction{
		CaseID: caseID, ActorAdminID: actorAdminID, ActorName: actorName,
		ActionType: models.ActionTypeEscalation, Result: "escalated", Notes: reason,
	})
	audit.RecordDetailed(nil, audit.Entry{
		ActorType: "admin", ActorID: actorAdminID, ActorRef: actorName,
		Action: "collection.escalated", TargetType: "collection_case", TargetID: itoa(caseID),
		Result: audit.ResultSuccess, Reason: reason,
	})
	CreateEscalationAlert(c.LoanID, caseID)
	return &c, nil
}

// systemActorRiskJob is the ActorRef/actor name recorded when the daily
// risk job (not a human admin) drives a collections state change.
const systemActorRiskJob = "risk-job"

// ErrCaseAlreadyClosed guards both ResolveCase and the auto-resolve path in
// internal/risk's daily job against double-resolving (or resolving an
// already-written-off) case.
var ErrCaseAlreadyClosed = errors.New("collections: case is already closed")

// ResolveCase closes a case once the account is no longer delinquent —
// either a collection manager doing it explicitly (actorAdminID > 0,
// resolution text required) or the daily risk job doing it automatically
// when a loan's DPD returns to 0 (actorAdminID == 0, "system" actor). This
// is the only path that can ever move a CollectionCase into CLOSED/
// RESOLVED — before this existed, a case had no way back out of OVERDUE/
// ASSIGNED/ESCALATED once opened, even after the borrower paid in full.
func ResolveCase(caseID uint, actorAdminID uint, actorName, resolution string) (*models.CollectionCase, error) {
	var c models.CollectionCase
	if err := database.DB.First(&c, caseID).Error; err != nil {
		return nil, err
	}
	if c.Status == models.CollectionStatusClosed || c.Status == models.CollectionStatusResolved {
		return nil, ErrCaseAlreadyClosed
	}
	now := time.Now()
	c.Status = models.CollectionStatusResolved
	c.Resolution = resolution
	c.ResolvedAt = &now
	if err := database.DB.Save(&c).Error; err != nil {
		return nil, err
	}
	actorType := "admin"
	if actorAdminID == 0 {
		actorType = "system"
		actorName = systemActorRiskJob
	} else {
		database.DB.Create(&models.CollectionAction{
			CaseID: caseID, ActorAdminID: actorAdminID, ActorName: actorName,
			ActionType: models.ActionTypeResolution, Result: "resolved", Notes: resolution,
		})
	}
	audit.RecordDetailed(nil, audit.Entry{
		ActorType: actorType, ActorID: actorAdminID, ActorRef: actorName,
		Action: "collection.resolved", TargetType: "collection_case", TargetID: itoa(caseID),
		Result: audit.ResultSuccess, Reason: resolution,
	})
	return &c, nil
}

// RecordAction (item J) — logs one collection interaction. Never touches
// any financial/accounting field — a collection officer's action is
// purely informational/workflow, per item I: "Do not allow collection
// staff to modify accounting balances directly."
func RecordAction(caseID, actorAdminID uint, actorName, actionType, result, notes, communicationRef string, nextAction *time.Time) (*models.CollectionAction, error) {
	action := models.CollectionAction{
		CaseID: caseID, ActorAdminID: actorAdminID, ActorName: actorName,
		ActionType: actionType, Result: result, Notes: notes,
		NextAction: nextAction, CommunicationRef: communicationRef,
	}
	if err := database.DB.Create(&action).Error; err != nil {
		return nil, err
	}
	if nextAction != nil {
		database.DB.Model(&models.CollectionCase{}).Where("id = ?", caseID).Update("next_action_date", nextAction)
	}
	audit.RecordDetailed(nil, audit.Entry{
		ActorType: "admin", ActorID: actorAdminID, ActorRef: actorName,
		Action: "collection.action", TargetType: "collection_case", TargetID: itoa(caseID),
		Result: audit.ResultSuccess, Details: actionType + ": " + result,
	})
	return &action, nil
}

// CreatePromise (item K) — recording a promise never itself settles
// anything; only CheckPromiseAfterPayment (called from the verified
// payment path) can mark one PAID.
func CreatePromise(caseID, loanID uint, promisedAmountPaise int64, promisedDate time.Time, createdByAdminID uint, createdByName string) (*models.CollectionPromise, error) {
	if promisedAmountPaise <= 0 {
		return nil, errors.New("collections: a promise must have a positive promised amount")
	}
	promise := models.CollectionPromise{
		CaseID: caseID, LoanID: loanID, PromisedAmountPaise: promisedAmountPaise, PromisedDate: promisedDate,
		Status: models.PTPStatusOpen, CreatedByAdminID: createdByAdminID, CreatedByName: createdByName,
	}
	if err := database.DB.Create(&promise).Error; err != nil {
		return nil, err
	}
	database.DB.Create(&models.CollectionAction{
		CaseID: caseID, ActorAdminID: createdByAdminID, ActorName: createdByName,
		ActionType: models.ActionTypePTP, Result: "promised", NextAction: &promisedDate,
	})
	audit.RecordDetailed(nil, audit.Entry{
		ActorType: "admin", ActorID: createdByAdminID, ActorRef: createdByName,
		Action: "collection.ptp.created", TargetType: "collection_promise", TargetID: itoa(promise.ID),
		Result: audit.ResultSuccess, Details: "loan #" + itoa(loanID),
	})
	return &promise, nil
}

// CheckPromiseAfterPayment (item K: "Only verified payment transactions
// can settle the loan") — called after a verified EMI payment; marks the
// oldest OPEN promise for the loan PAID if the payment covers it. Never
// invoked from a timer or a customer claim.
func CheckPromiseAfterPayment(loanID uint, paymentID uint, paidAmountPaise int64) {
	var promise models.CollectionPromise
	err := database.DB.Where("loan_id = ? AND status = ?", loanID, models.PTPStatusOpen).
		Order("promised_date ASC").First(&promise).Error
	if err != nil {
		return
	}
	if paidAmountPaise < promise.PromisedAmountPaise {
		return // partial payment doesn't auto-settle a promise
	}
	now := time.Now()
	promise.Status = models.PTPStatusPaid
	promise.SettledPaymentID = &paymentID
	promise.ResolvedAt = &now
	database.DB.Save(&promise)
	audit.RecordDetailed(nil, audit.Entry{
		ActorType: "system", ActorRef: "payment-webhook",
		Action: "collection.ptp.paid", TargetType: "collection_promise", TargetID: itoa(promise.ID),
		Result: audit.ResultSuccess, Details: "settled by payment #" + itoa(paymentID),
	})
}

// MarkBrokenPromises (called from the daily risk job) flags every OPEN
// promise whose PromisedDate has passed as BROKEN and raises an alert —
// a promise is never silently left OPEN past its date, and never
// auto-marked PAID by the mere passage of time.
func MarkBrokenPromises(asOf time.Time) int {
	var promises []models.CollectionPromise
	database.DB.Where("status = ? AND promised_date < ?", models.PTPStatusOpen, asOf).Find(&promises)
	for _, p := range promises {
		now := time.Now()
		p.Status = models.PTPStatusBroken
		p.ResolvedAt = &now
		database.DB.Save(&p)
		audit.RecordDetailed(nil, audit.Entry{
			ActorType: "system", ActorRef: systemActorRiskJob,
			Action: "collection.ptp.broken", TargetType: "collection_promise", TargetID: itoa(p.ID),
			Result: audit.ResultSuccess, Details: "loan #" + itoa(p.LoanID),
		})
		CreateBrokenPTPAlert(p.LoanID, p.ID)
	}
	return len(promises)
}
