package finance

import (
	"errors"
	"strconv"
	"time"

	"github.com/santosh0921/nbfc-backend/internal/accounting"
	"github.com/santosh0921/nbfc-backend/internal/audit"
	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/models"
)

var (
	ErrSameActor         = errors.New("finance: the admin who created this expense cannot also approve it")
	ErrNotPendingApproval = errors.New("finance: expense is not awaiting approval")
	ErrNotApproved       = errors.New("finance: only an APPROVED, unpaid expense can be marked paid")
	ErrUnknownAccount    = accounting.ErrUnknownAccount
)

// CreateExpense (item I) — status always starts PENDING_APPROVAL; nothing
// in this function can post an accounting entry — that only happens once
// MarkPaid runs, after approval, mirroring the same "money only moves
// after independent verification" pattern established for disbursements
// in Phase 2/4.
func CreateExpense(e *models.Expense, createdByAdminID uint, createdBy string) (*models.Expense, error) {
	if e.GrossAmountPaise <= 0 {
		return nil, errors.New("finance: expense gross amount must be positive")
	}
	if e.Category == "" || e.AccountCode == "" {
		return nil, errors.New("finance: category and accountCode are required")
	}
	if e.ExpenseDate.IsZero() {
		e.ExpenseDate = time.Now()
	}
	e.NetPayablePaise = e.GrossAmountPaise + e.TaxAmountPaise - e.TDSAmountPaise
	e.Status = models.ExpenseStatusPendingApproval
	e.CreatedByAdminID = createdByAdminID
	e.CreatedBy = createdBy

	if err := database.DB.Create(e).Error; err != nil {
		return nil, err
	}
	audit.RecordDetailed(nil, audit.Entry{
		ActorType: "admin", ActorID: createdByAdminID, ActorRef: createdBy,
		Action: "finance.expense.created", TargetType: "expense", TargetID: itoa(e.ID),
		Result: audit.ResultSuccess, Details: e.Category + ", gross paise " + strconv.FormatInt(e.GrossAmountPaise, 10),
	})
	return e, nil
}

// ApproveExpense — maker != checker, same defense-in-depth pattern as
// Phase 5's write-off approval.
func ApproveExpense(expenseID, approvedByAdminID uint, approvedByName string) (*models.Expense, error) {
	var e models.Expense
	if err := database.DB.First(&e, expenseID).Error; err != nil {
		return nil, err
	}
	if e.Status != models.ExpenseStatusPendingApproval {
		return nil, ErrNotPendingApproval
	}
	if e.CreatedByAdminID == approvedByAdminID {
		return nil, ErrSameActor
	}
	now := time.Now()
	e.Status = models.ExpenseStatusApproved
	e.ApprovedByAdminID = &approvedByAdminID
	e.ApprovedBy = approvedByName
	e.ApprovedAt = &now
	if err := database.DB.Save(&e).Error; err != nil {
		return nil, err
	}
	audit.RecordDetailed(nil, audit.Entry{
		ActorType: "admin", ActorID: approvedByAdminID, ActorRef: approvedByName,
		Action: "finance.expense.approved", TargetType: "expense", TargetID: itoa(e.ID),
		Result: audit.ResultSuccess,
	})
	return &e, nil
}

func RejectExpense(expenseID, rejectedByAdminID uint, rejectedByName, reason string) (*models.Expense, error) {
	var e models.Expense
	if err := database.DB.First(&e, expenseID).Error; err != nil {
		return nil, err
	}
	if e.Status != models.ExpenseStatusPendingApproval {
		return nil, ErrNotPendingApproval
	}
	if e.CreatedByAdminID == rejectedByAdminID {
		return nil, ErrSameActor
	}
	e.Status = models.ExpenseStatusRejected
	e.RejectionReason = reason
	if err := database.DB.Save(&e).Error; err != nil {
		return nil, err
	}
	audit.RecordDetailed(nil, audit.Entry{
		ActorType: "admin", ActorID: rejectedByAdminID, ActorRef: rejectedByName,
		Action: "finance.expense.rejected", TargetType: "expense", TargetID: itoa(e.ID),
		Result: audit.ResultSuccess, Reason: reason,
	})
	return &e, nil
}

// MarkPaid (item I: payment status/date/reference) posts the accounting
// journal via accounting.Service — Dr <AccountCode expense account> / Cr
// Bank-Cash, for NetPayablePaise (TDS, if any, is deliberately NOT
// credited to a payable account automatically here — see the Phase 8
// report's CA-review flag on TDS treatment; today the full net payable is
// booked as paid out of cash, and a TDS-payable entry would need to be
// added once compliance confirms this NBFC's actual TDS obligations).
func MarkPaid(expenseID uint, paymentReference, actor string) (*models.Expense, error) {
	var e models.Expense
	if err := database.DB.First(&e, expenseID).Error; err != nil {
		return nil, err
	}
	if e.Status != models.ExpenseStatusApproved {
		return nil, ErrNotApproved
	}

	entry, err := accounting.Service.PostJournalEntry(accounting.PostingInput{
		TransactionType: "EXPENSE_PAYMENT",
		ReferenceType:   "expense",
		ReferenceID:     itoa(e.ID),
		Description:     "Expense payment: " + e.Category + " — " + e.Description,
		CreatedBy:       actor,
		Lines: []accounting.LineInput{
			{AccountCode: e.AccountCode, DebitPaise: e.NetPayablePaise, Description: e.Category + " expense"},
			{AccountCode: accounting.AcctBankCash, CreditPaise: e.NetPayablePaise, Description: "Expense payout, ref " + paymentReference},
		},
	})
	if err != nil {
		return nil, err
	}

	now := time.Now()
	e.Status = models.ExpenseStatusPaid
	e.PaymentStatus = "PAID"
	e.PaymentDate = &now
	e.PaymentReference = paymentReference
	e.JournalReference = entry.JournalNumber
	if err := database.DB.Save(&e).Error; err != nil {
		return nil, err
	}
	audit.RecordDetailed(nil, audit.Entry{
		ActorType: "admin", ActorRef: actor,
		Action: "finance.expense.paid", TargetType: "expense", TargetID: itoa(e.ID),
		Result: audit.ResultSuccess, Details: "journal " + entry.JournalNumber,
	})
	return &e, nil
}

var ErrNotPaid = errors.New("finance: only a PAID expense can be reversed")

// ReverseExpense (Phase 8 gap-closure — item D's reversal-not-edit
// requirement, applied to expenses) — the only correction mechanism for
// a PAID expense found to be wrong. Never edits GrossAmountPaise/
// NetPayablePaise/AccountCode in place; instead reverses the original
// payment journal via accounting.ReverseJournalEntry (a new offsetting
// entry, the original left untouched — Phase 4 convention) and moves the
// expense to REVERSED. A corrected expense, if still needed, must be
// entered as a brand-new Expense row — this function does not create one
// automatically, since the correct amount/category is a business
// decision this code cannot infer from "the old one was wrong."
func ReverseExpense(expenseID, reversedByAdminID uint, reversedByName, reason string) (*models.Expense, error) {
	if reason == "" {
		return nil, errors.New("finance: a reversal reason is required")
	}
	var e models.Expense
	if err := database.DB.First(&e, expenseID).Error; err != nil {
		return nil, err
	}
	if e.Status != models.ExpenseStatusPaid {
		return nil, ErrNotPaid
	}

	var journal models.JournalEntry
	if err := database.DB.Where("journal_number = ?", e.JournalReference).First(&journal).Error; err != nil {
		return nil, err
	}
	reversal, err := accounting.ReverseJournalEntry(journal.ID, "expense #"+itoa(e.ID)+" reversed: "+reason, reversedByName)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	e.Status = models.ExpenseStatusReversed
	e.ReversalReason = reason
	e.ReversedByAdminID = &reversedByAdminID
	e.ReversedBy = reversedByName
	e.ReversedAt = &now
	e.ReversalJournalRef = reversal.JournalNumber
	if err := database.DB.Save(&e).Error; err != nil {
		return nil, err
	}

	audit.RecordDetailed(nil, audit.Entry{
		ActorType: "admin", ActorID: reversedByAdminID, ActorRef: reversedByName,
		Action: "finance.expense.reversed", TargetType: "expense", TargetID: itoa(e.ID),
		Result: audit.ResultSuccess, Reason: reason, Details: "reversal journal " + reversal.JournalNumber,
	})
	return &e, nil
}
