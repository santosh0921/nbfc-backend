package reconciliation_test

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
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
	"github.com/santosh0921/nbfc-backend/internal/middleware"
	"github.com/santosh0921/nbfc-backend/internal/models"
	"github.com/santosh0921/nbfc-backend/internal/rbac"
	"github.com/santosh0921/nbfc-backend/internal/reconciliation"
)

func setupReconTestApp(t *testing.T) *gin.Engine {
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
	adminGroup.POST("/reconciliation/bank-accounts", middleware.RequirePermission(rbac.ReconciliationManage), reconciliation.AdminCreateBankAccountHandler)
	adminGroup.POST("/reconciliation/bank-accounts/:id/import", middleware.RequirePermission(rbac.ReconciliationManage), reconciliation.AdminImportBankStatementHandler)
	adminGroup.POST("/reconciliation/transactions/:id/match", middleware.RequirePermission(rbac.ReconciliationManage), reconciliation.AdminManualMatchHandler)
	adminGroup.GET("/reconciliation/exceptions", middleware.RequirePermission(rbac.ReconciliationView), reconciliation.AdminListReconciliationExceptionsHandler)
	return r
}

func createReconTestAdmin(t *testing.T, username, role string) string {
	t.Helper()
	password := "Sup3rSecure!Passw0rd_1234"
	hash, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	u := models.AdminUser{Username: username, Email: username + "@test.local", PasswordHash: string(hash), Role: role, Status: models.AdminStatusActive}
	database.DB.Create(&u)
	t.Cleanup(func() { database.DB.Unscoped().Delete(&u) })

	r := gin.New()
	r.POST("/admin/login", admin.LoginHandler)
	req := httptest.NewRequest(http.MethodPost, "/admin/login", bytes.NewReader(mustJSON(gin.H{"username": username, "password": password})))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	return resp["token"].(string)
}

func mustJSON(v interface{}) []byte {
	b, _ := json.Marshal(v)
	return b
}

func createTestBankAccount(t *testing.T) models.BankAccount {
	t.Helper()
	acc := models.BankAccount{AccountName: "Test Ops Account", AccountNumberMasked: "XXXXXXXX1234", Active: true}
	database.DB.Create(&acc)
	t.Cleanup(func() { database.DB.Unscoped().Delete(&acc) })
	return acc
}

func createOutstandingDisbursement(t *testing.T, amountPaise int64, providerRef string) models.Disbursement {
	t.Helper()
	tenure := 12
	rate := 10.0
	loan := models.LoanApplication{
		CustomerID: uuid.New(), Category: "Personal Loan", AmountRequested: float64(amountPaise) / 100,
		Status: models.LoanStatusDisbursementProcessing, TenureMonths: &tenure, InterestRatePercent: &rate,
	}
	database.DB.Create(&loan)
	t.Cleanup(func() { database.DB.Unscoped().Delete(&loan) })

	now := time.Now()
	disb := models.Disbursement{
		LoanID: loan.ID, CustomerID: loan.CustomerID.String(), AmountPaise: amountPaise, Currency: "INR",
		Status: models.DisbursementStatusInitiated, Provider: "manual_bank_transfer",
		ProviderReference: providerRef, InitiatedAt: &now,
	}
	database.DB.Create(&disb)
	t.Cleanup(func() { database.DB.Unscoped().Delete(&disb) })
	return disb
}

// --- 15. Bank transaction matched correctly (exact UTR/reference match) ---

func TestBankTransactionAutoMatchesByReference(t *testing.T) {
	r := setupReconTestApp(t)
	token := createReconTestAdmin(t, "recon_match_"+t.Name(), models.AdminRoleAccountant)
	account := createTestBankAccount(t)
	disb := createOutstandingDisbursement(t, 2000000, "UTRMATCH0001") // Rs. 20,000.00

	csvBody := "date,valueDate,amount,drCr,description,reference\n" +
		"2026-01-15,2026-01-15,20000.00,DR,NEFT to borrower,UTRMATCH0001\n"

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	part, _ := mw.CreateFormFile("file", "statement.csv")
	part.Write([]byte(csvBody))
	mw.Close()

	req := httptest.NewRequest(http.MethodPost, "/admin/reconciliation/bank-accounts/"+idStr2(account.ID)+"/import", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("import failed: %d %s", w.Code, w.Body.String())
	}

	var txn models.BankTransaction
	if err := database.DB.Where("reference = ?", "UTRMATCH0001").First(&txn).Error; err != nil {
		t.Fatalf("expected the imported transaction to exist: %v", err)
	}
	if txn.Status != models.BankTxnStatusReconciled {
		t.Fatalf("expected RECONCILED after exact-reference auto-match, got %s", txn.Status)
	}

	var reloadedDisb models.Disbursement
	database.DB.First(&reloadedDisb, disb.ID)
	if reloadedDisb.Status != models.DisbursementStatusSuccess {
		t.Fatalf("expected disbursement SUCCESS after matched reconciliation, got %s", reloadedDisb.Status)
	}
	if reloadedDisb.UTR != "UTRMATCH0001" {
		t.Fatalf("expected UTR captured on the disbursement, got %q", reloadedDisb.UTR)
	}

	var reloadedLoan models.LoanApplication
	database.DB.First(&reloadedLoan, disb.LoanID)
	if reloadedLoan.Status != models.LoanStatusDisbursed {
		t.Fatalf("expected loan DISBURSED after matched reconciliation, got %s", reloadedLoan.Status)
	}
}

// --- 16. Wrong amount during reconciliation -> exception ---

func TestReconciliationWrongAmountRaisesException(t *testing.T) {
	r := setupReconTestApp(t)
	token := createReconTestAdmin(t, "recon_wrongamt_"+t.Name(), models.AdminRoleAccountant)
	account := createTestBankAccount(t)
	// Outstanding disbursement for Rs. 20,000, but the statement line
	// (below) is Rs. 15,000 with NO reference — amount won't match, so it
	// must be left UNMATCHED (not silently linked to the wrong amount) —
	// and a same-amount duplicate below produces an EXCEPTION since it's
	// ambiguous which of two same-amount candidates it belongs to.
	createOutstandingDisbursement(t, 2000000, "")
	createOutstandingDisbursement(t, 2000000, "")

	today := time.Now().Format("2006-01-02")
	csvBody := "date,valueDate,amount,drCr,description,reference\n" +
		today + "," + today + ",20000.00,DR,NEFT to borrower,\n"

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	part, _ := mw.CreateFormFile("file", "statement.csv")
	part.Write([]byte(csvBody))
	mw.Close()
	req := httptest.NewRequest(http.MethodPost, "/admin/reconciliation/bank-accounts/"+idStr2(account.ID)+"/import", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("import failed: %d %s", w.Code, w.Body.String())
	}

	var exceptions int64
	database.DB.Model(&models.ReconciliationException{}).Count(&exceptions)
	if exceptions == 0 {
		t.Fatal("expected at least one reconciliation exception for the ambiguous same-amount match")
	}
}

// --- 17. Manual reconciliation creates audit event ---

func TestManualMatchCreatesAuditEvent(t *testing.T) {
	r := setupReconTestApp(t)
	token := createReconTestAdmin(t, "recon_manual_"+t.Name(), models.AdminRoleAccountant)
	disb := createOutstandingDisbursement(t, 500000, "")

	txn := models.BankTransaction{
		BankAccountID: 0, TxnDate: time.Now(), ValueDate: time.Now(),
		AmountPaise: 500000, DrCr: "DR", Description: "manual test line", Status: models.BankTxnStatusUnmatched,
	}
	database.DB.Create(&txn)
	t.Cleanup(func() { database.DB.Unscoped().Delete(&txn) })

	req := httptest.NewRequest(http.MethodPost, "/admin/reconciliation/transactions/"+idStr2(txn.ID)+"/match",
		bytes.NewReader(mustJSON(gin.H{
			"matchedEntityType": "disbursement", "matchedEntityId": disb.ID,
			"reason": "Confirmed with bank portal reference screenshot",
		})))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("manual match failed: %d %s", w.Code, w.Body.String())
	}

	var auditCount int64
	database.DB.Model(&models.AuditLog{}).
		Where("action = ? AND target_id = ?", "reconciliation.manual_match", idStr2(txn.ID)).
		Count(&auditCount)
	if auditCount == 0 {
		t.Fatal("expected an audit log entry for the manual reconciliation match")
	}

	var record models.ReconciliationRecord
	if err := database.DB.Where("bank_transaction_id = ? AND match_type = ?", txn.ID, "manual").First(&record).Error; err != nil {
		t.Fatalf("expected a manual ReconciliationRecord: %v", err)
	}
	if record.Reason == "" || record.ReconciledBy == "" {
		t.Fatal("manual match must record who and why")
	}
}

// --- Manual match with no reason is rejected ---

func TestManualMatchRequiresReason(t *testing.T) {
	r := setupReconTestApp(t)
	token := createReconTestAdmin(t, "recon_noreason_"+t.Name(), models.AdminRoleAccountant)
	disb := createOutstandingDisbursement(t, 500000, "")

	txn := models.BankTransaction{TxnDate: time.Now(), ValueDate: time.Now(), AmountPaise: 500000, DrCr: "DR", Status: models.BankTxnStatusUnmatched}
	database.DB.Create(&txn)
	t.Cleanup(func() { database.DB.Unscoped().Delete(&txn) })

	req := httptest.NewRequest(http.MethodPost, "/admin/reconciliation/transactions/"+idStr2(txn.ID)+"/match",
		bytes.NewReader(mustJSON(gin.H{"matchedEntityType": "disbursement", "matchedEntityId": disb.ID})))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 without a reason, got %d", w.Code)
	}
}

func idStr2(id uint) string {
	b, _ := json.Marshal(id)
	return string(b)
}
