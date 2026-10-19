package loans

import (
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm/clause"

	"github.com/santosh0921/nbfc-backend/internal/accounting"
	"github.com/santosh0921/nbfc-backend/internal/audit"
	"github.com/santosh0921/nbfc-backend/internal/collections"
	"github.com/santosh0921/nbfc-backend/internal/config"
	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/models"
)

type emiSummary struct {
	TotalInstallments int        `json:"totalInstallments"`
	PaidCount         int        `json:"paidCount"`
	RemainingCount    int        `json:"remainingCount"`
	TotalPaid         float64    `json:"totalPaid"`
	TotalRemaining    float64    `json:"totalRemaining"`
	NextDueDate       *time.Time `json:"nextDueDate"`
	NextDueAmount     float64    `json:"nextDueAmount"`
}

type emiScheduleResponse struct {
	LoanID              uint                    `json:"loanId"`
	TenureMonths        int                     `json:"tenureMonths"`
	InterestRatePercent float64                 `json:"interestRatePercent"`
	Installments        []models.EmiInstallment `json:"installments"`
	Summary             emiSummary              `json:"summary"`
}

// buildScheduleResponse computes read-time "overdue" status on each
// installment and aggregates the summary block.
func buildScheduleResponse(loan models.LoanApplication, installments []models.EmiInstallment) emiScheduleResponse {
	resp := emiScheduleResponse{
		LoanID:       loan.ID,
		Installments: make([]models.EmiInstallment, 0, len(installments)),
	}
	if loan.TenureMonths != nil {
		resp.TenureMonths = *loan.TenureMonths
	}
	if loan.InterestRatePercent != nil {
		resp.InterestRatePercent = *loan.InterestRatePercent
	}

	var paidCount, remainingCount int
	var totalPaid, totalRemaining float64
	var nextDueDate *time.Time
	var nextDueAmount float64

	for _, inst := range installments {
		inst.Status = effectiveStatus(inst)
		resp.Installments = append(resp.Installments, inst)

		if inst.Status == models.EmiStatusPaid {
			paidCount++
			totalPaid += inst.Amount
		} else {
			remainingCount++
			totalRemaining += inst.Amount
			if nextDueDate == nil || inst.DueDate.Before(*nextDueDate) {
				d := inst.DueDate
				nextDueDate = &d
				nextDueAmount = inst.Amount
			}
		}
	}

	resp.Summary = emiSummary{
		TotalInstallments: len(installments),
		PaidCount:         paidCount,
		RemainingCount:    remainingCount,
		TotalPaid:         round2(totalPaid),
		TotalRemaining:    round2(totalRemaining),
		NextDueDate:       nextDueDate,
		NextDueAmount:     nextDueAmount,
	}
	return resp
}

// CustomerEmiScheduleHandler handles GET /auth/loans/:id/emi-schedule
func CustomerEmiScheduleHandler(c *gin.Context) {
	user, ok := currentUser(c)
	if !ok {
		return
	}
	id, _ := strconv.Atoi(c.Param("id"))
	var loan models.LoanApplication
	if err := database.DB.First(&loan, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"message": "Loan not found"})
		return
	}
	if loan.CustomerID != user.ID {
		c.JSON(http.StatusForbidden, gin.H{"message": "Not your loan"})
		return
	}
	var installments []models.EmiInstallment
	database.DB.Where("loan_id = ?", loan.ID).Order("installment_number asc").Find(&installments)
	c.JSON(http.StatusOK, buildScheduleResponse(loan, installments))
}

// CustomerPayEmiHandler handles POST /auth/loans/:id/emi/:installmentId/pay
// — disabled as of the Phase 2 real-payment-gateway rework. This endpoint
// used to mark an installment paid on nothing more than a customer-
// supplied transaction-reference string (VerifyGatewaySignature only
// checked it was non-empty) — a customer could settle any EMI by typing
// any string in. An EMI is now only ever marked paid by
// internal/payments.RazorpayWebhookHandler, after independently verifying
// a real gateway webhook's HMAC signature and matching its amount/order
// against a Payment row this backend created. See
// POST /auth/loans/:id/emi/:installmentId/create-order in
// internal/payments/order_handler.go for the replacement flow.
func CustomerPayEmiHandler(c *gin.Context) {
	c.JSON(http.StatusGone, gin.H{"message": "Direct EMI settlement has been disabled. Use POST /auth/loans/:id/emi/:installmentId/create-order to start a verified gateway payment."})
}

// CustomerDemoPayEmiHandler handles
// POST /auth/loans/:id/emi/:installmentId/demo-pay — a deliberately
// narrow, clearly-labeled exception to the rule above, NOT a reopening of
// the vulnerability CustomerPayEmiHandler was disabled for. The old
// endpoint's flaw was that ANY string counted as proof of payment, in
// production, for any loan the request happened to name. This endpoint:
//
//  1. Is a 404 unless config.DemoMode is true — never reachable in
//     production regardless of what a client sends (checked first, before
//     any other logic).
//  2. Still enforces the same ownership check every real payment path
//     enforces (loan.CustomerID must equal the authenticated customer).
//  3. Accepts no customer-supplied "proof" at all — there is no reference
//     string parameter to fake. The installment ID from the URL, already
//     ownership-checked, is the only input.
//  4. Runs through the exact same durable state changes a real verified
//     webhook would (accounting posting, promise-to-pay check, receipt
//     generation, audit log) — so a demo environment's own data (upcoming/
//     paid EMI lists, payment history) is genuinely, durably correct, not
//     a UI-only illusion — while every record it creates is tagged
//     "demo_settlement" so it can never be mistaken for a real gateway
//     confirmation in the admin panel, accounting ledger, or audit trail.
func CustomerDemoPayEmiHandler(c *gin.Context) {
	if !config.DemoMode {
		c.JSON(http.StatusNotFound, gin.H{"message": "Not found"})
		return
	}

	user, ok := currentUser(c)
	if !ok {
		return
	}

	loanID, _ := strconv.Atoi(c.Param("id"))
	var loan models.LoanApplication
	if err := database.DB.First(&loan, loanID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"message": "Loan not found"})
		return
	}
	if loan.CustomerID != user.ID {
		c.JSON(http.StatusForbidden, gin.H{"message": "Not your loan"})
		return
	}
	if loan.Status != models.LoanStatusDisbursed {
		c.JSON(http.StatusBadRequest, gin.H{"message": "This loan has no active EMI schedule to pay against"})
		return
	}

	installmentID, _ := strconv.Atoi(c.Param("installmentId"))

	tx := database.DB.Begin()
	if tx.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Database error"})
		return
	}

	var inst models.EmiInstallment
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id = ? AND loan_id = ?", installmentID, loan.ID).First(&inst).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusNotFound, gin.H{"message": "Installment not found"})
		return
	}
	if inst.Status == models.EmiStatusPaid {
		tx.Rollback()
		c.JSON(http.StatusBadRequest, gin.H{"message": "Installment already paid"})
		return
	}

	amountPaise := int64(math.Round(inst.Amount * 100))
	demoRef := "DEMO-" + strconv.Itoa(int(loan.ID)) + "-" + strconv.Itoa(int(inst.ID)) + "-" + strconv.FormatInt(time.Now().UnixNano(), 10)

	payment := models.Payment{
		LoanID:         loan.ID,
		EmiID:          inst.ID,
		CustomerID:     user.ID.String(),
		Gateway:        "demo_settlement",
		GatewayOrderID: demoRef,
		AmountPaise:    amountPaise,
		Currency:       "INR",
		Status:         models.PaymentStatusSuccess,
		Method:         "demo",
	}
	if err := tx.Create(&payment).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Failed to record payment"})
		return
	}

	now := time.Now()
	payment.VerifiedAt = &now
	payment.GatewayPaymentID = demoRef
	if err := tx.Save(&payment).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Failed to record payment"})
		return
	}

	inst.Status = models.EmiStatusPaid
	inst.PaidAt = &now
	inst.PaymentReference = demoRef
	inst.PaymentMethod = "demo"
	if err := tx.Save(&inst).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Failed to mark installment paid"})
		return
	}

	if err := tx.Commit().Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Database error"})
		return
	}

	// Best-effort, same as the real webhook path — the EMI/payment state is
	// already durably committed above regardless of these succeeding.
	if _, err := accounting.PostEmiPayment(payment, inst, "demo_settlement"); err != nil {
		audit.RecordDetailed(c, audit.Entry{
			ActorType: "system", ActorRef: "demo_settlement",
			Action: "accounting.post.failed", TargetType: "payment", TargetID: strconv.Itoa(int(payment.ID)),
			Result: audit.ResultFailure, Reason: err.Error(),
		})
	}
	collections.CheckPromiseAfterPayment(loan.ID, payment.ID, payment.AmountPaise)
	if err := GenerateEmiPaymentReceipt(loan, inst, "demo_settlement"); err != nil {
		audit.RecordDetailed(c, audit.Entry{
			ActorType: "system", ActorRef: "demo_settlement",
			Action: "receipt.generate.failed", TargetType: "payment", TargetID: strconv.Itoa(int(payment.ID)),
			Result: audit.ResultFailure, Reason: err.Error(),
		})
	}
	if err := CheckAndGenerateNOC(loan.ID); err != nil {
		audit.RecordDetailed(c, audit.Entry{
			ActorType: "system", ActorRef: "demo_settlement",
			Action: "noc.generate.failed", TargetType: "loan", TargetID: strconv.Itoa(int(loan.ID)),
			Result: audit.ResultFailure, Reason: err.Error(),
		})
	}

	audit.RecordDetailed(c, audit.Entry{
		ActorType: "customer", ActorRef: user.Mobile,
		Action: "payment.demo_settlement", TargetType: "payment", TargetID: strconv.Itoa(int(payment.ID)),
		Result: audit.ResultSuccess,
		Details: "DEMO MODE ONLY — loan #" + strconv.Itoa(int(loan.ID)) + ", installment #" + strconv.Itoa(inst.InstallmentNumber) +
			", amount " + strconv.FormatInt(amountPaise, 10) + " paise — no real gateway involved",
	})

	var schedule []models.EmiInstallment
	database.DB.Where("loan_id = ?", loan.ID).Order("installment_number asc").Find(&schedule)
	c.JSON(http.StatusOK, buildScheduleResponse(loan, schedule))
}

func isUniqueViolation(err error) bool {
	return err != nil && strings.Contains(strings.ToLower(err.Error()), "duplicate")
}

// CustomerEmiSummaryHandler handles GET /auth/dashboard/emi-summary
func CustomerEmiSummaryHandler(c *gin.Context) {
	user, ok := currentUser(c)
	if !ok {
		return
	}

	var loanList []models.LoanApplication
	database.DB.Where("customer_id = ? AND status = ?", user.ID, models.LoanStatusDisbursed).Find(&loanList)

	resp := gin.H{
		"hasUpcomingEmi":            false,
		"nextDueDate":               nil,
		"nextDueAmount":             0,
		"loanId":                    nil,
		"totalPaidAcrossLoans":      0,
		"totalRemainingAcrossLoans": 0,
		"totalActiveLoans":          0,
	}
	if len(loanList) == 0 {
		c.JSON(http.StatusOK, resp)
		return
	}

	loanIDs := make([]uint, 0, len(loanList))
	for _, l := range loanList {
		loanIDs = append(loanIDs, l.ID)
	}

	var installments []models.EmiInstallment
	database.DB.Where("loan_id IN ?", loanIDs).Order("due_date asc").Find(&installments)
	if len(installments) == 0 {
		c.JSON(http.StatusOK, resp)
		return
	}

	var totalPaid, totalRemaining float64
	var nextDueDate *time.Time
	var nextDueAmount float64
	var nextLoanID uint
	activeLoans := map[uint]bool{}

	for _, inst := range installments {
		activeLoans[inst.LoanID] = true
		st := effectiveStatus(inst)
		if st == models.EmiStatusPaid {
			totalPaid += inst.Amount
		} else {
			totalRemaining += inst.Amount
			if nextDueDate == nil || inst.DueDate.Before(*nextDueDate) {
				d := inst.DueDate
				nextDueDate = &d
				nextDueAmount = inst.Amount
				nextLoanID = inst.LoanID
			}
		}
	}

	resp["totalPaidAcrossLoans"] = round2(totalPaid)
	resp["totalRemainingAcrossLoans"] = round2(totalRemaining)
	resp["totalActiveLoans"] = len(activeLoans)
	if nextDueDate != nil {
		resp["hasUpcomingEmi"] = true
		resp["nextDueDate"] = nextDueDate
		resp["nextDueAmount"] = nextDueAmount
		resp["loanId"] = nextLoanID
	}

	c.JSON(http.StatusOK, resp)
}

// AdminEmiScheduleHandler handles GET /admin/loans/:id/emi-schedule
func AdminEmiScheduleHandler(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var loan models.LoanApplication
	if err := database.DB.First(&loan, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"message": "Not found"})
		return
	}
	var installments []models.EmiInstallment
	database.DB.Where("loan_id = ?", loan.ID).Order("installment_number asc").Find(&installments)
	c.JSON(http.StatusOK, buildScheduleResponse(loan, installments))
}
