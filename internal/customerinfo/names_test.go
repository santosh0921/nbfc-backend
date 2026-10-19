package customerinfo_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/joho/godotenv"

	"github.com/santosh0921/nbfc-backend/internal/config"
	"github.com/santosh0921/nbfc-backend/internal/customerinfo"
	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/models"
)

func setupDB(t *testing.T) {
	t.Helper()
	if database.DB != nil {
		return
	}
	_ = godotenv.Load("../../.env")
	cfg := config.Load()
	database.Connect(cfg)
}

func TestBatchNamesReturnsRealNamesOnly(t *testing.T) {
	setupDB(t)

	withProfile := models.User{Mobile: "9" + uuid.New().String()[:9]}
	database.DB.Create(&withProfile)
	t.Cleanup(func() { database.DB.Unscoped().Delete(&withProfile) })

	profile := models.CustomerProfile{UserID: withProfile.ID, FirstName: "Anita", LastName: "Sharma"}
	database.DB.Create(&profile)
	t.Cleanup(func() { database.DB.Unscoped().Delete(&profile) })

	withoutProfile := models.User{Mobile: "9" + uuid.New().String()[:9]}
	database.DB.Create(&withoutProfile)
	t.Cleanup(func() { database.DB.Unscoped().Delete(&withoutProfile) })

	names := customerinfo.BatchNames([]uuid.UUID{withProfile.ID, withoutProfile.ID})

	if names[withProfile.ID] != "Anita Sharma" {
		t.Fatalf("expected real name for customer with a profile, got %q", names[withProfile.ID])
	}
	if _, present := names[withoutProfile.ID]; present {
		t.Fatalf("must not fabricate a name for a customer with no profile row, got %q", names[withoutProfile.ID])
	}
}

func TestBatchNamesByStringIDSkipsUnparsableIDs(t *testing.T) {
	setupDB(t)

	user := models.User{Mobile: "9" + uuid.New().String()[:9]}
	database.DB.Create(&user)
	t.Cleanup(func() { database.DB.Unscoped().Delete(&user) })

	profile := models.CustomerProfile{UserID: user.ID, FirstName: "Deepak", LastName: "Rao"}
	database.DB.Create(&profile)
	t.Cleanup(func() { database.DB.Unscoped().Delete(&profile) })

	// Includes a garbage string ID that isn't a valid UUID at all — must
	// not error out or crash a list endpoint over one bad row.
	names := customerinfo.BatchNamesByStringID([]string{user.ID.String(), "not-a-uuid"})

	if names[user.ID.String()] != "Deepak Rao" {
		t.Fatalf("expected real name, got %q", names[user.ID.String()])
	}
	if _, present := names["not-a-uuid"]; present {
		t.Fatalf("an unparsable ID must never produce a fabricated entry")
	}
}

func TestLoanApplicantNamesReturnsRealNamesOnly(t *testing.T) {
	setupDB(t)

	loan := models.LoanApplication{
		CustomerID: uuid.New(), Category: "Personal Loan", AmountRequested: 10000,
		Status: models.LoanStatusSubmitted, ApplicantName: "Meera Iyer",
	}
	database.DB.Create(&loan)
	t.Cleanup(func() { database.DB.Unscoped().Delete(&loan) })

	names := customerinfo.LoanApplicantNames([]uint{loan.ID, 999999999})

	if names[loan.ID] != "Meera Iyer" {
		t.Fatalf("expected real applicant name, got %q", names[loan.ID])
	}
	if _, present := names[999999999]; present {
		t.Fatalf("a non-existent loan ID must never produce a fabricated entry")
	}
}
