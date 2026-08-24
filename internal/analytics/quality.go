// Growth quality (item D) and the management decision center (item AA).
//
// Item D is explicit that fast growth with deteriorating quality must NOT
// be labelled a bad business decision — it is labelled MANAGEMENT
// ATTENTION REQUIRED, with the observed figures stated so a human draws
// the conclusion. Item AA is equally explicit that no unsupported
// recommendation ("stop lending") may be produced; every observation here
// states what moved, by how much, over what period, from what source.
package analytics

import (
	"fmt"
	"time"
)

const ManagementAttentionRequired = "MANAGEMENT ATTENTION REQUIRED"

const (
	AttentionSeverityInfo    = "INFO"
	AttentionSeverityWarning = "WARNING"
	AttentionSeverityHigh    = "HIGH"
)

// AttentionItem is a factual observation for the management decision
// center. It deliberately has no "recommendedAction" field — this layer
// does not tell management what to do.
type AttentionItem struct {
	Title    string `json:"title"`
	Severity string `json:"severity"`
	Status   string `json:"status"` // always ManagementAttentionRequired

	// What/Why/Source are item AA's mandated explanation fields.
	What   string `json:"what"`
	Why    string `json:"why"`
	Source string `json:"source"`

	PeriodFrom time.Time `json:"periodFrom"`
	PeriodTo   time.Time `json:"periodTo"`

	// DrillDown is the API path a reader follows to the underlying data.
	DrillDown string `json:"drillDown,omitempty"`
}

type GrowthQualityReport struct {
	PeriodFrom time.Time `json:"periodFrom"`
	PeriodTo   time.Time `json:"periodTo"`

	// The paired metrics the assessment was made from, so the reader sees
	// the same numbers the assessment used.
	Signals []Metric `json:"signals"`

	Attention   []AttentionItem `json:"attention"`
	GeneratedAt time.Time       `json:"generatedAt"`
}

// GenerateGrowthQuality compares volume growth against revenue, expense,
// collection and overdue movement over the same window. The comparison
// rules below are DERIVED FROM THE SPEC'S OWN WORKED EXAMPLE (item D:
// disbursement +30% / revenue +10% / expenses +25% / overdue +40% →
// flag), not from any credit-policy or risk framework. They are
// management-attention heuristics, explicitly not regulatory rules, and
// they are stated inline in each observation so no reader mistakes them
// for one.
func GenerateGrowthQuality(from, to time.Time) GrowthQualityReport {
	d := GenerateGrowthDashboard(from, to)

	r := GrowthQualityReport{PeriodFrom: from, PeriodTo: to, GeneratedAt: time.Now()}

	disb := findMetric(d.Disbursements, "disbursed_amount")
	rev := findMetric(d.Revenue, "revenue")
	exp := findMetric(d.Expenses, "expenses")
	coll := findMetric(d.Collections, "collections_amount")
	overdue := findMetric(d.Portfolio, "overdue_amount")
	r.Signals = []Metric{disb, rev, exp, coll, overdue}

	// Growth outpacing revenue: volume rising materially faster than the
	// income it produces.
	if bothKnown(disb, rev) && *disb.ChangePercent > 0 && *disb.ChangePercent-*rev.ChangePercent >= 15 {
		r.Attention = append(r.Attention, AttentionItem{
			Title:    "Disbursement growth outpacing revenue growth",
			Severity: AttentionSeverityWarning,
			Status:   ManagementAttentionRequired,
			What: fmt.Sprintf("Disbursement changed %.1f%% while revenue changed %.1f%% over the same period.",
				*disb.ChangePercent, *rev.ChangePercent),
			Why:        "Triggered because disbursement growth exceeded revenue growth by 15 percentage points or more — a management-attention heuristic taken from this phase's own specification, not a credit-policy or regulatory rule.",
			Source:     "accounting.ProfitAndLoss + models.Disbursement",
			PeriodFrom: from, PeriodTo: to,
			DrillDown: "/admin/analytics/growth",
		})
	}

	// Expense growth outpacing revenue growth.
	if bothKnown(exp, rev) && *exp.ChangePercent-*rev.ChangePercent >= 15 {
		r.Attention = append(r.Attention, AttentionItem{
			Title:    "Expense growth outpacing revenue growth",
			Severity: AttentionSeverityWarning,
			Status:   ManagementAttentionRequired,
			What: fmt.Sprintf("Expenses changed %.1f%% while revenue changed %.1f%%.",
				*exp.ChangePercent, *rev.ChangePercent),
			Why:        "Triggered because expense growth exceeded revenue growth by 15 percentage points or more. This states a relationship between two accounting totals; it does not assert that any specific cost is inefficient (see item S).",
			Source:     "accounting.ProfitAndLoss",
			PeriodFrom: from, PeriodTo: to,
			DrillDown: "/admin/analytics/expense-efficiency",
		})
	}

	// High growth with deteriorating portfolio quality — item D's worked
	// example, and the reason that example says MANAGEMENT ATTENTION
	// REQUIRED rather than a judgement.
	if bothKnown(disb, overdue) && *disb.ChangePercent > 0 && *overdue.ChangePercent > 0 &&
		*overdue.ChangePercent > *disb.ChangePercent {
		r.Attention = append(r.Attention, AttentionItem{
			Title:    "High growth with deteriorating portfolio quality",
			Severity: AttentionSeverityHigh,
			Status:   ManagementAttentionRequired,
			What: fmt.Sprintf("Disbursement changed %.1f%% while the overdue portfolio changed %.1f%% — overdue is growing faster than the book.",
				*disb.ChangePercent, *overdue.ChangePercent),
			Why:        "Triggered because overdue growth exceeded disbursement growth over the same window. This is an operational observation about arrears, NOT a regulatory asset classification and NOT a statement that lending decisions were wrong.",
			Source:     "risk.PortfolioDashboard + models.Disbursement",
			PeriodFrom: from, PeriodTo: to,
			DrillDown: "/admin/analytics/dpd-trend",
		})
	}

	// Collections falling while the book grows.
	if bothKnown(coll, disb) && *coll.ChangePercent < 0 && *disb.ChangePercent > 0 {
		r.Attention = append(r.Attention, AttentionItem{
			Title:    "Collections declining while disbursement grows",
			Severity: AttentionSeverityHigh,
			Status:   ManagementAttentionRequired,
			What: fmt.Sprintf("Cash collected changed %.1f%% while disbursement changed %.1f%%.",
				*coll.ChangePercent, *disb.ChangePercent),
			Why:        "Triggered because collections fell in absolute terms while the book grew. Collections here are cash received including principal recovery — this is a cash/operational observation, not a revenue or profit statement.",
			Source:     "journal entries (account 1000, EMI_PAYMENT)",
			PeriodFrom: from, PeriodTo: to,
			DrillDown: "/admin/analytics/collection-risk",
		})
	}

	return r
}

func findMetric(list []Metric, key string) Metric {
	for _, m := range list {
		if m.Key == key {
			return m
		}
	}
	return Metric{Key: key}
}

// bothKnown guards every comparison above: a metric with no comparable
// prior-period base (item C) is never fed into a growth-quality
// judgement, because the change is unknown rather than zero.
func bothKnown(a, b Metric) bool {
	return a.ChangePercent != nil && b.ChangePercent != nil
}
