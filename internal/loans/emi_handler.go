package loans

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

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
