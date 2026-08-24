package tax_test

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
	"github.com/santosh0921/nbfc-backend/internal/middleware"
	"github.com/santosh0921/nbfc-backend/internal/models"
	"github.com/santosh0921/nbfc-backend/internal/rbac"
	"github.com/santosh0921/nbfc-backend/internal/tax"
)

// Phase 10 TDS/tax-rule-engine regression suite. Reuses the real
// Postgres dev DB, same pattern as Phases 1-9. No test in this file
// invents or assumes a real section number/rate/threshold — every rule
// fixture uses an obviously-synthetic value ("TEST-SECTION", 10%) purely
// to exercise the engine's arithmetic/versioning logic, never presented
// as a real applicable rate.

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
	adminGroup.POST("/tax/rules", middleware.RequirePermission(rbac.TaxRuleManage), tax.AdminDraftRuleHandler)
	adminGroup.POST("/tax/rules/:id/approve", middleware.RequirePermission(rbac.TaxRuleApprove), tax.AdminApproveRuleHandler)
	adminGroup.GET("/tax/dashboard", middleware.RequirePermission(rbac.TaxView), tax.AdminTaxDashboardHandler)
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

func approvedTestRule(t *testing.T, nature string, thresholdPaise int64, ratePercent float64, from time.Time, to *time.Time) models.TDSRule {
	t.Helper()
	rule := &models.TDSRule{
		TaxAct: "TEST-ACT", SectionRef: "TEST-SECTION", RuleVersion: uniqueRef(t),
		NatureOfPayment: nature, ThresholdPaise: thresholdPaise, RatePercent: ratePercent,
		EffectiveFrom: from, EffectiveTo: to, SourceReference: "unit-test fixture, not a real legal source",
	}
	created, err := tax.DraftRule(rule, "maker_"+uniqueRef(t))
	if err != nil {
		t.Fatalf("DraftRule failed: %v", err)
	}
	approved, err := tax.ApproveRule(created.ID, 1, "checker_"+uniqueRef(t))
	if err != nil {
		t.Fatalf("ApproveRule failed: %v", err)
	}
	t.Cleanup(func() { database.DB.Unscoped().Delete(approved) })
	return *approved
}

// --- 1. Rule selection by effective date ---

func TestRuleSelectionByEffectiveDate(t *testing.T) {
	ensureDB(t)
	nature := "TestNature_" + uniqueRef(t)
	cutover := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)

	oldRule := approvedTestRule(t, nature, 0, 5, time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC), &cutover)
	newRule := approvedTestRule(t, nature, 0, 8, cutover, nil)

	before := time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC)
	after := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)

	selectedBefore, err := tax.SelectApplicableRule(nature, before)
	if err != nil || selectedBefore == nil || selectedBefore.ID != oldRule.ID {
		t.Fatalf("expected the pre-cutover rule to apply for a transaction dated %v, got %v (err=%v)", before, selectedBefore, err)
	}
	selectedAfter, err := tax.SelectApplicableRule(nature, after)
	if err != nil || selectedAfter == nil || selectedAfter.ID != newRule.ID {
		t.Fatalf("expected the post-cutover rule to apply for a transaction dated %v, got %v (err=%v)", after, selectedAfter, err)
	}
}

// --- No rule -> ErrNoApplicableRule, never a guessed default ---

func TestNoRuleReturnsExplicitError(t *testing.T) {
	ensureDB(t)
	_, err := tax.CalculateTDS(tax.CalculateInput{
		ReferenceType: "test", ReferenceID: uniqueRef(t), DeducteeName: "Nobody",
		GrossAmountPaise: 100000, NatureOfPayment: "NoSuchNature_" + uniqueRef(t),
		PANStatus: models.PANStatusValid, TransactionDate: time.Now(), Actor: "system",
	})
	if err != tax.ErrNoApplicableRule {
		t.Fatalf("expected ErrNoApplicableRule when no rule matches, got %v", err)
	}
}

// --- Threshold + rate calculation ---

func TestThresholdAndRateCalculation(t *testing.T) {
	ensureDB(t)
	nature := "TestNature_" + uniqueRef(t)
	approvedTestRule(t, nature, 5000000, 10, time.Now().AddDate(-1, 0, 0), nil)

	below, err := tax.CalculateTDS(tax.CalculateInput{
		ReferenceType: "test", ReferenceID: uniqueRef(t), DeducteeName: "Vendor",
		GrossAmountPaise: 3000000, NatureOfPayment: nature, PANStatus: models.PANStatusValid,
		TransactionDate: time.Now(), Actor: "system",
	})
	if err != nil {
		t.Fatalf("CalculateTDS failed: %v", err)
	}
	if below.Status != models.TDSStatusNotApplicable || below.TDSAmountPaise != 0 {
		t.Fatalf("below-threshold amount should be NOT_APPLICABLE with 0 TDS, got status=%s amount=%d", below.Status, below.TDSAmountPaise)
	}

	above, err := tax.CalculateTDS(tax.CalculateInput{
		ReferenceType: "test", ReferenceID: uniqueRef(t), DeducteeName: "Vendor",
		GrossAmountPaise: 10000000, NatureOfPayment: nature, PANStatus: models.PANStatusValid,
		TransactionDate: time.Now(), Actor: "system",
	})
	if err != nil {
		t.Fatalf("CalculateTDS failed: %v", err)
	}
	if above.Status != models.TDSStatusApplicable || above.TDSAmountPaise != 1000000 {
		t.Fatalf("expected APPLICABLE with TDS 1000000 paise (10%% of 10000000), got status=%s amount=%d", above.Status, above.TDSAmountPaise)
	}
	if above.NetPayablePaise != 9000000 {
		t.Fatalf("expected net payable 9000000, got %d", above.NetPayablePaise)
	}
}

// --- PAN-missing scenario applies the higher no-PAN rate ---

func TestNoPANRateApplied(t *testing.T) {
	ensureDB(t)
	nature := "TestNature_" + uniqueRef(t)
	rule := &models.TDSRule{
		TaxAct: "TEST-ACT", SectionRef: "TEST-SECTION", RuleVersion: uniqueRef(t),
		NatureOfPayment: nature, RatePercent: 10, RequiresPAN: true, NoPANRatePercent: 20,
		EffectiveFrom: time.Now().AddDate(-1, 0, 0), SourceReference: "unit-test fixture",
	}
	created, _ := tax.DraftRule(rule, "maker_"+uniqueRef(t))
	approved, _ := tax.ApproveRule(created.ID, 1, "checker_"+uniqueRef(t))
	t.Cleanup(func() { database.DB.Unscoped().Delete(approved) })

	txn, err := tax.CalculateTDS(tax.CalculateInput{
		ReferenceType: "test", ReferenceID: uniqueRef(t), DeducteeName: "No PAN Vendor",
		GrossAmountPaise: 1000000, NatureOfPayment: nature, PANStatus: models.PANStatusNotProvided,
		TransactionDate: time.Now(), Actor: "system",
	})
	if err != nil {
		t.Fatalf("CalculateTDS failed: %v", err)
	}
	if txn.RatePercent != 20 || txn.TDSAmountPaise != 200000 {
		t.Fatalf("expected the no-PAN rate (20%%, 200000 paise) to apply, got rate=%v amount=%d", txn.RatePercent, txn.TDSAmountPaise)
	}
}

// --- Maker cannot approve own rule ---

func TestMakerCannotApproveOwnRule(t *testing.T) {
	r := setupApp(t)
	username := "tax_maker_" + uniqueRef(t)
	pw := createTestAdmin(t, username, models.AdminRoleSuperAdmin)
	token := loginAs(t, r, username, pw)

	w := doJSON(r, "POST", "/admin/tax/rules", token, gin.H{
		"taxAct": "TEST-ACT", "sectionRef": "TEST-SECTION", "ruleVersion": uniqueRef(t),
		"natureOfPayment": "TestNature_" + uniqueRef(t), "ratePercent": 10,
		"effectiveFrom": time.Now().AddDate(-1, 0, 0).Format(time.RFC3339),
		"sourceReference": "unit-test fixture",
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("draft should succeed: %d %s", w.Code, w.Body.String())
	}
	var rule models.TDSRule
	json.Unmarshal(w.Body.Bytes(), &rule)
	t.Cleanup(func() { database.DB.Unscoped().Delete(&rule) })

	wApprove := doJSON(r, "POST", "/admin/tax/rules/"+itoaT(rule.ID)+"/approve", token, nil)
	if wApprove.Code != http.StatusForbidden {
		t.Fatalf("the admin who drafted this rule must not be able to approve it, got %d: %s", wApprove.Code, wApprove.Body.String())
	}
}

// --- Rule without SourceReference rejected ---

func TestRuleWithoutSourceReferenceRejected(t *testing.T) {
	ensureDB(t)
	rule := &models.TDSRule{
		TaxAct: "TEST-ACT", SectionRef: "TEST-SECTION", RuleVersion: uniqueRef(t),
		NatureOfPayment: "TestNature_" + uniqueRef(t), RatePercent: 10, EffectiveFrom: time.Now(),
	}
	_, err := tax.DraftRule(rule, "system")
	if err != tax.ErrSourceRequired {
		t.Fatalf("expected ErrSourceRequired for a rule with no SourceReference, got %v", err)
	}
}

// --- Duplicate deduction prevented (already-DEDUCTED cannot be posted twice) ---

func TestDuplicateDeductionPrevented(t *testing.T) {
	ensureDB(t)
	nature := "TestNature_" + uniqueRef(t)
	approvedTestRule(t, nature, 0, 10, time.Now().AddDate(-1, 0, 0), nil)

	txn, err := tax.CalculateTDS(tax.CalculateInput{
		ReferenceType: "test", ReferenceID: uniqueRef(t), DeducteeName: "Vendor",
		GrossAmountPaise: 1000000, NatureOfPayment: nature, PANStatus: models.PANStatusValid,
		TransactionDate: time.Now(), Actor: "system",
	})
	if err != nil {
		t.Fatalf("CalculateTDS failed: %v", err)
	}

	first, err := tax.PostTDSDeduction(txn.ID, accounting.AcctOperatingExpenses, "system")
	if err != nil {
		t.Fatalf("first PostTDSDeduction should succeed: %v", err)
	}
	if first.JournalReference == "" {
		t.Fatal("expected a journal reference after posting")
	}

	_, err = tax.PostTDSDeduction(txn.ID, accounting.AcctOperatingExpenses, "system")
	if err != tax.ErrAlreadyDeducted {
		t.Fatalf("expected ErrAlreadyDeducted on a repeat post, got %v", err)
	}
}

// --- TDS reversal creates a new correcting journal, never edits the original ---

func TestTDSReversal(t *testing.T) {
	ensureDB(t)
	nature := "TestNature_" + uniqueRef(t)
	approvedTestRule(t, nature, 0, 10, time.Now().AddDate(-1, 0, 0), nil)

	txn, _ := tax.CalculateTDS(tax.CalculateInput{
		ReferenceType: "test", ReferenceID: uniqueRef(t), DeducteeName: "Vendor",
		GrossAmountPaise: 1000000, NatureOfPayment: nature, PANStatus: models.PANStatusValid,
		TransactionDate: time.Now(), Actor: "system",
	})
	posted, err := tax.PostTDSDeduction(txn.ID, accounting.AcctOperatingExpenses, "system")
	if err != nil {
		t.Fatalf("PostTDSDeduction failed: %v", err)
	}

	reversed, err := tax.ReverseTDSDeduction(posted.ID, "test correction", "system")
	if err != nil {
		t.Fatalf("ReverseTDSDeduction failed: %v", err)
	}
	if reversed.Status != models.TDSStatusReversed {
		t.Fatalf("expected REVERSED status, got %s", reversed.Status)
	}

	var journal models.JournalEntry
	database.DB.Where("journal_number = ?", posted.JournalReference).First(&journal)
	if journal.Status != models.JournalStatusReversed {
		t.Fatalf("the original TDS deduction journal must be marked REVERSED, not edited, got status %s", journal.Status)
	}
}

// --- Unauthorized user cannot view the tax dashboard ---

func TestUnauthorizedCannotAccessTaxDashboard(t *testing.T) {
	r := setupApp(t)
	username := "tax_noaccess_" + uniqueRef(t)
	pw := createTestAdmin(t, username, models.AdminRoleCollectionOfficer)
	token := loginAs(t, r, username, pw)

	w := doJSON(r, "GET", "/admin/tax/dashboard", token, nil)
	if w.Code != http.StatusForbidden {
		t.Fatalf("COLLECTION_OFFICER must not have TaxView, got %d: %s", w.Code, w.Body.String())
	}
}

func itoaT(id uint) string {
	b, _ := json.Marshal(id)
	return string(b)
}
