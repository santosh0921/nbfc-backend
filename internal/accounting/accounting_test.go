package accounting_test

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
)

// Phase 4 accounting regression suite. Reuses the real Postgres dev DB,
// same pattern as Phase 1/2/3 (internal/admin/security_test.go etc).

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

func setupAcctTestApp(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	ensureDB(t)

	r := gin.New()
	r.POST("/admin/login", admin.LoginHandler)
	adminGroup := r.Group("/admin")
	adminGroup.Use(middleware.AdminAuthMiddleware())
	adminGroup.POST("/accounting/journal", middleware.RequirePermission(rbac.AccountingPost), accounting.AdminPostManualJournalHandler)
	adminGroup.POST("/accounting/journal/:id/reverse", middleware.RequirePermission(rbac.AccountingPost), accounting.AdminReverseJournalHandler)
	adminGroup.POST("/accounting/periods/:id/close", middleware.RequirePermission(rbac.PeriodManage), accounting.AdminClosePeriodHandler)
	return r
}

func acctDoJSON(r *gin.Engine, method, path, token string, body interface{}) *httptest.ResponseRecorder {
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

func createAcctTestAdmin(t *testing.T, username, role string) string {
	t.Helper()
	password := "Sup3rSecure!Passw0rd_1234"
	hash, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	u := models.AdminUser{Username: username, Email: username + "@test.local", PasswordHash: string(hash), Role: role, Status: models.AdminStatusActive}
	database.DB.Create(&u)
	t.Cleanup(func() { database.DB.Unscoped().Delete(&u) })
	return password
}

func acctLoginAs(t *testing.T, r *gin.Engine, username, password string) string {
	t.Helper()
	w := acctDoJSON(r, "POST", "/admin/login", "", gin.H{"username": username, "password": password})
	if w.Code != http.StatusOK {
		t.Fatalf("login failed: %d %s", w.Code, w.Body.String())
	}
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	return resp["token"].(string)
}

func balancedLines(t *testing.T, amountPaise int64) []accounting.LineInput {
	t.Helper()
	return []accounting.LineInput{
		{AccountCode: accounting.AcctBankCash, DebitPaise: amountPaise, Description: "test debit"},
		{AccountCode: accounting.AcctEquityCapital, CreditPaise: amountPaise, Description: "test credit"},
	}
}

// --- 1. Debit = credit validation / balanced entry posts successfully ---

func TestBalancedJournalPosts(t *testing.T) {
	ensureDB(t)
	entry, err := accounting.Service.PostJournalEntry(accounting.PostingInput{
		TransactionType: "TEST", ReferenceType: "test", ReferenceID: uniqueRef(t),
		Description: "balanced test entry", CreatedBy: "system",
		Lines: balancedLines(t, 10000),
	})
	if err != nil {
		t.Fatalf("expected a balanced entry to post: %v", err)
	}
	if entry.Status != models.JournalStatusPosted {
		t.Fatalf("expected POSTED status, got %s", entry.Status)
	}
}

// --- 2. Unbalanced journal rejected ---

func TestUnbalancedJournalRejected(t *testing.T) {
	ensureDB(t)
	_, err := accounting.Service.PostJournalEntry(accounting.PostingInput{
		TransactionType: "TEST", ReferenceType: "test", ReferenceID: uniqueRef(t),
		CreatedBy: "system",
		Lines: []accounting.LineInput{
			{AccountCode: accounting.AcctBankCash, DebitPaise: 10000},
			{AccountCode: accounting.AcctEquityCapital, CreditPaise: 9000},
		},
	})
	if err != accounting.ErrUnbalancedEntry {
		t.Fatalf("expected ErrUnbalancedEntry, got %v", err)
	}
}

// --- 3. Duplicate journal (same idempotency key) prevented ---

func TestDuplicateJournalPrevented(t *testing.T) {
	ensureDB(t)
	ref := uniqueRef(t)
	input := accounting.PostingInput{
		TransactionType: "TEST", ReferenceType: "test", ReferenceID: ref,
		CreatedBy: "system", Lines: balancedLines(t, 5000),
	}
	if _, err := accounting.Service.PostJournalEntry(input); err != nil {
		t.Fatalf("first post should succeed: %v", err)
	}
	_, err := accounting.Service.PostJournalEntry(input)
	if err != accounting.ErrDuplicatePosting {
		t.Fatalf("expected ErrDuplicatePosting on repeat, got %v", err)
	}
}

// --- 11. Unauthorized user cannot post journals ---

func TestUnauthorizedCannotPostJournal(t *testing.T) {
	r := setupAcctTestApp(t)
	username := "acct_readonly_" + t.Name()
	pw := createAcctTestAdmin(t, username, models.AdminRoleReadOnly)
	token := acctLoginAs(t, r, username, pw)

	w := acctDoJSON(r, "POST", "/admin/accounting/journal", token, gin.H{
		"description": "unauthorized attempt",
		"lines": []gin.H{
			{"accountCode": accounting.AcctBankCash, "debitPaise": 1000},
			{"accountCode": accounting.AcctEquityCapital, "creditPaise": 1000},
		},
	})
	if w.Code != http.StatusForbidden {
		t.Fatalf("READ_ONLY must not be able to post a journal entry, got %d: %s", w.Code, w.Body.String())
	}
}

// --- 12. Auditor cannot modify accounting data ---

func TestAuditorCannotPostJournal(t *testing.T) {
	r := setupAcctTestApp(t)
	username := "acct_auditor_" + t.Name()
	pw := createAcctTestAdmin(t, username, models.AdminRoleAuditor)
	token := acctLoginAs(t, r, username, pw)

	w := acctDoJSON(r, "POST", "/admin/accounting/journal", token, gin.H{
		"description": "auditor attempt",
		"lines": []gin.H{
			{"accountCode": accounting.AcctBankCash, "debitPaise": 1000},
			{"accountCode": accounting.AcctEquityCapital, "creditPaise": 1000},
		},
	})
	if w.Code != http.StatusForbidden {
		t.Fatalf("AUDITOR must not be able to post a journal entry, got %d: %s", w.Code, w.Body.String())
	}
}

// --- 13. Reversal creates a new correcting entry, original marked REVERSED ---

func TestReversalCreatesNewEntry(t *testing.T) {
	r := setupAcctTestApp(t)
	username := "acct_super_" + t.Name()
	pw := createAcctTestAdmin(t, username, models.AdminRoleSuperAdmin)
	token := acctLoginAs(t, r, username, pw)

	entry, err := accounting.Service.PostJournalEntry(accounting.PostingInput{
		TransactionType: "TEST", ReferenceType: "test", ReferenceID: uniqueRef(t),
		CreatedBy: "system", Lines: balancedLines(t, 7000),
	})
	if err != nil {
		t.Fatalf("post failed: %v", err)
	}

	w := acctDoJSON(r, "POST", "/admin/accounting/journal/"+itoa(entry.ID)+"/reverse", token, gin.H{"reason": "test correction"})
	if w.Code != http.StatusCreated {
		t.Fatalf("reversal should succeed: %d %s", w.Code, w.Body.String())
	}
	var reversal models.JournalEntry
	json.Unmarshal(w.Body.Bytes(), &reversal)
	if reversal.ID == entry.ID {
		t.Fatal("reversal must be a NEW journal entry, not the same row")
	}

	var reloaded models.JournalEntry
	database.DB.First(&reloaded, entry.ID)
	if reloaded.Status != models.JournalStatusReversed {
		t.Fatalf("original entry should be marked REVERSED, got %s", reloaded.Status)
	}
}

// --- 10. Closed accounting period rejects normal posting ---

func TestClosedPeriodRejectsPosting(t *testing.T) {
	r := setupAcctTestApp(t)
	username := "acct_close_" + t.Name()
	pw := createAcctTestAdmin(t, username, models.AdminRoleSuperAdmin)
	token := acctLoginAs(t, r, username, pw)

	// Post once to guarantee the period row for "now" exists, then close it.
	ref := uniqueRef(t)
	if _, err := accounting.Service.PostJournalEntry(accounting.PostingInput{
		TransactionType: "TEST", ReferenceType: "test", ReferenceID: ref,
		CreatedBy: "system", Lines: balancedLines(t, 1000),
	}); err != nil {
		t.Fatalf("seed post failed: %v", err)
	}
	code := time.Now().Format("2006-01")
	var period models.AccountingPeriod
	if err := database.DB.Where("code = ?", code).First(&period).Error; err != nil {
		t.Fatalf("expected the current period to exist: %v", err)
	}

	w := acctDoJSON(r, "POST", "/admin/accounting/periods/"+itoa(period.ID)+"/close", token, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("close should succeed: %d %s", w.Code, w.Body.String())
	}
	t.Cleanup(func() {
		database.DB.Model(&models.AccountingPeriod{}).Where("id = ?", period.ID).Update("status", models.PeriodStatusOpen)
	})

	_, err := accounting.Service.PostJournalEntry(accounting.PostingInput{
		TransactionType: "TEST", ReferenceType: "test", ReferenceID: uniqueRef(t),
		CreatedBy: "system", Lines: balancedLines(t, 2000),
	})
	if err != accounting.ErrPeriodNotOpen {
		t.Fatalf("expected ErrPeriodNotOpen for a posting into a CLOSED period, got %v", err)
	}
}

// --- 14. Trial balance balances ---

func TestTrialBalanceBalances(t *testing.T) {
	if database.DB == nil {
		setupAcctTestApp(t)
	}
	accounting.Service.PostJournalEntry(accounting.PostingInput{
		TransactionType: "TEST", ReferenceType: "test", ReferenceID: uniqueRef(t),
		CreatedBy: "system", Lines: balancedLines(t, 12345),
	})
	from := time.Now().AddDate(0, 0, -1)
	to := time.Now().AddDate(0, 0, 1)
	report := accounting.TrialBalance(from, to)
	if !report.Balanced {
		t.Fatalf("trial balance must always balance: totalDebit=%d totalCredit=%d", report.TotalClosingDebit, report.TotalClosingCredit)
	}
}

// --- 15. Balance sheet balances (Assets = Liabilities + Equity) ---

func TestBalanceSheetBalances(t *testing.T) {
	if database.DB == nil {
		setupAcctTestApp(t)
	}
	accounting.Service.PostJournalEntry(accounting.PostingInput{
		TransactionType: "TEST", ReferenceType: "test", ReferenceID: uniqueRef(t),
		CreatedBy: "system", Lines: balancedLines(t, 5555),
	})
	report := accounting.BalanceSheet(time.Now().AddDate(0, 0, 1))
	if !report.Balanced {
		t.Fatalf("balance sheet must satisfy Assets = Liabilities + Equity: assets=%d liab=%d equity=%d",
			report.TotalAssetsPaise, report.TotalLiabilitiesPaise, report.TotalEquityPaise)
	}
}

// --- 7. Interest accrual cannot run twice for the same period ---

func TestInterestAccrualIdempotent(t *testing.T) {
	if database.DB == nil {
		setupAcctTestApp(t)
	}
	rate := 12.0
	tenure := 12
	loan := models.LoanApplication{
		CustomerID: uuid.New(), Category: "Personal Loan", AmountRequested: 100000,
		Status: models.LoanStatusDisbursed, TenureMonths: &tenure, InterestRatePercent: &rate,
		ApplicantName: "Accrual Test", DisbursedAmount: 100000,
	}
	database.DB.Create(&loan)
	t.Cleanup(func() { database.DB.Unscoped().Delete(&loan) })
	inst := models.EmiInstallment{LoanID: loan.ID, InstallmentNumber: 1, PrincipalComponent: 100000, InterestComponent: 1000, Status: models.EmiStatusPending}
	database.DB.Create(&inst)
	t.Cleanup(func() { database.DB.Unscoped().Delete(&inst) })

	period := "2099-01"
	start := time.Date(2099, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2099, 1, 31, 0, 0, 0, 0, time.UTC)

	first, err := accounting.AccrueInterestForLoan(loan.ID, period, start, end, "system")
	if err != nil {
		t.Fatalf("first accrual should succeed: %v", err)
	}
	second, err := accounting.AccrueInterestForLoan(loan.ID, period, start, end, "system")
	if err != nil {
		t.Fatalf("second call should be a safe idempotent no-op, not an error: %v", err)
	}
	if second.ID != first.ID {
		t.Fatal("a second accrual call for the same loan+period must return the SAME row, not create a new one")
	}
	var count int64
	database.DB.Model(&models.InterestAccrual{}).Where("loan_id = ? AND accrual_period = ?", loan.ID, period).Count(&count)
	if count != 1 {
		t.Fatalf("expected exactly one accrual row for this loan+period, got %d", count)
	}
}

func uniqueRef(t *testing.T) string {
	return t.Name() + "-" + time.Now().Format("150405.000000000")
}

func itoa(id uint) string {
	b, _ := json.Marshal(id)
	return string(b)
}
