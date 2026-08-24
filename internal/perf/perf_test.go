package perf_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
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
	"github.com/santosh0921/nbfc-backend/internal/models"
	"github.com/santosh0921/nbfc-backend/internal/perf"
	"github.com/santosh0921/nbfc-backend/internal/routes"
)

// Phase 15 performance measurement suite.
//
// These tests SEED DATA and are therefore opt-in: they only run when
// PERF=1 is set, so `go test ./...` in CI or a normal regression run
// stays fast and never writes perf rows. Run with:
//
//	PERF=1 go test ./internal/perf/ -run TestPerf -v -timeout 30m
//
// PERF_USERS controls the seeded customer count (default 2000).

var testRouter *gin.Engine

func requirePerf(t *testing.T) {
	t.Helper()
	if os.Getenv("PERF") != "1" {
		t.Skip("perf suite is opt-in — set PERF=1 to run (it seeds and removes test data)")
	}
}

func ensureApp(t *testing.T) *gin.Engine {
	t.Helper()
	if testRouter != nil {
		return testRouter
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

	gin.SetMode(gin.TestMode)
	r := gin.New()
	routes.RegisterRoutes(r)
	testRouter = r
	return r
}

func seededUserCount() int {
	if v := os.Getenv("PERF_USERS"); v != "" {
		var n int
		fmt.Sscanf(v, "%d", &n)
		if n > 0 {
			return n
		}
	}
	return 2000
}

func adminToken(t *testing.T, r *gin.Engine) string {
	t.Helper()
	username := "perf_admin_" + time.Now().Format("150405.000000000")
	password := "Sup3rSecure!Passw0rd_1234"
	hash, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	u := models.AdminUser{
		Username: username, Email: username + "@perf.local",
		PasswordHash: string(hash), Role: models.AdminRoleSuperAdmin,
		Status: models.AdminStatusActive,
	}
	database.DB.Create(&u)
	t.Cleanup(func() { database.DB.Unscoped().Delete(&u) })

	body := strings.NewReader(fmt.Sprintf(`{"username":%q,"password":%q}`, username, password))
	req := httptest.NewRequest("POST", "/admin/login", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("perf admin login failed: %d %s", w.Code, w.Body.String())
	}
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	return resp["token"].(string)
}

func authGet(token, path string) *http.Request {
	req := httptest.NewRequest("GET", path, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	return req
}

func report(t *testing.T, r perf.Result) {
	t.Helper()
	t.Logf("\n"+
		"  scenario        : %s\n"+
		"  concurrent users: %d\n"+
		"  duration        : %s\n"+
		"  requests        : %d\n"+
		"  req/sec         : %.1f\n"+
		"  p50 / p95 / p99 : %s / %s / %s\n"+
		"  min / max       : %s / %s\n"+
		"  error rate      : %.2f%% (%d 5xx)\n"+
		"  heap alloc      : %.1f MiB\n"+
		"  goroutines      : %d",
		r.Name, r.ConcurrentUsers, r.Duration.Round(time.Millisecond), r.Requests,
		r.RequestsPerSec,
		r.P50.Round(time.Microsecond), r.P95.Round(time.Microsecond), r.P99.Round(time.Microsecond),
		r.Min.Round(time.Microsecond), r.Max.Round(time.Microsecond),
		r.ErrorRatePct, r.Errors, r.AllocMiB, r.Goroutines)
}

// TestPerfSeed populates the isolated perf dataset. Runs first by name
// ordering within the opt-in suite.
func TestPerfSeed(t *testing.T) {
	requirePerf(t)
	ensureApp(t)

	want := seededUserCount()
	have := perf.CountSeeded()
	if have >= int64(want) {
		t.Logf("perf dataset already present: %d customers", have)
		return
	}
	t0 := time.Now()
	if err := perf.SeedCustomers(want - int(have)); err != nil {
		t.Fatalf("seed failed: %v", err)
	}
	t.Logf("seeded %d perf customers in %s (total now %d)",
		want-int(have), time.Since(t0).Round(time.Millisecond), perf.CountSeeded())
}

// --- Baseline: the endpoints item B names ---

func TestPerfBaselineHealth(t *testing.T) {
	requirePerf(t)
	r := ensureApp(t)
	res := perf.Run("health", r, 50, 5*time.Second, func(i int) *http.Request {
		return httptest.NewRequest("GET", "/health", nil)
	})
	report(t, res)
}

func TestPerfBaselineAdminUserList(t *testing.T) {
	requirePerf(t)
	r := ensureApp(t)
	token := adminToken(t, r)
	t.Logf("dataset: %d perf customers seeded", perf.CountSeeded())

	res := perf.Run("admin user list", r, 10, 5*time.Second, func(i int) *http.Request {
		return authGet(token, "/admin/users")
	})
	report(t, res)
}

func TestPerfBaselineLoanList(t *testing.T) {
	requirePerf(t)
	r := ensureApp(t)
	token := adminToken(t, r)
	res := perf.Run("admin loan list", r, 25, 5*time.Second, func(i int) *http.Request {
		return authGet(token, "/admin/loans")
	})
	report(t, res)
}

func TestPerfBaselineDashboard(t *testing.T) {
	requirePerf(t)
	r := ensureApp(t)
	token := adminToken(t, r)
	res := perf.Run("admin dashboard", r, 25, 5*time.Second, func(i int) *http.Request {
		return authGet(token, "/admin/dashboard")
	})
	report(t, res)
}

func TestPerfBaselineAnalyticsGrowth(t *testing.T) {
	requirePerf(t)
	r := ensureApp(t)
	token := adminToken(t, r)
	res := perf.Run("analytics growth dashboard", r, 10, 5*time.Second, func(i int) *http.Request {
		return authGet(token, "/admin/analytics/growth")
	})
	report(t, res)
}

func TestPerfBaselineExecutiveMIS(t *testing.T) {
	requirePerf(t)
	r := ensureApp(t)
	token := adminToken(t, r)
	res := perf.Run("executive MIS", r, 10, 5*time.Second, func(i int) *http.Request {
		return authGet(token, "/admin/mis/executive")
	})
	report(t, res)
}

// --- Load profiles (item T) ---

func TestPerfLoadProfiles(t *testing.T) {
	requirePerf(t)
	r := ensureApp(t)
	token := adminToken(t, r)

	for _, users := range []int{100, 500, 1000, 2500} {
		res := perf.Run(fmt.Sprintf("admin user list @ %d concurrent", users),
			r, users, 5*time.Second, func(i int) *http.Request {
				return authGet(token, "/admin/users?limit=25")
			})
		report(t, res)
	}
}

// TestPerfSpike (item U) — sudden jump from low to high concurrency,
// measuring whether the system recovers rather than collapsing.
func TestPerfSpike(t *testing.T) {
	requirePerf(t)
	r := ensureApp(t)
	token := adminToken(t, r)

	for _, users := range []int{100, 1000, 5000, 100} {
		res := perf.Run(fmt.Sprintf("spike step @ %d concurrent", users),
			r, users, 4*time.Second, func(i int) *http.Request {
				return authGet(token, "/admin/users?limit=25")
			})
		report(t, res)
	}
}

// TestPerfCleanup removes the seeded dataset. Named to sort last.
func TestPerfZCleanup(t *testing.T) {
	requirePerf(t)
	ensureApp(t)
	if os.Getenv("PERF_KEEP") == "1" {
		t.Skip("PERF_KEEP=1 — leaving the perf dataset in place")
	}
	if err := perf.Cleanup(); err != nil {
		t.Fatalf("cleanup failed: %v", err)
	}
	t.Logf("perf dataset removed (remaining: %d)", perf.CountSeeded())
}

// TestPerfSoak (item AM) — sustained load to surface leaks, connection
// exhaustion, and latency drift that a 5-second run cannot show.
//
// Duration is configurable via PERF_SOAK_MINUTES (default 30, item AM's
// preferred minimum). Samples heap and goroutine count at intervals so
// growth over time is visible rather than only a final figure.
func TestPerfSoak(t *testing.T) {
	requirePerf(t)
	r := ensureApp(t)
	token := adminToken(t, r)

	minutes := 30
	if v := os.Getenv("PERF_SOAK_MINUTES"); v != "" {
		fmt.Sscanf(v, "%d", &minutes)
	}
	total := time.Duration(minutes) * time.Minute
	const segments = 6
	segment := total / segments

	t.Logf("soak starting: %d minutes in %d segments of %s", minutes, segments, segment.Round(time.Second))

	var first, last perf.Result
	for i := 0; i < segments; i++ {
		res := perf.Run(fmt.Sprintf("soak segment %d/%d", i+1, segments),
			r, 50, segment, func(n int) *http.Request {
				return authGet(token, "/admin/users?limit=25")
			})
		report(t, res)
		if i == 0 {
			first = res
		}
		last = res
	}

	// Leak signal: heap and goroutines should not grow materially across
	// segments under identical load. This asserts rather than merely
	// logging, so a genuine leak fails the test instead of hiding in
	// output nobody reads.
	if last.Goroutines > first.Goroutines*3 && last.Goroutines > 100 {
		t.Errorf("possible goroutine leak: %d at start, %d at end", first.Goroutines, last.Goroutines)
	}
	t.Logf("soak complete: heap %.1f MiB -> %.1f MiB, goroutines %d -> %d, p95 %s -> %s",
		first.AllocMiB, last.AllocMiB, first.Goroutines, last.Goroutines,
		first.P95.Round(time.Millisecond), last.P95.Round(time.Millisecond))
}
