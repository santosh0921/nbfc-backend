package reconciliation

import (
	"time"

	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/loans"
	"github.com/santosh0921/nbfc-backend/internal/models"
)

// dateTolerance bounds how far apart a bank transaction's value date and
// a candidate disbursement's initiation date can be and still be
// considered the same event — a same-day NEFT can settle up to a couple
// of business days later.
const dateTolerance = 3 * 24 * time.Hour

// autoMatch attempts to resolve one freshly-imported BankTransaction
// against outstanding Disbursement records. Debit ("DR") lines are the
// only ones matched against disbursements in this phase — inbound EMI
// collections are already confirmed independently via the payment
// gateway webhook (internal/payments), so a bank-statement credit line is
// informational rather than the source of truth for those.
//
// Matching, in order of confidence:
//  1. Exact reference/UTR match against Disbursement.ProviderReference —
//     if exactly one such disbursement is outstanding, auto-match and
//     confirm.
//  2. No reference match: fall back to amount + date-tolerance. Exactly
//     one candidate -> auto-match. Zero or more-than-one candidates ->
//     leave UNMATCHED (zero) or raise a ReconciliationException
//     (ambiguous - more than one).
func autoMatch(txn *models.BankTransaction) {
	if txn.DrCr != "DR" {
		return
	}

	var outstanding []models.Disbursement
	database.DB.Where("status IN ?", []string{models.DisbursementStatusInitiated, models.DisbursementStatusProcessing}).
		Find(&outstanding)
	if len(outstanding) == 0 {
		return
	}

	if txn.Reference != "" {
		var exact []models.Disbursement
		for _, d := range outstanding {
			if d.ProviderReference != "" && d.ProviderReference == txn.Reference {
				exact = append(exact, d)
			}
		}
		if len(exact) == 1 {
			applyMatch(txn, "disbursement", exact[0].ID, "auto", "system", nil,
				"exact reference match: "+txn.Reference)
			return
		}
		if len(exact) > 1 {
			raiseException(txn, "disbursement", exact[0].ID,
				"multiple outstanding disbursements share reference "+txn.Reference)
			return
		}
	}

	var amountCandidates []models.Disbursement
	for _, d := range outstanding {
		if d.AmountPaise != txn.AmountPaise {
			continue
		}
		if d.InitiatedAt == nil {
			continue
		}
		diff := txn.ValueDate.Sub(*d.InitiatedAt)
		if diff < 0 {
			diff = -diff
		}
		if diff <= dateTolerance {
			amountCandidates = append(amountCandidates, d)
		}
	}
	switch len(amountCandidates) {
	case 0:
		// Leave UNMATCHED — nothing to flag as an exception, this is
		// simply an unrelated statement line (rent, salary, other
		// vendor payment, etc.).
		return
	case 1:
		applyMatch(txn, "disbursement", amountCandidates[0].ID, "auto", "system", nil,
			"amount + date-tolerance match, no reference on statement line")
	default:
		raiseException(txn, "disbursement", amountCandidates[0].ID,
			"amount matches "+itoaR(len(amountCandidates))+" outstanding disbursements — ambiguous, needs manual review")
	}
}

func applyMatch(txn *models.BankTransaction, entityType string, entityID uint, matchType, reconciledBy string, reconciledByAdminID *uint, reason string) {
	now := time.Now()
	txn.Status = models.BankTxnStatusMatched
	txn.MatchedEntityType = entityType
	txn.MatchedEntityID = entityID
	database.DB.Save(txn)

	record := models.ReconciliationRecord{
		BankTransactionID:   txn.ID,
		MatchedEntityType:   entityType,
		MatchedEntityID:     entityID,
		MatchType:           matchType,
		Reason:              reason,
		ReconciledBy:        reconciledBy,
		ReconciledByAdminID: reconciledByAdminID,
		ReconciledAt:        now,
	}
	database.DB.Create(&record)

	if entityType == "disbursement" {
		if err := loans.ConfirmDisbursement(entityID, txn.Reference, reconciledBy, reconciledByAdminID); err == nil {
			txn.Status = models.BankTxnStatusReconciled
			database.DB.Save(txn)
		}
	}
}

func raiseException(txn *models.BankTransaction, candidateType string, candidateID uint, reason string) {
	txn.Status = models.BankTxnStatusException
	database.DB.Save(txn)

	var existing models.ReconciliationException
	if database.DB.Where("bank_transaction_id = ?", txn.ID).First(&existing).Error == nil {
		return // already flagged
	}
	database.DB.Create(&models.ReconciliationException{
		BankTransactionID:   txn.ID,
		Reason:              reason,
		CandidateEntityType: candidateType,
		CandidateEntityID:   candidateID,
	})
}

func itoaR(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	if neg {
		b = append([]byte{'-'}, b...)
	}
	return string(b)
}
