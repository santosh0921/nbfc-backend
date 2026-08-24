// Portfolio risk (item J), DPD trend (item K), collection risk (item L),
// concentration (item M) and outlier detection (item N).
//
// THE CENTRAL CONSTRAINT OF THIS FILE: every figure here is an
// OPERATIONAL indicator derived from actual dues and payments. None of it
// is a regulatory asset classification. Regulatory classification in this
// codebase happens only in internal/risk against approved
// ClassificationRule rows (Phase 5), of which zero are currently
// configured — and nothing here substitutes for that.
package analytics

import (
	"fmt"
	"time"

	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/models"
	"github.com/santosh0921/nbfc-backend/internal/risk"
)

// RegulatoryDisclaimer is attached to every operational risk response so
// the distinction travels with the data, not just with the documentation.
const RegulatoryDisclaimer = "OPERATIONAL INDICATOR ONLY — these DPD/overdue figures are derived from actual installment due dates and payments. They are NOT a regulatory asset classification. Regulatory classification requires approved ClassificationRule rows (Phase 5) and is produced only by internal/risk."

// --- Portfolio risk dashboard (item J) ---

type PortfolioRiskReport struct {
	// Portfolio is Phase 5's own dashboard, embedded rather than
	// recomputed, so these figures cannot diverge from the risk module's.
	Portfolio risk.PortfolioDashboardReport `json:"portfolio"`

	Disclaimer  string    `json:"disclaimer"`
	GeneratedAt time.Time `json:"generatedAt"`
}

func GeneratePortfolioRisk(from, to time.Time) PortfolioRiskReport {
	return PortfolioRiskReport{
		Portfolio:   risk.PortfolioDashboard(risk.DashboardFilters{From: &from, To: &to}),
		Disclaimer:  RegulatoryDisclaimer,
		GeneratedAt: time.Now(),
	}
}

// --- DPD trend (item K) ---

const (
	TrendImproving     = "IMPROVING"
	TrendStable        = "STABLE"
	TrendDeteriorating = "DETERIORATING"
	TrendInsufficient  = "INSUFFICIENT DATA"
)

type DPDBucketPoint struct {
	Bucket           string  `json:"bucket"`
	Accounts         int64   `json:"accounts"`
	AmountPaise      int64   `json:"amountPaise"`
	PortfolioPercent float64 `json:"portfolioPercent"`
}

type DPDTrendPeriod struct {
	PeriodFrom time.Time        `json:"periodFrom"`
	PeriodTo   time.Time        `json:"periodTo"`
	Buckets    []DPDBucketPoint `json:"buckets"`
}

type DPDTrendReport struct {
	Periods []DPDTrendPeriod `json:"periods"`

	Trend string `json:"trend"`
	// Methodology is required by item K ("define the methodology") and is
	// returned with the data so it is never separated from it.
	Methodology string    `json:"methodology"`
	Disclaimer  string    `json:"disclaimer"`
	GeneratedAt time.Time `json:"generatedAt"`
}

// GenerateDPDTrend walks back `months` monthly snapshots. DPD buckets are
// the fixed operational ranges item K names (1-30/31-60/61-90/90+), NOT
// Phase 5's configurable DelinquencyBucket rows — those are the
// regulatory-facing ones and are deliberately not reused here so an
// operational chart can never be mistaken for a classification.
func GenerateDPDTrend(asOf time.Time, months int) DPDTrendReport {
	if months < 2 {
		months = 6
	}
	r := DPDTrendReport{
		Disclaimer: RegulatoryDisclaimer,
		Methodology: "For each month-end, an installment is counted as overdue if its due date had passed as at that month-end and it was not paid on or before that date. Days past due is measured from the earliest such unpaid installment per loan. Buckets are the fixed operational ranges 1-30, 31-60, 61-90 and 90+ days. Amounts are the unpaid installment amounts. Portfolio percentage uses total outstanding principal at that month-end as the denominator. Trend compares the 90+ bucket's portfolio share in the latest period against the earliest period.",
		GeneratedAt: time.Now(),
	}

	for i := months - 1; i >= 0; i-- {
		monthEnd := endOfMonth(asOf.AddDate(0, -i, 0))
		r.Periods = append(r.Periods, DPDTrendPeriod{
			PeriodFrom: startOfMonth(monthEnd),
			PeriodTo:   monthEnd,
			Buckets:    dpdBucketsAsOf(monthEnd),
		})
	}

	r.Trend = classifyTrend(r.Periods)
	return r
}

func dpdBucketsAsOf(asOf time.Time) []DPDBucketPoint {
	type agg struct {
		Bucket      string
		Accounts    int64
		AmountPaise int64
	}
	var rows []agg
	// Per-loan DPD is the age of its OLDEST unpaid past-due installment,
	// which is the standard operational definition; the bucketed amount is
	// the total unpaid past-due amount for those loans.
	database.DB.Raw(`
		WITH overdue AS (
			SELECT ei.loan_id,
			       MAX(DATE_PART('day', ?::timestamp - ei.due_date))::int AS dpd,
			       SUM((ei.amount*100)::bigint) AS amount_paise
			FROM emi_installments ei
			WHERE ei.due_date < ?
			  AND (ei.paid_at IS NULL OR ei.paid_at > ?)
			GROUP BY ei.loan_id
		)
		SELECT CASE
		         WHEN dpd BETWEEN 1 AND 30  THEN '1-30'
		         WHEN dpd BETWEEN 31 AND 60 THEN '31-60'
		         WHEN dpd BETWEEN 61 AND 90 THEN '61-90'
		         WHEN dpd > 90              THEN '90+'
		       END AS bucket,
		       COUNT(*) AS accounts,
		       SUM(amount_paise) AS amount_paise
		FROM overdue
		WHERE dpd >= 1
		GROUP BY 1
	`, asOf, asOf, asOf).Scan(&rows)

	outstanding := outstandingPrincipalAsOf(asOf)

	byBucket := map[string]agg{}
	for _, x := range rows {
		byBucket[x.Bucket] = x
	}
	// Always emit all four buckets, including empty ones — an absent
	// bucket in a trend chart is ambiguous, an explicit zero is not.
	out := []DPDBucketPoint{}
	for _, name := range []string{"1-30", "31-60", "61-90", "90+"} {
		x := byBucket[name]
		out = append(out, DPDBucketPoint{
			Bucket: name, Accounts: x.Accounts, AmountPaise: x.AmountPaise,
			PortfolioPercent: pctOf(x.AmountPaise, outstanding),
		})
	}
	return out
}

func outstandingPrincipalAsOf(asOf time.Time) int64 {
	var rupees float64
	database.DB.Raw(`
		SELECT COALESCE(SUM(ei.principal_component),0)
		FROM emi_installments ei
		JOIN loan_applications la ON la.id = ei.loan_id
		WHERE la.status = ? AND (ei.paid_at IS NULL OR ei.paid_at > ?)
	`, models.LoanStatusDisbursed, asOf).Scan(&rupees)
	return int64(rupees * 100)
}

// classifyTrend compares the 90+ bucket's portfolio share between the
// first and last period. The ±0.5 percentage-point band that separates
// STABLE from a real move is a presentation choice, documented in the
// methodology string, not a risk-policy threshold.
func classifyTrend(periods []DPDTrendPeriod) string {
	if len(periods) < 2 {
		return TrendInsufficient
	}
	first := bucketShare(periods[0], "90+")
	last := bucketShare(periods[len(periods)-1], "90+")
	switch {
	case last-first > 0.5:
		return TrendDeteriorating
	case first-last > 0.5:
		return TrendImproving
	default:
		return TrendStable
	}
}

func bucketShare(p DPDTrendPeriod, bucket string) float64 {
	for _, b := range p.Buckets {
		if b.Bucket == bucket {
			return b.PortfolioPercent
		}
	}
	return 0
}

// --- Collection risk (item L) ---

type CollectionRiskReport struct {
	PeriodFrom time.Time `json:"periodFrom"`
	PeriodTo   time.Time `json:"periodTo"`

	DuePaise       int64   `json:"duePaise"`
	CollectedPaise int64   `json:"collectedPaise"`
	OverduePaise   int64   `json:"overduePaise"`
	CollectionRatePercent float64 `json:"collectionRatePercent"`
	OverdueRatePercent    float64 `json:"overdueRatePercent"`

	RepeatOverdueCustomers int64 `json:"repeatOverdueCustomers"`

	// Deteriorating dimensions item L asks to identify. Only the
	// dimensions this codebase actually has are returned.
	DeterioratingProducts []DimensionRow `json:"deterioratingProducts"`

	Trend       string    `json:"trend"`
	Disclaimer  string    `json:"disclaimer"`
	Unavailable []string  `json:"unavailable,omitempty"`
	GeneratedAt time.Time `json:"generatedAt"`
}

func GenerateCollectionRisk(from, to time.Time) CollectionRiskReport {
	r := CollectionRiskReport{
		PeriodFrom: from, PeriodTo: to,
		Disclaimer:  RegulatoryDisclaimer,
		GeneratedAt: time.Now(),
	}

	r.DuePaise = emiDueSum(from, to)
	r.CollectedPaise = collectionsSum(from, to)

	var overdueRupees float64
	database.DB.Model(&models.EmiInstallment{}).
		Where("due_date BETWEEN ? AND ? AND status != ?", from, to, models.EmiStatusPaid).
		Select("COALESCE(SUM(amount),0)").Scan(&overdueRupees)
	r.OverduePaise = int64(overdueRupees * 100)

	r.CollectionRatePercent = pctOf(r.CollectedPaise, r.DuePaise)
	r.OverdueRatePercent = pctOf(r.OverduePaise, r.DuePaise)

	database.DB.Raw(`
		SELECT COUNT(*) FROM (
			SELECT la.customer_id
			FROM emi_installments ei
			JOIN loan_applications la ON la.id = ei.loan_id
			WHERE ei.status = ?
			GROUP BY la.customer_id HAVING COUNT(*) > 1
		) t`, models.EmiStatusOverdue).Scan(&r.RepeatOverdueCustomers)

	// Products whose delinquency rate exceeds the portfolio's own average
	// — a relative comparison against this book's own behaviour, not an
	// externally-defined risk limit.
	products := ProductAnalytics(from, to)
	portfolioAvg := 0.0
	if len(products.Rows) > 0 {
		var totalOverdue, totalDisbursed int64
		for _, p := range products.Rows {
			totalOverdue += p.OverdueLoans
			totalDisbursed += p.Disbursed
		}
		portfolioAvg = pctOf(totalOverdue, totalDisbursed)
	}
	for _, p := range products.Rows {
		if p.Disbursed > 0 && p.DelinquencyRatePercent > portfolioAvg {
			r.DeterioratingProducts = append(r.DeterioratingProducts, p)
		}
	}

	prior := PriorCollectionRate(from, to)
	switch {
	case prior == nil:
		r.Trend = TrendInsufficient
	case r.CollectionRatePercent > *prior+0.5:
		r.Trend = TrendImproving
	case r.CollectionRatePercent < *prior-0.5:
		r.Trend = TrendDeteriorating
	default:
		r.Trend = TrendStable
	}

	r.Unavailable = []string{
		"Collection performance by region — " + NotCalculable + " (payments carry no geographic attribution)",
		"Collection performance by channel — " + NotCalculable + " (no channel field exists)",
	}
	return r
}

// PriorCollectionRate returns the previous equal-length period's
// collection rate, or nil when that period had no dues at all (so the
// trend reads INSUFFICIENT DATA rather than a misleading 0%).
func PriorCollectionRate(from, to time.Time) *float64 {
	pf, pt := PriorPeriod(from, to)
	due := emiDueSum(pf, pt)
	if due == 0 {
		return nil
	}
	rate := pctOf(collectionsSum(pf, pt), due)
	return &rate
}

// --- Concentration (item M) ---

type ConcentrationEntry struct {
	Label            string  `json:"label"`
	ExposurePaise    int64   `json:"exposurePaise"`
	PortfolioPercent float64 `json:"portfolioPercent"`
}

type ConcentrationReport struct {
	TotalPortfolioPaise int64 `json:"totalPortfolioPaise"`

	TopBorrowers []ConcentrationEntry `json:"topBorrowers"`
	TopProducts  []ConcentrationEntry `json:"topProducts"`
	TopStates    []ConcentrationEntry `json:"topStates"`

	// Item M: never call a concentration a regulatory breach.
	Indicator   string    `json:"indicator"`
	GeneratedAt time.Time `json:"generatedAt"`
}

func GenerateConcentration(asOf time.Time) ConcentrationReport {
	total := outstandingPrincipalAsOf(asOf)
	r := ConcentrationReport{
		TotalPortfolioPaise: total,
		Indicator: "CONCENTRATION INDICATOR — exposure shares only. No applicable regulatory concentration limit has been configured or verified in this system, so no breach of any limit is asserted or implied.",
		GeneratedAt: time.Now(),
	}

	// Borrowers are identified by an opaque customer reference, never by
	// name/PAN/mobile — item M needs exposure shares, not identities.
	r.TopBorrowers = concentrationQuery(total, `
		SELECT 'customer:' || la.customer_id::text AS label,
		       COALESCE(SUM(ei.principal_component)*100,0)::bigint AS exposure_paise
		FROM emi_installments ei
		JOIN loan_applications la ON la.id = ei.loan_id
		WHERE la.status = ? AND (ei.paid_at IS NULL OR ei.paid_at > ?)
		GROUP BY 1 ORDER BY exposure_paise DESC LIMIT 10
	`, models.LoanStatusDisbursed, asOf)

	r.TopProducts = concentrationQuery(total, `
		SELECT COALESCE(NULLIF(la.category,''),'(unspecified)') AS label,
		       COALESCE(SUM(ei.principal_component)*100,0)::bigint AS exposure_paise
		FROM emi_installments ei
		JOIN loan_applications la ON la.id = ei.loan_id
		WHERE la.status = ? AND (ei.paid_at IS NULL OR ei.paid_at > ?)
		GROUP BY 1 ORDER BY exposure_paise DESC LIMIT 10
	`, models.LoanStatusDisbursed, asOf)

	r.TopStates = concentrationQuery(total, `
		SELECT COALESCE(NULLIF(ca.current_state,''),'(unspecified)') AS label,
		       COALESCE(SUM(ei.principal_component)*100,0)::bigint AS exposure_paise
		FROM emi_installments ei
		JOIN loan_applications la ON la.id = ei.loan_id
		LEFT JOIN customer_addresses ca ON ca.user_id::text = la.customer_id::text
		WHERE la.status = ? AND (ei.paid_at IS NULL OR ei.paid_at > ?)
		GROUP BY 1 ORDER BY exposure_paise DESC LIMIT 10
	`, models.LoanStatusDisbursed, asOf)

	return r
}

func concentrationQuery(total int64, sql string, args ...interface{}) []ConcentrationEntry {
	type row struct {
		Label         string
		ExposurePaise int64
	}
	var rows []row
	database.DB.Raw(sql, args...).Scan(&rows)
	out := make([]ConcentrationEntry, 0, len(rows))
	for _, x := range rows {
		out = append(out, ConcentrationEntry{
			Label: x.Label, ExposurePaise: x.ExposurePaise,
			PortfolioPercent: pctOf(x.ExposurePaise, total),
		})
	}
	return out
}

// --- Outliers (item N) ---

type Outlier struct {
	Type   string `json:"type"`
	Ref    string `json:"ref"`
	Detail string `json:"detail"`
	// Status is always "ANOMALY / REVIEW REQUIRED" — item N forbids
	// labelling an outlier as fraud.
	Status string `json:"status"`
}

const AnomalyReviewRequired = "ANOMALY / REVIEW REQUIRED"

type OutlierReport struct {
	PeriodFrom time.Time `json:"periodFrom"`
	PeriodTo   time.Time `json:"periodTo"`
	Outliers   []Outlier `json:"outliers"`
	// Methodology states the statistical basis so a reader can judge the
	// finding rather than take it on trust.
	Methodology string    `json:"methodology"`
	GeneratedAt time.Time `json:"generatedAt"`
}

// DetectOutliers flags values more than three standard deviations above
// the mean for the same measure over the same window — a purely
// statistical definition, applied identically to every record, with no
// business rule about what "should" be large.
func DetectOutliers(from, to time.Time) OutlierReport {
	r := OutlierReport{
		PeriodFrom: from, PeriodTo: to,
		Methodology: "A value is flagged when it exceeds the mean plus three standard deviations of the same measure across the selected period, computed over at least 5 records. This is a statistical dispersion test only — it carries no business or fraud judgement, and a flagged record is a candidate for human review, nothing more.",
		GeneratedAt: time.Now(),
	}

	type row struct {
		Ref    string
		Amount int64
	}

	var large []row
	database.DB.Raw(`
		WITH stats AS (
			SELECT AVG(amount_paise) AS mu, STDDEV_POP(amount_paise) AS sd, COUNT(*) AS n
			FROM disbursements WHERE status = ? AND completed_at BETWEEN ? AND ?
		)
		SELECT 'disbursement:' || d.id::text AS ref, d.amount_paise AS amount
		FROM disbursements d, stats s
		WHERE d.status = ? AND d.completed_at BETWEEN ? AND ?
		  AND s.n >= 5 AND s.sd > 0 AND d.amount_paise > s.mu + 3*s.sd
		ORDER BY d.amount_paise DESC LIMIT 50
	`, models.DisbursementStatusSuccess, from, to, models.DisbursementStatusSuccess, from, to).Scan(&large)
	for _, x := range large {
		r.Outliers = append(r.Outliers, Outlier{
			Type: "UNUSUALLY_LARGE_DISBURSEMENT", Ref: x.Ref,
			Detail: fmt.Sprintf("Disbursement of %d paise exceeds the period mean by more than three standard deviations.", x.Amount),
			Status: AnomalyReviewRequired,
		})
	}

	var payments []row
	database.DB.Raw(`
		WITH stats AS (
			SELECT AVG(amount_paise) AS mu, STDDEV_POP(amount_paise) AS sd, COUNT(*) AS n
			FROM payments WHERE status = ? AND verified_at BETWEEN ? AND ?
		)
		SELECT 'payment:' || p.id::text AS ref, p.amount_paise AS amount
		FROM payments p, stats s
		WHERE p.status = ? AND p.verified_at BETWEEN ? AND ?
		  AND s.n >= 5 AND s.sd > 0 AND p.amount_paise > s.mu + 3*s.sd
		ORDER BY p.amount_paise DESC LIMIT 50
	`, models.PaymentStatusSuccess, from, to, models.PaymentStatusSuccess, from, to).Scan(&payments)
	for _, x := range payments {
		r.Outliers = append(r.Outliers, Outlier{
			Type: "UNUSUALLY_LARGE_COLLECTION", Ref: x.Ref,
			Detail: fmt.Sprintf("Payment of %d paise exceeds the period mean by more than three standard deviations.", x.Amount),
			Status: AnomalyReviewRequired,
		})
	}

	return r
}

func startOfMonth(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, t.Location())
}

func endOfMonth(t time.Time) time.Time {
	return startOfMonth(t).AddDate(0, 1, 0).Add(-time.Second)
}
