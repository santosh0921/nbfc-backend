package admin_test

import (
	"net/http"
	"testing"

	"github.com/santosh0921/nbfc-backend/internal/models"
)

// Portal role-selector regression tests: expectedRole is optional and, when
// sent, must be enforced server-side, not merely a frontend label.

func TestLoginWithMatchingExpectedRoleSucceeds(t *testing.T) {
	r := setupTestApp(t)
	username := "sectest_role_match_" + t.Name()
	_, password := createTestAdmin(t, username, models.AdminRoleSuperAdmin)

	w := doJSON(r, "POST", "/admin/login", "", map[string]interface{}{
		"username": username, "password": password, "expectedRole": models.AdminRoleSuperAdmin,
	})
	if w.Code != http.StatusOK {
		t.Fatalf("login with a correctly matching expectedRole must succeed, got %d: %s", w.Code, w.Body.String())
	}
}

func TestLoginWithWrongExpectedRoleRejected(t *testing.T) {
	r := setupTestApp(t)
	username := "sectest_role_mismatch_" + t.Name()
	_, password := createTestAdmin(t, username, "COLLECTION_OFFICER")

	w := doJSON(r, "POST", "/admin/login", "", map[string]interface{}{
		"username": username, "password": password, "expectedRole": "COMPLIANCE_OFFICER",
	})
	if w.Code != http.StatusForbidden {
		t.Fatalf("a COLLECTION_OFFICER account logging in as 'COMPLIANCE_OFFICER' must be rejected, got %d: %s", w.Code, w.Body.String())
	}
}

func TestLoginWithoutExpectedRoleStillWorks(t *testing.T) {
	r := setupTestApp(t)
	username := "sectest_role_omitted_" + t.Name()
	_, password := createTestAdmin(t, username, models.AdminRoleSuperAdmin)

	w := doJSON(r, "POST", "/admin/login", "", map[string]interface{}{
		"username": username, "password": password,
	})
	if w.Code != http.StatusOK {
		t.Fatalf("omitting expectedRole entirely must behave exactly as before, got %d: %s", w.Code, w.Body.String())
	}
}
