package admin_test

import (
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/santosh0921/nbfc-backend/internal/collections"
	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/middleware"
	"github.com/santosh0921/nbfc-backend/internal/models"
	"github.com/santosh0921/nbfc-backend/internal/rbac"
)

// Phase 6 — collection-case assignment permission regression tests.
//
// The prior audit claimed POST /admin/collections/cases/:id/assign checked
// COLLECTION_RECORD (an officer-level permission) instead of the intended
// COLLECTION_MANAGE, letting an ordinary Collection Officer self-assign
// cases like a manager. Reading internal/routes/routes.go directly shows
// this route is already gated on rbac.CollectionManage, which
// COLLECTION_OFFICER does not hold — so the claimed gap does not exist in
// the current code. These tests lock that in, as required by Phase 6:
// Collection Officer -> denied, Collection Manager -> allowed, direct API
// (no portal) -> denied identically.

func setupCollectionsTestApp(t *testing.T) *gin.Engine {
	t.Helper()
	r := setupTestApp(t)
	adminGroup := r.Group("/admin2") // avoid clashing with setupTestApp's own /admin group
	adminGroup.Use(middleware.AdminAuthMiddleware())
	adminGroup.POST("/collections/cases/:id/assign", middleware.RequirePermission(rbac.CollectionManage), collections.AdminAssignCaseHandler)
	return r
}

func createTestCollectionCase(t *testing.T) models.CollectionCase {
	t.Helper()
	cc := models.CollectionCase{
		LoanID: 1, CustomerID: testCustomerID().String(),
		DPD: 45, OverdueAmountPaise: 500000, Status: models.CollectionStatusOverdue,
	}
	if err := database.DB.Create(&cc).Error; err != nil {
		t.Fatalf("create test collection case: %v", err)
	}
	t.Cleanup(func() { database.DB.Unscoped().Delete(&cc) })
	return cc
}

func TestCollectionOfficerCannotAssign(t *testing.T) {
	r := setupCollectionsTestApp(t)
	username := "sectest_colloff_" + t.Name()
	_, password := createTestAdmin(t, username, "COLLECTION_OFFICER")
	token, _ := loginAs(t, r, username, password)

	cc := createTestCollectionCase(t)
	w := doJSON(r, "POST", "/admin2/collections/cases/"+itoaTest(cc.ID)+"/assign", token,
		map[string]interface{}{"assignToAdminId": 999, "assignToName": "Someone"})
	if w.Code != http.StatusForbidden {
		t.Fatalf("a Collection Officer must not be able to assign cases, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCollectionManagerCanAssign(t *testing.T) {
	r := setupCollectionsTestApp(t)
	username := "sectest_collmgr_" + t.Name()
	_, password := createTestAdmin(t, username, "COLLECTION_MANAGER")
	token, _ := loginAs(t, r, username, password)

	cc := createTestCollectionCase(t)
	w := doJSON(r, "POST", "/admin2/collections/cases/"+itoaTest(cc.ID)+"/assign", token,
		map[string]interface{}{"assignToAdminId": 999, "assignToName": "Someone"})
	if w.Code != http.StatusOK {
		t.Fatalf("a Collection Manager should be able to assign cases, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCollectionAssignDirectAPIDeniedForOfficer(t *testing.T) {
	r := setupCollectionsTestApp(t)
	username := "sectest_colloff_direct_" + t.Name()
	_, password := createTestAdmin(t, username, "COLLECTION_OFFICER")
	token, _ := loginAs(t, r, username, password)

	cc := createTestCollectionCase(t)
	// Simulates a raw HTTP client bypassing any portal UI entirely — same
	// request shape, no browser/React involved.
	w := doJSON(r, "POST", "/admin2/collections/cases/"+itoaTest(cc.ID)+"/assign", token,
		map[string]interface{}{"assignToAdminId": 999, "assignToName": "Someone"})
	if w.Code != http.StatusForbidden {
		t.Fatalf("direct API self-assignment attempt by an officer must be denied identically to the portal path, got %d: %s", w.Code, w.Body.String())
	}
}
