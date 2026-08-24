package reporting_test

import (
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/models"
	"github.com/santosh0921/nbfc-backend/internal/reporting"
)

// Phase 9 payment/gateway/bank reconciliation regression suite.

func uniqueRef9(t *testing.T) string {
	return t.Name() + "-" + time.Now().Format("150405.000000000")
}

// --- Disbursement amount mismatch detected ---

func TestDisbursementAmountMismatchDetected(t *testing.T) {
	ensureDB(t)

	tenure := 12
	rate := 10.0
	loan := models.LoanApplication{
		CustomerID: uuid.New(), Category: "Personal Loan", AmountRequested: 100000,
		Status: models.LoanStatusDisbursed, TenureMonths: &tenure, InterestRatePercent: &rate,
		ApplicantName: "Recon Test " + uniqueRef9(t),
	}
	database.DB.Create(&loan)
	t.Cleanup(func() { database.DB.Unscoped().Delete(&loan) })

	now := time.Now()
	disb := models.Disbursement{
		LoanID: loan.ID, CustomerID: loan.CustomerID.String(), AmountPaise: 10000000, Currency: "INR",
		Status: models.DisbursementStatusSuccess, Provider: "manual_bank_transfer", UTR: "UTR" + uniqueRef9(t),
		CompletedAt: &now,
	}
	database.DB.Create(&disb)
	t.Cleanup(func() { database.DB.Unscoped().Delete(&disb) })

	account := models.BankAccount{AccountName: "Recon Test Account " + uniqueRef9(t), AccountNumberMasked: "XXXXXXXX0001", Active: true}
	database.DB.Create(&account)
	t.Cleanup(func() { database.DB.Unscoped().Delete(&account) })

	imp := models.BankStatementImport{BankAccountID: account.ID, FileName: "test.csv"}
	database.DB.Create(&imp)
	t.Cleanup(func() { database.DB.Unscoped().Delete(&imp) })

	// Bank says a DIFFERENT amount than the loan/disbursement system does.
	bt := models.BankTransaction{
		ImportID: imp.ID, BankAccountID: account.ID, TxnDate: now, ValueDate: now,
		AmountPaise: 9900000, DrCr: "DR", Status: models.BankTxnStatusReconciled,
		MatchedEntityType: "disbursement", MatchedEntityID: disb.ID,
	}
	database.DB.Create(&bt)
	t.Cleanup(func() { database.DB.Unscoped().Delete(&bt) })

	found := reporting.RunCrossSystemChecks()
	if found == 0 {
		t.Fatal("expected RunCrossSystemChecks to detect the disbursement/bank amount mismatch")
	}

	var exception models.CrossSystemReconciliation
	err := database.DB.Where("entity = ? AND reference = ?", "disbursement", itoa9(disb.ID)).First(&exception).Error
	if err != nil {
		t.Fatalf("expected a CrossSystemReconciliation exception for the mismatched disbursement: %v", err)
	}
	t.Cleanup(func() { database.DB.Unscoped().Delete(&exception) })
	if exception.Severity != "CRITICAL" {
		t.Fatalf("expected CRITICAL severity for a disbursement amount mismatch, got %s", exception.Severity)
	}
}

// --- Stale pending payment flagged as missing gateway confirmation ---

func TestStalePendingPaymentFlagged(t *testing.T) {
	ensureDB(t)

	old := time.Now().Add(-48 * time.Hour)
	payment := models.Payment{
		LoanID: 999999, EmiID: 999999, CustomerID: uuid.New().String(), Gateway: "razorpay",
		GatewayOrderID: "order_" + uniqueRef9(t), AmountPaise: 900000, Currency: "INR",
		Status: models.PaymentStatusPending,
	}
	database.DB.Create(&payment)
	database.DB.Model(&payment).UpdateColumn("created_at", old)
	t.Cleanup(func() { database.DB.Unscoped().Delete(&payment) })

	found := reporting.RunCrossSystemChecks()
	if found == 0 {
		t.Fatal("expected RunCrossSystemChecks to flag the stale PENDING payment")
	}

	var exception models.CrossSystemReconciliation
	err := database.DB.Where("entity = ? AND reference = ?", "payment", itoa9(payment.ID)).First(&exception).Error
	if err != nil {
		t.Fatalf("expected a PENDING_REVIEW exception for the stale payment: %v", err)
	}
	t.Cleanup(func() { database.DB.Unscoped().Delete(&exception) })
}

// --- Gateway reconciliation summary aggregates correctly ---

func TestGatewayReconciliationSummary(t *testing.T) {
	ensureDB(t)
	from := time.Now().AddDate(0, 0, -1)
	to := time.Now().AddDate(0, 0, 1)
	summary := reporting.GatewayReconciliationSummary(from, to)
	if summary.Exceptions < 0 || summary.Matched < 0 {
		t.Fatal("reconciliation summary counts must never be negative")
	}
}

func itoa9(id uint) string {
	if id == 0 {
		return "0"
	}
	digits := []byte{}
	for id > 0 {
		digits = append([]byte{byte('0' + id%10)}, digits...)
		id /= 10
	}
	return string(digits)
}
