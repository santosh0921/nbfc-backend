package reporting_test

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

	"github.com/santosh0921/nbfc-backend/internal/accounting"
	"github.com/santosh0921/nbfc-backend/internal/admin"
	"github.com/santosh0921/nbfc-backend/internal/config"
	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/middleware"
	"github.com/santosh0921/nbfc-backend/internal/models"
	"github.com/santosh0921/nbfc-backend/internal/rbac"
	"github.com/santosh0921/nbfc-backend/internal/reporting"
	"github.com/santosh0921/nbfc-backend/internal/risk"
)

// Phase 6 MIS/regulatory-reporting/compliance-monitoring regression suite.
// Reuses the real Postgres dev DB, same pattern as Phases 1-5.

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
	risk.SeedDefaultBuckets()
}

func setupTestApp(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	ensureDB(t)

	r := gin.New()
	r.POST("/admin/login", admin.LoginHandler)
	adminGroup := r.Group("/admin")
	adminGroup.Use(middleware.AdminAuthMiddleware())
	adminGroup.GET("/reporting/mis", middleware.RequirePermission(rbac.MISView), reporting.AdminMISDashboardHandler)
	adminGroup.POST("/reporting/runs/:id/review", middleware.RequirePermission(rbac.RegReportApprove), reporting.AdminReviewRunHandler)
	adminGroup.GET("/reporting/audit-center/loans/:loanId", middleware.RequirePermission(rbac.AuditCenterAccess), reporting.AdminLoanTimelineHandler)
	adminGroup.POST("/reporting/exports", middleware.RequirePermission(rbac.ReportExport), reporting.AdminRequestExportHandler)
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

func itoa(id uint) string {
	b, _ := json.Marshal(id)
	return string(b)
}

func uniqueRef(t *testing.T) string {
	return t.Name() + "-" + time.Now().Format("150405.000000000")
}

// --- 1. MIS totals match authoritative backend data ---

func TestMISLoanCountMatchesBackend(t *testing.T) {
	ensureDB(t)
	tenure := 12
	rate := 12.0
	loan := models.LoanApplication{
		CustomerID: uuid.New(), Category: "MIS_TEST_" + uniqueRef(t), Status: models.LoanStatusDisbursed,
		AmountRequested: 50000, TenureMonths: &tenure, InterestRatePercent: &rate, ApplicantName: "MIS Test",
	}
	database.DB.Create(&loan)
	t.Cleanup(func() { database.DB.Unscoped().Delete(&loan) })

	var directCount int64
	database.DB.Model(&models.LoanApplication{}).Where("category = ? AND status = ?", loan.Category, models.LoanStatusDisbursed).Count(&directCount)

	dashboard := reporting.GenerateMISDashboard(reporting.MISFilters{Product: loan.Category})
	if dashboard.Loan.Disbursed != directCount {
		t.Fatalf("MIS disbursed count (%d) must match a direct backend query (%d)", dashboard.Loan.Disbursed, directCount)
	}
}

// --- 2. Report validation detects missing/negative data ---

func TestValidationDetectsNegativeAmount(t *testing.T) {
	ensureDB(t)
	tenure := 12
	rate := 12.0
	loan := models.LoanApplication{
		CustomerID: uuid.New(), Category: "Personal Loan", Status: models.LoanStatusDisbursed,
		AmountRequested: 50000, TenureMonths: &tenure, InterestRatePercent: &rate, ApplicantName: "Validation Test",
	}
	database.DB.Create(&loan)
	inst := models.EmiInstallment{LoanID: loan.ID, InstallmentNumber: 1, DueDate: time.Now().AddDate(0, 0, -10), Amount: 5000, PrincipalComponent: 4500, InterestComponent: 500, Status: models.EmiStatusPending}
	database.DB.Create(&inst)
	badRecord := models.DPDRecord{
		LoanID: loan.ID, ScheduleID: inst.ID, DueDate: inst.DueDate,
		DueAmountPaise: 5000, PaidAmountPaise: 10000, OutstandingPaise: -5000,
		DPD: 0, CalculationDate: time.Now().AddDate(0, 0, -5000), Status: models.DPDRecordStatusCalculated,
	}
	database.DB.Create(&badRecord)
	t.Cleanup(func() {
		database.DB.Unscoped().Delete(&badRecord)
		database.DB.Unscoped().Delete(&inst)
		database.DB.Unscoped().Delete(&loan)
	})

	def, err := reporting.CreateDefinition(uniqueRef(t), "TEST_AUTHORITY", "test", "MONTHLY", "test-maker")
	if err != nil {
		t.Fatalf("CreateDefinition failed: %v", err)
	}
	version, err := reporting.CreateVersion(def.ID, "v1", "", "test-maker", nil)
	if err != nil {
		t.Fatalf("CreateVersion failed: %v", err)
	}
	run, err := reporting.GenerateRun(def.ID, version.ID, "2026-08", "", "test-maker", 1)
	if err != nil {
		t.Fatalf("GenerateRun failed: %v", err)
	}

	var exceptions []models.RegulatoryReportException
	database.DB.Where("run_id = ? AND check_name = ?", run.ID, "negative_amount").Find(&exceptions)
	if len(exceptions) == 0 {
		t.Fatal("expected the validation engine to detect the negative-outstanding DPDRecord as a blocking error")
	}
	if exceptions[0].Severity != models.ReportValidationError {
		t.Fatalf("a negative amount must be ERROR severity, got %s", exceptions[0].Severity)
	}
}

// --- 4/5. Report with blocking errors cannot be approved / maker cannot approve own report ---

func TestBlockingErrorsPreventApprovalAndMakerCannotApproveOwn(t *testing.T) {
	ensureDB(t)
	def, _ := reporting.CreateDefinition(uniqueRef(t), "TEST_AUTHORITY", "test", "MONTHLY", "maker1")
	version, _ := reporting.CreateVersion(def.ID, "v1", "", "maker1", nil)
	run, err := reporting.GenerateRun(def.ID, version.ID, "2026-08", "", "maker1", 100)
	if err != nil {
		t.Fatalf("GenerateRun failed: %v", err)
	}
	// Force a blocking error onto this run directly.
	database.DB.Create(&models.RegulatoryReportException{RunID: run.ID, Severity: models.ReportValidationError, Check: "manual_test", Detail: "forced error"})

	if err := reporting.SubmitForReview(run.ID, "maker1"); err != nil {
		t.Fatalf("SubmitForReview failed: %v", err)
	}

	// Maker cannot approve their own run (item 5), even without the error.
	_, err = reporting.ReviewRun(run.ID, 100, "maker1", models.RegReportStatusApproved, "")
	if err != reporting.ErrSameActor {
		t.Fatalf("expected ErrSameActor when the preparer reviews their own run, got %v", err)
	}

	// A different reviewer is still blocked by the unresolved ERROR (item 4).
	_, err = reporting.ReviewRun(run.ID, 200, "checker1", models.RegReportStatusApproved, "")
	if err != reporting.ErrBlockingErrors {
		t.Fatalf("expected ErrBlockingErrors, got %v", err)
	}
}

// --- 6/7/15. Approved version immutable / correct version retained / historical report reproducible ---

func TestApprovedRunVersionFrozenAndReproducible(t *testing.T) {
	ensureDB(t)
	def, _ := reporting.CreateDefinition(uniqueRef(t), "TEST_AUTHORITY", "test", "MONTHLY", "maker2")
	versionV1, _ := reporting.CreateVersion(def.ID, "v1", "original", "maker2", nil)
	run, err := reporting.GenerateRun(def.ID, versionV1.ID, "2026-08", `{"filter":"original"}`, "maker2", 300)
	if err != nil {
		t.Fatalf("GenerateRun failed: %v", err)
	}
	reporting.SubmitForReview(run.ID, "maker2")
	approved, err := reporting.ReviewRun(run.ID, 400, "checker2", models.RegReportStatusApproved, "looks good")
	if err != nil {
		t.Fatalf("ReviewRun (approve) failed: %v", err)
	}
	if approved.Status != models.RegReportStatusApproved {
		t.Fatalf("expected APPROVED, got %s", approved.Status)
	}

	// A newer version of the SAME definition must not retroactively change
	// what this already-approved run points to.
	versionV2, _ := reporting.CreateVersion(def.ID, "v2", "revised mapping", "maker2", nil)
	if versionV2.ID == versionV1.ID {
		t.Fatal("expected a distinct new version")
	}
	var reloaded models.RegulatoryReportRun
	database.DB.First(&reloaded, run.ID)
	if reloaded.DefinitionVersionID != versionV1.ID {
		t.Fatalf("an approved run's DefinitionVersionID must stay frozen at v1, got version id %d (expected %d)", reloaded.DefinitionVersionID, versionV1.ID)
	}
	if reloaded.Parameters != `{"filter":"original"}` {
		t.Fatal("an approved run's original parameters must remain exactly as generated")
	}
}

// --- 9. Unauthorized user cannot access sensitive reports ---

func TestUnauthorizedCannotAccessMIS(t *testing.T) {
	r := setupTestApp(t)
	// No token at all.
	w := doJSON(r, "GET", "/admin/reporting/mis", "", nil)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 with no token, got %d", w.Code)
	}
}

// --- 8/11. Auditor read-only / unauthorized export blocked ---

func TestAuditorCannotExport(t *testing.T) {
	r := setupTestApp(t)
	username := "reporting_auditor_" + uniqueRef(t)
	pw := createTestAdmin(t, username, models.AdminRoleAuditor)
	token := loginAs(t, r, username, pw)

	w := doJSON(r, "POST", "/admin/reporting/exports", token, gin.H{
		"reportType": "test", "format": "CSV", "headers": []string{"a"}, "rows": [][]string{{"1"}},
	})
	if w.Code != http.StatusForbidden {
		t.Fatalf("AUDITOR must not be able to request an export, got %d: %s", w.Code, w.Body.String())
	}
}

// --- 10/12. Export creates an audit event and runs asynchronously ---

func TestExportCreatesAuditEventAndCompletesAsync(t *testing.T) {
	r := setupTestApp(t)
	username := "reporting_super_" + uniqueRef(t)
	pw := createTestAdmin(t, username, models.AdminRoleSuperAdmin)
	token := loginAs(t, r, username, pw)

	w := doJSON(r, "POST", "/admin/reporting/exports", token, gin.H{
		"reportType": "test_export", "format": "CSV", "headers": []string{"col1", "col2"}, "rows": [][]string{{"a", "b"}},
	})
	if w.Code != http.StatusAccepted {
		t.Fatalf("expected 202 Accepted, got %d: %s", w.Code, w.Body.String())
	}
	var export models.ReportExport
	json.Unmarshal(w.Body.Bytes(), &export)
	t.Cleanup(func() { database.DB.Unscoped().Delete(&export) })

	var auditCount int64
	database.DB.Model(&models.AuditLog{}).
		Where("action = ? AND target_id = ?", "report.export.requested", itoa(export.ID)).Count(&auditCount)
	if auditCount == 0 {
		t.Fatal("expected an audit event for the export request")
	}

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		var reloaded models.ReportExport
		database.DB.First(&reloaded, export.ID)
		if reloaded.Status == models.ExportStatusReady || reloaded.Status == models.ExportStatusFailed {
			if reloaded.Status == models.ExportStatusFailed {
				t.Logf("export finished as FAILED (likely no storage provider configured in this test env): %s", reloaded.FailureReason)
			}
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("export did not complete (QUEUED/RUNNING) within timeout — async generation appears stuck")
}

// --- 13. Reconciliation detects mismatched balances ---

func TestReconciliationDetectsLoanWithoutClassification(t *testing.T) {
	ensureDB(t)
	tenure := 12
	rate := 12.0
	loan := models.LoanApplication{
		CustomerID: uuid.New(), Category: "Personal Loan", Status: models.LoanStatusDisbursed,
		AmountRequested: 50000, TenureMonths: &tenure, InterestRatePercent: &rate, ApplicantName: "Recon Test",
	}
	database.DB.Create(&loan)
	inst := models.EmiInstallment{LoanID: loan.ID, InstallmentNumber: 1, DueDate: time.Now().AddDate(0, 0, -120), Amount: 5000, PrincipalComponent: 4500, InterestComponent: 500, Status: models.EmiStatusPending}
	database.DB.Create(&inst)
	dpd := models.DPDRecord{
		LoanID: loan.ID, ScheduleID: inst.ID, DueDate: inst.DueDate,
		DueAmountPaise: 5000, PaidAmountPaise: 0, OutstandingPaise: 5000,
		DPD: 120, CalculationDate: time.Now(), Status: models.DPDRecordStatusCalculated,
	}
	database.DB.Create(&dpd)
	t.Cleanup(func() {
		database.DB.Unscoped().Delete(&dpd)
		database.DB.Unscoped().Delete(&inst)
		database.DB.Unscoped().Delete(&loan)
		database.DB.Unscoped().Where("entity = ? AND reference = ?", "loan", itoa(loan.ID)).Delete(&models.CrossSystemReconciliation{})
	})

	reporting.RunCrossSystemChecks()

	var found models.CrossSystemReconciliation
	err := database.DB.Where("entity = ? AND reference = ? AND status = ?", "loan", itoa(loan.ID), models.ReconStatusMissing).First(&found).Error
	if err != nil {
		t.Fatalf("expected a MISSING reconciliation finding for a 120-DPD loan with no classification: %v", err)
	}
}

// --- 14. Compliance task lifecycle works ---

func TestComplianceTaskLifecycle(t *testing.T) {
	ensureDB(t)
	task, err := reporting.CreateTask(models.TaskCategoryDataQuality, models.TaskSeverityMedium, "test task", uniqueRef(t), "system", nil)
	if err != nil {
		t.Fatalf("CreateTask failed: %v", err)
	}
	t.Cleanup(func() { database.DB.Unscoped().Delete(task) })

	if err := reporting.AssignTask(task.ID, 999, "test-officer", "test-manager"); err != nil {
		t.Fatalf("AssignTask failed: %v", err)
	}
	var reloaded models.ComplianceTask
	database.DB.First(&reloaded, task.ID)
	if reloaded.Status != models.TaskStatusAssigned {
		t.Fatalf("expected ASSIGNED after assignment, got %s", reloaded.Status)
	}

	if err := reporting.UpdateTaskStatus(task.ID, models.TaskStatusResolved, "fixed", "test-officer"); err != nil {
		t.Fatalf("UpdateTaskStatus failed: %v", err)
	}
	database.DB.First(&reloaded, task.ID)
	if reloaded.Status != models.TaskStatusResolved || reloaded.ClosedAt == nil {
		t.Fatal("expected RESOLVED status with ClosedAt set")
	}
}

// --- 16. Retention configuration cannot delete protected records accidentally ---

func TestRetentionConfigurationNeverDeletes(t *testing.T) {
	ensureDB(t)
	var before int64
	database.DB.Model(&models.AuditLog{}).Count(&before)

	_, err := reporting.ConfigureRetentionPolicy("audit_logs", 8, "placeholder — CA to confirm", "test-compliance")
	if err != nil {
		t.Fatalf("ConfigureRetentionPolicy failed: %v", err)
	}
	if err := reporting.ValidateRetentionPolicy("audit_logs", "test-compliance-officer"); err != nil {
		t.Fatalf("ValidateRetentionPolicy failed: %v", err)
	}

	var after int64
	database.DB.Model(&models.AuditLog{}).Count(&after)
	if after < before {
		t.Fatal("configuring/validating a retention policy must never delete existing audit log rows")
	}

	var policy models.RetentionPolicy
	database.DB.Where("record_category = ?", "audit_logs").First(&policy)
	if policy.Status != models.RetentionStatusValidated {
		t.Fatal("expected the policy to be marked VALIDATED after ValidateRetentionPolicy")
	}
}
