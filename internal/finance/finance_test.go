package finance_test

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
	"github.com/santosh0921/nbfc-backend/internal/models"
	"github.com/santosh0921/nbfc-backend/internal/rbac"
)

// Phase 8 finance (vendor/expense/bank-balance/dashboard) regression
// suite. Reuses the real Postgres dev DB, same pattern as Phases 1-7.

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
	adminGroup.POST("/finance/expenses", middleware.RequirePermission(rbac.ExpenseManage), finance.AdminCreateExpenseHandler)
	adminGroup.POST("/finance/expenses/:id/approve", middleware.RequirePermission(rbac.ExpenseApprove), finance.AdminApproveExpenseHandler)
	adminGroup.GET("/finance/dashboard", middleware.RequirePermission(rbac.FinanceView), finance.AdminFinancialDashboardHandler)
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

// --- Vendor creation + PAN encryption round-trip ---

func TestCreateVendorEncryptsPAN(t *testing.T) {
	ensureDB(t)
	v, err := finance.CreateVendor("Test Vendor "+uniqueRef(t), "Cloud Infra", "ABCDE1234F", "22AAAAA0000A1Z5", "XXXXXXXX1234", "HDFC0001234", "system")
	if err != nil {
		t.Fatalf("CreateVendor failed: %v", err)
	}
	t.Cleanup(func() { database.DB.Unscoped().Delete(v) })
	if v.PANLast4 != "234F" {
		t.Fatalf("expected PANLast4 234F, got %s", v.PANLast4)
	}
	if v.PANEncrypted == "" || v.PANEncrypted == "ABCDE1234F" {
		t.Fatal("PAN must be encrypted at rest, not stored in plaintext")
	}
}

// --- Expense maker-checker: creator cannot approve own expense ---

func TestExpenseMakerCannotApproveOwn(t *testing.T) {
	r := setupApp(t)
	username := "fin_maker_" + uniqueRef(t)
	pw := createTestAdmin(t, username, models.AdminRoleAccountant)
	token := loginAs(t, r, username, pw)

	w := doJSON(r, "POST", "/admin/finance/expenses", token, gin.H{
		"expenseDate": time.Now().Format(time.RFC3339), "category": "Technology",
		"description": "test expense", "grossAmountPaise": 500000, "accountCode": accounting.AcctOperatingExpenses,
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("expense creation should succeed: %d %s", w.Code, w.Body.String())
	}
	var created models.Expense
	json.Unmarshal(w.Body.Bytes(), &created)
	t.Cleanup(func() { database.DB.Unscoped().Delete(&created) })

	wApprove := doJSON(r, "POST", "/admin/finance/expenses/"+itoa(created.ID)+"/approve", token, nil)
	if wApprove.Code != http.StatusForbidden {
		t.Fatalf("the admin who created this expense must not be able to approve it, got %d: %s", wApprove.Code, wApprove.Body.String())
	}
}

// --- Expense approval by a different admin + accounting posting on payment ---

func TestExpenseApprovalAndPaymentPosting(t *testing.T) {
	ensureDB(t)
	makerID, checkerID := uint(9001), uint(9002)
	e := &models.Expense{
		ExpenseDate: time.Now(), Category: "Technology", Description: "server hosting",
		GrossAmountPaise: 200000, AccountCode: accounting.AcctOperatingExpenses,
	}
	created, err := finance.CreateExpense(e, makerID, "maker_test")
	if err != nil {
		t.Fatalf("CreateExpense failed: %v", err)
	}
	t.Cleanup(func() { database.DB.Unscoped().Delete(created) })

	approved, err := finance.ApproveExpense(created.ID, checkerID, "checker_test")
	if err != nil {
		t.Fatalf("ApproveExpense failed: %v", err)
	}
	if approved.Status != models.ExpenseStatusApproved {
		t.Fatalf("expected APPROVED, got %s", approved.Status)
	}

	paid, err := finance.MarkPaid(created.ID, "UTR"+uniqueRef(t), "payer_test")
	if err != nil {
		t.Fatalf("MarkPaid failed: %v", err)
	}
	if paid.Status != models.ExpenseStatusPaid || paid.JournalReference == "" {
		t.Fatalf("expected PAID with a journal reference, got status=%s journalRef=%s", paid.Status, paid.JournalReference)
	}

	var journal models.JournalEntry
	database.DB.Preload("Lines").Where("journal_number = ?", paid.JournalReference).First(&journal)
	var debit, credit int64
	for _, l := range journal.Lines {
		debit += l.DebitPaise
		credit += l.CreditPaise
	}
	if debit != credit {
		t.Fatalf("expense payment journal must balance: debit=%d credit=%d", debit, credit)
	}
	if debit != 200000 {
		t.Fatalf("expected journal amount 200000 paise, got %d", debit)
	}
}

// --- Unauthorized user cannot view financial dashboard ---

func TestUnauthorizedCannotAccessFinancialDashboard(t *testing.T) {
	r := setupApp(t)
	username := "fin_noaccess_" + uniqueRef(t)
	pw := createTestAdmin(t, username, models.AdminRoleCollectionOfficer)
	token := loginAs(t, r, username, pw)

	w := doJSON(r, "GET", "/admin/finance/dashboard", token, nil)
	if w.Code != http.StatusForbidden {
		t.Fatalf("COLLECTION_OFFICER must not have FinanceView, got %d: %s", w.Code, w.Body.String())
	}
}

// --- Bank account balance computation ---

func TestBankAccountBalanceComputation(t *testing.T) {
	ensureDB(t)
	account := models.BankAccount{
		AccountName: "Test Settlement " + uniqueRef(t), AccountNumberMasked: "XXXXXXXX9999",
		OpeningBalancePaise: 1000000, Active: true,
	}
	database.DB.Create(&account)
	t.Cleanup(func() {
		database.DB.Unscoped().Where("bank_account_id = ?", account.ID).Delete(&models.BankTransaction{})
		database.DB.Unscoped().Delete(&account)
	})

	txns := []models.BankTransaction{
		{BankAccountID: account.ID, TxnDate: time.Now(), AmountPaise: 500000, DrCr: "CR", Description: "collection"},
		{BankAccountID: account.ID, TxnDate: time.Now(), AmountPaise: 200000, DrCr: "DR", Description: "disbursement"},
	}
	for i := range txns {
		database.DB.Create(&txns[i])
	}

	summary, err := finance.BankAccountBalance(account.ID, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatalf("BankAccountBalance failed: %v", err)
	}
	expectedClosing := int64(1000000 + 500000 - 200000)
	if summary.ClosingBalancePaise != expectedClosing {
		t.Fatalf("expected closing balance %d, got %d", expectedClosing, summary.ClosingBalancePaise)
	}
}

func itoa(id uint) string {
	b, _ := json.Marshal(id)
	return string(b)
}
