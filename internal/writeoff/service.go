// Package writeoff implements the Phase 5 controlled write-off/recovery
// framework (item Q). No ordinary admin can move a loan through this
// workflow — every state transition below is gated by the caller (the
// HTTP handler) checking rbac.WriteOffRequest/rbac.WriteOffApprove, with
// an explicit maker != checker rule enforced here as defense in depth,
// mirroring Phase 1's loan-sanction maker-checker pattern.
package writeoff

import (
	"errors"
	"time"

	"github.com/santosh0921/nbfc-backend/internal/accounting"
	"github.com/santosh0921/nbfc-backend/internal/audit"
	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/models"
)

var (
	ErrSameActor        = errors.New("writeoff: the admin who requested a write-off cannot also approve it")
	ErrNotPendingReview = errors.New("writeoff: request is not awaiting review")
	ErrNotApproved      = errors.New("writeoff: only an APPROVED write-off can be finalized")
)

// RequestWriteOff — the loan/payment history is never touched here, only
// a new WriteOffRequest row is created, PENDING an approval that hasn't
// happened yet.
func RequestWriteOff(loanID uint, amountPaise int64, reason string, requestedByAdminID uint, requestedByName string) (*models.WriteOffRequest, error) {
	if amountPaise <= 0 || reason == "" {
		return nil, errors.New("writeoff: a positive amount and a reason are required")
	}
	req := models.WriteOffRequest{
		LoanID: loanID, AmountPaise: amountPaise, Reason: reason,
		Status: models.WriteOffStatusReview, RequestedByAdminID: requestedByAdminID, RequestedByName: requestedByName,
		RequestedAt: time.Now(),
	}
	if err := database.DB.Create(&req).Error; err != nil {
		return nil, err
	}
	audit.RecordDetailed(nil, audit.Entry{
		ActorType: "admin", ActorID: requestedByAdminID, ActorRef: requestedByName,
		Action: "writeoff.requested", TargetType: "write_off_request", TargetID: itoa(req.ID),
		Result: audit.ResultSuccess, Reason: reason, Details: "loan #" + itoa(loanID),
	})
	return &req, nil
}

// ApproveWriteOff finalizes the write-off: marks WRITTEN_OFF and posts the
// accounting entry (a loss recognized against the receivable — see
// postWriteOffAccounting) — all in the same call, since a write-off
// approval and its financial effect must never be allowed to disagree
// about whether it happened.
func ApproveWriteOff(requestID, approvedByAdminID uint, approvedByName string) (*models.WriteOffRequest, error) {
	var req models.WriteOffRequest
	if err := database.DB.First(&req, requestID).Error; err != nil {
		return nil, err
	}
	if req.Status != models.WriteOffStatusReview {
		return nil, ErrNotPendingReview
	}
	if req.RequestedByAdminID == approvedByAdminID {
		return nil, ErrSameActor
	}

	entry, err := accounting.Service.PostJournalEntry(accounting.PostingInput{
		TransactionType: "WRITE_OFF",
		ReferenceType:   "write_off",
		ReferenceID:     itoa(req.ID),
		Description:     "Write-off, loan #" + itoa(req.LoanID) + ": " + req.Reason,
		CreatedBy:       approvedByName,
		ApprovedBy:      approvedByName,
		Lines: []accounting.LineInput{
			// Debiting the NPA provision account (rather than a separate
			// "bad debt expense" account) assumes the loan was already
			// fully provisioned before write-off — a reasonable default,
			// but the exact write-off accounting treatment (provision
			// utilization vs. a fresh expense) must be confirmed by the
			// NBFC's CA before this is relied on in production; see the
			// Phase 5 report.
			{AccountCode: accounting.AcctProvisionForNPA, DebitPaise: req.AmountPaise, Description: "Provision utilized for write-off"},
			{AccountCode: accounting.AcctLoanPrincipalReceivable, CreditPaise: req.AmountPaise, Description: "Principal receivable written off"},
		},
	})
	if err != nil {
		return nil, err
	}

	now := time.Now()
	req.Status = models.WriteOffStatusWrittenOff
	req.ApprovedByAdminID = &approvedByAdminID
	req.ApprovedByName = approvedByName
	req.ApprovedAt = &now
	req.JournalReference = entry.JournalNumber
	req.UpdatedAt = now
	if err := database.DB.Save(&req).Error; err != nil {
		return nil, err
	}

	audit.RecordDetailed(nil, audit.Entry{
		ActorType: "admin", ActorID: approvedByAdminID, ActorRef: approvedByName,
		Action: "writeoff.approved", TargetType: "write_off_request", TargetID: itoa(req.ID),
		Result: audit.ResultSuccess, Details: "journal " + entry.JournalNumber,
	})
	return &req, nil
}

// RejectWriteOff — the maker/checker split applies here too: whoever
// requested it cannot also reject their own request (defense in depth,
// same as approval).
func RejectWriteOff(requestID, rejectedByAdminID uint, rejectedByName, reason string) (*models.WriteOffRequest, error) {
	var req models.WriteOffRequest
	if err := database.DB.First(&req, requestID).Error; err != nil {
		return nil, err
	}
	if req.Status != models.WriteOffStatusReview {
		return nil, ErrNotPendingReview
	}
	if req.RequestedByAdminID == rejectedByAdminID {
		return nil, ErrSameActor
	}
	req.Status = models.WriteOffStatusRejected
	req.RejectionReason = reason
	req.UpdatedAt = time.Now()
	if err := database.DB.Save(&req).Error; err != nil {
		return nil, err
	}
	audit.RecordDetailed(nil, audit.Entry{
		ActorType: "admin", ActorID: rejectedByAdminID, ActorRef: rejectedByName,
		Action: "writeoff.rejected", TargetType: "write_off_request", TargetID: itoa(req.ID),
		Result: audit.ResultSuccess, Reason: reason,
	})
	return &req, nil
}

// RecordRecovery (item Q's RECOVERY_AFTER_WRITE_OFF state) — money
// recovered after write-off is booked back as income, never by editing
// the original write-off journal.
func RecordRecovery(writeOffID uint, amountPaise int64, reference string, recordedByAdminID uint, recordedByName string) (*models.RecoveryPayment, error) {
	var wo models.WriteOffRequest
	if err := database.DB.First(&wo, writeOffID).Error; err != nil {
		return nil, err
	}
	if wo.Status != models.WriteOffStatusWrittenOff && wo.Status != models.WriteOffStatusRecoveryAfter {
		return nil, ErrNotApproved
	}

	entry, err := accounting.Service.PostJournalEntry(accounting.PostingInput{
		TransactionType: "WRITE_OFF_RECOVERY",
		ReferenceType:   "recovery_payment",
		ReferenceID:     itoa(wo.ID) + ":" + time.Now().Format("20060102150405.000000000"),
		Description:     "Recovery after write-off, loan #" + itoa(wo.LoanID),
		CreatedBy:       recordedByName,
		Lines: []accounting.LineInput{
			{AccountCode: accounting.AcctBankCash, DebitPaise: amountPaise, Description: "Recovered cash"},
			{AccountCode: accounting.AcctPenaltyOtherIncome, CreditPaise: amountPaise, Description: "Recovery income (post-write-off)"},
		},
	})
	if err != nil {
		return nil, err
	}

	recovery := models.RecoveryPayment{
		WriteOffID: writeOffID, LoanID: wo.LoanID, AmountPaise: amountPaise, ReceivedAt: time.Now(),
		RecordedByAdminID: recordedByAdminID, RecordedByName: recordedByName, Reference: reference,
		JournalReference: entry.JournalNumber,
	}
	if err := database.DB.Create(&recovery).Error; err != nil {
		return nil, err
	}

	if wo.Status != models.WriteOffStatusRecoveryAfter {
		wo.Status = models.WriteOffStatusRecoveryAfter
		database.DB.Save(&wo)
	}

	audit.RecordDetailed(nil, audit.Entry{
		ActorType: "admin", ActorID: recordedByAdminID, ActorRef: recordedByName,
		Action: "writeoff.recovery", TargetType: "recovery_payment", TargetID: itoa(recovery.ID),
		Result: audit.ResultSuccess, Details: "loan #" + itoa(wo.LoanID) + ", journal " + entry.JournalNumber,
	})
	return &recovery, nil
}
