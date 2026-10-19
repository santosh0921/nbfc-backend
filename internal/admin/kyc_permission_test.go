package admin_test

import (
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/kyc"
	"github.com/santosh0921/nbfc-backend/internal/middleware"
	"github.com/santosh0921/nbfc-backend/internal/models"
	"github.com/santosh0921/nbfc-backend/internal/rbac"
)

// Phase 13 — KYC/verification decision permission-bypass regression tests.
// Proves a raw direct-API call (no portal, no special headers — the exact
// same shape a legitimate admin client would send) to
// POST /admin/kyc/:customerId/approve and /reject is denied identically for
// an admin lacking KYC_APPROVE, and allowed for one holding it. Reuses
// setupTestApp/createTestAdmin/loginAs/doJSON from security_test.go in this
// same package (Phase 1 pattern).

func setupKYCPermissionTestApp(t *testing.T) *gin.Engine {
	t.Helper()
	r := setupTestApp(t)
	adminGroup := r.Group("/admin3") // avoid clashing with setupTestApp's own /admin group
	adminGroup.Use(middleware.AdminAuthMiddleware())
	adminGroup.POST("/kyc/:customerId/approve", middleware.RequirePermission(rbac.KYCApprove), kyc.AdminApproveHandler)
	adminGroup.POST("/kyc/:customerId/reject", middleware.RequirePermission(rbac.KYCApprove), kyc.AdminRejectHandler)
	return r
}

func createTestKYCCustomer(t *testing.T) uuid.UUID {
	t.Helper()
	id := uuid.New()
	user := models.User{ID: id, Mobile: "9" + uuid.NewString()[0:9], MPIN: "x", IsActive: true}
	if err := database.DB.Create(&user).Error; err != nil {
		t.Fatalf("create test customer: %v", err)
	}
	t.Cleanup(func() { database.DB.Unscoped().Delete(&user) })
	return id
}

func TestKYCApproveDeniedWithoutPermission(t *testing.T) {
	r := setupKYCPermissionTestApp(t)
	username := "sectest_kyc_noperm_" + t.Name()
	// READ_ONLY holds neither KYCApprove nor any KYC-decision permission.
	_, password := createTestAdmin(t, username, models.AdminRoleReadOnly)
	token, _ := loginAs(t, r, username, password)

	customerID := createTestKYCCustomer(t)
	w := doJSON(r, "POST", "/admin3/kyc/"+customerID.String()+"/approve", token, nil)
	if w.Code != http.StatusForbidden {
		t.Fatalf("an admin without KYC_APPROVE must be denied, got %d: %s", w.Code, w.Body.String())
	}
}

func TestKYCRejectDeniedWithoutPermission(t *testing.T) {
	r := setupKYCPermissionTestApp(t)
	username := "sectest_kyc_noperm_reject_" + t.Name()
	_, password := createTestAdmin(t, username, models.AdminRoleReadOnly)
	token, _ := loginAs(t, r, username, password)

	customerID := createTestKYCCustomer(t)
	w := doJSON(r, "POST", "/admin3/kyc/"+customerID.String()+"/reject", token,
		map[string]interface{}{"reason": "attempted bypass"})
	if w.Code != http.StatusForbidden {
		t.Fatalf("an admin without KYC_APPROVE must be denied for reject too, got %d: %s", w.Code, w.Body.String())
	}
}

func TestKYCApproveDirectAPIDeniedIdenticallyWithoutPortal(t *testing.T) {
	r := setupKYCPermissionTestApp(t)
	username := "sectest_kyc_direct_" + t.Name()
	_, password := createTestAdmin(t, username, "COLLECTION_OFFICER") // a real, unrelated limited role, no KYC permission
	token, _ := loginAs(t, r, username, password)

	customerID := createTestKYCCustomer(t)
	// Raw HTTP request, no portal/browser involved at all — same shape
	// doJSON always sends, proving the backend enforces this identically
	// regardless of what client called it.
	w := doJSON(r, "POST", "/admin3/kyc/"+customerID.String()+"/approve", token, nil)
	if w.Code != http.StatusForbidden {
		t.Fatalf("direct API KYC approval bypass attempt must be denied, got %d: %s", w.Code, w.Body.String())
	}
}

func TestKYCApproveAllowedWithPermission(t *testing.T) {
	r := setupKYCPermissionTestApp(t)
	username := "sectest_kyc_allowed_" + t.Name()
	_, password := createTestAdmin(t, username, models.AdminRoleSuperAdmin) // holds KYC_APPROVE
	token, _ := loginAs(t, r, username, password)

	customerID := createTestKYCCustomer(t)
	w := doJSON(r, "POST", "/admin3/kyc/"+customerID.String()+"/approve", token, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("an admin holding KYC_APPROVE should be able to approve, got %d: %s", w.Code, w.Body.String())
	}
}
