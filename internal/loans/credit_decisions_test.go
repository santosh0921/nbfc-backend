package loans_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/loans"
	"github.com/santosh0921/nbfc-backend/internal/middleware"
	"github.com/santosh0921/nbfc-backend/internal/models"
	"github.com/santosh0921/nbfc-backend/internal/rbac"
)

// GET /admin/credit-decisions — previously no route existed for this real,
// already-populated table (internal/loans/eligibility.go writes rows on
// every eligibility check and sanction decision); the credit portal's
// "Credit Decision" screen had nothing to call. Confirms the new endpoint
// returns real rows enriched with a real customer name, filterable by
// customerId/loanId.

func TestAdminListCreditDecisionsReturnsRealRowsWithNames(t *testing.T) {
	r := setupDisbursementTestApp(t)
	r.GET("/admin/credit-decisions", middleware.AdminAuthMiddleware(), middleware.RequirePermission(rbac.LoanView), loans.AdminListCreditDecisionsHandler)
	_, pw := createDisbTestAdmin(t, "sectest_creditdec_"+t.Name(), models.AdminRoleSuperAdmin)
	token := disbLoginAs(t, r, "sectest_creditdec_"+t.Name(), pw)

	customer := models.User{Mobile: "9" + uuid.New().String()[:9]}
	if err := database.DB.Create(&customer).Error; err != nil {
		t.Fatalf("create customer: %v", err)
	}
	t.Cleanup(func() { database.DB.Unscoped().Delete(&customer) })

	profile := models.CustomerProfile{UserID: customer.ID, FirstName: "Neha", LastName: "Verma"}
	if err := database.DB.Create(&profile).Error; err != nil {
		t.Fatalf("create profile: %v", err)
	}
	t.Cleanup(func() { database.DB.Unscoped().Delete(&profile) })

	decision := models.CreditDecision{
		CustomerID: customer.ID, PolicyVersion: "v1-test", InputsJSON: "{}",
		Decision: "ELIGIBLE", ReviewerOrSystem: "system",
	}
	if err := database.DB.Create(&decision).Error; err != nil {
		t.Fatalf("create decision: %v", err)
	}
	t.Cleanup(func() { database.DB.Unscoped().Delete(&decision) })

	w := disbDoJSON(r, "GET", "/admin/credit-decisions?customerId="+customer.ID.String(), token, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	body := w.Body.String()
	for _, want := range []string{customer.ID.String(), "Neha Verma", "ELIGIBLE"} {
		if !strings.Contains(body, want) {
			t.Fatalf("expected response to contain %q, got: %s", want, body)
		}
	}
}
