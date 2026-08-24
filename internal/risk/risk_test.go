package risk_test

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
	"github.com/santosh0921/nbfc-backend/internal/risk"
)

// Phase 5 risk/DPD/classification/provisioning regression suite. Reuses
// the real Postgres dev DB, same pattern as Phases 1-4.

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

func setupRiskTestApp(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	ensureDB(t)

	r := gin.New()
	r.POST("/admin/login", admin.LoginHandler)
	adminGroup := r.Group("/admin")
	adminGroup.Use(middleware.AdminAuthMiddleware())
	adminGroup.POST("/risk/loans/:loanId/override-classification", middleware.RequirePermission(rbac.ClassificationOverride), risk.AdminOverrideClassificationHandler)
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

// createTestLoan creates a DISBURSED loan with one overdue installment
// (due 45 days before asOf, unpaid) and one paid installment (due 15 days
// before asOf, paid) — enough to exercise DPD/classification/provisioning
// without going through the full apply->sanction->disburse pipeline.
func createTestLoan(t *testing.T, asOf time.Time) models.LoanApplication {
	t.Helper()
	tenure := 12
	rate := 12.0
	loan := models.LoanApplication{
		CustomerID: uuid.New(), Category: "Personal Loan", AmountRequested: 100000,
		Status: models.LoanStatusDisbursed, TenureMonths: &tenure, InterestRatePercent: &rate,
		ApplicantName: "Risk Test", DisbursedAmount: 100000,
	}
	database.DB.Create(&loan)
	t.Cleanup(func() {
		database.DB.Unscoped().Where("loan_id = ?", loan.ID).Delete(&models.EmiInstallment{})
		database.DB.Unscoped().Where("loan_id = ?", loan.ID).Delete(&models.DPDRecord{})
		database.DB.Unscoped().Where("loan_id = ?", loan.ID).Delete(&models.LoanProvision{})
		database.DB.Unscoped().Where("loan_id = ?", loan.ID).Delete(&models.LoanClassificationHistory{})
		database.DB.Unscoped().Where("loan_id = ?", loan.ID).Delete(&models.LoanClassification{})
		database.DB.Unscoped().Delete(&loan)
	})

	overdueInst := models.EmiInstallment{
		LoanID: loan.ID, InstallmentNumber: 1, DueDate: asOf.AddDate(0, 0, -45),
		Amount: 9000, PrincipalComponent: 8000, InterestComponent: 1000, Status: models.EmiStatusPending,
	}
	database.DB.Create(&overdueInst)

	paidInst := models.EmiInstallment{
		LoanID: loan.ID, InstallmentNumber: 2, DueDate: asOf.AddDate(0, 0, -15),
		Amount: 9000, PrincipalComponent: 8000, InterestComponent: 1000, Status: models.EmiStatusPaid,
		PaidAt: ptrTime(asOf.AddDate(0, 0, -15)),
	}
	database.DB.Create(&paidInst)

	return loan
}

func ptrTime(t time.Time) *time.Time { return &t }

func approvedClassificationRule(t *testing.T, minDPD, maxDPD int, classification string) models.ClassificationRule {
	t.Helper()
	now := time.Now()
	rule := models.ClassificationRule{
		RuleVersion: uniqueRef(t), EffectiveDate: now.AddDate(0, 0, -365),
		Classification: classification, MinDPD: minDPD, MaxDPD: maxDPD,
		Status: models.ClassificationStatusApproved, CreatedBy: "test-maker", ApprovedBy: "test-checker", ApprovalDate: &now,
	}
	database.DB.Create(&rule)
	t.Cleanup(func() { database.DB.Unscoped().Delete(&rule) })
	return rule
}

func approvedProvisioningRule(t *testing.T, classification string, ratePercent float64) models.ProvisioningRule {
	t.Helper()
	now := time.Now()
	rule := models.ProvisioningRule{
		RuleVersion: uniqueRef(t), Classification: classification, ProvisionRatePercent: ratePercent,
		EffectiveDate: now.AddDate(0, 0, -365), Status: models.ClassificationStatusApproved,
		CreatedBy: "test-maker", ApprovedBy: "test-checker", ApprovalDate: &now,
	}
	database.DB.Create(&rule)
	t.Cleanup(func() { database.DB.Unscoped().Delete(&rule) })
	return rule
}

func uniqueRef(t *testing.T) string {
	return t.Name() + "-" + time.Now().Format("150405.000000000")
}

func itoa(id uint) string {
	b, _ := json.Marshal(id)
	return string(b)
}

// --- 1. Correct DPD calculation ---

func TestDPDCalculationCorrect(t *testing.T) {
	ensureDB(t)
	asOf := time.Now()
	loan := createTestLoan(t, asOf)

	dpd, err := risk.CalculateDPDForLoan(loan.ID, asOf)
	if err != nil {
		t.Fatalf("CalculateDPDForLoan failed: %v", err)
	}
	if dpd != 45 {
		t.Fatalf("expected DPD 45 for an installment due 45 days ago and unpaid, got %d", dpd)
	}
}

// --- 2/4. DPD updates after verified payment / full payment clears overdue ---

func TestDPDZeroForPaidInstallment(t *testing.T) {
	ensureDB(t)
	asOf := time.Now()
	loan := createTestLoan(t, asOf)
	risk.CalculateDPDForLoan(loan.ID, asOf)

	var paidRecord models.DPDRecord
	var paidInst models.EmiInstallment
	database.DB.Where("loan_id = ? AND status = ?", loan.ID, models.EmiStatusPaid).First(&paidInst)
	database.DB.Where("loan_id = ? AND schedule_id = ?", loan.ID, paidInst.ID).First(&paidRecord)

	if paidRecord.DPD != 0 || paidRecord.OutstandingPaise != 0 {
		t.Fatalf("a paid installment must show DPD 0 and zero outstanding, got dpd=%d outstanding=%d", paidRecord.DPD, paidRecord.OutstandingPaise)
	}
}

// --- 5. Duplicate payment/duplicate calculation does not duplicate records ---

func TestDPDCalculationIdempotent(t *testing.T) {
	ensureDB(t)
	asOf := time.Now()
	loan := createTestLoan(t, asOf)

	risk.CalculateDPDForLoan(loan.ID, asOf)
	risk.CalculateDPDForLoan(loan.ID, asOf) // second call, same date

	var count int64
	database.DB.Model(&models.DPDRecord{}).Where("loan_id = ? AND calculation_date = ?", loan.ID,
		time.Date(asOf.Year(), asOf.Month(), asOf.Day(), 0, 0, 0, 0, asOf.Location())).Count(&count)
	if count != 2 { // 2 installments, one DPDRecord each — not 4
		t.Fatalf("expected exactly 2 DPDRecord rows (one per installment) after two calls for the same date, got %d", count)
	}
}

// --- 7. Unapproved rule cannot be used in production ---

func TestUnapprovedClassificationRuleNotUsed(t *testing.T) {
	ensureDB(t)
	now := time.Now()
	draftRule := models.ClassificationRule{
		RuleVersion: uniqueRef(t), EffectiveDate: now.AddDate(0, 0, -365),
		Classification: "SUBSTANDARD", MinDPD: 40, MaxDPD: 50,
		Status: models.ClassificationStatusDraft, CreatedBy: "test-maker",
	}
	database.DB.Create(&draftRule)
	t.Cleanup(func() { database.DB.Unscoped().Delete(&draftRule) })

	loan := createTestLoan(t, now)
	_, err := risk.ClassifyLoan(loan.ID, loan.Category, "", 45, now, "system")
	if err != risk.ErrNoApprovedRule {
		t.Fatalf("a DRAFT (unapproved) rule must never be used to classify a loan — expected ErrNoApprovedRule, got %v", err)
	}
}

// --- 6/9. Classification/provisioning rules are versioned and configured rate is used ---

func TestApprovedRuleClassifiesAndProvisions(t *testing.T) {
	ensureDB(t)
	now := time.Now()
	loan := createTestLoan(t, now)
	rule := approvedClassificationRule(t, 40, 60, "SUBSTANDARD")
	provRule := approvedProvisioningRule(t, "SUBSTANDARD", 10.0)

	classification, err := risk.ClassifyLoan(loan.ID, loan.Category, "", 45, now, "system")
	if err != nil {
		t.Fatalf("ClassifyLoan failed: %v", err)
	}
	if classification != "SUBSTANDARD" {
		t.Fatalf("expected SUBSTANDARD, got %s", classification)
	}

	var history models.LoanClassificationHistory
	database.DB.Where("loan_id = ?", loan.ID).Order("id desc").First(&history)
	if history.RuleVersion != rule.RuleVersion {
		t.Fatalf("classification history must record the rule version that produced it")
	}

	provision, err := risk.CalculateProvision(loan.ID, "SUBSTANDARD", now, "system")
	if err != nil {
		t.Fatalf("CalculateProvision failed: %v", err)
	}
	// exposure = ₹8000 outstanding principal (only the unpaid installment
	// counts, in paise = 800000) * 10% = 80000 paise (₹800).
	if provision.ProvisionAmountPaise != 80000 {
		t.Fatalf("expected provision amount 80000 paise (10%% of ₹8000 outstanding principal), got %d", provision.ProvisionAmountPaise)
	}
	if provision.RuleVersion != provRule.RuleVersion {
		t.Fatal("provision must record the rule version used")
	}
	if provision.JournalReference == "" {
		t.Fatal("a nonzero provision must have posted an accounting journal entry")
	}

	// --- 12. Accounting entry balances ---
	var journal models.JournalEntry
	database.DB.Preload("Lines").Where("journal_number = ?", provision.JournalReference).First(&journal)
	var debit, credit int64
	for _, l := range journal.Lines {
		debit += l.DebitPaise
		credit += l.CreditPaise
	}
	if debit != credit {
		t.Fatalf("provision journal must balance: debit=%d credit=%d", debit, credit)
	}
}

// --- 10. Duplicate provisioning prevented ---

func TestDuplicateProvisioningPrevented(t *testing.T) {
	ensureDB(t)
	now := time.Now()
	loan := createTestLoan(t, now)
	approvedProvisioningRule(t, "DOUBTFUL", 25.0)

	first, err := risk.CalculateProvision(loan.ID, "DOUBTFUL", now, "system")
	if err != nil {
		t.Fatalf("first provision call failed: %v", err)
	}
	second, err := risk.CalculateProvision(loan.ID, "DOUBTFUL", now, "system")
	if err != nil {
		t.Fatalf("second call should be a safe idempotent no-op: %v", err)
	}
	if second.ID != first.ID {
		t.Fatal("a second CalculateProvision call for the same loan+date must return the SAME row, not create a new one")
	}
	var count int64
	database.DB.Model(&models.LoanProvision{}).Where("loan_id = ? AND calculation_date = ?", loan.ID,
		time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())).Count(&count)
	if count != 1 {
		t.Fatalf("expected exactly one LoanProvision row for this loan+date, got %d", count)
	}
}

// --- 11. Provision reversal works correctly ---

func TestProvisionReversalOnReclassification(t *testing.T) {
	ensureDB(t)
	day1 := time.Now().AddDate(0, 0, -2)
	day2 := time.Now().AddDate(0, 0, -1)
	loan := createTestLoan(t, time.Now())
	approvedProvisioningRule(t, "SUBSTANDARD", 10.0)
	approvedProvisioningRule(t, "DOUBTFUL", 40.0)

	first, err := risk.CalculateProvision(loan.ID, "SUBSTANDARD", day1, "system")
	if err != nil {
		t.Fatalf("first provision failed: %v", err)
	}
	second, err := risk.CalculateProvision(loan.ID, "DOUBTFUL", day2, "system")
	if err != nil {
		t.Fatalf("reclassified provision failed: %v", err)
	}
	if second.ID == first.ID {
		t.Fatal("a reclassification must create a NEW provision row, not reuse the old one")
	}

	var reloaded models.LoanProvision
	database.DB.First(&reloaded, first.ID)
	if reloaded.Status != models.ProvisionStatusReversed {
		t.Fatalf("the old SUBSTANDARD provision must be marked REVERSED after reclassification, got %s", reloaded.Status)
	}

	var reversalJournal models.JournalEntry
	err = database.DB.Where("reversal_of_journal_id = (SELECT id FROM journal_entries WHERE journal_number = ?)", first.JournalReference).
		First(&reversalJournal).Error
	if err != nil {
		t.Fatalf("expected a reversal journal entry linked to the original provision journal: %v", err)
	}
}

// --- 8. Unauthorized user cannot change classification ---

func TestUnauthorizedCannotOverrideClassification(t *testing.T) {
	r := setupRiskTestApp(t)
	username := "risk_readonly_" + t.Name()
	pw := createTestAdmin(t, username, models.AdminRoleReadOnly)
	token := loginAs(t, r, username, pw)

	loan := createTestLoan(t, time.Now())
	w := doJSON(r, "POST", "/admin/risk/loans/"+itoa(loan.ID)+"/override-classification", token, gin.H{
		"classification": "LOSS", "reason": "test", "approvedBy": "someone",
	})
	if w.Code != http.StatusForbidden {
		t.Fatalf("READ_ONLY must not be able to override classification, got %d: %s", w.Code, w.Body.String())
	}
}

// --- 19. Manual override creates audit trail ---

func TestManualOverrideCreatesHistory(t *testing.T) {
	ensureDB(t)
	loan := createTestLoan(t, time.Now())
	err := risk.ManualOverrideClassification(loan.ID, "LOSS", "collateral seized, total loss expected", "credit_manager_1", "compliance_officer_1")
	if err != nil {
		t.Fatalf("ManualOverrideClassification failed: %v", err)
	}
	var history models.LoanClassificationHistory
	database.DB.Where("loan_id = ? AND new_classification = ?", loan.ID, "LOSS").Order("id desc").First(&history)
	if history.ApprovedBy != "compliance_officer_1" || history.Reason == "" {
		t.Fatal("a manual override must record both a reason and an approver in the history row")
	}

	err = risk.ManualOverrideClassification(loan.ID, "LOSS", "", "someone", "")
	if err == nil {
		t.Fatal("a manual override without a reason/approver must be rejected")
	}
}

// --- 16. Daily job is idempotent ---

func TestDailyJobIdempotentAtRunLevel(t *testing.T) {
	ensureDB(t)
	runDate := time.Now().AddDate(0, 0, -3000) // a date guaranteed unused by any real run
	t.Cleanup(func() {
		database.DB.Unscoped().Where("run_date = ?", time.Date(runDate.Year(), runDate.Month(), runDate.Day(), 0, 0, 0, 0, runDate.Location())).
			Delete(&models.RiskJobRun{})
	})

	first, err := risk.RunDailyRiskJob(runDate)
	if err != nil {
		t.Fatalf("first run failed: %v", err)
	}
	second, err := risk.RunDailyRiskJob(runDate)
	if err != nil {
		t.Fatalf("second run should be a safe no-op: %v", err)
	}
	if second.ID != first.ID {
		t.Fatal("a second run for a date that already COMPLETED must return the same RiskJobRun row")
	}
}

// --- 20. Reconciliation identifies mismatches ---

func TestConsistencyCheckDetectsNegativeOutstanding(t *testing.T) {
	ensureDB(t)
	asOf := time.Now()
	loan := createTestLoan(t, asOf)
	var inst models.EmiInstallment
	database.DB.Where("loan_id = ?", loan.ID).First(&inst)

	bad := models.DPDRecord{
		LoanID: loan.ID, ScheduleID: inst.ID, DueDate: inst.DueDate,
		DueAmountPaise: 9000, PaidAmountPaise: 15000, OutstandingPaise: -6000,
		DPD: 0, CalculationDate: asOf.AddDate(0, 0, -1000), Status: models.DPDRecordStatusCalculated,
	}
	database.DB.Create(&bad)
	t.Cleanup(func() { database.DB.Unscoped().Delete(&bad) })

	found := risk.RunConsistencyChecks(asOf)
	if found == 0 {
		t.Fatal("expected RunConsistencyChecks to detect the negative-outstanding DPDRecord")
	}
	var exception models.DataConsistencyException
	err := database.DB.Where("exception_type = ? AND loan_id = ?", models.ExceptionTypeNegativeOutstanding, loan.ID).First(&exception).Error
	if err != nil {
		t.Fatalf("expected a DataConsistencyException row for the negative-outstanding loan: %v", err)
	}
	t.Cleanup(func() { database.DB.Unscoped().Delete(&exception) })
}
