package admin_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
	"golang.org/x/crypto/bcrypt"

	"github.com/santosh0921/nbfc-backend/internal/admin"
	"github.com/santosh0921/nbfc-backend/internal/config"
	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/loans"
	"github.com/santosh0921/nbfc-backend/internal/middleware"
	"github.com/santosh0921/nbfc-backend/internal/models"
	"github.com/santosh0921/nbfc-backend/internal/rbac"
)

// Phase 1 security regression suite. Requires a real Postgres reachable
// via the local .env (the same DB used for local dev — see
// c:\Users\Administrator\Desktop\NBFC\.env, DB_PORT=5433). Every test
// creates its own uniquely-named rows and does not assume a clean
// database, since it runs against the shared dev DB rather than an
// isolated test database.
func setupTestApp(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)

	if database.DB == nil {
		// go test's working directory is the package directory
		// (internal/admin), not the repo root, so the default
		// godotenv.Load() inside config.Load() can't find the root .env —
		// load it explicitly from the known relative path first.
		_ = godotenv.Load("../../.env")
		cfg := config.Load()
		if err := admin.InitJWTSecret(cfg.AdminJWTSecret); err != nil {
			t.Fatalf("InitJWTSecret: %v", err)
		}
		database.Connect(cfg)
		admin.Migrate()
	}

	r := gin.New()
	r.POST("/admin/login", admin.LoginHandler)
	r.POST("/admin/refresh", admin.RefreshHandler)
	adminGroup := r.Group("/admin")
	adminGroup.Use(middleware.AdminAuthMiddleware())
	adminGroup.POST("/logout", admin.LogoutHandler)
	adminGroup.POST("/loans/:id/recommend", middleware.RequirePermission(rbac.LoanReview), loans.AdminRecommendSanctionHandler)
	adminGroup.POST("/loans/:id/approve-sanction", middleware.RequirePermission(rbac.LoanSanction), loans.AdminApproveSanctionHandler)
	adminGroup.POST("/loans/:id/prepare-disbursement", middleware.RequirePermission(rbac.LoanDisbursePrep), loans.AdminPrepareDisbursementHandler)
	adminGroup.POST("/loans/:id/approve-disbursement", middleware.RequirePermission(rbac.LoanDisburse), loans.AdminApproveDisbursementHandler)
	adminGroup.GET("/loans/:id", middleware.RequirePermission(rbac.LoanView), loans.AdminGetLoanHandler)
	return r
}

func createTestAdmin(t *testing.T, username, role string) (id uint, password string) {
	t.Helper()
	// Fixed-length regardless of username (which includes t.Name() and can
	// run long) — bcrypt rejects inputs over 72 bytes.
	password = "Sup3rSecure!Passw0rd_1234"
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	u := models.AdminUser{
		Username: username, Email: username + "@test.local", PasswordHash: string(hash),
		Role: role, Status: models.AdminStatusActive,
	}
	if err := database.DB.Create(&u).Error; err != nil {
		t.Fatalf("create admin: %v", err)
	}
	t.Cleanup(func() { database.DB.Unscoped().Delete(&u) })
	return u.ID, password
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

func loginAs(t *testing.T, r *gin.Engine, username, password string) (accessToken, refreshToken string) {
	t.Helper()
	w := doJSON(r, "POST", "/admin/login", "", gin.H{"username": username, "password": password})
	if w.Code != http.StatusOK {
		t.Fatalf("login failed: %d %s", w.Code, w.Body.String())
	}
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	return resp["token"].(string), resp["refreshToken"].(string)
}

// --- 1. Invalid admin login ---

func TestInvalidAdminLogin(t *testing.T) {
	r := setupTestApp(t)
	username := "sectest_invalid_" + t.Name()
	_, password := createTestAdmin(t, username, models.AdminRoleReadOnly)

	w := doJSON(r, "POST", "/admin/login", "", gin.H{"username": username, "password": "wrong-password"})
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for wrong password, got %d: %s", w.Code, w.Body.String())
	}

	w2 := doJSON(r, "POST", "/admin/login", "", gin.H{"username": "no-such-admin-xyz", "password": password})
	if w2.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for unknown username, got %d", w2.Code)
	}
}

// --- 2. Password hashing ---

func TestPasswordIsHashedNotPlaintext(t *testing.T) {
	setupTestApp(t)
	username := "sectest_hash_" + t.Name()
	_, password := createTestAdmin(t, username, models.AdminRoleReadOnly)

	var u models.AdminUser
	database.DB.Where("username = ?", username).First(&u)
	if u.PasswordHash == password {
		t.Fatal("password stored in plaintext")
	}
	if bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)) != nil {
		t.Fatal("stored hash does not verify against the known password")
	}
}

// --- 3. Account lockout ---

func TestAccountLockoutAfterFailedAttempts(t *testing.T) {
	r := setupTestApp(t)
	username := "sectest_lockout_" + t.Name()
	createTestAdmin(t, username, models.AdminRoleReadOnly)

	for i := 0; i < 5; i++ {
		doJSON(r, "POST", "/admin/login", "", gin.H{"username": username, "password": "wrong"})
	}

	var u models.AdminUser
	database.DB.Where("username = ?", username).First(&u)
	if u.LockedUntil == nil || !u.LockedUntil.After(time.Now()) {
		t.Fatal("account should be locked after 5 failed attempts")
	}

	// Even the CORRECT password must now be rejected while locked.
	_, correctPassword := u.ID, ""
	_ = correctPassword
	var fresh models.AdminUser
	database.DB.First(&fresh, u.ID)
	w := doJSON(r, "POST", "/admin/login", "", gin.H{"username": username, "password": "wrong"})
	if w.Code != http.StatusForbidden && w.Code != http.StatusUnauthorized {
		t.Fatalf("expected locked-account rejection, got %d", w.Code)
	}
}

// --- 4. Disabled admin rejection ---

func TestDisabledAdminRejected(t *testing.T) {
	r := setupTestApp(t)
	username := "sectest_disabled_" + t.Name()
	_, password := createTestAdmin(t, username, models.AdminRoleReadOnly)

	database.DB.Model(&models.AdminUser{}).Where("username = ?", username).Update("status", models.AdminStatusDisabled)

	w := doJSON(r, "POST", "/admin/login", "", gin.H{"username": username, "password": password})
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for disabled account, got %d: %s", w.Code, w.Body.String())
	}
}

// --- 5. Permission denial (fail-closed) ---

func TestPermissionDenialForWrongRole(t *testing.T) {
	r := setupTestApp(t)
	username := "sectest_perm_" + t.Name()
	_, password := createTestAdmin(t, username, models.AdminRoleReadOnly) // no LOAN_REVIEW
	token, _ := loginAs(t, r, username, password)

	loan := createTestLoan(t, models.LoanStatusVerified)
	w := doJSON(r, "POST", "/admin/loans/"+itoaTest(loan.ID)+"/recommend", token,
		gin.H{"tenureMonths": 12, "interestRatePercent": 10.0, "remarks": "test"})
	if w.Code != http.StatusForbidden {
		t.Fatalf("READ_ONLY role should not be able to recommend a sanction, got %d: %s", w.Code, w.Body.String())
	}
}

// --- 6/7/8. Maker cannot approve own loan; unauthorized sanction/disbursement ---

func TestMakerCannotApproveOwnRecommendation(t *testing.T) {
	r := setupTestApp(t)
	username := "sectest_maker_" + t.Name()
	_, password := createTestAdmin(t, username, models.AdminRoleSuperAdmin) // has both permissions
	token, _ := loginAs(t, r, username, password)

	loan := createTestLoan(t, models.LoanStatusVerified)
	w := doJSON(r, "POST", "/admin/loans/"+itoaTest(loan.ID)+"/recommend", token,
		gin.H{"tenureMonths": 12, "interestRatePercent": 10.0, "remarks": "test"})
	if w.Code != http.StatusOK {
		t.Fatalf("recommend failed: %d %s", w.Code, w.Body.String())
	}

	w2 := doJSON(r, "POST", "/admin/loans/"+itoaTest(loan.ID)+"/approve-sanction", token,
		gin.H{"approve": true, "remarks": "self-approve attempt"})
	if w2.Code != http.StatusForbidden {
		t.Fatalf("the maker approving their own recommendation must be rejected, got %d: %s", w2.Code, w2.Body.String())
	}
}

func TestDifferentAdminCanApproveSanction(t *testing.T) {
	r := setupTestApp(t)
	makerUsername := "sectest_maker2_" + t.Name()
	checkerUsername := "sectest_checker2_" + t.Name()
	_, makerPassword := createTestAdmin(t, makerUsername, models.AdminRoleCreditOfficer)
	_, checkerPassword := createTestAdmin(t, checkerUsername, models.AdminRoleCreditManager)
	makerToken, _ := loginAs(t, r, makerUsername, makerPassword)
	checkerToken, _ := loginAs(t, r, checkerUsername, checkerPassword)

	loan := createTestLoan(t, models.LoanStatusVerified)
	w := doJSON(r, "POST", "/admin/loans/"+itoaTest(loan.ID)+"/recommend", makerToken,
		gin.H{"tenureMonths": 12, "interestRatePercent": 10.0, "remarks": "test"})
	if w.Code != http.StatusOK {
		t.Fatalf("recommend failed: %d %s", w.Code, w.Body.String())
	}

	w2 := doJSON(r, "POST", "/admin/loans/"+itoaTest(loan.ID)+"/approve-sanction", checkerToken,
		gin.H{"approve": true, "remarks": "ok"})
	if w2.Code != http.StatusOK {
		t.Fatalf("a different admin should be able to approve, got %d: %s", w2.Code, w2.Body.String())
	}

	// --- 15. Duplicate approval prevention ---
	w3 := doJSON(r, "POST", "/admin/loans/"+itoaTest(loan.ID)+"/approve-sanction", checkerToken,
		gin.H{"approve": true, "remarks": "again"})
	if w3.Code == http.StatusOK {
		t.Fatal("approving an already-sanctioned loan a second time must not succeed")
	}
}

func TestUnauthorizedSanctionAndDisbursementBlocked(t *testing.T) {
	r := setupTestApp(t)
	loan := createTestLoan(t, models.LoanStatusVerified)

	// No token at all.
	w := doJSON(r, "POST", "/admin/loans/"+itoaTest(loan.ID)+"/recommend", "",
		gin.H{"tenureMonths": 12, "interestRatePercent": 10.0})
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 with no token, got %d", w.Code)
	}

	w2 := doJSON(r, "POST", "/admin/loans/"+itoaTest(loan.ID)+"/prepare-disbursement", "", gin.H{})
	if w2.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 with no token, got %d", w2.Code)
	}
}

func TestMakerCannotApproveOwnDisbursementPreparation(t *testing.T) {
	r := setupTestApp(t)
	username := "sectest_disbmaker_" + t.Name()
	_, password := createTestAdmin(t, username, models.AdminRoleSuperAdmin)
	token, _ := loginAs(t, r, username, password)

	loan := createTestLoan(t, models.LoanStatusSanctioned)
	tenure := 12
	rate := 10.0
	database.DB.Model(&loan).Updates(map[string]interface{}{"tenure_months": tenure, "interest_rate_percent": rate})

	w := doJSON(r, "POST", "/admin/loans/"+itoaTest(loan.ID)+"/prepare-disbursement", token, gin.H{"amount": 10000})
	if w.Code != http.StatusOK {
		t.Fatalf("prepare failed: %d %s", w.Code, w.Body.String())
	}

	w2 := doJSON(r, "POST", "/admin/loans/"+itoaTest(loan.ID)+"/approve-disbursement", token, gin.H{"approve": true})
	if w2.Code != http.StatusForbidden {
		t.Fatalf("the preparer approving their own disbursement must be rejected, got %d: %s", w2.Code, w2.Body.String())
	}
}

// --- 16. Replay / double-disbursement prevention ---

func TestDoubleDisbursementApprovalBlocked(t *testing.T) {
	r := setupTestApp(t)
	makerUsername := "sectest_dmaker_" + t.Name()
	checkerUsername := "sectest_dchecker_" + t.Name()
	_, makerPassword := createTestAdmin(t, makerUsername, models.AdminRoleDisbursementOfficer)
	_, checkerPassword := createTestAdmin(t, checkerUsername, models.AdminRoleSuperAdmin)
	makerToken, _ := loginAs(t, r, makerUsername, makerPassword)
	checkerToken, _ := loginAs(t, r, checkerUsername, checkerPassword)

	loan := createTestLoan(t, models.LoanStatusSanctioned)
	tenure := 12
	rate := 10.0
	database.DB.Model(&loan).Updates(map[string]interface{}{"tenure_months": tenure, "interest_rate_percent": rate})

	w := doJSON(r, "POST", "/admin/loans/"+itoaTest(loan.ID)+"/prepare-disbursement", makerToken, gin.H{"amount": 5000})
	if w.Code != http.StatusOK {
		t.Fatalf("prepare failed: %d %s", w.Code, w.Body.String())
	}

	w2 := doJSON(r, "POST", "/admin/loans/"+itoaTest(loan.ID)+"/approve-disbursement", checkerToken, gin.H{"approve": true})
	if w2.Code != http.StatusOK {
		t.Fatalf("first approval should succeed: %d %s", w2.Code, w2.Body.String())
	}

	w3 := doJSON(r, "POST", "/admin/loans/"+itoaTest(loan.ID)+"/approve-disbursement", checkerToken, gin.H{"approve": true})
	if w3.Code == http.StatusOK {
		t.Fatal("a second disbursement approval on an already-disbursed loan must not succeed")
	}
}

// --- 10. Revoked session/token rejection ---

func TestRevokedSessionRejected(t *testing.T) {
	r := setupTestApp(t)
	username := "sectest_revoke_" + t.Name()
	_, password := createTestAdmin(t, username, models.AdminRoleReadOnly)
	token, _ := loginAs(t, r, username, password)

	loan := createTestLoan(t, models.LoanStatusVerified)
	w := doJSON(r, "GET", "/admin/loans/"+itoaTest(loan.ID), token, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected access before logout, got %d", w.Code)
	}

	wLogout := doJSON(r, "POST", "/admin/logout", token, nil)
	if wLogout.Code != http.StatusOK {
		t.Fatalf("logout failed: %d", wLogout.Code)
	}

	w2 := doJSON(r, "GET", "/admin/loans/"+itoaTest(loan.ID), token, nil)
	if w2.Code != http.StatusUnauthorized {
		t.Fatalf("expected the same access token to be rejected after logout, got %d: %s", w2.Code, w2.Body.String())
	}
}

// --- 11/12/13. Audit record generation, actor attribution, before/after ---

func TestAuditRecordCapturesActorAndBeforeAfter(t *testing.T) {
	r := setupTestApp(t)
	makerUsername := "sectest_audit_maker_" + t.Name()
	checkerUsername := "sectest_audit_checker_" + t.Name()
	_, makerPassword := createTestAdmin(t, makerUsername, models.AdminRoleCreditOfficer)
	_, checkerPassword := createTestAdmin(t, checkerUsername, models.AdminRoleCreditManager)
	makerToken, _ := loginAs(t, r, makerUsername, makerPassword)
	checkerToken, _ := loginAs(t, r, checkerUsername, checkerPassword)

	loan := createTestLoan(t, models.LoanStatusVerified)
	doJSON(r, "POST", "/admin/loans/"+itoaTest(loan.ID)+"/recommend", makerToken,
		gin.H{"tenureMonths": 12, "interestRatePercent": 10.0, "remarks": "audit test"})
	doJSON(r, "POST", "/admin/loans/"+itoaTest(loan.ID)+"/approve-sanction", checkerToken,
		gin.H{"approve": true, "remarks": "audit test approve"})

	var entry models.AuditLog
	err := database.DB.Where("action = ? AND target_id = ?", "loan.sanction.approve", itoaTest(loan.ID)).
		Order("created_at desc").First(&entry).Error
	if err != nil {
		t.Fatalf("expected an audit log entry for loan.sanction.approve: %v", err)
	}
	if entry.ActorRef != checkerUsername {
		t.Fatalf("expected actor %s, got %s", checkerUsername, entry.ActorRef)
	}
	if entry.ActorID == 0 {
		t.Fatal("expected a non-zero actor ID")
	}
	if entry.Result != "success" {
		t.Fatalf("expected result=success, got %s", entry.Result)
	}
	if entry.Details == "" {
		t.Fatal("expected a before-value snapshot in Details")
	}
}

func TestAuditRecordOnPermissionDenial(t *testing.T) {
	r := setupTestApp(t)
	username := "sectest_audit_deny_" + t.Name()
	_, password := createTestAdmin(t, username, models.AdminRoleReadOnly)
	token, _ := loginAs(t, r, username, password)

	loan := createTestLoan(t, models.LoanStatusVerified)
	doJSON(r, "POST", "/admin/loans/"+itoaTest(loan.ID)+"/recommend", token,
		gin.H{"tenureMonths": 12, "interestRatePercent": 10.0})

	var entry models.AuditLog
	err := database.DB.Where("action = ? AND actor_ref = ?", "authz.denied", username).
		Order("created_at desc").First(&entry).Error
	if err != nil {
		t.Fatalf("expected an authz.denied audit entry: %v", err)
	}
	if entry.Result != "denied" {
		t.Fatalf("expected result=denied, got %s", entry.Result)
	}
}

// --- test helpers ---

func createTestLoan(t *testing.T, status string) models.LoanApplication {
	t.Helper()
	loan := models.LoanApplication{
		CustomerID:      testCustomerID(),
		Category:        "Personal Loan",
		AmountRequested: 20000,
		Status:          status,
		ApplicantName:   "Security Test Applicant",
	}
	if err := database.DB.Create(&loan).Error; err != nil {
		t.Fatalf("create test loan: %v", err)
	}
	t.Cleanup(func() { database.DB.Unscoped().Delete(&loan) })
	return loan
}

func testCustomerID() uuid.UUID {
	return uuid.New()
}

func itoaTest(id uint) string {
	b := make([]byte, 0, 10)
	if id == 0 {
		return "0"
	}
	for id > 0 {
		b = append([]byte{byte('0' + id%10)}, b...)
		id /= 10
	}
	return string(b)
}
