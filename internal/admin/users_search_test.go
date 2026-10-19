package admin_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/santosh0921/nbfc-backend/internal/admin"
	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/middleware"
	"github.com/santosh0921/nbfc-backend/internal/models"
	"github.com/santosh0921/nbfc-backend/internal/rbac"
)

// The KYC portal's "Customer Search" page only ever accepted an exact
// customer UUID — typing a phone number (what a user actually has in
// hand) failed with "Invalid customer id", and GET /admin/users itself
// had no search/filter capability at all to fall back on, only plain
// pagination. These tests cover the new ?search= support added to
// ListUsersHandler to fix that.

func setupUsersSearchApp(t *testing.T) *gin.Engine {
	t.Helper()
	// Reuses setupTestApp's full init sequence (godotenv, JWT secrets,
	// database.Connect, admin.Migrate) rather than re-implementing it —
	// this package has already been bitten once by a test file whose own
	// "if database.DB == nil" guard was weaker than the others and broke
	// whichever test happened to run first (see internal/kyc's
	// approval_verifies_test.go fix earlier this session for the same
	// bug class). Calling the existing, known-correct helper sidesteps
	// that entirely.
	setupTestApp(t)
	r := gin.New()
	adminGroup := r.Group("/admin")
	adminGroup.Use(middleware.AdminAuthMiddleware())
	adminGroup.GET("/users", middleware.RequirePermission(rbac.CustomerView), admin.ListUsersHandler)
	return r
}

func createTestCustomer(t *testing.T, mobile, firstName, lastName string) uuid.UUID {
	t.Helper()
	u := models.User{Mobile: mobile, IsMobileVerified: true, IsActive: true}
	if err := database.DB.Create(&u).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	t.Cleanup(func() { database.DB.Unscoped().Delete(&u) })

	if firstName != "" || lastName != "" {
		p := models.CustomerProfile{UserID: u.ID, FirstName: firstName, LastName: lastName}
		if err := database.DB.Create(&p).Error; err != nil {
			t.Fatalf("create profile: %v", err)
		}
		t.Cleanup(func() { database.DB.Unscoped().Delete(&p) })
	}
	return u.ID
}

func TestListUsersSearchByMobileFindsCustomerWithNoProfile(t *testing.T) {
	r := setupUsersSearchApp(t)
	_, pw := createTestAdmin(t, "sectest_search_mobile_"+t.Name(), models.AdminRoleSuperAdmin)
	token, _ := loginAs(t, r2WithLogin(t), "sectest_search_mobile_"+t.Name(), pw)
	_ = token

	// A customer who applied for a loan without ever completing the
	// profile step of KYC has no CustomerProfile row at all — mobile must
	// still find them.
	mobile := "9" + uuid.New().String()[:9]
	wantID := createTestCustomer(t, mobile, "", "")

	w := doJSON(r, "GET", "/admin/users?search="+mobile, token, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp struct {
		Users []struct {
			ID     string `json:"id"`
			Mobile string `json:"mobile"`
		} `json:"users"`
		Total int `json:"total"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Total != 1 || len(resp.Users) != 1 {
		t.Fatalf("expected exactly 1 match for an exact mobile search, got total=%d rows=%d", resp.Total, len(resp.Users))
	}
	if resp.Users[0].ID != wantID.String() {
		t.Fatalf("expected the matched customer's id %s, got %s", wantID, resp.Users[0].ID)
	}
}

func TestListUsersSearchByNameFindsCustomer(t *testing.T) {
	r := setupUsersSearchApp(t)
	_, pw := createTestAdmin(t, "sectest_search_name_"+t.Name(), models.AdminRoleSuperAdmin)
	token, _ := loginAs(t, r2WithLogin(t), "sectest_search_name_"+t.Name(), pw)

	uniqueLast := "Zzyxtest" + uuid.New().String()[:8]
	mobile := "9" + uuid.New().String()[:9]
	wantID := createTestCustomer(t, mobile, "Santosh", uniqueLast)

	w := doJSON(r, "GET", "/admin/users?search="+uniqueLast, token, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp struct {
		Users []struct{ ID string `json:"id"` } `json:"users"`
		Total int `json:"total"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Total != 1 || len(resp.Users) != 1 || resp.Users[0].ID != wantID.String() {
		t.Fatalf("expected exactly 1 match on last name, got total=%d rows=%v", resp.Total, resp.Users)
	}
}

// TestListUsersSearchByLoanApplicantPhoneFindsCustomer covers a real
// gap: LoanApplication.ApplicantName/ApplicantPhone are free-text fields
// typed on the loan form, entirely separate from the authenticated
// User.Mobile and CustomerProfile — a customer can (and in a real
// observed case, did) enter a different contact number on their loan
// application than the mobile they actually log in with. Searching by
// the exact number shown on their own loan detail page returned zero
// matches until this was added.
func TestListUsersSearchByLoanApplicantPhoneFindsCustomer(t *testing.T) {
	r := setupUsersSearchApp(t)
	_, pw := createTestAdmin(t, "sectest_search_loanphone_"+t.Name(), models.AdminRoleSuperAdmin)
	token, _ := loginAs(t, r2WithLogin(t), "sectest_search_loanphone_"+t.Name(), pw)

	// This customer's real login mobile is deliberately DIFFERENT from
	// the phone they typed on the loan form — that's the whole point.
	loginMobile := "9" + uuid.New().String()[:9]
	loanPhone := "7" + uuid.New().String()[:9]
	customerID := createTestCustomer(t, loginMobile, "", "")

	loan := models.LoanApplication{
		CustomerID: customerID, Category: "Personal Loan", AmountRequested: 10000,
		Status: "submitted", ApplicantName: "Loan Form Name " + uuid.New().String()[:6], ApplicantPhone: loanPhone,
	}
	if err := database.DB.Create(&loan).Error; err != nil {
		t.Fatalf("create test loan: %v", err)
	}
	t.Cleanup(func() { database.DB.Unscoped().Delete(&loan) })

	w := doJSON(r, "GET", "/admin/users?search="+loanPhone, token, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp struct {
		Users []struct {
			ID string `json:"id"`
		} `json:"users"`
		Total int `json:"total"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Total != 1 || len(resp.Users) != 1 || resp.Users[0].ID != customerID.String() {
		t.Fatalf("expected exactly 1 match via loan applicant phone, got total=%d rows=%v", resp.Total, resp.Users)
	}
}

func TestListUsersSearchNoMatchReturnsEmptyNotError(t *testing.T) {
	r := setupUsersSearchApp(t)
	_, pw := createTestAdmin(t, "sectest_search_empty_"+t.Name(), models.AdminRoleSuperAdmin)
	token, _ := loginAs(t, r2WithLogin(t), "sectest_search_empty_"+t.Name(), pw)

	w := doJSON(r, "GET", "/admin/users?search=definitely-nobody-has-this-xyz123", token, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 with an empty result, not an error, got %d: %s", w.Code, w.Body.String())
	}
	var resp struct {
		Users []json.RawMessage `json:"users"`
		Total int               `json:"total"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Total != 0 || len(resp.Users) != 0 {
		t.Fatalf("expected zero matches, got total=%d rows=%d", resp.Total, len(resp.Users))
	}
}

// r2WithLogin: loginAs needs a router with the real POST /admin/login
// route (setupUsersSearchApp deliberately only mounts /admin/users, the
// thing under test), so log in against the shared setupTestApp router
// from security_test.go instead — both share the same database.DB/admin
// JWT secret, so a token minted there is valid against setupUsersSearchApp
// too.
func r2WithLogin(t *testing.T) *gin.Engine {
	t.Helper()
	return setupTestApp(t) //nolint:unused // kept distinct from setupUsersSearchApp's router for clarity, not deduped
}
