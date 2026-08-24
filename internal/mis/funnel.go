// Loan funnel (item O) — counts at each stage of the existing
// LoanApplication state machine (Phase 1/2), never an invented stage.
// KYC-completed and credit-evaluated stages reuse Phase 3/5's own state
// (CustomerKYC.Status / LoanClassification existence is NOT the same
// thing as "credit evaluated" — this codebase's actual credit-evaluation
// signal is a loan reaching LoanStatusVerified or later, so that's what
// "credit evaluated" is derived from here, not a separate table).
package mis

import (
	"time"

	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/models"
)

type FunnelStage struct {
	Stage string `json:"stage"`
	Count int64  `json:"count"`
}

type FunnelReport struct {
	Stages []FunnelStage `json:"stages"`

	// Conversion rates — nil when the denominator stage has zero
	// records, never a fabricated 0%/100%.
	ApplicationToApprovalPercent *float64 `json:"applicationToApprovalPercent"`
	ApprovalToSanctionPercent    *float64 `json:"approvalToSanctionPercent"`
	SanctionToDisbursementPercent *float64 `json:"sanctionToDisbursementPercent"`

	PeriodFrom time.Time `json:"periodFrom"`
	PeriodTo   time.Time `json:"periodTo"`
	GeneratedAt          time.Time `json:"generatedAt"`
}

func GenerateFunnelReport(from, to time.Time) FunnelReport {
	r := FunnelReport{PeriodFrom: from, PeriodTo: to, GeneratedAt: time.Now()}

	count := func(statuses ...string) int64 {
		var c int64
		database.DB.Model(&models.LoanApplication{}).
			Where("status IN ? AND created_at BETWEEN ? AND ?", statuses, from, to).Count(&c)
		return c
	}

	applications := count(models.LoanStatusSubmitted, models.LoanStatusAutoVerified, models.LoanStatusAssigned,
		models.LoanStatusVerified, models.LoanStatusPendingSanctionApproval, models.LoanStatusSanctioned,
		models.LoanStatusReadyForDisbursement, models.LoanStatusDisbursementProcessing, models.LoanStatusDisbursed, models.LoanStatusRejected)

	var kycVerifiedApplications int64
	database.DB.Raw(`
		SELECT COUNT(*) FROM loan_applications la
		WHERE la.created_at BETWEEN ? AND ?
		AND EXISTS (SELECT 1 FROM customer_kycs ck WHERE ck.customer_id = la.customer_id AND ck.status = 'VERIFIED')
	`, from, to).Scan(&kycVerifiedApplications)

	creditEvaluated := count(models.LoanStatusVerified, models.LoanStatusPendingSanctionApproval, models.LoanStatusSanctioned,
		models.LoanStatusReadyForDisbursement, models.LoanStatusDisbursementProcessing, models.LoanStatusDisbursed)

	approved := count(models.LoanStatusSanctioned, models.LoanStatusReadyForDisbursement,
		models.LoanStatusDisbursementProcessing, models.LoanStatusDisbursed)

	sanctioned := approved // same set today — sanction IS the approval decision in this codebase's state machine

	disbursed := count(models.LoanStatusDisbursed)

	r.Stages = []FunnelStage{
		{"Applications", applications},
		{"KYC Completed", kycVerifiedApplications},
		{"Credit Evaluated", creditEvaluated},
		{"Approved", approved},
		{"Sanctioned", sanctioned},
		{"Disbursed", disbursed},
	}

	r.ApplicationToApprovalPercent = ratioPercent(approved, applications)
	r.ApprovalToSanctionPercent = ratioPercent(sanctioned, approved)
	r.SanctionToDisbursementPercent = ratioPercent(disbursed, sanctioned)

	return r
}

// ratioPercent — a plain part/whole percentage (unlike SafePercentChange,
// which computes a period-over-period DELTA) — nil when whole is zero.
func ratioPercent(part, whole int64) *float64 {
	if whole == 0 {
		return nil
	}
	pct := float64(part) / float64(whole) * 100
	return &pct
}
