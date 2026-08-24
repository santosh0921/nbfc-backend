// TDS liability ledger integration (item E) — deliberately additive to,
// never a modification of, Phase 8's finance.MarkPaid. That function
// already posts Dr <expense account> / Cr Bank-Cash for the NET payable;
// PostTDSDeduction below posts a SEPARATE entry — Dr <same expense
// account> / Cr TDS Payable — for the withheld TDS portion. Combined,
// the two entries correctly show the full gross expense recognized, the
// net cash actually paid, and the TDS held back as a liability, without
// touching or duplicating Phase 8's own posting logic. This split was
// flagged as an open CA-review item in the Phase 8 report ("MarkPaid's
// journal debits the full net-payable straight out of cash with no
// separate TDS-payable credit leg") — this function is the fix, but
// still requires the expense's AccountCode to be correct (itself a
// Phase 8 CA-review item) and produces a materially different Trial
// Balance/P&L than before, which a CA should review once real TDS rules
// are configured.
package tax

import (
	"errors"
	"time"

	"github.com/santosh0921/nbfc-backend/internal/accounting"
	"github.com/santosh0921/nbfc-backend/internal/audit"
	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/models"
)

var (
	ErrNotApplicable  = errors.New("tax: this TDS transaction is not in an APPLICABLE state")
	ErrAlreadyDeducted = errors.New("tax: this TDS transaction has already been posted as deducted")
)

// PostTDSDeduction — moves an APPLICABLE TDSTransaction to DEDUCTED and
// posts Dr expenseAccountCode / Cr TDS Payable for TDSAmountPaise.
// Idempotent via TDSTransaction's own status check (never posts twice
// for the same row) and via accounting.Service's own IdempotencyKey
// mechanism as a second layer.
func PostTDSDeduction(txnID uint, expenseAccountCode, actor string) (*models.TDSTransaction, error) {
	var txn models.TDSTransaction
	if err := database.DB.First(&txn, txnID).Error; err != nil {
		return nil, err
	}
	if txn.Status == models.TDSStatusDeducted {
		return &txn, ErrAlreadyDeducted
	}
	if txn.Status != models.TDSStatusApplicable {
		return nil, ErrNotApplicable
	}
	if txn.TDSAmountPaise <= 0 {
		return nil, errors.New("tax: cannot post a zero/negative TDS deduction")
	}

	entry, err := accounting.Service.PostJournalEntry(accounting.PostingInput{
		TransactionType: "TDS_DEDUCTION",
		ReferenceType:   "tds_transaction",
		ReferenceID:     itoa(txn.ID),
		Description:     "TDS withheld: " + txn.SectionRef + ", " + txn.Deductee,
		CreatedBy:       actor,
		Lines: []accounting.LineInput{
			{AccountCode: expenseAccountCode, DebitPaise: txn.TDSAmountPaise, Description: "TDS withheld from " + txn.Deductee},
			{AccountCode: accounting.AcctTDSPayable, CreditPaise: txn.TDSAmountPaise, Description: "TDS payable, section " + txn.SectionRef},
		},
	})
	if err != nil {
		return nil, err
	}

	txn.Status = models.TDSStatusDeducted
	txn.JournalReference = entry.JournalNumber
	if err := database.DB.Save(&txn).Error; err != nil {
		return nil, err
	}
	audit.RecordDetailed(nil, audit.Entry{
		ActorType: actorTypeFor(actor), ActorRef: actor,
		Action: "tax.tds.deducted", TargetType: "tds_transaction", TargetID: itoa(txn.ID),
		Result: audit.ResultSuccess, Details: "journal " + entry.JournalNumber,
	})
	return &txn, nil
}

// ReverseTDSDeduction (item E: "TDS reversal") — the only correction path
// for a DEDUCTED TDSTransaction, mirroring the reversal-not-edit
// convention used everywhere else in this codebase.
func ReverseTDSDeduction(txnID uint, reason, actor string) (*models.TDSTransaction, error) {
	if reason == "" {
		return nil, errors.New("tax: a reversal reason is required")
	}
	var txn models.TDSTransaction
	if err := database.DB.First(&txn, txnID).Error; err != nil {
		return nil, err
	}
	if txn.Status != models.TDSStatusDeducted {
		return nil, ErrNotApplicable
	}
	var journal models.JournalEntry
	if err := database.DB.Where("journal_number = ?", txn.JournalReference).First(&journal).Error; err != nil {
		return nil, err
	}
	if _, err := accounting.ReverseJournalEntry(journal.ID, "TDS transaction #"+itoa(txn.ID)+" reversed: "+reason, actor); err != nil {
		return nil, err
	}
	txn.Status = models.TDSStatusReversed
	database.DB.Save(&txn)
	audit.RecordDetailed(nil, audit.Entry{
		ActorType: actorTypeFor(actor), ActorRef: actor,
		Action: "tax.tds.reversed", TargetType: "tds_transaction", TargetID: itoa(txn.ID),
		Result: audit.ResultSuccess, Reason: reason,
	})
	return &txn, nil
}

// --- Challan / deposit tracking (item F) ---

// CreateChallan — a PENDING deposit record; recording it does NOT itself
// move any TDS transaction to "deposited" — that only happens via
// MarkChallanPaid, and even then only for the specific TDSTransaction
// rows explicitly mapped to it (LinkTransactionToChallan), never a bulk
// assumption.
func CreateChallan(challan *models.TDSChallan, actor string) (*models.TDSChallan, error) {
	if challan.AmountPaise <= 0 {
		return nil, errors.New("tax: challan amount must be positive")
	}
	if challan.Status == "" {
		challan.Status = models.ChallanStatusPending
	}
	if err := database.DB.Create(challan).Error; err != nil {
		return nil, err
	}
	audit.RecordDetailed(nil, audit.Entry{
		ActorType: "admin", ActorRef: actor,
		Action: "tax.challan.created", TargetType: "tds_challan", TargetID: itoa(challan.ID),
		Result: audit.ResultSuccess, Details: challan.TaxYear + " " + challan.Quarter,
	})
	return challan, nil
}

// LinkTransactionToChallan maps a DEDUCTED TDSTransaction to a challan —
// this is what "TDS deducted" and "TDS deposited" being distinguished
// (item E) actually means operationally: a deduction only counts as
// covered by a deposit once explicitly linked.
func LinkTransactionToChallan(txnID, challanID uint, actor string) error {
	var txn models.TDSTransaction
	if err := database.DB.First(&txn, txnID).Error; err != nil {
		return err
	}
	if txn.Status != models.TDSStatusDeducted {
		return ErrNotApplicable
	}
	txn.ChallanID = &challanID
	if err := database.DB.Save(&txn).Error; err != nil {
		return err
	}
	audit.RecordDetailed(nil, audit.Entry{
		ActorType: "admin", ActorRef: actor,
		Action: "tax.challan.mapped", TargetType: "tds_transaction", TargetID: itoa(txnID),
		Result: audit.ResultSuccess, Details: "challan " + itoa(challanID),
	})
	return nil
}

// MarkChallanPaid — posts Dr TDS Payable / Cr Bank-Cash for the challan
// amount, requires bank/challan evidence (bankReference,
// actualDepositDate) to be supplied — this function does not itself
// verify the deposit against a bank statement (that's
// ReconcileChallanAgainstBank, item G); it only records that a human
// says it was paid, with the evidence fields required.
func MarkChallanPaid(challanID uint, actualDepositDate time.Time, bankReference, challanNumber, actor string) (*models.TDSChallan, error) {
	if bankReference == "" {
		return nil, errors.New("tax: a bank reference is required to mark a challan paid")
	}
	var challan models.TDSChallan
	if err := database.DB.First(&challan, challanID).Error; err != nil {
		return nil, err
	}
	if challan.Status == models.ChallanStatusPaid {
		return &challan, nil // idempotent
	}

	entry, err := accounting.Service.PostJournalEntry(accounting.PostingInput{
		TransactionType: "TDS_DEPOSIT",
		ReferenceType:   "tds_challan",
		ReferenceID:     itoa(challan.ID),
		TransactionDate: actualDepositDate,
		Description:     "TDS deposit to government, " + challan.TaxYear + " " + challan.Quarter,
		CreatedBy:       actor,
		Lines: []accounting.LineInput{
			{AccountCode: accounting.AcctTDSPayable, DebitPaise: challan.AmountPaise, Description: "TDS deposited"},
			{AccountCode: accounting.AcctBankCash, CreditPaise: challan.AmountPaise, Description: "TDS payout, ref " + bankReference},
		},
	})
	if err != nil {
		return nil, err
	}

	status := models.ChallanStatusPaid
	if challan.DepositDueDate != nil && actualDepositDate.After(*challan.DepositDueDate) {
		status = models.ChallanStatusLate
	}
	challan.Status = status
	challan.ActualDepositDate = &actualDepositDate
	challan.BankReference = bankReference
	if challanNumber != "" {
		challan.ChallanNumber = challanNumber
	}
	challan.JournalReference = entry.JournalNumber
	if err := database.DB.Save(&challan).Error; err != nil {
		return nil, err
	}
	audit.RecordDetailed(nil, audit.Entry{
		ActorType: "admin", ActorRef: actor,
		Action: "tax.challan.paid", TargetType: "tds_challan", TargetID: itoa(challan.ID),
		Result: audit.ResultSuccess, Details: "journal " + entry.JournalNumber + ", status " + status,
	})
	return &challan, nil
}
