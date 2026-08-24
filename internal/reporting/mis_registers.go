// Phase 12 controlled MIS report builders (Loan/Customer registers) —
// same pattern as registers.go/tds_registers.go.
package reporting

import (
	"time"

	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/models"
)

func buildLoanRegister(from, to time.Time) *reportRows {
	var loans []models.LoanApplication
	database.DB.Where("created_at BETWEEN ? AND ?", from, to).Order("id").Find(&loans)
	headers := []string{"Loan ID", "Customer ID", "Category", "Amount Requested", "Status", "CIBIL Score", "Disbursed Amount", "Disbursed At", "Created At"}
	var rows [][]string
	var totalRequested, totalDisbursed float64
	for _, l := range loans {
		disbursedAt := ""
		if l.DisbursedAt != nil {
			disbursedAt = l.DisbursedAt.Format(time.RFC3339)
		}
		rows = append(rows, []string{
			itoa(l.ID), l.CustomerID.String(), l.Category, floatStr(l.AmountRequested), l.Status,
			itoaInt(l.CibilScore), floatStr(l.DisbursedAmount), disbursedAt, l.CreatedAt.Format(time.RFC3339),
		})
		totalRequested += l.AmountRequested
		if l.Status == models.LoanStatusDisbursed {
			totalDisbursed += l.DisbursedAmount
		}
	}
	return &reportRows{Headers: headers, Rows: rows, Totals: map[string]string{
		"Total Amount Requested": floatStr(totalRequested),
		"Total Disbursed":        floatStr(totalDisbursed),
		"Record Count":           itoaInt(len(rows)),
	}}
}

func buildCustomerRegister(from, to time.Time) *reportRows {
	var users []models.User
	database.DB.Where("created_at BETWEEN ? AND ?", from, to).Order("id").Find(&users)
	headers := []string{"Customer ID", "Mobile", "Created At"}
	var rows [][]string
	for _, u := range users {
		// Mobile is intentionally the only PII column — item Q: "avoid
		// exposing unnecessary PII in aggregate reports." No PAN/Aadhaar/
		// address is ever included in this register.
		rows = append(rows, []string{u.ID.String(), maskMobile(u.Mobile), u.CreatedAt.Format(time.RFC3339)})
	}
	return &reportRows{Headers: headers, Rows: rows, Totals: map[string]string{"Record Count": itoaInt(len(rows))}}
}

func maskMobile(m string) string {
	if len(m) < 4 {
		return "XXXXXXXXXX"
	}
	return "XXXXXX" + m[len(m)-4:]
}
