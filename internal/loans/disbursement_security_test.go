package loans_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

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

// Phase 2 disbursement maker-checker + confirmation regression suite.
// Reuses the real Postgres dev DB, same as internal/admin/security_test.go
// (Phase 1). This suite specifically exercises the rework where checker
// approval no longer finalizes a loan as DISBURSED by itself — only
// loans.ConfirmDisbursement (called here directly, standing in for what
// internal/reconciliation calls once a bank statement match is found)
// does that.

func setupDisbursementTestApp(t *testing.T) *gin.Engine {
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
	adminGroup.POST("/loans/:id/prepare-disbursement", middleware.RequirePermission(rbac.LoanDisbursePrep), loans.AdminPrepareDisbursementHandler)
	adminGroup.POST("/loans/:id/approve-disbursement", middleware.RequirePermission(rbac.LoanDisburse), loans.AdminApproveDisbursementHandler)
	adminGroup.GET("/loans/:id", middleware.RequirePermission(rbac.LoanView), loans.AdminGetLoanHandler)
	return r
}

func createDisbTestAdmin(t *testing.T, username, role string) (id uint, password string) {
	t.Helper()
	password = "Sup3rSecure!Passw0rd_1234"
	hash, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	u := models.AdminUser{Username: username, Email: username + "@test.local", PasswordHash: string(hash), Role: role, Status: models.AdminStatusActive}
	database.DB.Create(&u)
	t.Cleanup(func() { database.DB.Unscoped().Delete(&u) })
	return u.ID, password
}

func disbDoJSON(r *gin.Engine, method, path, token string, body interface{}) *httptest.ResponseRecorder {
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

func disbLoginAs(t *testing.T, r *gin.Engine, username, password string) string {
	t.Helper()
	w := disbDoJSON(r, "POST", "/admin/login", "", gin.H{"username": username, "password": password})
	if w.Code != http.StatusOK {
		t.Fatalf("login failed: %d %s", w.Code, w.Body.String())
	}
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	return resp["token"].(string)
}

func createSanctionedLoan(t *testing.T, amount float64) models.LoanApplication {
	t.Helper()
	tenure := 12
	rate := 10.0
	loan := models.LoanApplication{
		CustomerID: uuid.New(), Category: "Personal Loan", AmountRequested: amount,
		Status: models.LoanStatusSanctioned, TenureMonths: &tenure, InterestRatePercent: &rate,
		ApplicantName: "Disb Test",
	}
	database.DB.Create(&loan)
	t.Cleanup(func() { database.DB.Unscoped().Delete(&loan) })
	return loan
}

func idStr(id uint) string {
	b, _ := json.Marshal(id)
	return string(b)
}

// --- 9. Duplicate disbursement -> rejected ---

func TestDuplicateDisbursementPrepareRejected(t *testing.T) {
	r := setupDisbursementTestApp(t)
	_, pw := createDisbTestAdmin(t, "disb_dup_"+t.Name(), models.AdminRoleSuperAdmin)
	token := disbLoginAs(t, r, "disb_dup_"+t.Name(), pw)

	loan := createSanctionedLoan(t, 20000)
	w1 := disbDoJSON(r, "POST", "/admin/loans/"+idStr(loan.ID)+"/prepare-disbursement", token, gin.H{"amount": 20000})
	if w1.Code != http.StatusOK {
		t.Fatalf("first prepare should succeed: %d %s", w1.Code, w1.Body.String())
	}
	// Second prepare on the same loan (still ready_for_disbursement, not sanctioned) must be rejected.
	w2 := disbDoJSON(r, "POST", "/admin/loans/"+idStr(loan.ID)+"/prepare-disbursement", token, gin.H{"amount": 20000})
	if w2.Code == http.StatusOK {
		t.Fatal("a loan already prepared for disbursement must not be preparable again")
	}
}

// --- 10. Disbursement without approval -> rejected (loan never reaches DISBURSED) ---

func TestDisbursementWithoutApprovalNeverFinalizes(t *testing.T) {
	r := setupDisbursementTestApp(t)
	_, pw := createDisbTestAdmin(t, "disb_noappr_"+t.Name(), models.AdminRoleSuperAdmin)
	token := disbLoginAs(t, r, "disb_noappr_"+t.Name(), pw)

	loan := createSanctionedLoan(t, 20000)
	disbDoJSON(r, "POST", "/admin/loans/"+idStr(loan.ID)+"/prepare-disbursement", token, gin.H{"amount": 20000})

	w := disbDoJSON(r, "GET", "/admin/loans/"+idStr(loan.ID), token, nil)
	var reloaded models.LoanApplication
	json.Unmarshal(w.Body.Bytes(), &reloaded)
	if reloaded.Status == models.LoanStatusDisbursed {
		t.Fatal("a merely-prepared disbursement must never reach DISBURSED without a checker's approval")
	}
}

// --- 11. Maker attempting own approval -> rejected ---

func TestDisbursementMakerCannotApproveOwn(t *testing.T) {
	r := setupDisbursementTestApp(t)
	username := "disb_selfappr_" + t.Name()
	_, pw := createDisbTestAdmin(t, username, models.AdminRoleSuperAdmin)
	token := disbLoginAs(t, r, username, pw)

	loan := createSanctionedLoan(t, 20000)
	disbDoJSON(r, "POST", "/admin/loans/"+idStr(loan.ID)+"/prepare-disbursement", token, gin.H{"amount": 20000})

	w := disbDoJSON(r, "POST", "/admin/loans/"+idStr(loan.ID)+"/approve-disbursement", token, gin.H{"approve": true})
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for self-approval, got %d: %s", w.Code, w.Body.String())
	}
}

// --- Phase 8: OPERATIONS can act as checker; DISBURSEMENT_OFFICER (prep-only) cannot ---

func TestOperationsCanApproveDisbursementPreparedByDisbursementOfficer(t *testing.T) {
	r := setupDisbursementTestApp(t)
	makerU, checkerU := "disb_prep_"+t.Name(), "disb_ops_"+t.Name()
	_, makerPw := createDisbTestAdmin(t, makerU, models.AdminRoleDisbursementOfficer)
	_, checkerPw := createDisbTestAdmin(t, checkerU, models.AdminRoleOperations)
	makerToken := disbLoginAs(t, r, makerU, makerPw)
	checkerToken := disbLoginAs(t, r, checkerU, checkerPw)

	loan := createSanctionedLoan(t, 20000)
	w0 := disbDoJSON(r, "POST", "/admin/loans/"+idStr(loan.ID)+"/prepare-disbursement", makerToken, gin.H{"amount": 20000})
	if w0.Code != http.StatusOK {
		t.Fatalf("prepare failed: %d %s", w0.Code, w0.Body.String())
	}

	w := disbDoJSON(r, "POST", "/admin/loans/"+idStr(loan.ID)+"/approve-disbursement", checkerToken, gin.H{"approve": true})
	if w.Code != http.StatusOK {
		t.Fatalf("OPERATIONS should now be able to act as checker for a disbursement it did not prepare, got %d: %s", w.Code, w.Body.String())
	}
}

func TestDisbursementOfficerCannotApproveAnyDisbursement(t *testing.T) {
	r := setupDisbursementTestApp(t)
	makerU, otherU := "disb_prep2_"+t.Name(), "disb_off2_"+t.Name()
	_, makerPw := createDisbTestAdmin(t, makerU, models.AdminRoleDisbursementOfficer)
	_, otherPw := createDisbTestAdmin(t, otherU, models.AdminRoleDisbursementOfficer)
	makerToken := disbLoginAs(t, r, makerU, makerPw)
	otherToken := disbLoginAs(t, r, otherU, otherPw)

	loan := createSanctionedLoan(t, 20000)
	disbDoJSON(r, "POST", "/admin/loans/"+idStr(loan.ID)+"/prepare-disbursement", makerToken, gin.H{"amount": 20000})

	// A second DISBURSEMENT_OFFICER (not the same person, so maker!=checker
	// is satisfied) must still be rejected — DISBURSEMENT_OFFICER only ever
	// holds LoanDisbursePrep, never LoanDisburse, by RBAC design.
	w := disbDoJSON(r, "POST", "/admin/loans/"+idStr(loan.ID)+"/approve-disbursement", otherToken, gin.H{"approve": true})
	if w.Code != http.StatusForbidden {
		t.Fatalf("DISBURSEMENT_OFFICER lacks LoanDisburse and must be denied regardless of identity, got %d: %s", w.Code, w.Body.String())
	}
}

// --- Approval only moves to DISBURSEMENT_PROCESSING, not DISBURSED ---

func TestApprovalAloneDoesNotFinalizeDisbursement(t *testing.T) {
	r := setupDisbursementTestApp(t)
	makerU, checkerU := "disb_maker_"+t.Name(), "disb_checker_"+t.Name()
	_, makerPw := createDisbTestAdmin(t, makerU, models.AdminRoleDisbursementOfficer)
	_, checkerPw := createDisbTestAdmin(t, checkerU, models.AdminRoleSuperAdmin)
	makerToken := disbLoginAs(t, r, makerU, makerPw)
	checkerToken := disbLoginAs(t, r, checkerU, checkerPw)

	loan := createSanctionedLoan(t, 20000)
	disbDoJSON(r, "POST", "/admin/loans/"+idStr(loan.ID)+"/prepare-disbursement", makerToken, gin.H{"amount": 20000})
	w := disbDoJSON(r, "POST", "/admin/loans/"+idStr(loan.ID)+"/approve-disbursement", checkerToken, gin.H{"approve": true})
	if w.Code != http.StatusOK {
		t.Fatalf("approval should succeed: %d %s", w.Code, w.Body.String())
	}

	var reloaded models.LoanApplication
	database.DB.First(&reloaded, loan.ID)
	if reloaded.Status != models.LoanStatusDisbursementProcessing {
		t.Fatalf("expected DISBURSEMENT_PROCESSING after approval alone, got %s", reloaded.Status)
	}

	var disb models.Disbursement
	if err := database.DB.Where("loan_id = ?", loan.ID).First(&disb).Error; err != nil {
		t.Fatalf("expected a Disbursement record: %v", err)
	}
	if disb.Status != models.DisbursementStatusInitiated {
		t.Fatalf("expected disbursement INITIATED, got %s", disb.Status)
	}

	// --- 14. Successful disbursement -> UTR captured (via ConfirmDisbursement) ---
	if err := loans.ConfirmDisbursement(disb.ID, "UTR123456789", "test-reconciler", nil); err != nil {
		t.Fatalf("ConfirmDisbursement failed: %v", err)
	}
	database.DB.First(&reloaded, loan.ID)
	if reloaded.Status != models.LoanStatusDisbursed {
		t.Fatalf("expected DISBURSED after confirmation, got %s", reloaded.Status)
	}
	database.DB.First(&disb, disb.ID)
	if disb.UTR != "UTR123456789" {
		t.Fatalf("expected UTR captured, got %q", disb.UTR)
	}

	// --- 13. Duplicate confirmation -> idempotent, not an error ---
	if err := loans.ConfirmDisbursement(disb.ID, "UTR123456789", "test-reconciler", nil); err != nil {
		t.Fatalf("a repeat confirmation of an already-SUCCESS disbursement must be a no-op, got error: %v", err)
	}
}

// --- 18b. Unauthorized user cannot access disbursement operations ---

func TestUnauthorizedDisbursementAccessBlocked(t *testing.T) {
	r := setupDisbursementTestApp(t)
	username := "disb_readonly_" + t.Name()
	_, pw := createDisbTestAdmin(t, username, models.AdminRoleReadOnly)
	token := disbLoginAs(t, r, username, pw)

	loan := createSanctionedLoan(t, 20000)
	w := disbDoJSON(r, "POST", "/admin/loans/"+idStr(loan.ID)+"/prepare-disbursement", token, gin.H{"amount": 20000})
	if w.Code != http.StatusForbidden {
		t.Fatalf("READ_ONLY role must not be able to prepare a disbursement, got %d", w.Code)
	}

	w2 := disbDoJSON(r, "POST", "/admin/loans/"+idStr(loan.ID)+"/prepare-disbursement", "", gin.H{"amount": 20000})
	if w2.Code != http.StatusUnauthorized {
		t.Fatalf("no token must be rejected, got %d", w2.Code)
	}
}
