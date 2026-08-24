// Loan funnel analytics (item E).
//
// Phase 12's internal/mis already produces a count-only funnel. This
// extends it with the amount and drop-off columns item E requires, rather
// than duplicating the stage definitions — the stage set and their status
// mappings are the same, because the underlying state machine is the same.
package analytics

import (
	"time"

	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/models"
)

type FunnelStage struct {
	Stage       string `json:"stage"`
	Count       int64  `json:"count"`
	AmountPaise int64  `json:"amountPaise"`

	// ConversionPercent is this stage as a share of the PREVIOUS stage;
	// nil at the first stage and whenever the previous stage is empty.
	ConversionPercent *float64 `json:"conversionPercent"`
	DropOffPercent    *float64 `json:"dropOffPercent"`
	DropOffCount      int64    `json:"dropOffCount"`

	Note string `json:"note,omitempty"`
}

type FunnelReport struct {
	PeriodFrom time.Time `json:"periodFrom"`
	PeriodTo   time.Time `json:"periodTo"`
	Stages     []FunnelStage `json:"stages"`

	// LargestLeakage names the stage transition with the highest drop-off,
	// which is the question item E is actually asking.
	LargestLeakage string    `json:"largestLeakage,omitempty"`
	GeneratedAt    time.Time `json:"generatedAt"`
}

func GenerateFunnel(from, to time.Time) FunnelReport {
	r := FunnelReport{PeriodFrom: from, PeriodTo: to, GeneratedAt: time.Now()}

	// Amounts use the requested amount up to sanction and the actual
	// disbursed amount thereafter, because those are different figures
	// and conflating them would overstate the disbursed stage.
	stage := func(label string, statuses []string, useDisbursed bool) FunnelStage {
		var count int64
		var rupees float64
		q := database.DB.Model(&models.LoanApplication{}).
			Where("status IN ? AND created_at BETWEEN ? AND ?", statuses, from, to)
		q.Count(&count)
		col := "amount_requested"
		if useDisbursed {
			col = "disbursed_amount"
		}
		q.Select("COALESCE(SUM(" + col + "),0)").Scan(&rupees)
		return FunnelStage{Stage: label, Count: count, AmountPaise: int64(rupees * 100)}
	}

	all := []string{models.LoanStatusSubmitted, models.LoanStatusAutoVerified, models.LoanStatusAssigned,
		models.LoanStatusVerified, models.LoanStatusPendingSanctionApproval, models.LoanStatusSanctioned,
		models.LoanStatusReadyForDisbursement, models.LoanStatusDisbursementProcessing,
		models.LoanStatusDisbursed, models.LoanStatusRejected}

	application := stage("Application", all, false)

	// KYC is a customer-level state, so this counts applications whose
	// customer has a verified KYC record.
	var kycCount int64
	var kycRupees float64
	database.DB.Raw(`
		SELECT COUNT(*), COALESCE(SUM(la.amount_requested),0)
		FROM loan_applications la
		WHERE la.created_at BETWEEN ? AND ?
		  AND EXISTS (SELECT 1 FROM customer_kycs ck WHERE ck.customer_id = la.customer_id AND ck.status = 'VERIFIED')
	`, from, to).Row().Scan(&kycCount, &kycRupees)
	kyc := FunnelStage{Stage: "KYC", Count: kycCount, AmountPaise: int64(kycRupees * 100)}

	credit := stage("Credit Assessment", []string{models.LoanStatusVerified,
		models.LoanStatusPendingSanctionApproval, models.LoanStatusSanctioned,
		models.LoanStatusReadyForDisbursement, models.LoanStatusDisbursementProcessing,
		models.LoanStatusDisbursed}, false)

	approvalStatuses := approvedStatuses()
	approval := stage("Approval", approvalStatuses, false)

	sanction := stage("Sanction", approvalStatuses, false)
	sanction.Note = "Identical to Approval: this codebase's state machine treats sanction as the approval decision itself, not a separate later transition."

	disbursement := stage("Disbursement", []string{models.LoanStatusDisbursed}, true)

	active, closed := activeAndClosedLoans()
	activeStage := FunnelStage{Stage: "Active", Count: active,
		Note: "Point-in-time count across the whole book, not restricted to the selected period."}
	closedStage := FunnelStage{Stage: "Closure", Count: closed,
		Note: "Derived: every installment paid. This codebase's loan state machine has no explicit closed status."}

	r.Stages = []FunnelStage{application, kyc, credit, approval, sanction, disbursement, activeStage, closedStage}

	// Conversion and drop-off are computed only across the period-scoped
	// application-to-disbursement stages; the Active and Closure stages
	// are point-in-time and a conversion rate against them would be
	// meaningless.
	worst := 0.0
	for i := 1; i < 6; i++ {
		prev := r.Stages[i-1]
		cur := &r.Stages[i]
		if prev.Count == 0 {
			continue
		}
		conv := float64(cur.Count) / float64(prev.Count) * 100
		drop := 100 - conv
		cur.ConversionPercent = &conv
		cur.DropOffPercent = &drop
		cur.DropOffCount = prev.Count - cur.Count
		if drop > worst {
			worst = drop
			r.LargestLeakage = prev.Stage + " → " + cur.Stage
		}
	}

	return r
}
