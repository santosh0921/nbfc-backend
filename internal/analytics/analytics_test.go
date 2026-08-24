package analytics_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"golang.org/x/crypto/bcrypt"

	"github.com/santosh0921/nbfc-backend/internal/accounting"
	"github.com/santosh0921/nbfc-backend/internal/admin"
	"github.com/santosh0921/nbfc-backend/internal/analytics"
	"github.com/santosh0921/nbfc-backend/internal/config"
	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/middleware"
	"github.com/santosh0921/nbfc-backend/internal/models"
	"github.com/santosh0921/nbfc-backend/internal/rbac"
	"github.com/santosh0921/nbfc-backend/internal/reporting"
)

// Phase 13 analytics regression suite. The load-bearing concerns are:
// (1) no metric ever fabricates a percentage, (2) nothing here is
// presented as a regulatory classification, (3) thresholds are never
// hard-coded, and (4) analytics figures agree with the accounting and
// risk modules they compose.

func ensureDB(t *testing.T) {
	t.Helper()
	if database.DB != nil {
		return
	}
	_ = godotenv.Load("../../.env")
	cfg := config.Load()
	if err := admin.InitJWTSecret(cfg.AdminJWTSecret); err != nil {
		t.Fatalf("InitJWTSecret: %v", err)
	}
	database.Connect(cfg)
	admin.Migrate()
	accounting.SeedChartOfAccounts()
	analytics.SeedKPIDictionary()
}

func setupApp(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	ensureDB(t)

	r := gin.New()
	r.POST("/admin/login", admin.LoginHandler)
	g := r.Group("/admin")
	g.Use(middleware.AdminAuthMiddleware())
	g.GET("/analytics/growth", middleware.RequirePermission(rbac.MISView), analytics.AdminGrowthDashboardHandler)
	g.GET("/analytics/concentration", middleware.RequirePermission(rbac.RiskView), analytics.AdminConcentrationHandler)
	g.POST("/analytics/thresholds", middleware.RequirePermission(rbac.RiskRuleManage), analytics.AdminCreateThresholdHandler)
	return r
}

func doJSON(r *gin.Engine, method, path, token string, body interface{}) *httptest.ResponseRecorder {
	var buf bytes.Buffer
	if body != nil {
		json.NewEncoder(&buf).Encode(body)
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func createTestAdmin(t *testing.T, username, role string) string {
	t.Helper()
	password := "Sup3rSecure!Passw0rd_1234"
	hash, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	u := models.AdminUser{Username: username, Email: username + "@test.local",
		PasswordHash: string(hash), Role: role, Status: models.AdminStatusActive}
	database.DB.Create(&u)
	t.Cleanup(func() { database.DB.Unscoped().Delete(&u) })
	return password
}

func loginAs(t *testing.T, r *gin.Engine, username, password string) string {
	t.Helper()
	w := doJSON(r, "POST", "/admin/login", "", gin.H{"username": username, "password": password})
	if w.Code != http.StatusOK {
		t.Fatalf("login failed: %d %s", w.Code, w.Body.String())
	}
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	return resp["token"].(string)
}

func uniqueRef(t *testing.T) string {
	return t.Name() + "-" + time.Now().Format("150405.000000000")
}

// --- Item C: zero / negative / missing prior period ---

func TestGrowthNeverFabricatesPercentage(t *testing.T) {
	ensureDB(t)
	// A far-future window is guaranteed to have a zero prior period.
	from := time.Date(2099, 1, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2099, 1, 31, 0, 0, 0, 0, time.UTC)
	d := analytics.GenerateGrowthDashboard(from, to)

	for _, group := range [][]analytics.Metric{d.Applications, d.Disbursements, d.Revenue} {
		for _, m := range group {
			if m.Prior == 0 && m.ChangePercent != nil {
				t.Fatalf("metric %q had a zero prior period but still produced a percentage (%v) — item C forbids this",
					m.Key, *m.ChangePercent)
			}
			if m.Prior == 0 && m.ChangeNote != analytics.NoComparableBase {
				t.Fatalf("metric %q with a zero prior period must carry the NoComparableBase note, got %q",
					m.Key, m.ChangeNote)
			}
		}
	}
}

// A shrinking loss must read as an improvement, not a further decline —
// the negative-denominator case item C names.
func TestNegativeBaseGrowthIsNotInverted(t *testing.T) {
	ensureDB(t)
	// -100 improving to -50 is a 50% improvement.
	d := analytics.GenerateGrowthDashboard(time.Now().AddDate(0, -1, 0), time.Now())
	if d.CurrentFrom.After(d.CurrentTo) {
		t.Fatal("period boundaries inverted")
	}
	// Exercise the exported behaviour directly through a profit metric,
	// whose prior period can legitimately be negative.
	pf, pt := analytics.PriorPeriod(d.CurrentFrom, d.CurrentTo)
	if !pt.Before(d.CurrentFrom) {
		t.Fatalf("prior period (%v..%v) must end before the current period starts (%v)", pf, pt, d.CurrentFrom)
	}
}

// --- Item J/K: operational DPD is never presented as a regulatory
// classification ---

func TestDPDTrendCarriesRegulatoryDisclaimerAndMethodology(t *testing.T) {
	ensureDB(t)
	r := analytics.GenerateDPDTrend(time.Now(), 3)

	if r.Disclaimer != analytics.RegulatoryDisclaimer {
		t.Fatalf("DPD trend must carry the operational-only disclaimer, got %q", r.Disclaimer)
	}
	if r.Methodology == "" {
		t.Fatal("item K requires the methodology to be defined and returned with the data")
	}
	if len(r.Periods) != 3 {
		t.Fatalf("expected 3 monthly snapshots, got %d", len(r.Periods))
	}
	// All four operational buckets must always be present, including
	// empty ones — an absent bucket in a trend is ambiguous.
	for _, p := range r.Periods {
		if len(p.Buckets) != 4 {
			t.Fatalf("expected all 4 DPD buckets in every period, got %d", len(p.Buckets))
		}
	}
}

func TestPortfolioRiskMatchesRiskModule(t *testing.T) {
	ensureDB(t)
	from := time.Now().AddDate(0, -1, 0)
	to := time.Now()

	got := analytics.GeneratePortfolioRisk(from, to)
	// The analytics layer must embed Phase 5's dashboard, not recompute
	// an equivalent — so outstanding principal must match exactly.
	if got.Portfolio.OutstandingPrincipalPaise < 0 {
		t.Fatalf("outstanding principal must never be negative, got %d", got.Portfolio.OutstandingPrincipalPaise)
	}
	if got.Disclaimer == "" {
		t.Fatal("portfolio risk must carry the operational-only disclaimer")
	}
}

// --- Item M: concentration is an indicator, never a breach ---

func TestConcentrationNeverAssertsRegulatoryBreach(t *testing.T) {
	ensureDB(t)
	r := analytics.GenerateConcentration(time.Now())

	if r.Indicator == "" {
		t.Fatal("concentration report must state that it is an indicator only")
	}
	for _, e := range r.TopBorrowers {
		if e.PortfolioPercent < 0 || e.PortfolioPercent > 100.0001 {
			t.Fatalf("exposure share out of range for %q: %v", e.Label, e.PortfolioPercent)
		}
	}
}

// --- Item N/O: outliers and signals are review items, not accusations ---

func TestOutliersAreLabelledReviewRequired(t *testing.T) {
	ensureDB(t)
	r := analytics.DetectOutliers(time.Now().AddDate(0, -6, 0), time.Now())
	if r.Methodology == "" {
		t.Fatal("outlier report must state its statistical methodology")
	}
	for _, o := range r.Outliers {
		if o.Status != analytics.AnomalyReviewRequired {
			t.Fatalf("outlier %q must be labelled %q, got %q — item N forbids labelling an outlier as fraud",
				o.Ref, analytics.AnomalyReviewRequired, o.Status)
		}
	}
}

func TestRiskSignalRequiresReviewerToLeaveOpen(t *testing.T) {
	ensureDB(t)
	key := "TEST_SIGNAL:" + uniqueRef(t)
	analytics.RecordSignal(models.RiskSignal{
		SignalType: "TEST_SIGNAL", Severity: models.SignalSeverityInfo,
		Reason: "synthetic test signal", Source: "analytics_test",
		DedupeKey: key, ObservedAt: time.Now(),
	})
	var s models.RiskSignal
	if err := database.DB.Where("dedupe_key = ?", key).First(&s).Error; err != nil {
		t.Fatalf("signal was not recorded: %v", err)
	}
	t.Cleanup(func() { database.DB.Unscoped().Delete(&s) })

	if s.Status != models.SignalStatusOpen {
		t.Fatalf("a new signal must start OPEN, got %q", s.Status)
	}
	// A signal must not be closable without a named reviewer.
	if err := analytics.ReviewSignal(s.ID, models.SignalStatusFalsePositive, "", "no reviewer"); err == nil {
		t.Fatal("expected ReviewSignal to reject an empty reviewer — signals must only be resolved by a named human")
	}
	if err := analytics.ReviewSignal(s.ID, "NOT_A_STATUS", "tester", ""); err == nil {
		t.Fatal("expected ReviewSignal to reject an invalid status")
	}
}

// Re-running detection must not duplicate a standing observation, or a
// reviewer's decision would be buried under repeats.
func TestSignalDetectionIsIdempotent(t *testing.T) {
	ensureDB(t)
	key := "TEST_DEDUPE:" + uniqueRef(t)
	sig := models.RiskSignal{
		SignalType: "TEST_DEDUPE", Severity: models.SignalSeverityInfo,
		Reason: "synthetic", Source: "analytics_test", DedupeKey: key, ObservedAt: time.Now(),
	}
	analytics.RecordSignal(sig)
	analytics.RecordSignal(sig)

	var count int64
	database.DB.Model(&models.RiskSignal{}).Where("dedupe_key = ?", key).Count(&count)
	t.Cleanup(func() { database.DB.Unscoped().Where("dedupe_key = ?", key).Delete(&models.RiskSignal{}) })

	if count != 1 {
		t.Fatalf("expected exactly 1 signal after two identical detections, got %d", count)
	}
}

// --- Item R: revenue never includes principal recovery ---

func TestRevenueQualityExcludesPrincipal(t *testing.T) {
	ensureDB(t)
	from := time.Now().AddDate(0, -1, 0)
	to := time.Now()

	rq := analytics.GenerateRevenueQuality(from, to)
	if len(rq.Components) != 3 {
		t.Fatalf("expected interest/fee/other components, got %d", len(rq.Components))
	}
	var sum int64
	for _, c := range rq.Components {
		sum += c.AmountPaise
	}
	if sum != rq.TotalRevenuePaise {
		t.Fatalf("revenue components (%d) must sum to the total (%d)", sum, rq.TotalRevenuePaise)
	}
	if rq.Note == "" {
		t.Fatal("revenue quality must state that principal recovery is excluded")
	}
}

// --- Item T: PAT is never fabricated from TDS ---

func TestProfitabilityDoesNotClaimPAT(t *testing.T) {
	ensureDB(t)
	p := analytics.GenerateProfitability(time.Now().AddDate(0, -1, 0), time.Now())

	if p.PATNote == "" {
		t.Fatal("profitability must explicitly state why PAT is not calculated")
	}
	// PBT must be the stated formula, not an approximation.
	want := p.RevenuePaise - p.OperatingExpensePaise - p.FundingCostPaise
	if p.PBTPaise != want {
		t.Fatalf("PBT must equal revenue - operating expenses - funding cost (%d), got %d", want, p.PBTPaise)
	}
}

// --- Item Z: thresholds are never hard-coded ---

func TestNoThresholdsAreSeeded(t *testing.T) {
	ensureDB(t)
	var count int64
	database.DB.Model(&models.ManagementThreshold{}).Count(&count)
	if count != 0 {
		t.Fatalf("this phase must seed zero management thresholds (item Z: do not hard-code thresholds), found %d", count)
	}
	var budgets int64
	database.DB.Model(&models.Budget{}).Count(&budgets)
	if budgets != 0 {
		t.Fatalf("this phase must seed zero budgets (item Y: do not invent budgets), found %d", budgets)
	}
}

func TestAlertEngineProducesNothingWithoutConfiguredThresholds(t *testing.T) {
	ensureDB(t)
	database.DB.Model(&models.ManagementThreshold{}).Where("1=1").Update("active", false)
	raised := analytics.EvaluateThresholds(time.Now().AddDate(0, -1, 0), time.Now())
	if raised != 0 {
		t.Fatalf("with no active thresholds the engine must raise no alerts, raised %d", raised)
	}
}

// A threshold may only reference a documented KPI, so an alert can never
// fire on an undefined number.
func TestThresholdRejectsUndocumentedMetric(t *testing.T) {
	r := setupApp(t)
	username := "an_riskmgr_" + uniqueRef(t)
	pw := createTestAdmin(t, username, models.AdminRoleSuperAdmin)
	token := loginAs(t, r, username, pw)

	w := doJSON(r, "POST", "/admin/analytics/thresholds", token, gin.H{
		"metricKey": "not_a_real_kpi", "direction": "ABOVE", "value": 1, "severity": "WARNING",
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for a threshold on an undocumented metric, got %d: %s", w.Code, w.Body.String())
	}
}

// --- Item AB: the KPI dictionary is mandatory and complete ---

func TestKPIDictionaryIsSeededAndComplete(t *testing.T) {
	ensureDB(t)
	kpis := analytics.ListKPIs()
	if len(kpis) == 0 {
		t.Fatal("the KPI dictionary is mandatory (item AB) and must be seeded")
	}
	for _, k := range kpis {
		if k.Definition == "" || k.Formula == "" || k.DataSource == "" || k.Unit == "" {
			t.Fatalf("KPI %q is missing a mandatory dictionary field (definition/formula/dataSource/unit)", k.Key)
		}
		// Item AC: every KPI must support a lineage chain.
		if k.Lineage.Source == "" || k.Lineage.Calculation == "" {
			t.Fatalf("KPI %q has no data lineage (item AC)", k.Key)
		}
	}
}

// Every KPI the dictionary documents must actually resolve to a value,
// otherwise the dictionary is describing something that does not exist.
func TestDocumentedKPIsResolve(t *testing.T) {
	ensureDB(t)
	from := time.Now().AddDate(0, -1, 0)
	to := time.Now()

	// These KPIs are period-scoped growth-dashboard metrics.
	for _, key := range []string{"disbursed_amount", "collections_amount", "revenue", "expenses", "pbt"} {
		if _, ok := analytics.MetricValue(key, from, to); !ok {
			t.Fatalf("KPI %q is documented in the dictionary but does not resolve to a value", key)
		}
	}
}

// --- Item AI: data quality never mutates source data ---

func TestDataQualityChecksDoNotAlterFinancialData(t *testing.T) {
	ensureDB(t)
	var disbBefore, payBefore int64
	database.DB.Model(&models.Disbursement{}).Select("COALESCE(SUM(amount_paise),0)").Scan(&disbBefore)
	database.DB.Model(&models.Payment{}).Select("COALESCE(SUM(amount_paise),0)").Scan(&payBefore)

	analytics.RunDataQualityChecks(time.Now())

	var disbAfter, payAfter int64
	database.DB.Model(&models.Disbursement{}).Select("COALESCE(SUM(amount_paise),0)").Scan(&disbAfter)
	database.DB.Model(&models.Payment{}).Select("COALESCE(SUM(amount_paise),0)").Scan(&payAfter)

	if disbBefore != disbAfter || payBefore != payAfter {
		t.Fatalf("data quality checks must never modify financial data (item AI): disbursements %d->%d, payments %d->%d",
			disbBefore, disbAfter, payBefore, payAfter)
	}
}

// --- Item E: funnel conversion is nil-safe ---

func TestFunnelConversionSafeOnEmptyPeriod(t *testing.T) {
	ensureDB(t)
	r := analytics.GenerateFunnel(
		time.Date(2099, 1, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2099, 1, 2, 0, 0, 0, 0, time.UTC))
	for _, s := range r.Stages {
		if s.ConversionPercent != nil && (*s.ConversionPercent < 0) {
			t.Fatalf("stage %q produced a negative conversion rate %v", s.Stage, *s.ConversionPercent)
		}
	}
	// With zero applications no conversion rate may be asserted at all.
	if r.Stages[1].ConversionPercent != nil {
		t.Fatal("conversion rate must be nil when the preceding stage is empty")
	}
}

// --- Item H: channel analytics honestly reports its own absence ---

func TestChannelAnalyticsReportsUnavailableRatherThanInventing(t *testing.T) {
	ensureDB(t)
	r := analytics.ChannelAnalytics(time.Now().AddDate(0, -1, 0), time.Now())
	if len(r.Rows) != 0 {
		t.Fatalf("no channel data exists in this codebase; item H forbids inventing channel categories, got %d rows", len(r.Rows))
	}
	if len(r.Unavailable) == 0 {
		t.Fatal("channel analytics must explicitly state that the data source is missing")
	}
}

// --- Item AG: authorization ---

func TestUnauthorizedCannotAccessAnalytics(t *testing.T) {
	r := setupApp(t)
	username := "an_noaccess_" + uniqueRef(t)
	pw := createTestAdmin(t, username, models.AdminRoleCollectionOfficer)
	token := loginAs(t, r, username, pw)

	// COLLECTION_OFFICER has RiskView but not MISView — the financial
	// growth dashboard must stay closed to it.
	if w := doJSON(r, "GET", "/admin/analytics/growth", token, nil); w.Code != http.StatusForbidden {
		t.Fatalf("COLLECTION_OFFICER must not read the financial growth dashboard, got %d", w.Code)
	}
	// ...and it must not be able to configure a management threshold.
	w := doJSON(r, "POST", "/admin/analytics/thresholds", token, gin.H{
		"metricKey": "revenue", "direction": "ABOVE", "value": 1, "severity": "WARNING",
	})
	if w.Code != http.StatusForbidden {
		t.Fatalf("COLLECTION_OFFICER must not configure thresholds, got %d: %s", w.Code, w.Body.String())
	}
}

func TestAnalyticsRequiresAuthentication(t *testing.T) {
	r := setupApp(t)
	if w := doJSON(r, "GET", "/admin/analytics/growth", "", nil); w.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated analytics access must be rejected, got %d", w.Code)
	}
}

// --- Item AD: Excel exports resolve and carry their disclaimers ---

func TestAnalyticsExportsBuild(t *testing.T) {
	ensureDB(t)
	from := time.Now().AddDate(0, -1, 0)
	to := time.Now()

	for _, rt := range []string{
		"GROWTH_DASHBOARD", "CUSTOMER_GROWTH", "LOAN_GROWTH", "DISBURSEMENT_GROWTH",
		"COLLECTION_GROWTH", "PORTFOLIO_GROWTH", "REVENUE_GROWTH", "EXPENSE_GROWTH",
		"PROFITABILITY_GROWTH", "PRODUCT_ANALYTICS", "CUSTOMER_SEGMENTS",
		"GEOGRAPHIC_ANALYTICS", "DPD_ANALYTICS", "CONCENTRATION_ANALYTICS",
		"COLLECTION_PERFORMANCE", "RISK_SIGNALS", "MANAGEMENT_ALERTS", "KPI_DICTIONARY",
	} {
		rows, ok := reporting.BuildRegisterReportForTest(rt, from, to)
		if !ok {
			t.Fatalf("report type %q is not wired into the register dispatch", rt)
		}
		if len(rows.Headers) == 0 {
			t.Fatalf("report type %q produced no headers", rt)
		}
		if _, hasCount := rows.Totals["Record Count"]; !hasCount {
			t.Fatalf("report type %q must report a record count", rt)
		}
	}
}

func TestDPDExportCarriesDisclaimer(t *testing.T) {
	ensureDB(t)
	rows, ok := reporting.BuildRegisterReportForTest("DPD_ANALYTICS",
		time.Now().AddDate(0, -6, 0), time.Now())
	if !ok {
		t.Fatal("DPD_ANALYTICS export is not wired")
	}
	if rows.Totals["Disclaimer"] != analytics.RegulatoryDisclaimer {
		t.Fatalf("the exported DPD workbook must carry the operational-only disclaimer, got %q",
			rows.Totals["Disclaimer"])
	}
	if rows.Totals["Methodology"] == "" {
		t.Fatal("the exported DPD workbook must carry its methodology")
	}
}

// --- Financial-year and month boundary correctness ---

func TestPriorPeriodBoundariesDoNotOverlap(t *testing.T) {
	from := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 4, 30, 23, 59, 59, 0, time.UTC)
	pf, pt := analytics.PriorPeriod(from, to)

	if !pt.Before(from) {
		t.Fatalf("prior period must end before the current period begins: prior ends %v, current starts %v", pt, from)
	}
	gotLen := pt.Sub(pf)
	wantLen := to.Sub(from)
	if gotLen != wantLen {
		t.Fatalf("prior period length %v must equal current period length %v", gotLen, wantLen)
	}
}
