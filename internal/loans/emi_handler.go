package loans

import (
	"errors"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm/clause"

	"github.com/santosh0921/nbfc-backend/internal/audit"
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
	LoanID              uint                     `json:"loanId"`
	TenureMonths        int                      `json:"tenureMonths"`
	InterestRatePercent float64                  `json:"interestRatePercent"`
	Installments        []models.EmiInstallment `json:"installments"`
	Summary             emiSummary               `json:"summary"`
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

// payEmiRequest is the proof-of-payment the caller must present. There is
// no payment gateway wired into this project yet (no Razorpay/Cashfree/PayU
// account or API keys are configured anywhere in the codebase) — until one
// is, VerifyGatewaySignature below is the seam a real webhook-signature
// check plugs into. What this handler enforces today, for real: the caller
// cannot mark an installment paid by simply asserting so with no
// information — a non-empty, single-use transaction reference and an
// amount that matches the installment exactly are both required, the
// check-then-write is atomic and row-locked against concurrent double-pay,
// and the same reference can never be replayed against a second
// installment (PaymentReference has a unique index).
type payEmiRequest struct {
	TransactionID string  `json:"transactionId" binding:"required"`
	Amount        float64 `json:"amount" binding:"required"`
	Method        string  `json:"method"`
}

// VerifyGatewaySignature is a placeholder seam for real payment-gateway
// webhook/signature verification (e.g. Razorpay's X-Razorpay-Signature
// HMAC). Wiring up a real gateway requires that provider's API
// credentials, which this project does not have configured — replace this
// function's body with the real HMAC check once they're available. Until
// then it only enforces that a transaction reference was actually
// supplied, which is what the unique index and unit tests below assume.
func VerifyGatewaySignature(transactionID string) error {
	if strings.TrimSpace(transactionID) == "" {
		return errors.New("missing transaction reference")
	}
	return nil
}

// CustomerPayEmiHandler handles POST /auth/loans/:id/emi/:installmentId/pay
func CustomerPayEmiHandler(c *gin.Context) {
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
	if loan.Status != models.LoanStatusDisbursed {
		c.JSON(http.StatusBadRequest, gin.H{"message": "This loan has no active EMI schedule to pay against"})
		return
	}

	var body payEmiRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "A transaction reference and amount are required"})
		return
	}
	if err := VerifyGatewaySignature(body.TransactionID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Could not verify payment: " + err.Error()})
		return
	}

	installmentID, _ := strconv.Atoi(c.Param("installmentId"))

	tx := database.DB.Begin()
	if tx.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Database error"})
		return
	}

	var inst models.EmiInstallment
	// SELECT ... FOR UPDATE — holds a row lock for the rest of this
	// transaction so two concurrent pay requests for the same installment
	// can't both read "pending" and both write "paid".
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id = ? AND loan_id = ?", installmentID, loan.ID).First(&inst).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusBadRequest, gin.H{"message": "Installment not found"})
		return
	}
	if inst.Status == models.EmiStatusPaid {
		tx.Rollback()
		c.JSON(http.StatusBadRequest, gin.H{"message": "Installment already paid"})
		return
	}
	if math.Abs(body.Amount-inst.Amount) > 0.01 {
		tx.Rollback()
		c.JSON(http.StatusBadRequest, gin.H{"message": "Payment amount does not match the amount due for this installment"})
		return
	}

	now := time.Now()
	inst.Status = models.EmiStatusPaid
	inst.PaidAt = &now
	inst.PaymentReference = strings.TrimSpace(body.TransactionID)
	inst.PaymentMethod = body.Method

	if err := tx.Save(&inst).Error; err != nil {
		tx.Rollback()
		if isUniqueViolation(err) {
			c.JSON(http.StatusConflict, gin.H{"message": "This transaction reference has already been used to settle a different installment"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Failed to record payment"})
		return
	}
	if err := tx.Commit().Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Failed to record payment"})
		return
	}

	audit.RecordAs("customer", user.Mobile, "emi.pay", "emi_installment", strconv.Itoa(int(inst.ID)),
		"Paid installment #"+strconv.Itoa(inst.InstallmentNumber)+" on loan #"+itoa(loan.ID)+
			" - amount "+strconv.FormatFloat(inst.Amount, 'f', 2, 64)+", ref "+inst.PaymentReference)

	c.JSON(http.StatusOK, inst)
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
