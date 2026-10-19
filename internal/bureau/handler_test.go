package bureau_test

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
	"github.com/santosh0921/nbfc-backend/internal/bureau"
	"github.com/santosh0921/nbfc-backend/internal/config"
	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/middleware"
	"github.com/santosh0921/nbfc-backend/internal/models"
	"github.com/santosh0921/nbfc-backend/internal/rbac"
)

// New workflow: a completed bureau pull is not enough on its own for
// underwriting to act — a credit officer with BUREAU_APPROVE must
// explicitly approve (or flag) it first. See
// internal/loans.requireApprovedBureauRequest for the enforcement side.

func setupBureauTestApp(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	if database.DB == nil {
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
	adminGroup := r.Group("/admin")
	adminGroup.Use(middleware.AdminAuthMiddleware())
	adminGroup.POST("/bureau/requests/:id/approve", middleware.RequirePermission(rbac.BureauApprove), bureau.AdminApproveHandler)
	adminGroup.POST("/bureau/requests/:id/flag", middleware.RequirePermission(rbac.BureauApprove), bureau.AdminFlagHandler)
	return r
}

func bureauCreateTestAdmin(t *testing.T, username, role string) string {
	t.Helper()
	password := "Sup3rSecure!Passw0rd_1234"
	hash, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	u := models.AdminUser{Username: username, Email: username + "@test.local", PasswordHash: string(hash), Role: role, Status: models.AdminStatusActive}
	if err := database.DB.Create(&u).Error; err != nil {
		t.Fatalf("create admin: %v", err)
	}
	t.Cleanup(func() { database.DB.Unscoped().Delete(&u) })
	return password
}

func bureauLoginAs(t *testing.T, r *gin.Engine, username, password string) string {
	t.Helper()
	w := bureauDoJSON(r, "POST", "/admin/login", "", gin.H{"username": username, "password": password})
	if w.Code != http.StatusOK {
		t.Fatalf("admin login failed: %d %s", w.Code, w.Body.String())
	}
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	return resp["token"].(string)
}

func bureauDoJSON(r *gin.Engine, method, path, token string, body interface{}) *httptest.ResponseRecorder {
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

func createCompletedBureauRequest(t *testing.T) models.BureauRequest {
	t.Helper()
	customerID := uuid.New()
	consent := models.BureauConsent{CustomerID: customerID, Bureau: models.BureauSandbox, Purpose: "test", ConsentVersion: "v0-test", ConsentedAt: time.Now()}
	if err := database.DB.Create(&consent).Error; err != nil {
		t.Fatalf("create consent: %v", err)
	}
	t.Cleanup(func() { database.DB.Unscoped().Delete(&consent) })

	req := models.BureauRequest{
		CustomerID: customerID, Bureau: models.BureauSandbox, ConsentID: consent.ID,
		Status: models.BureauRequestStatusCompleted, Score: 750, RequestedAt: time.Now(),
	}
	if err := database.DB.Create(&req).Error; err != nil {
		t.Fatalf("create bureau request: %v", err)
	}
	t.Cleanup(func() { database.DB.Unscoped().Delete(&req) })
	return req
}

func TestAdminApproveSetsReviewStatusApproved(t *testing.T) {
	r := setupBureauTestApp(t)
	pw := bureauCreateTestAdmin(t, "sectest_bureau_approve_"+t.Name(), "CREDIT_MANAGER")
	token := bureauLoginAs(t, r, "sectest_bureau_approve_"+t.Name(), pw)

	req := createCompletedBureauRequest(t)

	w := bureauDoJSON(r, "POST", "/admin/bureau/requests/"+req.ID.String()+"/approve", token, gin.H{"notes": "looks fine"})
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var reloaded models.BureauRequest
	database.DB.First(&reloaded, "id = ?", req.ID)
	if reloaded.ReviewStatus != models.BureauReviewStatusApproved {
		t.Fatalf("expected APPROVED, got %s", reloaded.ReviewStatus)
	}
	if reloaded.ReviewedByUsername == "" || reloaded.ReviewedAt == nil {
		t.Fatal("expected ReviewedByUsername/ReviewedAt to be recorded")
	}
}

func TestAdminFlagSetsReviewStatusFlagged(t *testing.T) {
	r := setupBureauTestApp(t)
	pw := bureauCreateTestAdmin(t, "sectest_bureau_flag_"+t.Name(), "CREDIT_MANAGER")
	token := bureauLoginAs(t, r, "sectest_bureau_flag_"+t.Name(), pw)

	req := createCompletedBureauRequest(t)

	w := bureauDoJSON(r, "POST", "/admin/bureau/requests/"+req.ID.String()+"/flag", token, gin.H{"notes": "looks suspicious"})
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var reloaded models.BureauRequest
	database.DB.First(&reloaded, "id = ?", req.ID)
	if reloaded.ReviewStatus != models.BureauReviewStatusFlagged {
		t.Fatalf("expected FLAGGED, got %s", reloaded.ReviewStatus)
	}
}

func TestAdminApproveDeniedWithoutPermission(t *testing.T) {
	r := setupBureauTestApp(t)
	// READ_ONLY has BUREAU_VIEW... actually has neither — holds no bureau
	// permission at all, the clearest case for "must not be able to
	// approve." (CREDIT_OFFICER was granted BUREAU_APPROVE alongside
	// CREDIT_MANAGER/SUPER_ADMIN per a later product decision, so it's no
	// longer a valid "denied" fixture here.)
	pw := bureauCreateTestAdmin(t, "sectest_bureau_noperm_"+t.Name(), "READ_ONLY")
	token := bureauLoginAs(t, r, "sectest_bureau_noperm_"+t.Name(), pw)

	req := createCompletedBureauRequest(t)

	w := bureauDoJSON(r, "POST", "/admin/bureau/requests/"+req.ID.String()+"/approve", token, nil)
	if w.Code != http.StatusForbidden {
		t.Fatalf("READ_ONLY (no BUREAU_APPROVE) must not be able to approve, got %d: %s", w.Code, w.Body.String())
	}
}

func TestAdminApproveRejectsNonCompletedRequest(t *testing.T) {
	r := setupBureauTestApp(t)
	pw := bureauCreateTestAdmin(t, "sectest_bureau_pending_"+t.Name(), "CREDIT_MANAGER")
	token := bureauLoginAs(t, r, "sectest_bureau_pending_"+t.Name(), pw)

	customerID := uuid.New()
	consent := models.BureauConsent{CustomerID: customerID, Bureau: models.BureauSandbox, Purpose: "test", ConsentVersion: "v0-test", ConsentedAt: time.Now()}
	database.DB.Create(&consent)
	t.Cleanup(func() { database.DB.Unscoped().Delete(&consent) })
	req := models.BureauRequest{
		CustomerID: customerID, Bureau: models.BureauSandbox, ConsentID: consent.ID,
		Status: models.BureauRequestStatusFailed, RequestedAt: time.Now(),
	}
	database.DB.Create(&req)
	t.Cleanup(func() { database.DB.Unscoped().Delete(&req) })

	w := bureauDoJSON(r, "POST", "/admin/bureau/requests/"+req.ID.String()+"/approve", token, nil)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("a FAILED (not COMPLETED) pull must not be approvable, got %d: %s", w.Code, w.Body.String())
	}
}
