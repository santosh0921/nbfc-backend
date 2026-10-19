package loans_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/loans"
	"github.com/santosh0921/nbfc-backend/internal/models"
)

// Instant Loan: a distinct 35-day, flat-8%-fee, single-lump-sum product —
// separate from the regular 12-60 month reducing-balance EMI product that
// every other loan Category uses.

func createInstantLoanReadyForDisbursement(t *testing.T, amount float64) models.LoanApplication {
	t.Helper()
	// AdminPrepareDisbursementHandler still requires TenureMonths/
	// InterestRatePercent to already be set from the (unchanged)
	// maker-checker sanction flow, even though generateEmiSchedule ignores
	// both for an Instant Loan — an admin recommending/sanctioning an
	// Instant Loan today still has to enter *some* values in that existing
	// form. Placeholder values here mirror that real, current admin
	// experience; they have zero effect on the actual repayment schedule.
	placeholderTenure := 1
	placeholderRate := 8.0
	loan := models.LoanApplication{
		CustomerID: uuid.New(), Category: "Instant Loan", AmountRequested: amount,
		Status: models.LoanStatusSanctioned, ApplicantName: "Instant Loan Test",
		TenureMonths: &placeholderTenure, InterestRatePercent: &placeholderRate,
	}
	if err := database.DB.Create(&loan).Error; err != nil {
		t.Fatalf("create instant loan: %v", err)
	}
	t.Cleanup(func() { database.DB.Unscoped().Delete(&loan) })
	return loan
}

func TestInstantLoanGeneratesSingleLumpSumInstallmentAt35Days(t *testing.T) {
	r := setupDisbursementTestApp(t)
	makerU, checkerU := "instant_maker_"+t.Name(), "instant_checker_"+t.Name()
	_, makerPw := createDisbTestAdmin(t, makerU, models.AdminRoleDisbursementOfficer)
	_, checkerPw := createDisbTestAdmin(t, checkerU, models.AdminRoleOperations)
	makerToken := disbLoginAs(t, r, makerU, makerPw)
	checkerToken := disbLoginAs(t, r, checkerU, checkerPw)

	loan := createInstantLoanReadyForDisbursement(t, 10000)

	w0 := disbDoJSON(r, "POST", "/admin/loans/"+idStr(loan.ID)+"/prepare-disbursement", makerToken, gin.H{"amount": 10000})
	if w0.Code != http.StatusOK {
		t.Fatalf("prepare failed: %d %s", w0.Code, w0.Body.String())
	}
	w1 := disbDoJSON(r, "POST", "/admin/loans/"+idStr(loan.ID)+"/approve-disbursement", checkerToken, gin.H{"approve": true})
	if w1.Code != http.StatusOK {
		t.Fatalf("approve failed: %d %s", w1.Code, w1.Body.String())
	}

	var disb models.Disbursement
	if err := database.DB.Where("loan_id = ?", loan.ID).First(&disb).Error; err != nil {
		t.Fatalf("expected a Disbursement record: %v", err)
	}
	beforeConfirm := time.Now()
	if err := loans.ConfirmDisbursement(disb.ID, "UTR-INSTANT-TEST", "test-reconciler", nil); err != nil {
		t.Fatalf("ConfirmDisbursement failed: %v", err)
	}
	t.Cleanup(func() {
		database.DB.Unscoped().Where("loan_id = ?", loan.ID).Delete(&models.EmiInstallment{})
	})

	var installments []models.EmiInstallment
	database.DB.Where("loan_id = ?", loan.ID).Order("installment_number asc").Find(&installments)

	if len(installments) != 1 {
		t.Fatalf("Instant Loan must generate exactly ONE lump-sum installment, got %d", len(installments))
	}
	inst := installments[0]

	// 10000 principal + 8% flat fee = 10800.
	if inst.Amount != 10800 {
		t.Fatalf("expected total payable 10800 (10000 principal + 8%% flat fee), got %v", inst.Amount)
	}
	if inst.PrincipalComponent != 10000 {
		t.Fatalf("expected principal component 10000, got %v", inst.PrincipalComponent)
	}
	if inst.InterestComponent != 800 {
		t.Fatalf("expected flat fee component 800, got %v", inst.InterestComponent)
	}

	// Due date must be ~35 days after disbursement, not one month (a
	// regular EMI's first installment would be ~30/31 days out via
	// AddDate(0,1,0) — Instant Loan must NOT reuse that path).
	wantDue := beforeConfirm.AddDate(0, 0, 35)
	diff := inst.DueDate.Sub(wantDue)
	if diff < -time.Hour || diff > time.Hour {
		t.Fatalf("expected due date ~35 days after disbursement (%v), got %v", wantDue, inst.DueDate)
	}
}

