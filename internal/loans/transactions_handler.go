package loans

import (
	"net/http"
	"sort"

	"github.com/gin-gonic/gin"

	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/models"
)

// CustomerTransaction is one row in the customer's unified transaction
// ledger — synthesized from real events (a loan being disbursed, an EMI
// installment being paid) rather than stored as its own table, since
// every one of these events already has a definitive source of truth
// elsewhere (LoanApplication, EmiInstallment).
type CustomerTransaction struct {
	ID              string  `json:"id"`
	Type            string  `json:"type"` // "disbursal" | "emiPayment"
	Status          string  `json:"status"`
	Amount          float64 `json:"amount"`
	Date            string  `json:"date"`
	Description     string  `json:"description"`
	LoanID          string  `json:"loanId"`
	ReferenceNumber string  `json:"referenceNumber"`
}

// CustomerTransactionsHandler handles GET /auth/transactions — every
// disbursal and EMI payment across every loan the customer has ever had,
// newest first. Replaces what used to be entirely mock/hardcoded data on
// the client with the customer's real payment/disbursal history.
func CustomerTransactionsHandler(c *gin.Context) {
	user, ok := currentUser(c)
	if !ok {
		return
	}

	var loanList []models.LoanApplication
	database.DB.Where("customer_id = ?", user.ID).Find(&loanList)

	var txns []CustomerTransaction
	for _, loan := range loanList {
		if loan.DisbursedAt != nil {
			txns = append(txns, CustomerTransaction{
				ID:              "disb-" + itoa(loan.ID),
				Type:            "disbursal",
				Status:          "success",
				Amount:          loan.DisbursedAmount,
				Date:            loan.DisbursedAt.Format("2006-01-02T15:04:05Z07:00"),
				Description:     loan.Category + " disbursed",
				LoanID:          itoa(loan.ID),
				ReferenceNumber: "DISB-" + itoa(loan.ID),
			})
		}

		var installments []models.EmiInstallment
		database.DB.Where("loan_id = ? AND status = ?", loan.ID, "paid").Find(&installments)
		for _, inst := range installments {
			date := inst.CreatedAt
			if inst.PaidAt != nil {
				date = *inst.PaidAt
			}
			ref := inst.PaymentReference
			if ref == "" {
				ref = "EMI-" + itoa(loan.ID) + "-" + itoa(uint(inst.InstallmentNumber))
			}
			txns = append(txns, CustomerTransaction{
				ID:              "emi-" + itoa(inst.ID),
				Type:            "emiPayment",
				Status:          "success",
				Amount:          inst.Amount,
				Date:            date.Format("2006-01-02T15:04:05Z07:00"),
				Description:     loan.Category + " — EMI #" + itoa(uint(inst.InstallmentNumber)),
				LoanID:          itoa(loan.ID),
				ReferenceNumber: ref,
			})
		}
	}

	sort.Slice(txns, func(i, j int) bool { return txns[i].Date > txns[j].Date })

	c.JSON(http.StatusOK, txns)
}
