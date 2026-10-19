package kyc_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/joho/godotenv"

	"github.com/santosh0921/nbfc-backend/internal/admin"
	"github.com/santosh0921/nbfc-backend/internal/auth"
	"github.com/santosh0921/nbfc-backend/internal/bureau"
	"github.com/santosh0921/nbfc-backend/internal/config"
	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/kyc"
	"github.com/santosh0921/nbfc-backend/internal/models"
	"github.com/santosh0921/nbfc-backend/internal/screening"
)

// Regression for the bug where admin approval never actually verified a
// case: Recompute previously required every provider's own IsVerified flag
// (internal/models.CustomerPAN/CustomerAadhaar/CustomerSelfie) to be true
// before VERIFIED was reachable — but Aadhaar and Selfie have no automated
// provider in this deployment, so those flags can never become true on
// their own, making admin approval a silent no-op. Admin approval is now
// authoritative once all three documents are actually on file.

func TestAdminApprovalVerifiesCaseWithAllDocumentsSubmitted(t *testing.T) {
	setupKYCDB(t)

	customer := models.User{Mobile: "9" + uuid.New().String()[:9]}
	if err := database.DB.Create(&customer).Error; err != nil {
		t.Fatalf("create customer: %v", err)
	}
	t.Cleanup(func() { database.DB.Unscoped().Delete(&customer) })

	pan := models.CustomerPAN{UserID: customer.ID, PANLast4: "1234"}
	if err := database.DB.Create(&pan).Error; err != nil {
		t.Fatalf("create pan: %v", err)
	}
	t.Cleanup(func() { database.DB.Unscoped().Delete(&pan) })

	aadhaar := models.CustomerAadhaar{UserID: customer.ID, Last4Digits: "5678"}
	if err := database.DB.Create(&aadhaar).Error; err != nil {
		t.Fatalf("create aadhaar: %v", err)
	}
	t.Cleanup(func() { database.DB.Unscoped().Delete(&aadhaar) })

	selfie := models.CustomerSelfie{UserID: customer.ID}
	if err := database.DB.Create(&selfie).Error; err != nil {
		t.Fatalf("create selfie: %v", err)
	}
	t.Cleanup(func() { database.DB.Unscoped().Delete(&selfie) })

	// Sanity check: none of PAN/Aadhaar/Selfie are provider-verified — the
	// exact real-world state (no automated Aadhaar/Selfie provider
	// configured), which previously made VERIFIED unreachable.
	if pan.IsVerified || aadhaar.IsVerified || selfie.IsVerified {
		t.Fatal("test fixture assumption violated: none of these should be provider-verified")
	}

	// First Recompute (as would happen on document submission) — should
	// land on VERIFICATION_PENDING, awaiting admin review.
	record, err := kyc.Recompute(customer.ID, "customer", customer.Mobile)
	if err != nil {
		t.Fatalf("recompute: %v", err)
	}
	t.Cleanup(func() { database.DB.Unscoped().Delete(record) })
	if record.Status != models.KYCStatusVerificationPending {
		t.Fatalf("expected VERIFICATION_PENDING before admin review, got %s", record.Status)
	}

	// Admin approves via the real handler path (CustomerKYCReview row +
	// Recompute), exactly as AdminApproveHandler does.
	review := models.CustomerKYCReview{UserID: customer.ID, Status: "APPROVED", ReviewedBy: "sectest_admin"}
	if err := database.DB.Create(&review).Error; err != nil {
		t.Fatalf("create review: %v", err)
	}
	t.Cleanup(func() { database.DB.Unscoped().Delete(&review) })

	record, err = kyc.Recompute(customer.ID, "admin", "sectest_admin")
	if err != nil {
		t.Fatalf("recompute after approval: %v", err)
	}
	if record.Status != models.KYCStatusVerified {
		t.Fatalf("admin approval with all documents submitted must reach VERIFIED even though no automated provider ever set IsVerified, got %s", record.Status)
	}
	if record.VerifiedBy != "sectest_admin" {
		t.Fatalf("expected VerifiedBy to record the approving admin, got %q", record.VerifiedBy)
	}
}

// TestAdminApprovalTriggersBureauPull covers the "pass to credit bureau"
// behavior: previously the ONLY trigger for a bureau pull was applying
// for a loan (internal/loans.getOrCreateCibilScore) — a customer's KYC
// being approved had no effect on the credit bureau at all. Recompute now
// fires a bureau pull (in a goroutine — see triggerBureauCheckAfterApproval)
// the moment a case first reaches VERIFIED, so the Credit Bureau portal
// shows a real BureauRequest for this customer without waiting for a loan
// application.
func TestAdminApprovalTriggersBureauPull(t *testing.T) {
	setupKYCDB(t)

	customer := models.User{Mobile: "9" + uuid.New().String()[:9]}
	if err := database.DB.Create(&customer).Error; err != nil {
		t.Fatalf("create customer: %v", err)
	}
	t.Cleanup(func() { database.DB.Unscoped().Delete(&customer) })
	t.Cleanup(func() {
		database.DB.Unscoped().Where("customer_id = ?", customer.ID).Delete(&models.BureauConsent{})
		database.DB.Unscoped().Where("customer_id = ?", customer.ID).Delete(&models.BureauRequest{})
	})

	for _, doc := range []interface{}{
		&models.CustomerPAN{UserID: customer.ID, PANLast4: "1234"},
		&models.CustomerAadhaar{UserID: customer.ID, Last4Digits: "5678"},
		&models.CustomerSelfie{UserID: customer.ID},
	} {
		if err := database.DB.Create(doc).Error; err != nil {
			t.Fatalf("create document: %v", err)
		}
	}
	t.Cleanup(func() {
		database.DB.Unscoped().Where("user_id = ?", customer.ID).Delete(&models.CustomerPAN{})
		database.DB.Unscoped().Where("user_id = ?", customer.ID).Delete(&models.CustomerAadhaar{})
		database.DB.Unscoped().Where("user_id = ?", customer.ID).Delete(&models.CustomerSelfie{})
	})

	review := models.CustomerKYCReview{UserID: customer.ID, Status: "APPROVED", ReviewedBy: "sectest_admin"}
	if err := database.DB.Create(&review).Error; err != nil {
		t.Fatalf("create review: %v", err)
	}
	t.Cleanup(func() { database.DB.Unscoped().Delete(&review) })

	record, err := kyc.Recompute(customer.ID, "admin", "sectest_admin")
	if err != nil {
		t.Fatalf("recompute: %v", err)
	}
	t.Cleanup(func() { database.DB.Unscoped().Delete(record) })
	if record.Status != models.KYCStatusVerified {
		t.Fatalf("expected VERIFIED, got %s", record.Status)
	}

	// The pull runs in a background goroutine, deliberately not awaited by
	// Recompute (an admin's "Approve" click must not block on a bureau
	// call) — poll briefly instead of sleeping a fixed, flaky amount.
	deadline := time.Now().Add(3 * time.Second)
	var found models.BureauRequest
	for time.Now().Before(deadline) {
		if err := database.DB.Where("customer_id = ?", customer.ID).First(&found).Error; err == nil {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if found.ID == uuid.Nil {
		t.Fatal("expected a BureauRequest to be created automatically once KYC reached VERIFIED, found none within 3s")
	}
	if found.Status != models.BureauRequestStatusCompleted || found.Score == 0 {
		t.Fatalf("expected a completed sandbox pull with a nonzero score, got status=%s score=%d", found.Status, found.Score)
	}

	// Recomputing again (e.g. a later unrelated event re-triggers
	// Recompute while already VERIFIED) must NOT fire a second pull —
	// justVerified only fires on the actual pending->verified transition.
	if _, err := kyc.Recompute(customer.ID, "admin", "sectest_admin"); err != nil {
		t.Fatalf("second recompute: %v", err)
	}
	time.Sleep(200 * time.Millisecond) // give a wrongly-fired goroutine a moment to land, if one did
	var count int64
	database.DB.Model(&models.BureauRequest{}).Where("customer_id = ?", customer.ID).Count(&count)
	if count != 1 {
		t.Fatalf("expected exactly 1 BureauRequest after re-recomputing an already-VERIFIED case, got %d", count)
	}
}

func TestAdminApprovalDoesNotVerifyWithMissingDocuments(t *testing.T) {
	setupKYCDB(t)

	customer := models.User{Mobile: "9" + uuid.New().String()[:9]}
	if err := database.DB.Create(&customer).Error; err != nil {
		t.Fatalf("create customer: %v", err)
	}
	t.Cleanup(func() { database.DB.Unscoped().Delete(&customer) })

	// Only PAN submitted — Aadhaar and Selfie missing entirely.
	pan := models.CustomerPAN{UserID: customer.ID, PANLast4: "1234"}
	if err := database.DB.Create(&pan).Error; err != nil {
		t.Fatalf("create pan: %v", err)
	}
	t.Cleanup(func() { database.DB.Unscoped().Delete(&pan) })

	review := models.CustomerKYCReview{UserID: customer.ID, Status: "APPROVED", ReviewedBy: "sectest_admin"}
	if err := database.DB.Create(&review).Error; err != nil {
		t.Fatalf("create review: %v", err)
	}
	t.Cleanup(func() { database.DB.Unscoped().Delete(&review) })

	record, err := kyc.Recompute(customer.ID, "admin", "sectest_admin")
	if err != nil {
		t.Fatalf("recompute: %v", err)
	}
	t.Cleanup(func() { database.DB.Unscoped().Delete(record) })

	if record.Status == models.KYCStatusVerified {
		t.Fatal("an approval must not verify a case that's missing required documents, even if somehow marked APPROVED")
	}
}

// setupKYCDB mirrors setupKYCTestApp's full init sequence exactly (not
// just database.Connect) — a bug this file used to have: connecting only
// the DB here, when this file's tests happen to run first (alphabetically
// before kyc_security_test.go), left database.DB non-nil while skipping
// bureau.Initialize()/screening.Initialize()/admin.Migrate(), which made
// setupKYCTestApp's own `if database.DB == nil` guard in other test files
// short-circuit and silently skip ITS init — breaking bureau/screening
// tests elsewhere in this package with no connection to this file's actual
// changes. Every test-local "init if nil" helper in this package must do
// the same full sequence, or none of them can be first safely.
func setupKYCDB(t *testing.T) {
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
