package admin_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/middleware"
	"github.com/santosh0921/nbfc-backend/internal/models"
	"github.com/santosh0921/nbfc-backend/internal/notifications"
)

// Phase 7 — admin-scoped notification routes. The KYC portal's bell was
// calling GET /notifications, which only resolves customer/employee JWTs
// (see internal/middleware/customer_or_employee_auth_middleware.go) and
// 401s for an admin token. These new /admin/notifications routes are gated
// by AdminAuthMiddleware instead, reusing the same Notification model.

func setupAdminNotificationsTestApp(t *testing.T) *gin.Engine {
	t.Helper()
	r := setupTestApp(t)
	adminGroup := r.Group("/admin3")
	adminGroup.Use(middleware.AdminAuthMiddleware())
	adminGroup.GET("/notifications", notifications.AdminListHandler)
	adminGroup.POST("/notifications/:id/read", notifications.AdminMarkReadHandler)
	return r
}

func TestAdminNotificationsListAndMarkRead(t *testing.T) {
	r := setupAdminNotificationsTestApp(t)
	username := "sectest_adminnotif_" + t.Name()
	adminID, password := createTestAdmin(t, username, models.AdminRoleSuperAdmin)
	token, _ := loginAs(t, r, username, password)

	notifications.Create("admin", itoaTest(adminID), "Test notice", "hello")
	t.Cleanup(func() {
		database.DB.Unscoped().Where("role = ? AND user_id = ?", "admin", itoaTest(adminID)).Delete(&models.Notification{})
	})

	w := doJSON(r, "GET", "/admin3/notifications", token, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("admin should be able to list their own notifications, got %d: %s", w.Code, w.Body.String())
	}

	var list []models.Notification
	if err := json.Unmarshal(w.Body.Bytes(), &list); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 notification, got %d", len(list))
	}

	w2 := doJSON(r, "POST", "/admin3/notifications/"+itoaTest(list[0].ID)+"/read", token, nil)
	if w2.Code != http.StatusOK {
		t.Fatalf("mark-read should succeed, got %d: %s", w2.Code, w2.Body.String())
	}
}

func TestAdminNotificationsRequireAdminAuth(t *testing.T) {
	r := setupAdminNotificationsTestApp(t)
	w := doJSON(r, "GET", "/admin3/notifications", "", nil)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("no token should be rejected, got %d: %s", w.Code, w.Body.String())
	}
}
