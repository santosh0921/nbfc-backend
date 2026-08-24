// Anomaly / risk-signal foundation (item O).
//
// EVERY function in this file produces a SIGNAL — an observation that a
// human should look at. Item O is explicit: "Do not automatically accuse
// customers/employees of fraud." Accordingly:
//
//   - No signal changes any loan, payment, customer, or employee state.
//   - Every signal records its Reason and Source, so the basis is always
//     inspectable.
//   - Status starts OPEN and only a human reviewer moves it to
//     FALSE_POSITIVE / CONFIRMED / CLOSED.
//   - No signal type is named "fraud".
package analytics

import (
	"fmt"
	"time"

	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/models"
)

const (
	SignalSharedBankAccount    = "SHARED_BANK_ACCOUNT"
	SignalRepeatedFailedPayment = "REPEATED_FAILED_PAYMENT"
	SignalMultipleLoansShortPeriod = "MULTIPLE_LOANS_SHORT_PERIOD"
	SignalRapidLoanClosure     = "RAPID_LOAN_CLOSURE"
)

// RecordSignal upserts by DedupeKey so a standing observation is not
// re-raised on every sweep, and so a reviewer's decision on an existing
// signal is never overwritten by a later detection run.
func RecordSignal(s models.RiskSignal) {
	if s.ObservedAt.IsZero() {
		s.ObservedAt = time.Now()
	}
	var existing models.RiskSignal
	if err := database.DB.Where("dedupe_key = ?", s.DedupeKey).First(&existing).Error; err == nil {
		return
	}
	database.DB.Create(&s)
}

// RunSignalDetection executes every detector and returns how many new
// signals were raised. It is safe to run repeatedly (see RecordSignal).
func RunSignalDetection(asOf time.Time) int {
	before := signalCount()
	detectSharedBankAccounts()
	detectRepeatedFailedPayments(asOf)
	detectMultipleLoansShortPeriod(asOf)
	return int(signalCount() - before)
}

func signalCount() int64 {
	var c int64
	database.DB.Model(&models.RiskSignal{}).Count(&c)
	return c
}

// detectSharedBankAccounts — the same disbursement bank account appearing
// across more than one customer. This is a legitimate pattern in some
// cases (family accounts, for example), which is exactly why it is a
// signal for review and not an adverse finding.
func detectSharedBankAccounts() {
	type row struct {
		AccountHash string
		Customers   int64
	}
	var rows []row
	// BankAccountReference is a masked reference, never a full account
	// number (see models.Disbursement) — so this detector never handles
	// raw account data.
	database.DB.Raw(`
		SELECT d.bank_account_reference AS account_hash, COUNT(DISTINCT la.customer_id) AS customers
		FROM disbursements d
		JOIN loan_applications la ON la.id = d.loan_id
		WHERE d.bank_account_reference IS NOT NULL AND d.bank_account_reference != ''
		GROUP BY 1 HAVING COUNT(DISTINCT la.customer_id) > 1
	`).Scan(&rows)

	for _, x := range rows {
		RecordSignal(models.RiskSignal{
			SignalType: SignalSharedBankAccount,
			Severity:   models.SignalSeverityWarning,
			Reason: fmt.Sprintf("A single disbursement bank account is associated with %d distinct customers. This can be legitimate (for example a shared family account) and is raised only for review.", x.Customers),
			Source:      "models.Disbursement.BankAccountReference joined to loan_applications",
			SubjectType: "BANK_ACCOUNT",
			SubjectRef:  x.AccountHash,
			DedupeKey:   SignalSharedBankAccount + ":" + x.AccountHash,
			ObservedAt:  time.Now(),
		})
	}
}

// detectRepeatedFailedPayments — five or more failed payment attempts on
// one loan within 30 days. Often a genuine bank/mandate problem, so the
// signal says so.
func detectRepeatedFailedPayments(asOf time.Time) {
	type row struct {
		LoanID   uint
		Attempts int64
	}
	var rows []row
	database.DB.Raw(`
		SELECT loan_id, COUNT(*) AS attempts
		FROM payments
		WHERE status = ? AND created_at BETWEEN ? AND ?
		GROUP BY loan_id HAVING COUNT(*) >= 5
	`, models.PaymentStatusFailed, asOf.AddDate(0, 0, -30), asOf).Scan(&rows)

	for _, x := range rows {
		RecordSignal(models.RiskSignal{
			SignalType: SignalRepeatedFailedPayment,
			Severity:   models.SignalSeverityInfo,
			Reason: fmt.Sprintf("%d failed payment attempts recorded on this loan in the last 30 days. This commonly indicates a mandate or bank-side problem rather than anything adverse, and is raised for operational review.", x.Attempts),
			Source:      "models.Payment (status = failed)",
			SubjectType: "LOAN",
			SubjectRef:  fmt.Sprintf("%d", x.LoanID),
			DedupeKey:   fmt.Sprintf("%s:%d:%s", SignalRepeatedFailedPayment, x.LoanID, asOf.Format("2006-01")),
			ObservedAt:  asOf,
		})
	}
}

// detectMultipleLoansShortPeriod — three or more applications from one
// customer within 30 days.
func detectMultipleLoansShortPeriod(asOf time.Time) {
	type row struct {
		CustomerID string
		Applications int64
	}
	var rows []row
	database.DB.Raw(`
		SELECT customer_id::text AS customer_id, COUNT(*) AS applications
		FROM loan_applications
		WHERE created_at BETWEEN ? AND ?
		GROUP BY 1 HAVING COUNT(*) >= 3
	`, asOf.AddDate(0, 0, -30), asOf).Scan(&rows)

	for _, x := range rows {
		RecordSignal(models.RiskSignal{
			SignalType: SignalMultipleLoansShortPeriod,
			Severity:   models.SignalSeverityInfo,
			Reason: fmt.Sprintf("%d loan applications from the same customer within 30 days. Raised for review only — this is not an eligibility or credit determination, which is made solely by the configured credit rules.", x.Applications),
			Source:      "models.LoanApplication",
			SubjectType: "CUSTOMER",
			SubjectRef:  x.CustomerID,
			DedupeKey:   fmt.Sprintf("%s:%s:%s", SignalMultipleLoansShortPeriod, x.CustomerID, asOf.Format("2006-01")),
			ObservedAt:  asOf,
		})
	}
}

// ReviewSignal is the only way a signal leaves OPEN — a named human
// records the decision and the reason for it.
func ReviewSignal(signalID uint, status, reviewer, note string) error {
	switch status {
	case models.SignalStatusUnderReview, models.SignalStatusFalsePositive,
		models.SignalStatusConfirmed, models.SignalStatusClosed:
	default:
		return fmt.Errorf("invalid review status %q", status)
	}
	if reviewer == "" {
		return fmt.Errorf("reviewer is required")
	}
	now := time.Now()
	return database.DB.Model(&models.RiskSignal{}).Where("id = ?", signalID).
		Updates(map[string]interface{}{
			"status": status, "reviewed_by": reviewer,
			"reviewed_at": &now, "resolution_note": note,
		}).Error
}
