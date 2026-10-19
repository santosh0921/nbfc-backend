package recovery_test

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
	"github.com/santosh0921/nbfc-backend/internal/middleware"
	"github.com/santosh0921/nbfc-backend/internal/models"
	"github.com/santosh0921/nbfc-backend/internal/rbac"
	"github.com/santosh0921/nbfc-backend/internal/recovery"
)

// Connects the Collections & Recovery portal to the real RecoveryCase
// table the Employee app already reads from — previously the portal had
// zero visibility into it (only /admin/recovery-reports existed, which
// lists REPORTS, not cases) and no way to assign a case to a specific
// recovery employee at all.

func setupRecoveryTestApp(t *testing.T) *gin.Engine {
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
	adminGroup.GET("/recovery/cases", middleware.RequirePermission(rbac.CollectionView), recovery.AdminListCasesHandler)
	adminGroup.GET("/recovery/employees", middleware.RequirePermission(rbac.CollectionView), recovery.AdminListRecoveryEmployeesHandler)
	adminGroup.POST("/recovery/cases/:id/assign", middleware.RequirePermission(rbac.CollectionManage), recovery.AdminManualAssignCaseHandler)
	return r
}

func recDoJSON(r *gin.Engine, method, path, token string, body interface{}) *httptest.ResponseRecorder {
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

func recCreateTestAdmin(t *testing.T, username, role string) string {
	t.Helper()
	password := "Sup3rSecure!Passw0rd_1234"
	hash, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	u := models.AdminUser{Username: username, Email: username + "@test.local", PasswordHash: string(hash), Role: role, Status: models.AdminStatusActive, Approved: true}
	database.DB.Create(&u)
	t.Cleanup(func() { database.DB.Unscoped().Delete(&u) })
	return password
}

func recLoginAsAdmin(t *testing.T, r *gin.Engine, username, password string) string {
	t.Helper()
	w := recDoJSON(r, "POST", "/admin/login", "", gin.H{"username": username, "password": password})
	if w.Code != http.StatusOK {
		t.Fatalf("admin login failed: %d %s", w.Code, w.Body.String())
	}
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	return resp["token"].(string)
}

func createTestRecoveryEmployee(t *testing.T, city string) models.Employee {
	t.Helper()
	e := models.Employee{
		Code: "REC-" + uuid.New().String()[:8], Name: "Test Recovery Agent",
		Role: "recovery", BranchCity: city, Active: true, Approved: true,
	}
	if err := database.DB.Create(&e).Error; err != nil {
		t.Fatalf("create recovery employee: %v", err)
	}
	t.Cleanup(func() { database.DB.Unscoped().Delete(&e) })
	return e
}

// createTestInactiveRecoveryEmployee: Employee.Active has a
// `gorm:"default:true"` tag, so GORM silently omits an explicit
// `Active: false` on Create (the same zero-value-omission gotcha this
// session already hit once with AdminUser.Approved) — the DB default then
// makes the row Active=true regardless. A follow-up single-column Update
// forces the real value.
func createTestInactiveRecoveryEmployee(t *testing.T, city string) models.Employee {
	t.Helper()
	e := models.Employee{
		Code: "REC-INACTIVE-" + uuid.New().String()[:8], Name: "Inactive Agent",
		Role: "recovery", BranchCity: city, Approved: true,
	}
	if err := database.DB.Create(&e).Error; err != nil {
		t.Fatalf("create inactive recovery employee: %v", err)
	}
	if err := database.DB.Model(&e).Update("active", false).Error; err != nil {
		t.Fatalf("force inactive: %v", err)
	}
	t.Cleanup(func() { database.DB.Unscoped().Delete(&e) })
	return e
}

func createTestRecoveryCase(t *testing.T, empID uint, city string) models.RecoveryCase {
	t.Helper()
	rc := models.RecoveryCase{
		LoanRef: "loan:test-" + uuid.New().String()[:8], CustomerName: "Test Customer",
		OverdueAmount: 5000, City: city, AssignedEmployeeID: empID, Status: "open",
	}
	if err := database.DB.Create(&rc).Error; err != nil {
		t.Fatalf("create recovery case: %v", err)
	}
	t.Cleanup(func() { database.DB.Unscoped().Delete(&rc) })
	return rc
}

func TestAdminListCasesIncludesRealEmployeeName(t *testing.T) {
	r := setupRecoveryTestApp(t)
	pw := recCreateTestAdmin(t, "sectest_reccases_"+t.Name(), models.AdminRoleSuperAdmin)
	token := recLoginAsAdmin(t, r, "sectest_reccases_"+t.Name(), pw)

	emp := createTestRecoveryEmployee(t, "Pune")
	rc := createTestRecoveryCase(t, emp.ID, "Pune")

	w := recDoJSON(r, "GET", "/admin/recovery/cases?city=Pune", token, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var rows []map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &rows)

	var found map[string]interface{}
	for _, row := range rows {
		if uint(row["id"].(float64)) == rc.ID {
			found = row
			break
		}
	}
	if found == nil {
		t.Fatalf("expected to find the test case in the response")
	}
	if found["assignedEmployeeName"] != emp.Name {
		t.Fatalf("expected assignedEmployeeName %q, got %v", emp.Name, found["assignedEmployeeName"])
	}
}

func TestAdminListRecoveryEmployeesFiltersInactiveAndSearches(t *testing.T) {
	r := setupRecoveryTestApp(t)
	pw := recCreateTestAdmin(t, "sectest_recemps_"+t.Name(), models.AdminRoleSuperAdmin)
	token := recLoginAsAdmin(t, r, "sectest_recemps_"+t.Name(), pw)

	active := createTestRecoveryEmployee(t, "Nagpur")
	inactive := createTestInactiveRecoveryEmployee(t, "Nagpur")

	w := recDoJSON(r, "GET", "/admin/recovery/employees?city=Nagpur", token, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var rows []map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &rows)

	foundActive, foundInactive := false, false
	for _, row := range rows {
		if row["id"].(float64) == float64(active.ID) {
			foundActive = true
		}
		if row["id"].(float64) == float64(inactive.ID) {
			foundInactive = true
		}
	}
	if !foundActive {
		t.Fatal("expected the active recovery employee to be listed")
	}
	if foundInactive {
		t.Fatal("an inactive employee must never be offered as an assignment target")
	}
}

func TestAdminManualAssignRequiresCollectionManage(t *testing.T) {
	r := setupRecoveryTestApp(t)
	officerPw := recCreateTestAdmin(t, "sectest_recoff_"+t.Name(), "COLLECTION_OFFICER")
	officerToken := recLoginAsAdmin(t, r, "sectest_recoff_"+t.Name(), officerPw)

	emp := createTestRecoveryEmployee(t, "Indore")
	rc := createTestRecoveryCase(t, emp.ID, "Indore")

	w := recDoJSON(r, "POST", "/admin/recovery/cases/"+itoaRec(rc.ID)+"/assign", officerToken, gin.H{"employeeId": emp.ID})
	if w.Code != http.StatusForbidden {
		t.Fatalf("a COLLECTION_OFFICER (no CollectionManage) must not be able to reassign, got %d: %s", w.Code, w.Body.String())
	}
}

func TestAdminManualAssignReassignsCaseToDifferentEmployee(t *testing.T) {
	r := setupRecoveryTestApp(t)
	pw := recCreateTestAdmin(t, "sectest_recmgr_"+t.Name(), "COLLECTION_MANAGER")
	token := recLoginAsAdmin(t, r, "sectest_recmgr_"+t.Name(), pw)

	empA := createTestRecoveryEmployee(t, "Bhopal")
	empB := createTestRecoveryEmployee(t, "Bhopal")
	rc := createTestRecoveryCase(t, empA.ID, "Bhopal")

	w := recDoJSON(r, "POST", "/admin/recovery/cases/"+itoaRec(rc.ID)+"/assign", token, gin.H{"employeeId": empB.ID, "remarks": "rebalancing load"})
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var reloaded models.RecoveryCase
	database.DB.First(&reloaded, rc.ID)
	if reloaded.AssignedEmployeeID != empB.ID {
		t.Fatalf("expected case reassigned to employee %d, got %d", empB.ID, reloaded.AssignedEmployeeID)
	}
}

func TestAdminManualAssignRejectsInactiveEmployee(t *testing.T) {
	r := setupRecoveryTestApp(t)
	pw := recCreateTestAdmin(t, "sectest_recmgr2_"+t.Name(), "COLLECTION_MANAGER")
	token := recLoginAsAdmin(t, r, "sectest_recmgr2_"+t.Name(), pw)

	activeEmp := createTestRecoveryEmployee(t, "Surat")
	rc := createTestRecoveryCase(t, activeEmp.ID, "Surat")
	inactive := createTestInactiveRecoveryEmployee(t, "Surat")

	w := recDoJSON(r, "POST", "/admin/recovery/cases/"+itoaRec(rc.ID)+"/assign", token, gin.H{"employeeId": inactive.ID})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("assigning to an inactive employee must be rejected, got %d: %s", w.Code, w.Body.String())
	}
}

func itoaRec(id uint) string {
	b, _ := json.Marshal(id)
	return string(b)
}
