package kyc_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/google/uuid"

	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/models"
)

// GET /admin/kyc/queue enrichment: adds a real customerName (from
// CustomerProfile) and loanCategories (from real LoanApplication rows) to
// each row, so an admin UI isn't stuck showing a bare customer UUID.

func TestKYCQueueIncludesRealCustomerNameAndLoanCategories(t *testing.T) {
	r := setupKYCTestApp(t)
	adminUsername := "sectest_kycqueue_" + t.Name()
	adminPassword := createKYCTestAdmin(t, adminUsername, models.AdminRoleSuperAdmin)
	adminToken := kycLoginAsAdmin(t, r, adminUsername, adminPassword)

	mobile := "9" + uuidLike()
	customer := createKYCTestCustomer(t, mobile)

	profile := models.CustomerProfile{UserID: customer.ID, FirstName: "Ravi", LastName: "Kumar"}
	if err := database.DB.Create(&profile).Error; err != nil {
		t.Fatalf("create profile: %v", err)
	}
	t.Cleanup(func() { database.DB.Unscoped().Delete(&profile) })

	loan := models.LoanApplication{CustomerID: customer.ID, Category: "Personal Loan", AmountRequested: 50000, Status: models.LoanStatusSubmitted, ApplicantName: "Ravi Kumar"}
	if err := database.DB.Create(&loan).Error; err != nil {
		t.Fatalf("create loan: %v", err)
	}
	t.Cleanup(func() { database.DB.Unscoped().Delete(&loan) })

	kycRecord := models.CustomerKYC{CustomerID: customer.ID, Status: models.KYCStatusVerificationPending}
	if err := database.DB.Create(&kycRecord).Error; err != nil {
		t.Fatalf("create kyc record: %v", err)
	}
	t.Cleanup(func() { database.DB.Unscoped().Delete(&kycRecord) })

	w := kycDoJSON(r, "GET", "/admin/kyc/queue?status="+models.KYCStatusVerificationPending, adminToken, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("queue fetch failed: %d %s", w.Code, w.Body.String())
	}

	var rows []map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &rows); err != nil {
		t.Fatalf("decode: %v", err)
	}

	var found map[string]interface{}
	for _, row := range rows {
		if row["customerId"] == customer.ID.String() {
			found = row
			break
		}
	}
	if found == nil {
		t.Fatalf("expected to find our test customer's row in the queue, got %d rows", len(rows))
	}
	if found["customerName"] != "Ravi Kumar" {
		t.Fatalf("expected real customerName 'Ravi Kumar', got %v", found["customerName"])
	}
	cats, ok := found["loanCategories"].([]interface{})
	if !ok || len(cats) != 1 || cats[0] != "Personal Loan" {
		t.Fatalf("expected loanCategories=['Personal Loan'], got %v", found["loanCategories"])
	}
}

func TestKYCQueueOmitsNameAndCategoriesWhenNoRealDataExists(t *testing.T) {
	r := setupKYCTestApp(t)
	adminUsername := "sectest_kycqueue_nodata_" + t.Name()
	adminPassword := createKYCTestAdmin(t, adminUsername, models.AdminRoleSuperAdmin)
	adminToken := kycLoginAsAdmin(t, r, adminUsername, adminPassword)

	mobile := "9" + uuidLike()
	customer := createKYCTestCustomer(t, mobile)
	// No CustomerProfile row, no LoanApplication row — customer has only
	// just started KYC, nothing else exists yet.

	kycRecord := models.CustomerKYC{CustomerID: customer.ID, Status: models.KYCStatusRequiresReview}
	if err := database.DB.Create(&kycRecord).Error; err != nil {
		t.Fatalf("create kyc record: %v", err)
	}
	t.Cleanup(func() { database.DB.Unscoped().Delete(&kycRecord) })

	w := kycDoJSON(r, "GET", "/admin/kyc/queue?status="+models.KYCStatusRequiresReview, adminToken, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("queue fetch failed: %d %s", w.Code, w.Body.String())
	}

	var rows []map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &rows); err != nil {
		t.Fatalf("decode: %v", err)
	}

	var found map[string]interface{}
	for _, row := range rows {
		if row["customerId"] == customer.ID.String() {
			found = row
			break
		}
	}
	if found == nil {
		t.Fatalf("expected to find our test customer's row in the queue, got %d rows", len(rows))
	}
	// Must never fabricate a name or category when no real row backs it —
	// the field should simply be absent (omitempty), not a placeholder.
	if _, present := found["customerName"]; present {
		t.Fatalf("customerName must be omitted (not fabricated) when no CustomerProfile exists, got %v", found["customerName"])
	}
	if _, present := found["loanCategories"]; present {
		t.Fatalf("loanCategories must be omitted (not fabricated) when no LoanApplication exists, got %v", found["loanCategories"])
	}
}

// uuidLike returns a short digit-ish string suitable for building a fake
// mobile number, mirroring the pattern used elsewhere in this test file
// (uuid.New().String()[:9]).
func uuidLike() string {
	return uuid.New().String()[:9]
}
