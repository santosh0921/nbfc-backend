package compliance_test

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

	"github.com/santosh0921/nbfc-backend/internal/admin"
	"github.com/santosh0921/nbfc-backend/internal/compliance"
	"github.com/santosh0921/nbfc-backend/internal/config"
	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/middleware"
	"github.com/santosh0921/nbfc-backend/internal/models"
	"github.com/santosh0921/nbfc-backend/internal/rbac"
	"github.com/santosh0921/nbfc-backend/internal/storage"
)

// Phase 11 compliance-management regression suite.

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
	storage.Initialize()
}

func setupApp(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	ensureDB(t)

	r := gin.New()
	r.POST("/admin/login", admin.LoginHandler)
	adminGroup := r.Group("/admin")
	adminGroup.Use(middleware.AdminAuthMiddleware())
	adminGroup.POST("/compliance/returns", middleware.RequirePermission(rbac.ComplianceManage), compliance.AdminCreateReturnHandler)
	adminGroup.POST("/compliance/returns/:id/approve", middleware.RequirePermission(rbac.ComplianceApprove), compliance.AdminApproveReturnHandler)
	adminGroup.GET("/compliance/dashboard", middleware.RequirePermission(rbac.ComplianceView), compliance.AdminDashboardHandler)
	adminGroup.GET("/compliance/evidence/:id/download", middleware.RequirePermission(rbac.ComplianceView), compliance.AdminDownloadEvidenceHandler)
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

func itoaC(id uint) string {
	b, _ := json.Marshal(id)
	return string(b)
}

// --- Maker cannot approve own regulatory return ---

func TestReturnMakerCannotApproveOwn(t *testing.T) {
	r := setupApp(t)
	username := "compliance_maker_" + uniqueRef(t)
	pw := createTestAdmin(t, username, models.AdminRoleSuperAdmin)
	token := loginAs(t, r, username, pw)

	w := doJSON(r, "POST", "/admin/compliance/returns", token, gin.H{
		"regulator": "TEST-REGULATOR", "returnType": "TestReturn_" + uniqueRef(t), "reportingPeriod": "2026-Q1",
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("return creation should succeed: %d %s", w.Code, w.Body.String())
	}
	var ret models.RegulatoryReturnTracking
	json.Unmarshal(w.Body.Bytes(), &ret)
	t.Cleanup(func() { database.DB.Unscoped().Delete(&ret) })

	wApprove := doJSON(r, "POST", "/admin/compliance/returns/"+itoaC(ret.ID)+"/approve", token, nil)
	if wApprove.Code != http.StatusForbidden {
		t.Fatalf("the admin who prepared this return must not be able to approve it, got %d: %s", wApprove.Code, wApprove.Body.String())
	}
}

// --- Approval blocked by open data exceptions (item F) ---

func TestReturnApprovalBlockedByOpenExceptions(t *testing.T) {
	ensureDB(t)
	ret, err := compliance.CreateReturn(&models.RegulatoryReturnTracking{
		Regulator: "TEST-REGULATOR", ReturnType: "TestReturn_" + uniqueRef(t), ReportingPeriod: "2026-Q1",
	}, 1, "maker")
	if err != nil {
		t.Fatalf("CreateReturn failed: %v", err)
	}
	t.Cleanup(func() { database.DB.Unscoped().Delete(ret) })

	exc, _ := compliance.RecordDataException(ret.ID, "MISSING_RECORDS", "test exception")
	t.Cleanup(func() { database.DB.Unscoped().Delete(exc) })

	_, err = compliance.ReviewAndApproveReturn(ret.ID, 2, "checker", "ok")
	if err != compliance.ErrHasOpenExceptions {
		t.Fatalf("expected ErrHasOpenExceptions while an exception is open, got %v", err)
	}

	if err := compliance.ResolveDataException(exc.ID, "tester"); err != nil {
		t.Fatalf("ResolveDataException failed: %v", err)
	}
	_, err = compliance.ReviewAndApproveReturn(ret.ID, 2, "checker", "ok")
	if err != nil {
		t.Fatalf("expected approval to succeed once the exception is resolved: %v", err)
	}
}

// --- Finding cannot be closed without verification evidence ---

func TestFindingCannotCloseWithoutEvidence(t *testing.T) {
	ensureDB(t)
	audit, _ := compliance.CreateAudit(&models.InternalAudit{AuditType: "Test Audit"}, "system")
	t.Cleanup(func() { database.DB.Unscoped().Delete(audit) })
	finding, err := compliance.CreateFinding(&models.AuditFinding{
		AuditID: audit.ID, Title: "Test Finding " + uniqueRef(t), Severity: models.FindingSeverityHigh,
	}, "system")
	if err != nil {
		t.Fatalf("CreateFinding failed: %v", err)
	}
	t.Cleanup(func() { database.DB.Unscoped().Delete(finding) })

	err = compliance.CloseFinding(finding.ID, "", "", "system")
	if err != compliance.ErrClosureRequiresEvidence {
		t.Fatalf("expected ErrClosureRequiresEvidence with no evidence/verifier, got %v", err)
	}

	err = compliance.CloseFinding(finding.ID, "evidence-doc-ref", "verifier_name", "system")
	if err != nil {
		t.Fatalf("expected close to succeed with evidence+verifier: %v", err)
	}
}

// --- Policy cannot be created without evidence (item J) ---

func TestPolicyRequiresEvidence(t *testing.T) {
	ensureDB(t)
	_, err := compliance.CreatePolicy(&models.Policy{Name: "Test Policy " + uniqueRef(t)}, "system")
	if err == nil {
		t.Fatal("a policy must not be creatable without a linked evidence document")
	}
}

// --- Evidence version control: new version supersedes, never overwrites ---

func TestEvidenceVersioning(t *testing.T) {
	ensureDB(t)
	if storage.Active() == nil {
		t.Skip("no storage provider configured in this environment — skipping evidence upload test")
	}
	v1, err := compliance.UploadEvidence(nil, nil, "Test Evidence "+uniqueRef(t), "Policy", []byte("version 1 content"), "text/plain", 1, "system")
	if err != nil {
		t.Fatalf("UploadEvidence failed: %v", err)
	}
	t.Cleanup(func() { database.DB.Unscoped().Delete(v1) })

	v2, err := compliance.UploadNewVersion(v1.ID, []byte("version 2 content"), "text/plain", "corrected typo", 1, "system")
	if err != nil {
		t.Fatalf("UploadNewVersion failed: %v", err)
	}
	t.Cleanup(func() { database.DB.Unscoped().Delete(v2) })

	if v2.Version != 2 || v2.PreviousVersionID == nil || *v2.PreviousVersionID != v1.ID {
		t.Fatalf("expected v2 to be version 2 linking back to v1, got version=%d previous=%v", v2.Version, v2.PreviousVersionID)
	}

	var reloadedV1 models.ComplianceEvidence
	database.DB.First(&reloadedV1, v1.ID)
	if reloadedV1.Status != models.EvidenceStatusSuperseded {
		t.Fatalf("expected v1 to be marked SUPERSEDED (never deleted), got %s", reloadedV1.Status)
	}

	// The old version's content must still be retrievable.
	_, data, err := compliance.RetrieveEvidence(v1.ID)
	if err != nil {
		t.Fatalf("superseded evidence must remain retrievable: %v", err)
	}
	if string(data) != "version 1 content" {
		t.Fatal("superseded evidence content must be unchanged, never overwritten")
	}
}

// --- Unauthorized user cannot access the compliance dashboard ---

func TestUnauthorizedCannotAccessDashboard(t *testing.T) {
	r := setupApp(t)
	username := "compliance_noaccess_" + uniqueRef(t)
	pw := createTestAdmin(t, username, models.AdminRoleCollectionOfficer)
	token := loginAs(t, r, username, pw)

	w := doJSON(r, "GET", "/admin/compliance/dashboard", token, nil)
	if w.Code != http.StatusForbidden {
		t.Fatalf("COLLECTION_OFFICER must not have ComplianceView, got %d: %s", w.Code, w.Body.String())
	}
}

// --- Requirement without SourceReference cannot be activated (item AB) ---

func TestRequirementWithoutSourceCannotActivate(t *testing.T) {
	ensureDB(t)
	req, err := compliance.CreateRequirement(&models.ComplianceRequirement{
		Title: "Test Requirement " + uniqueRef(t), Regulator: "TEST-REGULATOR",
	}, "system")
	if err != nil {
		t.Fatalf("CreateRequirement failed: %v", err)
	}
	t.Cleanup(func() { database.DB.Unscoped().Delete(req) })

	_, err = compliance.ActivateRequirement(req.ID, "system")
	if err != compliance.ErrSourceRequired {
		t.Fatalf("expected ErrSourceRequired, got %v", err)
	}
}
