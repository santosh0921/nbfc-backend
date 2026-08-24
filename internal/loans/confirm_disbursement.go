package loans

import (
	"errors"
	"log"
	"strconv"
	"time"

	"gorm.io/gorm/clause"

	"github.com/santosh0921/nbfc-backend/internal/accounting"
	"github.com/santosh0921/nbfc-backend/internal/audit"
	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/models"
)

// ConfirmDisbursement is the ONLY place a loan's status actually becomes
// LoanStatusDisbursed — called either by a real payout-provider webhook
// (not wired up in this deployment; no live payout-API credentials are
// configured) or, as the active path today, by
// internal/reconciliation when an imported bank statement line is matched
// to this Disbursement's UTR. A checker approving a disbursement
// (AdminApproveDisbursementHandler) only gets a loan to
// LoanStatusDisbursementProcessing — it is not final until this runs.
//
// confirmedBy/confirmedByAdminID identify who/what triggered the
// confirmation for audit purposes — "system" for an automated bank-
// statement auto-match, an admin's identity for a manual reconciliation
// match.
func ConfirmDisbursement(disbursementID uint, utr, confirmedBy string, confirmedByAdminID *uint) error {
	tx := database.DB.Begin()
	if tx.Error != nil {
		return tx.Error
	}

	var disb models.Disbursement
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&disb, disbursementID).Error; err != nil {
		tx.Rollback()
		return err
	}
	if disb.Status == models.DisbursementStatusSuccess {
		// Already confirmed — idempotent no-op, not an error, so a
		// duplicate reconciliation match or a retried webhook can't
		// double-finalize.
		tx.Rollback()
		return nil
	}
	if disb.Status != models.DisbursementStatusInitiated && disb.Status != models.DisbursementStatusProcessing {
		tx.Rollback()
		return errors.New("disbursement is not in a confirmable state (current status: " + disb.Status + ")")
	}

	var loan models.LoanApplication
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&loan, disb.LoanID).Error; err != nil {
		tx.Rollback()
		return err
	}
	if loan.Status == models.LoanStatusDisbursed {
		tx.Rollback()
		return nil
	}
	if loan.Status != models.LoanStatusDisbursementProcessing {
		tx.Rollback()
		return errors.New("loan is not awaiting disbursement confirmation (current status: " + loan.Status + ")")
	}

	now := time.Now()
	amt := float64(disb.AmountPaise) / 100

	disb.Status = models.DisbursementStatusSuccess
	disb.UTR = utr
	disb.CompletedAt = &now
	disb.VerifiedAt = &now
	if err := tx.Save(&disb).Error; err != nil {
		tx.Rollback()
		return err
	}

	loan.Status = models.LoanStatusDisbursed
	loan.DisbursedAmount = amt
	loan.DisbursedAt = &now
	if err := tx.Save(&loan).Error; err != nil {
		tx.Rollback()
		return err
	}

	if err := tx.Commit().Error; err != nil {
		return err
	}

	// Accounting posting happens only now — after the disbursement has
	// reached the verified SUCCESS state above, never merely because an
	// admin approved it (item D). A posting failure is logged, not
	// fatal: the disbursement/loan state itself is already durably
	// committed; accounting.PostLoanDisbursement's own idempotency key
	// means a later retry (e.g. a reconciliation admin re-running a
	// backfill) can safely post it then without double-booking.
	if _, err := accounting.PostLoanDisbursement(loan, disb, confirmedBy); err != nil {
		log.Println("ConfirmDisbursement: accounting posting failed:", err)
	}

	if err := generateEmiSchedule(&loan); err != nil {
		log.Println("ConfirmDisbursement: failed to generate EMI schedule:", err)
	}
	notifyDisbursement(loan, amt, now)
	if err := generateLoanLetter(loan, "Disbursement Letter"); err != nil {
		log.Println("ConfirmDisbursement: failed to generate disbursement letter:", err)
	}

	var actorID uint
	if confirmedByAdminID != nil {
		actorID = *confirmedByAdminID
	}
	audit.RecordDetailed(nil, audit.Entry{
		ActorType: actorTypeFor(confirmedByAdminID), ActorID: actorID, ActorRef: confirmedBy,
		Action: "disbursement.success", TargetType: "disbursement", TargetID: strconv.Itoa(int(disb.ID)),
		Result:  audit.ResultSuccess,
		Details: "loan #" + itoa(loan.ID) + " disbursed, amount " + strconv.FormatFloat(amt, 'f', 2, 64) + ", UTR " + utr,
	})
	audit.RecordDetailed(nil, audit.Entry{
		ActorType: actorTypeFor(confirmedByAdminID), ActorID: actorID, ActorRef: confirmedBy,
		Action: "utr.captured", TargetType: "disbursement", TargetID: strconv.Itoa(int(disb.ID)),
		Result: audit.ResultSuccess, Details: utr,
	})

	return nil
}

func actorTypeFor(adminID *uint) string {
	if adminID != nil {
		return "admin"
	}
	return "system"
}
