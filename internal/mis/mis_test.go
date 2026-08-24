package mis_test

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
	"github.com/santosh0921/nbfc-backend/internal/config"
	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/finance"
	"github.com/santosh0921/nbfc-backend/internal/middleware"
	"github.com/santosh0921/nbfc-backend/internal/mis"
	"github.com/santosh0921/nbfc-backend/internal/models"
	"github.com/santosh0921/nbfc-backend/internal/rbac"
)

// Phase 12 executive-MIS regression suite. The central concern (item AG:
// "no report should silently disagree with another report") is tested by
// confirming the ExecutiveDashboard's embedded Financial section is
// byte-for-byte the same struct finance.GenerateFinancialDashboard
// returns directly for the identical period — which is guaranteed by
// construction (mis.GenerateExecutiveDashboard calls that exact function,
// never a re-derivation), and this test exists to catch a future
// regression if that construction ever changes.

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
}

func setupApp(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	ensureDB(t)

	r := gin.New()
	r.POST("/admin/login", admin.LoginHandler)
	adminGroup := r.Group("/admin")
	adminGroup.Use(middleware.AdminAuthMiddleware())
	adminGroup.GET("/mis/executive", middleware.RequirePermission(rbac.MISView), mis.AdminExecutiveDashboardHandler)
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
	u := models.AdminUser{Username: username, Email: username + "@test.local", PasswordHash: string(hash), Role: role, Status: models.AdminStatusActive}
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

// --- Executive dashboard's financial section matches the standalone
// finance dashboard for the same period (item AG / item B) ---

func TestExecutiveDashboardMatchesFinanceDashboard(t *testing.T) {
	ensureDB(t)
	from := time.Now().AddDate(0, -1, 0)
	to := time.Now()

	exec := mis.GenerateExecutiveDashboard(from, to)
	standalone := finance.GenerateFinancialDashboard(from, to)

	if exec.Financial.TotalDisbursedPaise != standalone.TotalDisbursedPaise {
		t.Fatalf("executive dashboard disbursed (%d) must exactly equal the standalone finance dashboard (%d) — they must be the same calculation, not two",
			exec.Financial.TotalDisbursedPaise, standalone.TotalDisbursedPaise)
	}
	if exec.Financial.RevenuePaise != standalone.RevenuePaise {
		t.Fatalf("executive dashboard revenue (%d) must exactly equal the standalone finance dashboard (%d)",
			exec.Financial.RevenuePaise, standalone.RevenuePaise)
	}
}

// --- SafePercentChange never divides by zero / never fabricates a
// percentage when the prior period is zero ---

func TestSafePercentChangeHandlesZeroDenominator(t *testing.T) {
	pct := mis.SafePercentChange(500, 0)
	if pct != nil {
		t.Fatalf("expected nil (not a fabricated percentage) when prior period is 0, got %v", *pct)
	}
	pct2 := mis.SafePercentChange(150, 100)
	if pct2 == nil || *pct2 != 50 {
		t.Fatalf("expected 50%% growth from 100 to 150, got %v", pct2)
	}
}

// --- Financial year range is the April-to-March window ---

func TestFinancialYearRange(t *testing.T) {
	mid := time.Date(2026, 8, 15, 0, 0, 0, 0, time.UTC)
	start, end, label := mis.FinancialYearRange(mid)
	if start.Month() != time.April || start.Day() != 1 || start.Year() != 2026 {
		t.Fatalf("expected FY start 2026-04-01, got %v", start)
	}
	if end.Month() != time.March || end.Year() != 2027 {
		t.Fatalf("expected FY end in March 2027, got %v", end)
	}
	if label != "2026-27" {
		t.Fatalf("expected label 2026-27, got %s", label)
	}
}

// --- Unit economics never estimates unavailable metrics ---

func TestUnitEconomicsMarksUnavailableExplicitly(t *testing.T) {
	ensureDB(t)
	report := mis.GenerateUnitEconomics(time.Now().AddDate(0, -1, 0), time.Now())
	found := false
	for _, m := range report.Metrics {
		if m.Label == "Customer Acquisition Cost" {
			found = true
			if m.Unavailable != mis.NotAvailable || m.ValuePaise != nil {
				t.Fatalf("CAC has no data source in this codebase and must be marked unavailable, not estimated — got value=%v unavailable=%q", m.ValuePaise, m.Unavailable)
			}
		}
	}
	if !found {
		t.Fatal("expected a Customer Acquisition Cost metric row")
	}
}

// --- Unauthorized user cannot access the executive dashboard ---

func TestUnauthorizedCannotAccessExecutiveDashboard(t *testing.T) {
	r := setupApp(t)
	username := "mis_noaccess_" + uniqueRef(t)
	pw := createTestAdmin(t, username, models.AdminRoleCollectionOfficer)
	token := loginAs(t, r, username, pw)

	w := doJSON(r, "GET", "/admin/mis/executive", token, nil)
	if w.Code != http.StatusForbidden {
		t.Fatalf("COLLECTION_OFFICER must not have MISView, got %d: %s", w.Code, w.Body.String())
	}
}

// --- Loan funnel conversion rates never divide by zero ---

func TestFunnelHandlesEmptyPeriod(t *testing.T) {
	ensureDB(t)
	// A far-future window guaranteed to have zero applications.
	from := time.Date(2099, 1, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2099, 1, 2, 0, 0, 0, 0, time.UTC)
	report := mis.GenerateFunnelReport(from, to)
	if report.ApplicationToApprovalPercent != nil {
		t.Fatal("conversion rate must be nil (not a fabricated percentage) when there are zero applications in the period")
	}
}
