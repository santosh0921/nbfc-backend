package accounting

import (
	"strconv"

	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/models"
)

// PostLoanDisbursement (item D) — posts once a Disbursement has actually
// reached SUCCESS (verified UTR captured by internal/loans.ConfirmDisbursement
// or a reconciliation match), never merely because an admin approved it.
// The caller passes the already-loaded, already-verified loan/disbursement
// rows; this function does not re-check disbursement status itself — that
// verification belongs to the caller (see internal/loans/confirm_disbursement.go).
//
// Books the full sanctioned principal as receivable against actual cash
// paid out, and — separately — the processing fee + GST as a receivable
// and income/tax-payable, since the fee is not currently netted out of the
// disbursed amount (an unchanged commercial/product decision — see the
// Phase 4 report §18/20 on the accounting-treatment question this raises
// for the NBFC's CA: net-off-at-disbursement vs. separate collection).
func PostLoanDisbursement(loan models.LoanApplication, disb models.Disbursement, actor string) (*models.JournalEntry, error) {
	lines := []LineInput{
		{AccountCode: AcctLoanPrincipalReceivable, DebitPaise: disb.AmountPaise, Description: "Principal disbursed, loan #" + strconv.Itoa(int(loan.ID))},
		{AccountCode: AcctBankCash, CreditPaise: disb.AmountPaise, Description: "Disbursement payout, UTR " + disb.UTR},
	}
	entry, err := Service.PostJournalEntry(PostingInput{
		TransactionType: "DISBURSEMENT",
		ReferenceType:   "disbursement",
		ReferenceID:     strconv.Itoa(int(disb.ID)),
		TransactionDate: *disb.CompletedAt,
		Description:     "Loan disbursement, loan #" + strconv.Itoa(int(loan.ID)),
		CreatedBy:       actor,
		Lines:           lines,
	})
	if err != nil {
		return nil, err
	}

	// Processing fee + GST — a separate journal entry from the principal
	// disbursement, since it is a distinct financial event (a receivable
	// created, not cash moved) even though both happen at the same moment.
	if loan.ProcessingFeeTotal > 0 {
		if _, err := postProcessingFeeAndGST(loan, actor); err != nil {
			return entry, err
		}
	}

	return entry, nil
}

// postProcessingFeeAndGST (item G/H) books the fee receivable, fee income,
// and the correct CGST+SGST or IGST payable split, using the GST
// breakdown already computed and stored on the loan at sanction time
// (internal/loans/maker_checker_handler.go) — this function does not
// recompute GST, it only posts what was already approved/quoted, per the
// "do not change the approved commercial pricing rules" instruction.
func postProcessingFeeAndGST(loan models.LoanApplication, actor string) (*models.JournalEntry, error) {
	feeBase := toPaise(loan.ProcessingFeeBase)
	cgst := toPaise(loan.ProcessingFeeCGST)
	sgst := toPaise(loan.ProcessingFeeSGST)
	igst := toPaise(loan.ProcessingFeeIGST)
	total := feeBase + cgst + sgst + igst

	lines := []LineInput{
		{AccountCode: AcctProcessingFeeReceivable, DebitPaise: total, Description: "Processing fee + GST receivable, loan #" + strconv.Itoa(int(loan.ID))},
		{AccountCode: AcctProcessingFeeIncome, CreditPaise: feeBase, Description: "Processing fee income"},
	}
	if cgst > 0 {
		lines = append(lines, LineInput{AccountCode: AcctCGSTPayable, CreditPaise: cgst, Description: "CGST on processing fee"})
	}
	if sgst > 0 {
		lines = append(lines, LineInput{AccountCode: AcctSGSTPayable, CreditPaise: sgst, Description: "SGST on processing fee"})
	}
	if igst > 0 {
		lines = append(lines, LineInput{AccountCode: AcctIGSTPayable, CreditPaise: igst, Description: "IGST on processing fee"})
	}

	entry, err := Service.PostJournalEntry(PostingInput{
		TransactionType: "PROCESSING_FEE",
		ReferenceType:   "loan_processing_fee",
		ReferenceID:     strconv.Itoa(int(loan.ID)),
		Description:     "Processing fee + GST, loan #" + strconv.Itoa(int(loan.ID)),
		CreatedBy:       actor,
		Lines:           lines,
	})
	if err != nil {
		return nil, err
	}

	recordGSTTransaction(loan, feeBase, cgst, sgst, igst, entry.JournalNumber)
	return entry, nil
}

func recordGSTTransaction(loan models.LoanApplication, feeBase, cgst, sgst, igst int64, journalNumber string) {
	supplyType := models.GSTSupplyTypeInterState
	if loan.ProcessingFeeGSTType == "CGST_SGST" {
		supplyType = models.GSTSupplyTypeIntraState
	}
	txn := models.GSTTransaction{
		SupplyType: supplyType, PlaceOfSupply: loan.ProcessingFeeCustomerState,
		TaxableValuePaise: feeBase, CGSTPaise: cgst, SGSTPaise: sgst, IGSTPaise: igst,
		GSTRatePercent: 18.0, FinancialReference: "loan:" + strconv.Itoa(int(loan.ID)),
		JournalReference: journalNumber,
	}
	database.DB.Create(&txn)
}

// PostEmiPayment (item E) — posts once a Payment has reached SUCCESS via a
// verified gateway webhook (internal/payments/webhook_handler.go), never
// on an unverified/client-claimed payment. Splits into principal and
// interest using the same components already computed on the
// EmiInstallment by the EMI schedule (internal/loans's reducing-balance
// generator) — this function does not recompute the split, it only posts
// it, so the accounting and LMS numbers can never disagree.
func PostEmiPayment(payment models.Payment, inst models.EmiInstallment, actor string) (*models.JournalEntry, error) {
	principalPaise := toPaise(inst.PrincipalComponent)
	interestPaise := toPaise(inst.InterestComponent)
	totalPaise := principalPaise + interestPaise
	// Guard against any rounding drift between the schedule's own
	// principal+interest split and the amount actually collected — the
	// gateway-verified AmountPaise is the source of truth for cash
	// received; any residual paisa is folded into the principal leg so
	// the entry always balances against real cash.
	if diff := payment.AmountPaise - totalPaise; diff != 0 {
		principalPaise += diff
	}

	lines := []LineInput{
		{AccountCode: AcctBankCash, DebitPaise: payment.AmountPaise, Description: "EMI collection, payment #" + strconv.Itoa(int(payment.ID))},
		{AccountCode: AcctLoanPrincipalReceivable, CreditPaise: principalPaise, Description: "Principal repaid"},
	}
	if interestPaise > 0 {
		lines = append(lines, LineInput{AccountCode: AcctInterestReceivable, CreditPaise: interestPaise, Description: "Accrued interest collected"})
	}

	return Service.PostJournalEntry(PostingInput{
		TransactionType: "EMI_PAYMENT",
		ReferenceType:   "payment",
		ReferenceID:     strconv.Itoa(int(payment.ID)),
		TransactionDate: *payment.VerifiedAt,
		Description:     "EMI payment, loan #" + strconv.Itoa(int(payment.LoanID)) + ", installment #" + strconv.Itoa(int(inst.InstallmentNumber)),
		CreatedBy:       actor,
		Lines:           lines,
	})
}

// PostRefund (item C — payment reversal) reverses the cash/receivable
// legs of a previously-posted EMI payment journal. Only ever a new
// reversal-style entry against Bank/Cash and the same receivable
// accounts — the original EMI payment journal is never edited.
func PostRefund(payment models.Payment, refundAmountPaise int64, actor string) (*models.JournalEntry, error) {
	lines := []LineInput{
		{AccountCode: AcctLoanPrincipalReceivable, DebitPaise: refundAmountPaise, Description: "Refund reinstates receivable, payment #" + strconv.Itoa(int(payment.ID))},
		{AccountCode: AcctBankCash, CreditPaise: refundAmountPaise, Description: "Refund paid out"},
	}
	return Service.PostJournalEntry(PostingInput{
		TransactionType: "REFUND",
		ReferenceType:   "payment_refund",
		ReferenceID:     strconv.Itoa(int(payment.ID)),
		Description:     "Refund against payment #" + strconv.Itoa(int(payment.ID)),
		CreatedBy:       actor,
		Lines:           lines,
	})
}

// ReverseJournalEntry (items C/M/S) — the ONLY way to correct a POSTED
// journal entry. Creates a brand-new entry with every line's debit/credit
// swapped, links both entries to each other, and marks the original
// REVERSED. The original entry's own rows are never edited or deleted.
func ReverseJournalEntry(journalID uint, reason, actor string) (*models.JournalEntry, error) {
	var original models.JournalEntry
	if err := database.DB.Preload("Lines").First(&original, journalID).Error; err != nil {
		return nil, ErrJournalNotFound
	}
	if original.Status != models.JournalStatusPosted {
		return nil, ErrCannotReversePosted
	}
	if original.ReversedByJournalID != nil {
		return nil, ErrAlreadyReversed
	}

	lines := make([]LineInput, 0, len(original.Lines))
	for _, l := range original.Lines {
		var acct models.LedgerAccount
		database.DB.First(&acct, l.AccountID)
		lines = append(lines, LineInput{
			AccountCode: acct.AccountCode,
			DebitPaise:  l.CreditPaise, // swapped
			CreditPaise: l.DebitPaise,  // swapped
			Description: "Reversal: " + l.Description,
		})
	}

	reversal, err := Service.PostJournalEntry(PostingInput{
		TransactionType: "REVERSAL",
		ReferenceType:   original.ReferenceType,
		ReferenceID:     original.ReferenceID,
		IdempotencyKey:  "reversal:" + original.IdempotencyKey,
		Description:     "Reversal of " + original.JournalNumber + ": " + reason,
		CreatedBy:       actor,
		Lines:           lines,
	})
	if err != nil {
		return nil, err
	}

	database.DB.Model(&models.JournalEntry{}).Where("id = ?", reversal.ID).
		Update("status", models.JournalStatusReversal)
	original.Status = models.JournalStatusReversed
	original.ReversedByJournalID = &reversal.ID
	original.ReversalReason = reason
	database.DB.Save(&original)
	reversal.ReversalOfJournalID = &original.ID
	database.DB.Model(&models.JournalEntry{}).Where("id = ?", reversal.ID).
		Update("reversal_of_journal_id", original.ID)

	return reversal, nil
}
