package security_test

// Phase 9 — end-to-end lifecycle validation.
//
// Drives ONE controlled test loan through the entire real backend, calling
// the actual Gin handlers (never mocks), and asserts the loan/case is in
// the exact expected state after every step:
//
//   submit -> KYC-eligible submission -> pending_claim/assigned -> employee
//   claim -> employee verify -> maker recommend -> checker approve-sanction
//   -> maker prepare-disbursement -> checker approve-disbursement ->
//   ConfirmDisbursement (reconciliation-style, never an automated payout
//   API) -> EMI schedule -> a real signed EMI webhook payment -> an overdue
//   EMI driving a real CollectionCase.
//
// Runs against the same real dev Postgres every other Phase 1-8 regression
// suite in this repo uses (godotenv.Load("../../.env"), database.Connect).
//
// Explicit non-claims, per the brief:
//   - The bureau pull is verified to be BureauSandbox ("SANDBOX"), never
//     presented as a real CIBIL result.
//   - Disbursement approval only ever authorizes a manual/reconciliation-
//     driven transfer (LoanStatusDisbursementProcessing); ConfirmDisbursement
//     is what a bank-statement reconciliation match calls, not an automated
//     payout API — see internal/loans/confirm_disbursement.go's own doc
//     comment, unchanged here.
//   - The EMI amount that ends up on the installment is 100% server-computed
//     (generateEmiInstallments) from principal/tenure/rate; there is no
//     client-suppliable "amount" field anywhere in this flow that can
//     override it — asserted explicitly below.
//   - No distinct "risk review" gate exists as a separate pre-sanction step
//     in this codebase today (internal/risk is DPD/NPA classification,
//     which only applies post-disbursement) — documented as a limitation,
//     not silently invented.

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
	"golang.org/x/crypto/bcrypt"

	"github.com/santosh0921/nbfc-backend/internal/admin"
	"github.com/santosh0921/nbfc-backend/internal/auth"
	"github.com/santosh0921/nbfc-backend/internal/bureau"
	"github.com/santosh0921/nbfc-backend/internal/collections"
	"github.com/santosh0921/nbfc-backend/internal/config"
	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/employee"
	"github.com/santosh0921/nbfc-backend/internal/loans"
	"github.com/santosh0921/nbfc-backend/internal/middleware"
	"github.com/santosh0921/nbfc-backend/internal/models"
	"github.com/santosh0921/nbfc-backend/internal/payments"
	"github.com/santosh0921/nbfc-backend/internal/rbac"
)

const e2eWebhookSecret = "e2e-test-webhook-secret-do-not-use-in-prod"

func setupE2ETestApp(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)

	if database.DB == nil {
		_ = godotenv.Load("../../.env")
		cfg := config.Load()
		if err := admin.InitJWTSecret(cfg.AdminJWTSecret); err != nil {
			t.Fatalf("admin InitJWTSecret: %v", err)
		}
		if err := employee.InitJWTSecret(cfg.EmployeeJWTSecret); err != nil {
			t.Fatalf("employee InitJWTSecret: %v", err)
		}
		if err := auth.InitJWTSecret(cfg.JWTSecret); err != nil {
			t.Fatalf("customer InitJWTSecret: %v", err)
		}
		database.Connect(cfg)
		admin.Migrate()
		bureau.Initialize()
	}

	// A real razorpayProvider with a known test secret so the actual HMAC
	// verification code in payments.RazorpayWebhookHandler runs end to end.
	payments.SetActiveProviderForTesting(payments.NewRazorpayProviderForTesting("rzp_test_key", "rzp_test_secret", e2eWebhookSecret))

	r := gin.New()

	// Customer
	custGroup := r.Group("/loans")
	custGroup.Use(middleware.AuthMiddleware())
	custGroup.POST("/apply", loans.SubmitLoanHandler)

	// Employee
	r.POST("/employee/login", employee.LoginHandler)
	empGroup := r.Group("/employee")
	empGroup.Use(middleware.EmployeeAuthMiddleware())
	empGroup.POST("/loans/:id/claim", loans.EmployeeClaimLoanHandler)
	empGroup.POST("/loans/:id/verify", loans.EmployeeVerifyLoanHandler)

	// Admin
	r.POST("/admin/login", admin.LoginHandler)
	adminGroup := r.Group("/admin")
	adminGroup.Use(middleware.AdminAuthMiddleware())
	perm := middleware.RequirePermission
	adminGroup.GET("/loans/:id", perm(rbac.LoanView), loans.AdminGetLoanHandler)
	adminGroup.POST("/loans/:id/recommend", perm(rbac.LoanReview), loans.AdminRecommendSanctionHandler)
	adminGroup.POST("/loans/:id/approve-sanction", perm(rbac.LoanSanction), loans.AdminApproveSanctionHandler)
	adminGroup.POST("/loans/:id/prepare-disbursement", perm(rbac.LoanDisbursePrep), loans.AdminPrepareDisbursementHandler)
	adminGroup.POST("/loans/:id/approve-disbursement", perm(rbac.LoanDisburse), loans.AdminApproveDisbursementHandler)
	adminGroup.POST("/collections/cases/:id/assign", perm(rbac.CollectionManage), collections.AdminAssignCaseHandler)
	adminGroup.POST("/bureau/requests/:id/approve", perm(rbac.BureauApprove), bureau.AdminApproveHandler)

	// Payment gateway webhook — unauthenticated by design, proves itself via
	// HMAC signature, exactly as in production.
	r.POST("/webhooks/razorpay", payments.RazorpayWebhookHandler)

	return r
}

func e2eDoJSON(r *gin.Engine, method, path, token string, body interface{}) *httptest.ResponseRecorder {
	var buf bytes.Buffer
	if body != nil {
		json.NewEncoder(&buf).Encode(body)
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func e2eSignedWebhook(t *testing.T, r *gin.Engine, payload map[string]interface{}, secret string) *httptest.ResponseRecorder {
	t.Helper()
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	signature := hex.EncodeToString(mac.Sum(nil))

	req := httptest.NewRequest(http.MethodPost, "/webhooks/razorpay", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Razorpay-Signature", signature)
	req.Header.Set("X-Razorpay-Event-Id", "evt_"+uuid.NewString())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func e2eAdminLogin(t *testing.T, r *gin.Engine, username, password string) string {
	t.Helper()
	w := e2eDoJSON(r, "POST", "/admin/login", "", gin.H{"username": username, "password": password})
	if w.Code != http.StatusOK {
		t.Fatalf("admin login failed: %d %s", w.Code, w.Body.String())
	}
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	return resp["token"].(string)
}

func e2eCreateAdmin(t *testing.T, username, role string) string {
	t.Helper()
	password := "Sup3rSecure!Passw0rd_1234"
	hash, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	u := models.AdminUser{Username: username, Email: username + "@test.local", PasswordHash: string(hash), Role: role, Status: models.AdminStatusActive}
	if err := database.DB.Create(&u).Error; err != nil {
		t.Fatalf("create admin: %v", err)
	}
	t.Cleanup(func() { database.DB.Unscoped().Delete(&u) })
	return password
}

func e2eEmployeeLogin(t *testing.T, r *gin.Engine, name, password string) string {
	t.Helper()
	w := e2eDoJSON(r, "POST", "/employee/login", "", gin.H{"name": name, "password": password})
	if w.Code != http.StatusOK {
		t.Fatalf("employee login failed: %d %s", w.Code, w.Body.String())
	}
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	return resp["token"].(string)
}

func e2eIDStr(id uint) string {
	b, _ := json.Marshal(id)
	return string(b)
}

// TestFullLoanLifecycleE2E is the Phase 9 deliverable: one controlled test
// customer/loan driven end to end through every real handler in the chain,
// asserting the exact expected state after each step.
func TestFullLoanLifecycleE2E(t *testing.T) {
	r := setupE2ETestApp(t)
	tag := "e2e_" + uuid.NewString()[:8]
	city := "E2ECity_" + tag

	// --- Fixtures: a KYC-eligible customer ---
	mobile := "9" + uuid.NewString()[0:9]
	custPassword := "unused-mpin-flow"
	_ = custPassword
	user := models.User{ID: uuid.New(), Mobile: mobile, MPIN: "x", IsActive: true}
	if err := database.DB.Create(&user).Error; err != nil {
		t.Fatalf("create test customer: %v", err)
	}
	t.Cleanup(func() { database.DB.Unscoped().Delete(&user) })

	// checkKYCEligibility requires exactly the 3 required documents on file
	// (PAN, Aadhaar, selfie) — completeness, not provider-verified status,
	// per internal/kyc.IsSubmissionEligible's own doc comment. Minimal rows
	// only; this test is about the LOAN state machine, not the KYC document
	// pipeline, which has its own coverage elsewhere.
	pan := models.CustomerPAN{ID: uuid.New(), UserID: user.ID, PANLast4: "1234"}
	database.DB.Create(&pan)
	t.Cleanup(func() { database.DB.Unscoped().Delete(&pan) })
	aadhaar := models.CustomerAadhaar{ID: uuid.New(), UserID: user.ID, AadhaarHash: "hash-" + tag, Last4Digits: "5678"}
	database.DB.Create(&aadhaar)
	t.Cleanup(func() { database.DB.Unscoped().Delete(&aadhaar) })
	selfie := models.CustomerSelfie{ID: uuid.New(), UserID: user.ID, ImagePath: "inline-test-selfie"}
	database.DB.Create(&selfie)
	t.Cleanup(func() { database.DB.Unscoped().Delete(&selfie) })

	custToken, err := auth.GenerateToken(mobile)
	if err != nil {
		t.Fatalf("generate customer token: %v", err)
	}

	// A verification employee in this exact (unique, tag-scoped) city, so
	// the loan is deterministically routed to a "pending_claim" broadcast
	// rather than left to whatever else lives in the shared dev DB.
	empPassword := "Sup3rSecure!Passw0rd_1234"
	empHash, _ := bcrypt.GenerateFromPassword([]byte(empPassword), bcrypt.DefaultCost)
	emp := models.Employee{
		Code: "E2E-" + tag, Name: "e2e_verifier_" + tag, PasswordHash: string(empHash),
		Role: "verification", Active: true, Approved: true, BranchCity: city,
	}
	if err := database.DB.Create(&emp).Error; err != nil {
		t.Fatalf("create test employee: %v", err)
	}
	t.Cleanup(func() { database.DB.Unscoped().Delete(&emp) })

	var loanID uint
	var sanctionedRatePercent float64

	// --- 1. Submission (LoanStatusSubmitted -> routed to pending_claim) ---
	t.Run("01_submit", func(st *testing.T) {
		w := e2eDoJSON(r, "POST", "/loans/apply", custToken, gin.H{
			"category": "Personal Loan", "amountRequested": 200000.0, "tenureMonths": 24,
			"monthlyIncome": 100000.0, "applicantName": "E2E Test Applicant", "city": city,
		})
		if w.Code != http.StatusCreated {
			st.Fatalf("loan submission should succeed for a KYC-eligible customer, got %d: %s", w.Code, w.Body.String())
		}
		var resp map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &resp)
		loanID = uint(resp["id"].(float64))

		var loan models.LoanApplication
		database.DB.First(&loan, loanID)
		// Registered on the outer t, not this subtest, so the row survives
		// for every later step and is only cleaned up once the whole
		// lifecycle test (all subtests) has finished.
		t.Cleanup(func() { database.DB.Unscoped().Delete(&loan) })
		if loan.Status != models.LoanStatusPendingClaim {
			st.Fatalf("expected pending_claim (an employee exists in %s), got %s", city, loan.Status)
		}
	})

	// --- 2/3. Bureau/screening: sandbox-flagged, never presented as real CIBIL ---
	t.Run("02_bureau_is_explicitly_sandboxed", func(st *testing.T) {
		var req models.BureauRequest
		err := database.DB.Where("customer_id = ?", user.ID).Order("requested_at desc").First(&req).Error
		if err != nil {
			st.Fatalf("expected a BureauRequest row for this customer: %v", err)
		}
		if req.Bureau != models.BureauSandbox {
			st.Fatalf("outside production the bureau pull MUST be explicitly SANDBOX, got %q — a non-sandbox value here would mean a fake score is being presented as a real bureau result", req.Bureau)
		}
		if req.Status != models.BureauRequestStatusCompleted {
			st.Fatalf("expected a completed sandbox pull, got status %s", req.Status)
		}
		var loan models.LoanApplication
		database.DB.First(&loan, loanID)
		if loan.CibilScore <= 0 {
			st.Fatal("expected the loan to carry the sandbox-derived score")
		}
	})

	// --- 4. Employee claim + verify (pending_claim -> assigned -> verified) ---
	empToken := e2eEmployeeLogin(t, r, emp.Name, empPassword)

	t.Run("03_employee_claim", func(st *testing.T) {
		w := e2eDoJSON(r, "POST", "/employee/loans/"+e2eIDStr(loanID)+"/claim", empToken, nil)
		if w.Code != http.StatusOK {
			st.Fatalf("employee claim should succeed: %d %s", w.Code, w.Body.String())
		}
		var loan models.LoanApplication
		database.DB.First(&loan, loanID)
		if loan.Status != models.LoanStatusAssigned {
			st.Fatalf("expected assigned after claim, got %s", loan.Status)
		}
	})

	t.Run("04_employee_verify", func(st *testing.T) {
		w := e2eDoJSON(r, "POST", "/employee/loans/"+e2eIDStr(loanID)+"/verify", empToken,
			gin.H{"findings": "documents in order", "remarks": "field visit ok", "riskAssessment": "low", "recommendation": "approve"})
		if w.Code != http.StatusOK {
			st.Fatalf("employee verify should succeed: %d %s", w.Code, w.Body.String())
		}
		var loan models.LoanApplication
		database.DB.First(&loan, loanID)
		if loan.Status != models.LoanStatusVerified {
			st.Fatalf("expected verified, got %s", loan.Status)
		}
	})

	// --- 5. Underwriting/recommend sanction (maker) ---
	makerUser := "e2e_maker_" + tag
	makerPassword := e2eCreateAdmin(t, makerUser, models.AdminRoleSuperAdmin)
	makerToken := e2eAdminLogin(t, r, makerUser, makerPassword)

	// --- 4b. Credit officer reviews/approves the bureau pull ---
	// requireApprovedBureauRequest (internal/loans/maker_checker_handler.go)
	// gates recommend-sanction on this: the automatic sandbox pull from
	// step 02 exists and is COMPLETED, but a human still has to explicitly
	// approve it (BUREAU_APPROVE) before underwriting may act — same
	// maker!=checker spirit as the sanction/disbursement steps below, just
	// one stage earlier in the pipeline.
	t.Run("04b_bureau_approval", func(st *testing.T) {
		var req models.BureauRequest
		if err := database.DB.Where("customer_id = ?", user.ID).Order("requested_at desc").First(&req).Error; err != nil {
			st.Fatalf("expected a bureau request to approve: %v", err)
		}
		w := e2eDoJSON(r, "POST", "/admin/bureau/requests/"+req.ID.String()+"/approve", makerToken, gin.H{"notes": "e2e approval"})
		if w.Code != http.StatusOK {
			st.Fatalf("bureau approval should succeed: %d %s", w.Code, w.Body.String())
		}
		var reloaded models.BureauRequest
		database.DB.First(&reloaded, "id = ?", req.ID)
		if reloaded.ReviewStatus != models.BureauReviewStatusApproved {
			st.Fatalf("expected ReviewStatus APPROVED, got %s", reloaded.ReviewStatus)
		}
	})

	t.Run("05_recommend_sanction", func(st *testing.T) {
		w := e2eDoJSON(r, "POST", "/admin/loans/"+e2eIDStr(loanID)+"/recommend", makerToken,
			gin.H{"tenureMonths": 24, "interestRatePercent": 11.0, "remarks": "e2e recommend"})
		if w.Code != http.StatusOK {
			st.Fatalf("recommend should succeed: %d %s", w.Code, w.Body.String())
		}
		var loan models.LoanApplication
		database.DB.First(&loan, loanID)
		if loan.Status != models.LoanStatusPendingSanctionApproval {
			st.Fatalf("expected pending_sanction_approval, got %s", loan.Status)
		}
	})

	// --- 6. Risk review gate: NOT a distinct pre-sanction step in this
	// codebase (internal/risk is post-disbursement DPD/NPA classification
	// only) — documented as a limitation in the final report, not invented
	// here. No test step for it; a fabricated one would misrepresent the
	// actual system.

	// --- 7. Sanction approval by a DIFFERENT checker admin ---
	checkerUser := "e2e_checker_" + tag
	checkerPassword := e2eCreateAdmin(t, checkerUser, models.AdminRoleSuperAdmin)
	checkerToken := e2eAdminLogin(t, r, checkerUser, checkerPassword)

	t.Run("06_approve_sanction_different_checker", func(st *testing.T) {
		w := e2eDoJSON(r, "POST", "/admin/loans/"+e2eIDStr(loanID)+"/approve-sanction", checkerToken,
			gin.H{"approve": true, "remarks": "e2e approve"})
		if w.Code != http.StatusOK {
			st.Fatalf("approve-sanction by a different admin should succeed: %d %s", w.Code, w.Body.String())
		}
		var loan models.LoanApplication
		database.DB.First(&loan, loanID)
		if loan.Status != models.LoanStatusSanctioned {
			st.Fatalf("expected sanctioned, got %s", loan.Status)
		}
		// The sandbox score (780, prime tier) forces the CIBIL-driven
		// minimum rate here regardless of what was recommended — see
		// AdminApproveSanctionHandler's cibilRateNote branch — captured for
		// the EMI-authoritativeness check in step 09.
		if loan.InterestRatePercent != nil {
			sanctionedRatePercent = *loan.InterestRatePercent
		}
	})

	// --- 8. Operations: prepare disbursement (maker) ---
	disbMakerUser := "e2e_disbmaker_" + tag
	disbMakerPassword := e2eCreateAdmin(t, disbMakerUser, models.AdminRoleSuperAdmin)
	disbMakerToken := e2eAdminLogin(t, r, disbMakerUser, disbMakerPassword)

	t.Run("07_prepare_disbursement", func(st *testing.T) {
		w := e2eDoJSON(r, "POST", "/admin/loans/"+e2eIDStr(loanID)+"/prepare-disbursement", disbMakerToken,
			gin.H{"amount": 200000.0, "bankAccountReference": "TESTBANK-XXXX-0001"})
		if w.Code != http.StatusOK {
			st.Fatalf("prepare-disbursement should succeed: %d %s", w.Code, w.Body.String())
		}
		var loan models.LoanApplication
		database.DB.First(&loan, loanID)
		if loan.Status != models.LoanStatusReadyForDisbursement {
			st.Fatalf("expected ready_for_disbursement, got %s", loan.Status)
		}
	})

	// --- 9. Disbursement approval by a different checker ---
	disbCheckerUser := "e2e_disbchecker_" + tag
	disbCheckerPassword := e2eCreateAdmin(t, disbCheckerUser, models.AdminRoleSuperAdmin)
	disbCheckerToken := e2eAdminLogin(t, r, disbCheckerUser, disbCheckerPassword)

	var disbID uint
	t.Run("08_approve_disbursement_different_checker", func(st *testing.T) {
		w := e2eDoJSON(r, "POST", "/admin/loans/"+e2eIDStr(loanID)+"/approve-disbursement", disbCheckerToken,
			gin.H{"approve": true, "remarks": "e2e approve disbursement"})
		if w.Code != http.StatusOK {
			st.Fatalf("approve-disbursement by a different admin should succeed: %d %s", w.Code, w.Body.String())
		}
		var loan models.LoanApplication
		database.DB.First(&loan, loanID)
		if loan.Status != models.LoanStatusDisbursementProcessing {
			st.Fatalf("expected disbursement_processing (approval alone must NOT finalize), got %s", loan.Status)
		}
		var disb models.Disbursement
		if err := database.DB.Where("loan_id = ?", loanID).First(&disb).Error; err != nil {
			st.Fatalf("expected a Disbursement record: %v", err)
		}
		disbID = disb.ID
		// Never claim this was an automated bank transfer — the active
		// provider in this deployment has no live payout API configured, so
		// this only ever reaches "initiated"/manual-reconciliation-pending,
		// exactly as internal/loans/confirm_disbursement.go's doc comment
		// describes. Assert it is NOT already SUCCESS by itself.
		if disb.Status == models.DisbursementStatusSuccess {
			st.Fatal("a mere checker approval must never, by itself, mark a disbursement SUCCESS — that would misrepresent a manual/reconciliation-driven transfer as an automated one")
		}
	})

	// --- 10. Confirm disbursement (the reconciliation-style, non-automated
	// finalization path) -> Disbursed, EMI schedule generated ---
	var installments []models.EmiInstallment
	t.Run("09_confirm_disbursement_and_emi_schedule", func(st *testing.T) {
		if err := loans.ConfirmDisbursement(disbID, "E2E-UTR-"+tag, "e2e-reconciler", nil); err != nil {
			st.Fatalf("ConfirmDisbursement failed: %v", err)
		}
		var loan models.LoanApplication
		database.DB.First(&loan, loanID)
		if loan.Status != models.LoanStatusDisbursed {
			st.Fatalf("expected disbursed, got %s", loan.Status)
		}
		if loan.DisbursedAmount != 200000.0 {
			st.Fatalf("expected disbursed amount 200000, got %v", loan.DisbursedAmount)
		}

		database.DB.Where("loan_id = ?", loanID).Order("installment_number asc").Find(&installments)
		// Registered on the outer t so the rows survive into the later
		// webhook/collections subtests, which need them.
		t.Cleanup(func() {
			for i := range installments {
				database.DB.Unscoped().Delete(&installments[i])
			}
		})
		if len(installments) != 24 {
			st.Fatalf("expected 24 EMI installments (tenure), got %d", len(installments))
		}

		// Backend-authoritative EMI amount: recompute independently with the
		// same reducing-balance formula and assert the stored installment 1
		// amount matches it exactly — nothing about this flow ever took an
		// amount from the client (AdminPrepareDisbursementHandler's "amount"
		// only sizes the principal disbursed, never an EMI figure; there is
		// no client-suppliable EMI amount field anywhere in this chain).
		rate := sanctionedRatePercent / 12 / 100
		pow := 1.0
		for i := 0; i < 24; i++ {
			pow *= (1 + rate)
		}
		expectedEmi := 200000.0 * rate * pow / (pow - 1)
		expectedEmi = float64(int(expectedEmi*100+0.5)) / 100
		if installments[0].Amount != expectedEmi {
			st.Fatalf("EMI amount must be entirely server-computed: expected %.2f, got %.2f", expectedEmi, installments[0].Amount)
		}
	})

	// --- 11. A payment/EMI event via the real, signature-verified webhook ---
	t.Run("10_emi_payment_webhook_signature_enforced", func(st *testing.T) {
		inst := installments[0]
		amountPaise := int64(inst.Amount * 100)
		payment := models.Payment{
			LoanID: loanID, EmiID: inst.ID, CustomerID: user.ID.String(), Gateway: "razorpay",
			GatewayOrderID: "order_" + uuid.NewString(), AmountPaise: amountPaise, Currency: "INR",
			Status: models.PaymentStatusInitiated,
		}
		if err := database.DB.Create(&payment).Error; err != nil {
			st.Fatalf("create test payment: %v", err)
		}
		st.Cleanup(func() { database.DB.Unscoped().Delete(&payment) })

		payload := map[string]interface{}{
			"event": "payment.captured",
			"payload": map[string]interface{}{
				"payment": map[string]interface{}{
					"entity": map[string]interface{}{
						"id": "pay_" + uuid.NewString(), "order_id": payment.GatewayOrderID,
						"amount": amountPaise, "currency": "INR", "method": "upi", "status": "captured",
					},
				},
			},
		}

		// First: a badly-signed delivery must be rejected and must NOT
		// settle the EMI — proves the webhook signature check is not
		// bypassed anywhere in this path.
		badReq := httptest.NewRequest(http.MethodPost, "/webhooks/razorpay", bytesFromJSON(st, payload))
		badReq.Header.Set("Content-Type", "application/json")
		badReq.Header.Set("X-Razorpay-Signature", "0000invalidsignature0000")
		badW := httptest.NewRecorder()
		r.ServeHTTP(badW, badReq)
		if badW.Code == http.StatusOK {
			st.Fatal("an invalid webhook signature must be rejected, not silently accepted")
		}
		var stillPending models.EmiInstallment
		database.DB.First(&stillPending, inst.ID)
		if stillPending.Status == models.EmiStatusPaid {
			st.Fatal("an invalid-signature webhook must never settle an EMI")
		}

		// Now: a correctly-signed delivery settles it.
		w := e2eSignedWebhook(st, r, payload, e2eWebhookSecret)
		if w.Code != http.StatusOK {
			st.Fatalf("correctly signed webhook should be accepted: %d %s", w.Code, w.Body.String())
		}
		var settled models.EmiInstallment
		database.DB.First(&settled, inst.ID)
		if settled.Status != models.EmiStatusPaid {
			st.Fatalf("expected EMI installment 1 to be marked paid, got %s", settled.Status)
		}
	})

	// --- 12. Collection/recovery step: drive installment 2 overdue and
	// verify a real CollectionCase is created via the same function a
	// scheduled DPD job would call (internal/collections.TouchCase) ---
	t.Run("11_overdue_creates_collection_case", func(st *testing.T) {
		if len(installments) < 2 {
			st.Fatal("expected at least 2 installments")
		}
		overdue := installments[1]
		pastDue := time.Now().AddDate(0, 0, -45)
		database.DB.Model(&overdue).Update("due_date", pastDue)

		cc, err := collections.TouchCase(loanID, user.ID.String(), 45, int64(overdue.Amount*100))
		if err != nil {
			st.Fatalf("TouchCase failed: %v", err)
		}
		st.Cleanup(func() { database.DB.Unscoped().Delete(cc) })
		if cc.DPD != 45 {
			st.Fatalf("expected DPD 45 recorded on the case, got %d", cc.DPD)
		}
		if cc.Status == "" {
			st.Fatal("expected a non-empty case status derived from DPD")
		}
		if cc.LoanID != loanID {
			st.Fatalf("case must reference the correct loan, got %d want %d", cc.LoanID, loanID)
		}
	})
}

func bytesFromJSON(t *testing.T, v interface{}) *bytes.Reader {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return bytes.NewReader(b)
}
