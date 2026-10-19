package admin_test

import (
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/santosh0921/nbfc-backend/internal/admin"
	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/middleware"
	"github.com/santosh0921/nbfc-backend/internal/models"
	"github.com/santosh0921/nbfc-backend/internal/rbac"
)

// Self-registration + Super Admin approval regression tests: a
// self-registered portal account must be unable to log in until approved,
// only a Super Admin can approve it, and once approved it never needs
// re-approval on subsequent logins.

func setupRegistrationTestApp(t *testing.T) *gin.Engine {
	t.Helper()
	r := setupTestApp(t)
	r.POST("/admin/register", admin.RegisterHandler)
	adminGroup := r.Group("/admin4")
	adminGroup.Use(middleware.AdminAuthMiddleware())
	adminGroup.PUT("/admin-users/:id/approve", middleware.RequirePermission(rbac.AdminManage), admin.AdminApproveAdminUserHandler)
	adminGroup.DELETE("/admin-users/:id/reject", middleware.RequirePermission(rbac.AdminManage), admin.AdminRejectAdminUserHandler)
	return r
}

func TestSelfRegisteredAccountCannotLoginUntilApproved(t *testing.T) {
	r := setupRegistrationTestApp(t)
	username := "sectest_selfreg_" + t.Name()
	password := "Sup3rSecure!Passw0rd_1234"

	wReg := doJSON(r, "POST", "/admin/register", "", map[string]interface{}{
		"username": username, "email": username + "@test.local", "password": password, "role": "COLLECTION_OFFICER",
	})
	if wReg.Code != http.StatusCreated {
		t.Fatalf("self-registration should succeed, got %d: %s", wReg.Code, wReg.Body.String())
	}
	t.Cleanup(func() {
		database.DB.Unscoped().Where("username = ?", username).Delete(&models.AdminUser{})
	})

	wLogin := doJSON(r, "POST", "/admin/login", "", map[string]interface{}{"username": username, "password": password})
	if wLogin.Code != http.StatusForbidden {
		t.Fatalf("an unapproved self-registered account must not be able to log in, got %d: %s", wLogin.Code, wLogin.Body.String())
	}
}

func TestSelfRegisteredAccountCannotSelfRegisterAsSuperAdmin(t *testing.T) {
	r := setupRegistrationTestApp(t)
	username := "sectest_selfreg_sa_" + t.Name()
	w := doJSON(r, "POST", "/admin/register", "", map[string]interface{}{
		"username": username, "email": username + "@test.local", "password": "Sup3rSecure!Passw0rd_1234", "role": "SUPER_ADMIN",
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("self-registering as SUPER_ADMIN must be refused, got %d: %s", w.Code, w.Body.String())
	}
}

func TestOnlySuperAdminCanApprove(t *testing.T) {
	r := setupRegistrationTestApp(t)
	regUsername := "sectest_selfreg2_" + t.Name()
	regPassword := "Sup3rSecure!Passw0rd_1234"
	doJSON(r, "POST", "/admin/register", "", map[string]interface{}{
		"username": regUsername, "email": regUsername + "@test.local", "password": regPassword, "role": "COLLECTION_OFFICER",
	})
	var target models.AdminUser
	database.DB.Where("username = ?", regUsername).First(&target)
	t.Cleanup(func() { database.DB.Unscoped().Delete(&target) })

	// A non-SUPER_ADMIN admin (who lacks AdminManage) must be denied.
	nonAdminUsername := "sectest_nonadmin_" + t.Name()
	_, nonAdminPw := createTestAdmin(t, nonAdminUsername, "COLLECTION_MANAGER")
	nonAdminToken, _ := loginAs(t, r, nonAdminUsername, nonAdminPw)

	wDenied := doJSON(r, "PUT", "/admin4/admin-users/"+itoaTest(target.ID)+"/approve", nonAdminToken, nil)
	if wDenied.Code != http.StatusForbidden {
		t.Fatalf("a non-SUPER_ADMIN must not be able to approve accounts, got %d: %s", wDenied.Code, wDenied.Body.String())
	}

	// A SUPER_ADMIN must succeed.
	saUsername := "sectest_superadmin_" + t.Name()
	_, saPw := createTestAdmin(t, saUsername, models.AdminRoleSuperAdmin)
	saToken, _ := loginAs(t, r, saUsername, saPw)

	wApprove := doJSON(r, "PUT", "/admin4/admin-users/"+itoaTest(target.ID)+"/approve", saToken, nil)
	if wApprove.Code != http.StatusOK {
		t.Fatalf("a SUPER_ADMIN should be able to approve, got %d: %s", wApprove.Code, wApprove.Body.String())
	}
}

func TestApprovedAccountLogsInAndNeverNeedsReapproval(t *testing.T) {
	r := setupRegistrationTestApp(t)
	regUsername := "sectest_selfreg3_" + t.Name()
	regPassword := "Sup3rSecure!Passw0rd_1234"
	doJSON(r, "POST", "/admin/register", "", map[string]interface{}{
		"username": regUsername, "email": regUsername + "@test.local", "password": regPassword, "role": "COLLECTION_OFFICER",
	})
	var target models.AdminUser
	database.DB.Where("username = ?", regUsername).First(&target)
	t.Cleanup(func() { database.DB.Unscoped().Delete(&target) })

	saUsername := "sectest_superadmin2_" + t.Name()
	_, saPw := createTestAdmin(t, saUsername, models.AdminRoleSuperAdmin)
	saToken, _ := loginAs(t, r, saUsername, saPw)
	doJSON(r, "PUT", "/admin4/admin-users/"+itoaTest(target.ID)+"/approve", saToken, nil)

	// First login after approval — must succeed.
	w1 := doJSON(r, "POST", "/admin/login", "", map[string]interface{}{"username": regUsername, "password": regPassword})
	if w1.Code != http.StatusOK {
		t.Fatalf("login after approval should succeed, got %d: %s", w1.Code, w1.Body.String())
	}

	// Second (and any later) login — no re-approval needed, must also succeed.
	w2 := doJSON(r, "POST", "/admin/login", "", map[string]interface{}{"username": regUsername, "password": regPassword})
	if w2.Code != http.StatusOK {
		t.Fatalf("a second login after approval must succeed with no re-approval gate, got %d: %s", w2.Code, w2.Body.String())
	}
}

func TestRejectDeletesPendingAccount(t *testing.T) {
	r := setupRegistrationTestApp(t)
	regUsername := "sectest_reject_" + t.Name()
	regPassword := "Sup3rSecure!Passw0rd_1234"
	doJSON(r, "POST", "/admin/register", "", map[string]interface{}{
		"username": regUsername, "email": regUsername + "@test.local", "password": regPassword, "role": "COLLECTION_OFFICER",
	})
	var target models.AdminUser
	database.DB.Where("username = ?", regUsername).First(&target)

	saUsername := "sectest_superadmin4_" + t.Name()
	_, saPw := createTestAdmin(t, saUsername, models.AdminRoleSuperAdmin)
	saToken, _ := loginAs(t, r, saUsername, saPw)

	wReject := doJSON(r, "DELETE", "/admin4/admin-users/"+itoaTest(target.ID)+"/reject", saToken, nil)
	if wReject.Code != http.StatusOK {
		t.Fatalf("rejecting a pending account should succeed, got %d: %s", wReject.Code, wReject.Body.String())
	}

	var count int64
	database.DB.Model(&models.AdminUser{}).Where("id = ?", target.ID).Count(&count)
	if count != 0 {
		t.Fatalf("a rejected account must be deleted, but %d row(s) remain", count)
	}

	wLogin := doJSON(r, "POST", "/admin/login", "", map[string]interface{}{"username": regUsername, "password": regPassword})
	if wLogin.Code == http.StatusOK {
		t.Fatalf("a rejected (deleted) account must never be able to log in")
	}
}

func TestRejectRefusedForAlreadyApprovedAccount(t *testing.T) {
	r := setupRegistrationTestApp(t)
	saUsername := "sectest_superadmin5_" + t.Name()
	_, saPw := createTestAdmin(t, saUsername, models.AdminRoleSuperAdmin)
	saToken, _ := loginAs(t, r, saUsername, saPw)

	approvedUsername := "sectest_alreadyapproved_" + t.Name()
	_, _ = createTestAdmin(t, approvedUsername, "COLLECTION_OFFICER") // Approved: true
	var target models.AdminUser
	database.DB.Where("username = ?", approvedUsername).First(&target)

	w := doJSON(r, "DELETE", "/admin4/admin-users/"+itoaTest(target.ID)+"/reject", saToken, nil)
	if w.Code != http.StatusConflict {
		t.Fatalf("rejecting an already-approved account must be refused (use disable instead), got %d: %s", w.Code, w.Body.String())
	}
}

func TestApprovingAlreadyApprovedAccountConflicts(t *testing.T) {
	r := setupRegistrationTestApp(t)
	saUsername := "sectest_superadmin3_" + t.Name()
	_, saPw := createTestAdmin(t, saUsername, models.AdminRoleSuperAdmin)
	saToken, _ := loginAs(t, r, saUsername, saPw)

	alreadyApprovedUsername := "sectest_alreadyok_" + t.Name()
	_, _ = createTestAdmin(t, alreadyApprovedUsername, "COLLECTION_OFFICER") // admin-created => Approved: true
	var target models.AdminUser
	database.DB.Where("username = ?", alreadyApprovedUsername).First(&target)

	w := doJSON(r, "PUT", "/admin4/admin-users/"+itoaTest(target.ID)+"/approve", saToken, nil)
	if w.Code != http.StatusConflict {
		t.Fatalf("re-approving an already-approved account must conflict, got %d: %s", w.Code, w.Body.String())
	}
}
