package documents_test

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
	"github.com/santosh0921/nbfc-backend/internal/documents"
	"github.com/santosh0921/nbfc-backend/internal/middleware"
	"github.com/santosh0921/nbfc-backend/internal/models"
	"github.com/santosh0921/nbfc-backend/internal/rbac"
)

// AdminListCustomerDocumentsHandler never validated the :id path segment
// was a real UUID — a malformed id (e.g. a customer's NAME, reachable
// from a stale/buggy frontend URL) went straight into a SQL query
// against a uuid-typed column, which Postgres rejects; that error was
// silently ignored (Find()'s return value was never checked), so the
// handler still replied 200 with an inconsistent body. Fixed to validate
// up front and reply a clean 400, matching every sibling customer-lookup
// handler on the same case-detail page.

func setupDocumentsTestApp(t *testing.T) *gin.Engine {
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
	adminGroup.GET("/customers/:id/documents", middleware.RequirePermission(rbac.CustomerView), documents.AdminListCustomerDocumentsHandler)
	return r
}

func docCreateTestAdmin(t *testing.T, username string) string {
	t.Helper()
	password := "Sup3rSecure!Passw0rd_1234"
	hash, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	u := models.AdminUser{Username: username, Email: username + "@test.local", PasswordHash: string(hash), Role: models.AdminRoleSuperAdmin, Status: models.AdminStatusActive}
	database.DB.Create(&u)
	t.Cleanup(func() { database.DB.Unscoped().Delete(&u) })
	return password
}

func docDoJSON(r *gin.Engine, method, path, token string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, &bytes.Buffer{})
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func docLoginAs(t *testing.T, r *gin.Engine, username, password string) string {
	t.Helper()
	req := httptest.NewRequest("POST", "/admin/login", bytes.NewBufferString(`{"username":"`+username+`","password":"`+password+`"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("admin login failed: %d %s", w.Code, w.Body.String())
	}
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	return resp["token"].(string)
}

func TestAdminListCustomerDocumentsRejectsNonUUIDWithCleanError(t *testing.T) {
	r := setupDocumentsTestApp(t)
	pw := docCreateTestAdmin(t, "sectest_docs_badid_"+t.Name())
	token := docLoginAs(t, r, "sectest_docs_badid_"+t.Name(), pw)

	w := docDoJSON(r, "GET", "/admin/customers/santosh%20more/documents", token)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for a non-UUID customer id, got %d: %s", w.Code, w.Body.String())
	}
	if !bytes.Contains(w.Body.Bytes(), []byte("Invalid customer id")) {
		t.Fatalf("expected a clean 'Invalid customer id' message, got: %s", w.Body.String())
	}
	// Must be a JSON object, not a bare array/null — the exact shape the
	// frontend's Array.isArray guard exists to tolerate even if this ever
	// regresses.
	var body map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("expected a JSON object body, got unparseable/non-object: %v (%s)", err, w.Body.String())
	}
}

func TestAdminListCustomerDocumentsReturnsArrayForValidCustomer(t *testing.T) {
	r := setupDocumentsTestApp(t)
	pw := docCreateTestAdmin(t, "sectest_docs_ok_"+t.Name())
	token := docLoginAs(t, r, "sectest_docs_ok_"+t.Name(), pw)

	customer := models.User{Mobile: "9" + uuid.New().String()[:9]}
	if err := database.DB.Create(&customer).Error; err != nil {
		t.Fatalf("create customer: %v", err)
	}
	t.Cleanup(func() { database.DB.Unscoped().Delete(&customer) })

	w := docDoJSON(r, "GET", "/admin/customers/"+customer.ID.String()+"/documents", token)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for a valid (if document-less) customer, got %d: %s", w.Code, w.Body.String())
	}
	var body []interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("expected a bare JSON array (even if empty), got: %v (%s)", err, w.Body.String())
	}
}
