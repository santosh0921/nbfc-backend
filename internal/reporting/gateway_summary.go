// Gateway/bank reconciliation dashboard summary (Phase 9 item I) — server-
// side aggregation over CrossSystemReconciliation (Phase 6, extended in
// Phase 9 with the disbursement-amount and payment-gateway-status checks)
// and Phase 2's bank ReconciliationException, never a client-side count.
package reporting

import (
	"time"

	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/models"
)

type ReconciliationSummary struct {
	TotalTransactions int64 `json:"totalTransactions"` // total Payment rows in the period, the population reconciliation checks operate over
	Matched           int64 `json:"matched"`
	Unmatched         int64 `json:"unmatched"` // MISSING/PENDING_REVIEW status
	AmountMismatch    int64 `json:"amountMismatch"`
	Duplicate         int64 `json:"duplicate"`
	Exceptions        int64 `json:"exceptions"` // every non-MATCHED row
	AsOf              time.Time `json:"asOf"`
}

func GatewayReconciliationSummary(from, to time.Time) ReconciliationSummary {
	var s ReconciliationSummary
	database.DB.Model(&models.Payment{}).Where("created_at BETWEEN ? AND ?", from, to).Count(&s.TotalTransactions)

	database.DB.Model(&models.CrossSystemReconciliation{}).
		Where("detected_at BETWEEN ? AND ? AND status = ?", from, to, models.ReconStatusMatched).Count(&s.Matched)
	database.DB.Model(&models.CrossSystemReconciliation{}).
		Where("detected_at BETWEEN ? AND ? AND status IN ?", from, to, []string{models.ReconStatusMissing, models.ReconStatusPendingReview}).Count(&s.Unmatched)
	database.DB.Model(&models.CrossSystemReconciliation{}).
		Where("detected_at BETWEEN ? AND ? AND status = ?", from, to, models.ReconStatusMismatch).Count(&s.AmountMismatch)
	database.DB.Model(&models.CrossSystemReconciliation{}).
		Where("detected_at BETWEEN ? AND ? AND status = ?", from, to, models.ReconStatusDuplicate).Count(&s.Duplicate)
	database.DB.Model(&models.CrossSystemReconciliation{}).
		Where("detected_at BETWEEN ? AND ? AND status != ?", from, to, models.ReconStatusMatched).Count(&s.Exceptions)

	s.AsOf = time.Now()
	return s
}
