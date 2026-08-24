// Management alert engine (item Z), decision center (item AA), and
// budget vs actual (item Y).
//
// Item Z: "Do not hard-code thresholds." No threshold value appears
// anywhere in this file. Every alert derives from a ManagementThreshold
// row an admin created; with zero rows configured — which is the state
// this phase ships in — the engine produces zero alerts.
package analytics

import (
	"fmt"
	"time"

	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/models"
)

// EvaluateThresholds measures every active threshold's KPI over the
// period and raises an alert for each breach. Returns the number of new
// alerts raised.
func EvaluateThresholds(from, to time.Time) int {
	var thresholds []models.ManagementThreshold
	database.DB.Where("active = ?", true).Find(&thresholds)

	raised := 0
	for _, t := range thresholds {
		value, ok := MetricValue(t.MetricKey, from, to)
		if !ok {
			// A threshold pointing at an unknown KPI is a configuration
			// problem, not an alert condition — it is recorded as a data
			// quality exception so it gets fixed rather than silently
			// never firing.
			recordDataQuality("THRESHOLD_UNKNOWN_METRIC", models.SignalSeverityWarning,
				fmt.Sprintf("Threshold %d references metric key %q, which no KPI in the dictionary resolves to. This threshold can never fire until corrected.", t.ID, t.MetricKey),
				"models.ManagementThreshold",
				fmt.Sprintf("THRESHOLD_UNKNOWN_METRIC:%d", t.ID))
			continue
		}

		breached := (t.Direction == models.ThresholdDirectionAbove && value > t.Value) ||
			(t.Direction == models.ThresholdDirectionBelow && value < t.Value)
		if !breached {
			continue
		}

		direction := "above"
		if t.Direction == models.ThresholdDirectionBelow {
			direction = "below"
		}

		alert := models.ManagementAlert{
			ThresholdID: t.ID, MetricKey: t.MetricKey,
			CurrentValue: value, ThresholdValue: t.Value,
			Direction: t.Direction, Severity: t.Severity,
			// Factual statement only — item AA forbids unsupported
			// recommendations such as "stop lending".
			Explanation: fmt.Sprintf("%s measured %.2f for %s to %s, which is %s the configured threshold of %.2f.",
				t.MetricKey, value, from.Format("2006-01-02"), to.Format("2006-01-02"), direction, t.Value),
			Source:     "analytics.MetricValue over the KPI dictionary definition of " + t.MetricKey,
			PeriodFrom: from, PeriodTo: to,
			Owner:     t.Owner,
			DedupeKey: fmt.Sprintf("%d:%s:%s", t.ID, from.Format("20060102"), to.Format("20060102")),
		}

		var existing models.ManagementAlert
		if err := database.DB.Where("dedupe_key = ?", alert.DedupeKey).First(&existing).Error; err == nil {
			continue
		}
		if database.DB.Create(&alert).Error == nil {
			raised++
		}
	}
	return raised
}

// DecisionCenter (item AA) is the combined management-attention view:
// threshold breaches, growth-quality observations, and open risk signals
// in one place, each already carrying its own explanation.
type DecisionCenterReport struct {
	PeriodFrom time.Time `json:"periodFrom"`
	PeriodTo   time.Time `json:"periodTo"`

	Alerts    []models.ManagementAlert `json:"alerts"`
	Attention []AttentionItem          `json:"attention"`
	OpenSignals []models.RiskSignal    `json:"openSignals"`

	// Note is returned with the payload so the constraint travels with
	// the data rather than living only in documentation.
	Note        string    `json:"note"`
	GeneratedAt time.Time `json:"generatedAt"`
}

func GenerateDecisionCenter(from, to time.Time) DecisionCenterReport {
	r := DecisionCenterReport{
		PeriodFrom: from, PeriodTo: to, GeneratedAt: time.Now(),
		Note: "Every item here states what was observed, why it was flagged, and against which threshold. None of it is a recommendation to take a specific business action, and none of it is a credit, regulatory, accounting, or tax determination.",
	}

	database.DB.Where("status = ?", models.ManagementAlertStatusOpen).
		Order("severity, created_at DESC").Limit(100).Find(&r.Alerts)

	r.Attention = GenerateGrowthQuality(from, to).Attention

	database.DB.Where("status = ?", models.SignalStatusOpen).
		Order("observed_at DESC").Limit(100).Find(&r.OpenSignals)

	return r
}

// ResolveAlert — a named human closes an alert with a stated reason.
func ResolveAlert(alertID uint, status, actor, note string) error {
	switch status {
	case models.ManagementAlertStatusAcknowledged, models.ManagementAlertStatusResolved:
	default:
		return fmt.Errorf("invalid alert status %q", status)
	}
	if actor == "" {
		return fmt.Errorf("actor is required")
	}
	now := time.Now()
	return database.DB.Model(&models.ManagementAlert{}).Where("id = ?", alertID).
		Updates(map[string]interface{}{
			"status": status, "resolved_by": actor,
			"resolved_at": &now, "resolution_note": note,
		}).Error
}

// --- Budget vs actual (item Y) ---

type BudgetVarianceRow struct {
	MetricKey     string   `json:"metricKey"`
	Unit          string   `json:"unit"`
	BudgetValue   float64  `json:"budgetValue"`
	ActualValue   float64  `json:"actualValue"`
	Variance      float64  `json:"variance"`
	VariancePercent *float64 `json:"variancePercent"`
	VarianceNote  string   `json:"varianceNote,omitempty"`

	PeriodFrom time.Time `json:"periodFrom"`
	PeriodTo   time.Time `json:"periodTo"`
}

type BudgetVarianceReport struct {
	Rows []BudgetVarianceRow `json:"rows"`
	// Note explains an empty result, which is the expected state: item Y
	// says budgets must not be invented, so none are seeded.
	Note        string    `json:"note"`
	GeneratedAt time.Time `json:"generatedAt"`
}

func GenerateBudgetVariance(from, to time.Time) BudgetVarianceReport {
	r := BudgetVarianceReport{
		GeneratedAt: time.Now(),
		Note:        "Budget targets are admin-entered. This system seeds none — an empty result means no budget has been configured for the period, not that performance was zero.",
	}

	var budgets []models.Budget
	database.DB.Where("period_from <= ? AND period_to >= ?", to, from).Find(&budgets)

	for _, b := range budgets {
		actual, ok := MetricValue(b.MetricKey, b.PeriodFrom, b.PeriodTo)
		if !ok {
			continue
		}
		row := BudgetVarianceRow{
			MetricKey: b.MetricKey, Unit: b.Unit,
			BudgetValue: b.TargetValue, ActualValue: actual,
			Variance:   actual - b.TargetValue,
			PeriodFrom: b.PeriodFrom, PeriodTo: b.PeriodTo,
		}
		row.VariancePercent = safeChangeFloat(actual, b.TargetValue)
		if row.VariancePercent == nil {
			row.VarianceNote = "Budget target is zero — no meaningful variance percentage"
		}
		r.Rows = append(r.Rows, row)
	}
	return r
}
