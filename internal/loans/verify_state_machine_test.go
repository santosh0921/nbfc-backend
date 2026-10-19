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

	"github.com/santosh0921/nbfc-backend/internal/config"
	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/employee"
	"github.com/santosh0921/nbfc-backend/internal/loans"
	"github.com/santosh0921/nbfc-backend/internal/middleware"
	"github.com/santosh0921/nbfc-backend/internal/models"
)

// Phase 5 — EmployeeVerifyLoanHandler state-whitelist regression tests.
// Before this fix the handler only checked assignment ownership; it would
// happily flip a sanctioned, rejected or disbursed loan back to "verified".

func setupVerifyTestApp(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)

	if database.DB == nil {
		_ = godotenv.Load("../../.env")
		cfg := config.Load()
		if err := employee.InitJWTSecret(cfg.EmployeeJWTSecret); err != nil {
			t.Fatalf("InitJWTSecret: %v", err)
		}
		database.Connect(cfg)
	}

	r := gin.New()
	r.POST("/employee/login", employee.LoginHandler)
	empGroup := r.Group("/employee")
	empGroup.Use(middleware.EmployeeAuthMiddleware())
	empGroup.POST("/loans/:id/verify", loans.EmployeeVerifyLoanHandler)
	return r
}

func createVerifyTestEmployee(t *testing.T) (id uint, name, password string) {
	t.Helper()
	name = "verify_emp_" + t.Name()
	password = "Sup3rSecure!Passw0rd_1234"
	hash, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	e := models.Employee{
		Code: "V-" + t.Name(), Name: name, PasswordHash: string(hash),
		Role: "verification", Active: true, Approved: true,
	}
	if err := database.DB.Create(&e).Error; err != nil {
		t.Fatalf("create test employee: %v", err)
	}
	t.Cleanup(func() { database.DB.Unscoped().Delete(&e) })
	return e.ID, name, password
}

func verifyDoJSON(r *gin.Engine, method, path, token string, body interface{}) *httptest.ResponseRecorder {
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

func verifyLoginAs(t *testing.T, r *gin.Engine, name, password string) string {
	t.Helper()
	w := verifyDoJSON(r, "POST", "/employee/login", "", gin.H{"name": name, "password": password})
	if w.Code != http.StatusOK {
		t.Fatalf("employee login failed: %d %s", w.Code, w.Body.String())
	}
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	return resp["token"].(string)
}

func createAssignedTestLoan(t *testing.T, empID uint, status string) models.LoanApplication {
	t.Helper()
	loan := models.LoanApplication{
		CustomerID: uuid.New(), Category: "Personal Loan", AmountRequested: 20000,
		Status: status, ApplicantName: "Verify Test Applicant",
		AssignedEmployeeID: &empID,
	}
	if err := database.DB.Create(&loan).Error; err != nil {
		t.Fatalf("create test loan: %v", err)
	}
	t.Cleanup(func() { database.DB.Unscoped().Delete(&loan) })
	return loan
}

func TestVerifyValidFromAssigned(t *testing.T) {
	r := setupVerifyTestApp(t)
	empID, name, password := createVerifyTestEmployee(t)
	token := verifyLoginAs(t, r, name, password)

	loan := createAssignedTestLoan(t, empID, models.LoanStatusAssigned)
	w := verifyDoJSON(r, "POST", "/employee/loans/"+idStr(loan.ID)+"/verify", token,
		map[string]interface{}{"findings": "ok", "remarks": "ok", "riskAssessment": "low", "recommendation": "approve"})
	if w.Code != http.StatusOK {
		t.Fatalf("the normal ASSIGNED -> VERIFIED flow must keep working, got %d: %s", w.Code, w.Body.String())
	}
}

func TestVerifyValidFromPendingClaim(t *testing.T) {
	r := setupVerifyTestApp(t)
	empID, name, password := createVerifyTestEmployee(t)
	token := verifyLoginAs(t, r, name, password)

	loan := createAssignedTestLoan(t, empID, models.LoanStatusPendingClaim)
	w := verifyDoJSON(r, "POST", "/employee/loans/"+idStr(loan.ID)+"/verify", token,
		map[string]interface{}{"findings": "ok", "remarks": "ok", "riskAssessment": "low", "recommendation": "approve"})
	if w.Code != http.StatusOK {
		t.Fatalf("pending_claim re-verification must remain possible, got %d: %s", w.Code, w.Body.String())
	}
}

func TestVerifyRejectedFromAlreadyVerified(t *testing.T) {
	r := setupVerifyTestApp(t)
	empID, name, password := createVerifyTestEmployee(t)
	token := verifyLoginAs(t, r, name, password)

	loan := createAssignedTestLoan(t, empID, models.LoanStatusVerified)
	w := verifyDoJSON(r, "POST", "/employee/loans/"+idStr(loan.ID)+"/verify", token,
		map[string]interface{}{"findings": "ok", "remarks": "ok"})
	if w.Code != http.StatusConflict {
		t.Fatalf("re-verifying an already-verified loan must be refused, got %d: %s", w.Code, w.Body.String())
	}
}

func TestVerifyRejectedFromSanctioned(t *testing.T) {
	r := setupVerifyTestApp(t)
	empID, name, password := createVerifyTestEmployee(t)
	token := verifyLoginAs(t, r, name, password)

	loan := createAssignedTestLoan(t, empID, models.LoanStatusSanctioned)
	w := verifyDoJSON(r, "POST", "/employee/loans/"+idStr(loan.ID)+"/verify", token,
		map[string]interface{}{"findings": "ok", "remarks": "ok"})
	if w.Code != http.StatusConflict {
		t.Fatalf("a sanctioned loan must not be flipped back to verified, got %d: %s", w.Code, w.Body.String())
	}
}

func TestVerifyRejectedFromDisbursed(t *testing.T) {
	r := setupVerifyTestApp(t)
	empID, name, password := createVerifyTestEmployee(t)
	token := verifyLoginAs(t, r, name, password)

	loan := createAssignedTestLoan(t, empID, models.LoanStatusDisbursed)
	w := verifyDoJSON(r, "POST", "/employee/loans/"+idStr(loan.ID)+"/verify", token,
		map[string]interface{}{"findings": "ok", "remarks": "ok"})
	if w.Code != http.StatusConflict {
		t.Fatalf("a disbursed loan must never be reachable by loan verification, got %d: %s", w.Code, w.Body.String())
	}
}

func TestVerifyRejectedFromRejected(t *testing.T) {
	r := setupVerifyTestApp(t)
	empID, name, password := createVerifyTestEmployee(t)
	token := verifyLoginAs(t, r, name, password)

	loan := createAssignedTestLoan(t, empID, models.LoanStatusRejected)
	w := verifyDoJSON(r, "POST", "/employee/loans/"+idStr(loan.ID)+"/verify", token,
		map[string]interface{}{"findings": "ok", "remarks": "ok"})
	if w.Code != http.StatusConflict {
		t.Fatalf("a rejected loan must not be revivable via verification, got %d: %s", w.Code, w.Body.String())
	}
}
