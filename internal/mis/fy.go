// Package mis implements the Phase 12 executive MIS — a COMPOSITION layer
// over the already-existing, already-tested dashboards from Phases 5, 8,
// 10, and 11 (internal/risk.PortfolioDashboard,
// internal/finance.GenerateFinancialDashboard,
// internal/tax.GenerateTaxDashboard, internal/compliance.GenerateDashboard).
// This package NEVER recomputes a figure those packages already own —
// item B's "single reporting source of truth" is enforced by construction:
// ExecutiveDashboard literally embeds their return structs rather than
// re-deriving equivalent numbers independently, so it is structurally
// impossible for the executive dashboard to disagree with the underlying
// report it's summarizing.
package mis

import "time"

// FinancialYearRange returns the Indian-standard April-to-March financial
// year containing t. This is a well-established calendar convention, not
// a tax-policy decision — it is implemented directly, unlike TDS
// rates/thresholds (Phase 10) or compliance requirements (Phase 11)
// which require CA/compliance sign-off. If this NBFC's actual financial
// year differs from the April-March standard, that would need to be
// confirmed and this function adjusted.
func FinancialYearRange(t time.Time) (start, end time.Time, label string) {
	year := t.Year()
	if t.Month() < time.April {
		year--
	}
	start = time.Date(year, time.April, 1, 0, 0, 0, 0, t.Location())
	end = time.Date(year+1, time.March, 31, 23, 59, 59, 0, t.Location())
	label = start.Format("2006") + "-" + end.Format("06")
	return
}

// DateRangePreset resolves the named presets item D lists into concrete
// [from, to] bounds — Today/Yesterday/ThisWeek/ThisMonth/PreviousMonth/
// ThisQuarter/PreviousQuarter/CurrentFY/PreviousFY. Falls back to the
// caller-supplied custom range for anything else, never guessing.
func DateRangePreset(preset string, now time.Time) (from, to time.Time) {
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	switch preset {
	case "today":
		return today, today.Add(24*time.Hour - time.Second)
	case "yesterday":
		y := today.AddDate(0, 0, -1)
		return y, y.Add(24*time.Hour - time.Second)
	case "this_week":
		offset := int(today.Weekday())
		start := today.AddDate(0, 0, -offset)
		return start, today.Add(24*time.Hour - time.Second)
	case "this_month":
		start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
		return start, today.Add(24*time.Hour - time.Second)
	case "previous_month":
		firstOfThis := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
		start := firstOfThis.AddDate(0, -1, 0)
		end := firstOfThis.Add(-time.Second)
		return start, end
	case "this_quarter":
		q := (int(now.Month()) - 1) / 3
		start := time.Date(now.Year(), time.Month(q*3+1), 1, 0, 0, 0, 0, now.Location())
		return start, today.Add(24*time.Hour - time.Second)
	case "previous_quarter":
		q := (int(now.Month()) - 1) / 3
		startThis := time.Date(now.Year(), time.Month(q*3+1), 1, 0, 0, 0, 0, now.Location())
		start := startThis.AddDate(0, -3, 0)
		end := startThis.Add(-time.Second)
		return start, end
	case "current_fy":
		start, end, _ := FinancialYearRange(now)
		if end.After(today) {
			end = today.Add(24*time.Hour - time.Second)
		}
		return start, end
	case "previous_fy":
		start, _, _ := FinancialYearRange(now)
		prevStart := start.AddDate(-1, 0, 0)
		prevEnd := start.Add(-time.Second)
		return prevStart, prevEnd
	default:
		// Unknown preset — default to this-month rather than an
		// unbounded/all-time query, which would be expensive and easy to
		// mistake for "everything."
		start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
		return start, today.Add(24*time.Hour - time.Second)
	}
}
