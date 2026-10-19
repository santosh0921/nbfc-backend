package kyc_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/joho/godotenv"

	"github.com/santosh0921/nbfc-backend/internal/admin"
	"github.com/santosh0921/nbfc-backend/internal/auth"
	"github.com/santosh0921/nbfc-backend/internal/bureau"
	"github.com/santosh0921/nbfc-backend/internal/config"
	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/kyc"
	"github.com/santosh0921/nbfc-backend/internal/middleware"
	"github.com/santosh0921/nbfc-backend/internal/models"
	"github.com/santosh0921/nbfc-backend/internal/notifications"
	"github.com/santosh0921/nbfc-backend/internal/rbac"
	"github.com/santosh0921/nbfc-backend/internal/screening"
)

func setupVerificationRequestDB(t *testing.T) {
	t.Helper()
	if database.DB != nil {
		return
	}
	_ = godotenv.Load("../../.env")
	cfg := config.Load()
	if err := auth.InitJWTSecret(cfg.JWTSecret); err != nil {
		t.Fatalf("InitJWTSecret: %v", err)
	}
	if err := admin.InitJWTSecret(cfg.AdminJWTSecret); err != nil {
		t.Fatalf("InitJWTSecret: %v", err)
	}
	database.Connect(cfg)
	admin.Migrate()
	bureau.Initialize()
	screening.Initialize()
}

func createTestCustomer(t *testing.T) models.User {
	t.Helper()
	customer := models.User{Mobile: "9" + uuid.New().String()[:9]}
	if err := database.DB.Create(&customer).Error; err != nil {
		t.Fatalf("create customer: %v", err)
	}
	t.Cleanup(func() { database.DB.Unscoped().Delete(&customer) })
	return customer
}

func createTestAdmin(t *testing.T, role string) (models.AdminUser, models.AdminSession) {
	t.Helper()
	adminUser := models.AdminUser{
		Username:     "vr_" + uuid.New().String()[:8],
		Email:        uuid.New().String()[:8] + "@example.com",
		PasswordHash: "x",
		Role:         role,
		Status:       models.AdminStatusActive,
	}
	if err := database.DB.Create(&adminUser).Error; err != nil {
		t.Fatalf("create admin: %v", err)
	}
	t.Cleanup(func() { database.DB.Unscoped().Delete(&adminUser) })

	session := models.AdminSession{
		AdminUserID:      adminUser.ID,
		RefreshTokenHash: uuid.New().String(),
	}
	if err := database.DB.Create(&session).Error; err != nil {
		t.Fatalf("create admin session: %v", err)
	}
	t.Cleanup(func() { database.DB.Unscoped().Delete(&session) })

	return adminUser, session
}

func adminBearerToken(t *testing.T, adminUser models.AdminUser, session models.AdminSession) string {
	t.Helper()
	token, err := admin.GenerateAccessToken(adminUser.ID, adminUser.Username, adminUser.Role, session.ID)
	if err != nil {
		t.Fatalf("generate admin token: %v", err)
	}
	return token
}

func customerBearerToken(t *testing.T, mobile string) string {
	t.Helper()
	token, err := auth.GenerateToken(mobile)
	if err != nil {
		t.Fatalf("generate customer token: %v", err)
	}
	return token
}

func TestAdminSendVerificationRequestCreatesRowAndNotification(t *testing.T) {
	setupVerificationRequestDB(t)
	customer := createTestCustomer(t)
	adminUser, session := createTestAdmin(t, "CREDIT_OFFICER")

	router := gin.New()
	admGroup := router.Group("/admin")
	admGroup.Use(middleware.AdminAuthMiddleware())
	admGroup.POST("/kyc/customers/:customerId/verification-requests", middleware.RequirePermission(rbac.KYCView), kyc.AdminSendVerificationRequestHandler)

	req := httptest.NewRequest(http.MethodPost, "/admin/kyc/customers/"+customer.ID.String()+"/verification-requests", bytes.NewReader(nil))
	req.Header.Set("Authorization", "Bearer "+adminBearerToken(t, adminUser, session))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var created models.KycVerificationRequest
	if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	t.Cleanup(func() { database.DB.Unscoped().Delete(&created) })

	if created.CustomerID != customer.ID.String() || created.Status != "pending" {
		t.Fatalf("unexpected created row: %+v", created)
	}

	var count int64
	database.DB.Model(&models.KycVerificationRequest{}).Where("customer_id = ?", customer.ID.String()).Count(&count)
	if count != 1 {
		t.Fatalf("expected 1 verification request row, got %d", count)
	}

	var note models.Notification
	if err := database.DB.Where("role = ? AND user_id = ?", "customer", customer.ID.String()).First(&note).Error; err != nil {
		t.Fatalf("expected a Notification row to be created: %v", err)
	}
	t.Cleanup(func() { database.DB.Unscoped().Delete(&note) })
	if note.Title != "Video verification requested" {
		t.Fatalf("unexpected notification title: %q", note.Title)
	}
}

func TestAdminSendVerificationRequestDeniedWithoutPermission(t *testing.T) {
	setupVerificationRequestDB(t)
	customer := createTestCustomer(t)
	// DISBURSEMENT_OFFICER has no KYCView permission.
	adminUser, session := createTestAdmin(t, "DISBURSEMENT_OFFICER")

	router := gin.New()
	admGroup := router.Group("/admin")
	admGroup.Use(middleware.AdminAuthMiddleware())
	admGroup.POST("/kyc/customers/:customerId/verification-requests", middleware.RequirePermission(rbac.KYCView), kyc.AdminSendVerificationRequestHandler)

	req := httptest.NewRequest(http.MethodPost, "/admin/kyc/customers/"+customer.ID.String()+"/verification-requests", bytes.NewReader(nil))
	req.Header.Set("Authorization", "Bearer "+adminBearerToken(t, adminUser, session))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCustomerListsOnlyOwnVerificationRequests(t *testing.T) {
	setupVerificationRequestDB(t)
	customerA := createTestCustomer(t)
	customerB := createTestCustomer(t)

	reqA := models.KycVerificationRequest{CustomerID: customerA.ID.String(), Status: "pending"}
	database.DB.Create(&reqA)
	t.Cleanup(func() { database.DB.Unscoped().Delete(&reqA) })

	reqB := models.KycVerificationRequest{CustomerID: customerB.ID.String(), Status: "pending"}
	database.DB.Create(&reqB)
	t.Cleanup(func() { database.DB.Unscoped().Delete(&reqB) })

	router := gin.New()
	router.GET("/kyc/verification-requests", middleware.AuthMiddleware(), kyc.CustomerListVerificationRequestsHandler)

	req := httptest.NewRequest(http.MethodGet, "/kyc/verification-requests", nil)
	req.Header.Set("Authorization", "Bearer "+customerBearerToken(t, customerA.Mobile))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var results []models.KycVerificationRequest
	if err := json.Unmarshal(w.Body.Bytes(), &results); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(results) != 1 || results[0].CustomerID != customerA.ID.String() {
		t.Fatalf("expected only customer A's own request, got %+v", results)
	}
}

func TestCustomerRespondToAnotherCustomersRequestForbidden(t *testing.T) {
	setupVerificationRequestDB(t)
	customerA := createTestCustomer(t)
	customerB := createTestCustomer(t)

	reqB := models.KycVerificationRequest{CustomerID: customerB.ID.String(), Status: "pending"}
	database.DB.Create(&reqB)
	t.Cleanup(func() { database.DB.Unscoped().Delete(&reqB) })

	router := gin.New()
	router.POST("/kyc/verification-requests/:id/respond", middleware.AuthMiddleware(), kyc.CustomerRespondVerificationRequestHandler)

	body, _ := json.Marshal(map[string]interface{}{"ready": true, "preferredTime": "today 5pm"})
	req := httptest.NewRequest(http.MethodPost, "/kyc/verification-requests/"+strconv.Itoa(int(reqB.ID))+"/respond", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+customerBearerToken(t, customerA.Mobile))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCustomerRespondReadySetsStatusAndPreferredTime(t *testing.T) {
	setupVerificationRequestDB(t)
	customer := createTestCustomer(t)

	reqRow := models.KycVerificationRequest{CustomerID: customer.ID.String(), Status: "pending"}
	database.DB.Create(&reqRow)
	t.Cleanup(func() { database.DB.Unscoped().Delete(&reqRow) })

	router := gin.New()
	router.POST("/kyc/verification-requests/:id/respond", middleware.AuthMiddleware(), kyc.CustomerRespondVerificationRequestHandler)

	body, _ := json.Marshal(map[string]interface{}{"ready": true, "preferredTime": "today 5pm"})
	req := httptest.NewRequest(http.MethodPost, "/kyc/verification-requests/"+strconv.Itoa(int(reqRow.ID))+"/respond", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+customerBearerToken(t, customer.Mobile))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var updated models.KycVerificationRequest
	if err := json.Unmarshal(w.Body.Bytes(), &updated); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if updated.Status != "customer_ready" || updated.PreferredTime != "today 5pm" || updated.RespondedAt == nil {
		t.Fatalf("unexpected updated row: %+v", updated)
	}
}

// pingHandler is a minimal stand-in downstream handler for exercising
// CallSignalingAuthMiddleware in isolation (a real *gin.Context, not the
// WebSocket upgrade itself).
func pingHandler(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) }

// TestAdminJWTAuthenticatesOnCallSignalingRoute confirms the dedicated
// CallSignalingAuthMiddleware (used ONLY by GET /ws/call/:loanId, kept
// deliberately separate from the widely-shared
// CustomerOrEmployeeAuthMiddleware — see that middleware's doc comment)
// accepts an admin JWT.
func TestAdminJWTAuthenticatesOnCallSignalingRoute(t *testing.T) {
	setupVerificationRequestDB(t)
	adminUser, session := createTestAdmin(t, "CREDIT_OFFICER")

	router := gin.New()
	router.GET("/ws/call/:loanId", middleware.CallSignalingAuthMiddleware(), pingHandler)

	req := httptest.NewRequest(http.MethodGet, "/ws/call/kyc-test", nil)
	req.Header.Set("Authorization", "Bearer "+adminBearerToken(t, adminUser, session))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("admin JWT via Authorization header should authenticate on the call-signaling route, got %d: %s", w.Code, w.Body.String())
	}

	// Sanity: a bogus token is still rejected.
	req2 := httptest.NewRequest(http.MethodGet, "/ws/call/kyc-test", nil)
	req2.Header.Set("Authorization", "Bearer not-a-real-token")
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)
	if w2.Code != http.StatusUnauthorized || !bytes.Contains(w2.Body.Bytes(), []byte("Invalid Token")) {
		t.Fatalf("expected bogus token to be rejected with Invalid Token, got %d: %s", w2.Code, w2.Body.String())
	}
}

// TestCallSignalingQueryParamTokenFallback confirms a browser WebSocket
// client — which cannot set a custom Authorization header on the
// handshake — can authenticate via ?token= instead, AND that this
// fallback is genuinely scoped to this one route: CustomerOrEmployee
// AuthMiddleware (guarding /notifications, /device-token, etc.) must NOT
// accept a query-param token.
func TestCallSignalingQueryParamTokenFallback(t *testing.T) {
	setupVerificationRequestDB(t)
	adminUser, session := createTestAdmin(t, "CREDIT_OFFICER")
	token := adminBearerToken(t, adminUser, session)

	router := gin.New()
	router.GET("/ws/call/:loanId", middleware.CallSignalingAuthMiddleware(), pingHandler)
	router.GET("/notifications", middleware.CustomerOrEmployeeAuthMiddleware(), notifications.ListHandler)

	// The signaling route accepts ?token=.
	req := httptest.NewRequest(http.MethodGet, "/ws/call/kyc-test?token="+token, nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected ?token= to authenticate on the call-signaling route, got %d: %s", w.Code, w.Body.String())
	}

	// The shared middleware must NOT — a query-param token must never work
	// on a data-returning route like /notifications.
	req2 := httptest.NewRequest(http.MethodGet, "/notifications?token="+token, nil)
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)
	if w2.Code != http.StatusUnauthorized {
		t.Fatalf("CustomerOrEmployeeAuthMiddleware must reject a query-param token (header-only), got %d: %s", w2.Code, w2.Body.String())
	}
}

