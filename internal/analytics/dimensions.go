// Dimensional analytics — product (item G), geography (item I), customer
// segments (item F), channels (item H).
//
// Availability was determined by inspecting the actual data model, not
// assumed:
//
//	PRODUCT   — LoanApplication.Category is the only product-like
//	            dimension this codebase has. Used as-is; no product
//	            taxonomy was invented.
//	GEOGRAPHY — CustomerAddress.CurrentState / CurrentDistrict exist.
//	            Aggregated at state/district level only; precise addresses
//	            are never returned (item I).
//	SEGMENT   — new vs repeat borrower and loan-size band are genuinely
//	            derivable. Customer type, risk category, and acquisition
//	            channel are NOT — no such field exists anywhere.
//	CHANNEL   — NOT APPLICABLE. There is no acquisition- or collection-
//	            channel field on any model. Item H says "do not invent
//	            channel categories", so none were created.
package analytics

import (
	"time"

	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/models"
)

// DimensionRow holds the figures item G/F/I ask for per dimension value.
// Fields that cannot be reliably attributed to a dimension are absent
// rather than present-and-arbitrary — notably there is no Expenses or
// Profitability field here, because no expense in this codebase carries a
// product/geography attribution and item G/R forbid inventing one.
type DimensionRow struct {
	Value string `json:"value"`

	Customers        int64 `json:"customers"`
	Applications     int64 `json:"applications"`
	Approved         int64 `json:"approved"`
	Disbursed        int64 `json:"disbursedCount"`
	DisbursedPaise   int64 `json:"disbursedPaise"`
	OutstandingPaise int64 `json:"outstandingPaise"`
	OverdueLoans     int64 `json:"overdueLoans"`

	ApprovalRatePercent   float64 `json:"approvalRatePercent"`
	AverageLoanSizePaise  float64 `json:"averageLoanSizePaise"`
	DelinquencyRatePercent float64 `json:"delinquencyRatePercent"`
}

type DimensionReport struct {
	Dimension  string    `json:"dimension"`
	PeriodFrom time.Time `json:"periodFrom"`
	PeriodTo   time.Time `json:"periodTo"`
	Rows       []DimensionRow `json:"rows"`

	// Unavailable states, in the response itself, which item G/F/H/I
	// fields could not be produced and why — so a consumer never assumes
	// an absent field means zero.
	Unavailable []string `json:"unavailable,omitempty"`

	GeneratedAt time.Time `json:"generatedAt"`
}

// ProductAnalytics (item G). Profitability is deliberately NOT ranked:
// no expense or funding cost in this codebase is attributable to a
// product, and item G says not to rank profitability when cost
// allocation is unreliable.
func ProductAnalytics(from, to time.Time) DimensionReport {
	r := dimensionReport("product (LoanApplication.Category)",
		`SELECT COALESCE(NULLIF(la.category,''),'(unspecified)') AS value`, "la.category", from, to)
	r.Unavailable = []string{
		"Revenue by product — " + NotCalculable + " (journal entries carry no product attribution)",
		"Expenses by product — " + NotCalculable + " (no expense carries a product attribution)",
		"Funding cost by product — " + NotCalculable,
		"Profitability by product — NOT RELIABLY CALCULABLE; cost allocation does not exist and item G forbids inventing one",
	}
	return r
}

// GeographicAnalytics (item I). Aggregated at state level; no street
// address, pincode, or customer identity is returned.
func GeographicAnalytics(from, to time.Time) DimensionReport {
	r := DimensionReport{
		Dimension: "geography (CustomerAddress.CurrentState)",
		PeriodFrom: from, PeriodTo: to, GeneratedAt: time.Now(),
	}
	type row struct {
		Value          string
		Applications   int64
		Approved       int64
		Disbursed      int64
		DisbursedPaise int64
		Customers      int64
	}
	var rows []row
	database.DB.Raw(`
		SELECT COALESCE(NULLIF(ca.current_state,''),'(unspecified)') AS value,
		       COUNT(DISTINCT la.id)          AS applications,
		       COUNT(DISTINCT CASE WHEN la.status IN ? THEN la.id END) AS approved,
		       COUNT(DISTINCT CASE WHEN la.status = ? THEN la.id END)  AS disbursed,
		       COALESCE(SUM(CASE WHEN la.status = ? THEN (la.disbursed_amount*100)::bigint ELSE 0 END),0) AS disbursed_paise,
		       COUNT(DISTINCT la.customer_id) AS customers
		FROM loan_applications la
		LEFT JOIN customer_addresses ca ON ca.user_id::text = la.customer_id::text
		WHERE la.created_at BETWEEN ? AND ?
		GROUP BY 1 ORDER BY disbursed_paise DESC
	`, approvedStatuses(), models.LoanStatusDisbursed, models.LoanStatusDisbursed, from, to).Scan(&rows)

	for _, x := range rows {
		r.Rows = append(r.Rows, DimensionRow{
			Value: x.Value, Customers: x.Customers, Applications: x.Applications,
			Approved: x.Approved, Disbursed: x.Disbursed, DisbursedPaise: x.DisbursedPaise,
			ApprovalRatePercent:  pctOf(x.Approved, x.Applications),
			AverageLoanSizePaise: avg(x.DisbursedPaise, x.Disbursed),
		})
	}
	r.Unavailable = []string{
		"Collections/Outstanding/Revenue/Expenses by geography — " + NotCalculable +
			" (journal entries and payments carry no geographic attribution)",
		"Precise customer addresses are intentionally never returned by this endpoint (item I)",
	}
	return r
}

// SegmentAnalytics (item F) — only the segmentation this codebase's data
// genuinely supports: new vs repeat borrower.
func SegmentAnalytics(from, to time.Time) DimensionReport {
	r := DimensionReport{
		Dimension: "borrower segment (new vs repeat)",
		PeriodFrom: from, PeriodTo: to, GeneratedAt: time.Now(),
	}
	type row struct {
		Value          string
		Customers      int64
		Applications   int64
		Disbursed      int64
		DisbursedPaise int64
	}
	var rows []row
	database.DB.Raw(`
		WITH loan_counts AS (
			SELECT customer_id, COUNT(*) AS n
			FROM loan_applications WHERE status = ? GROUP BY customer_id
		)
		SELECT CASE WHEN lc.n > 1 THEN 'REPEAT' ELSE 'NEW' END AS value,
		       COUNT(DISTINCT la.customer_id) AS customers,
		       COUNT(la.id)                   AS applications,
		       COUNT(CASE WHEN la.status = ? THEN 1 END) AS disbursed,
		       COALESCE(SUM(CASE WHEN la.status = ? THEN (la.disbursed_amount*100)::bigint ELSE 0 END),0) AS disbursed_paise
		FROM loan_applications la
		JOIN loan_counts lc ON lc.customer_id = la.customer_id
		WHERE la.created_at BETWEEN ? AND ?
		GROUP BY 1
	`, models.LoanStatusDisbursed, models.LoanStatusDisbursed, models.LoanStatusDisbursed, from, to).Scan(&rows)

	for _, x := range rows {
		r.Rows = append(r.Rows, DimensionRow{
			Value: x.Value, Customers: x.Customers, Applications: x.Applications,
			Disbursed: x.Disbursed, DisbursedPaise: x.DisbursedPaise,
			AverageLoanSizePaise: avg(x.DisbursedPaise, x.Disbursed),
		})
	}
	r.Unavailable = []string{
		"Customer type segmentation — " + NotCalculable + " (no customer-type field exists)",
		"Acquisition channel segmentation — " + NotCalculable + " (no channel field exists; item H forbids inventing channel categories)",
		"Risk-category segmentation — available only where approved ClassificationRule rows exist (Phase 5); none are configured, so it is not produced here",
	}
	return r
}

// ChannelAnalytics (item H) — NOT APPLICABLE. Returned as an explicit,
// empty, self-describing report rather than omitted, so the absence is
// visible to a caller instead of looking like a missing endpoint.
func ChannelAnalytics(from, to time.Time) DimensionReport {
	return DimensionReport{
		Dimension:  "acquisition/collection channel",
		PeriodFrom: from, PeriodTo: to,
		GeneratedAt: time.Now(),
		Unavailable: []string{
			"Channel analytics — " + NotCalculable +
				": no acquisition-channel or collection-channel field exists on any model in this codebase. " +
				"Item H states channel categories must not be invented, so none were created.",
		},
	}
}

// dimensionReport runs the shared per-dimension aggregation over
// loan_applications for a column on that table.
func dimensionReport(name, selectExpr, groupCol string, from, to time.Time) DimensionReport {
	r := DimensionReport{Dimension: name, PeriodFrom: from, PeriodTo: to, GeneratedAt: time.Now()}

	type row struct {
		Value          string
		Customers      int64
		Applications   int64
		Approved       int64
		Disbursed      int64
		DisbursedPaise int64
		OverdueLoans   int64
	}
	var rows []row
	database.DB.Raw(`
		`+selectExpr+`,
		       COUNT(DISTINCT la.customer_id) AS customers,
		       COUNT(la.id)                   AS applications,
		       COUNT(CASE WHEN la.status IN ? THEN 1 END) AS approved,
		       COUNT(CASE WHEN la.status = ? THEN 1 END)  AS disbursed,
		       COALESCE(SUM(CASE WHEN la.status = ? THEN (la.disbursed_amount*100)::bigint ELSE 0 END),0) AS disbursed_paise,
		       COUNT(DISTINCT CASE WHEN EXISTS (
		            SELECT 1 FROM emi_installments ei
		            WHERE ei.loan_id = la.id AND ei.status = 'overdue'
		       ) THEN la.id END) AS overdue_loans
		FROM loan_applications la
		WHERE la.created_at BETWEEN ? AND ?
		GROUP BY `+groupCol+` ORDER BY disbursed_paise DESC
	`, approvedStatuses(), models.LoanStatusDisbursed, models.LoanStatusDisbursed, from, to).Scan(&rows)

	for _, x := range rows {
		r.Rows = append(r.Rows, DimensionRow{
			Value: x.Value, Customers: x.Customers, Applications: x.Applications,
			Approved: x.Approved, Disbursed: x.Disbursed, DisbursedPaise: x.DisbursedPaise,
			OverdueLoans:         x.OverdueLoans,
			ApprovalRatePercent:  pctOf(x.Approved, x.Applications),
			AverageLoanSizePaise: avg(x.DisbursedPaise, x.Disbursed),
			DelinquencyRatePercent: pctOf(x.OverdueLoans, x.Disbursed),
		})
	}
	return r
}
