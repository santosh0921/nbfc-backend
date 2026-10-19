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

// buildGSTRegister (CGST/SGST/IGST breakdown per taxable event) — one row
// per models.GSTTransaction, the authoritative source for every GST
// figure this codebase reports (internal/tax.GenerateTaxDashboard sums
// the same table for the MIS "tax" section, so this register's totals
// always reconcile with what the dashboard shows for the same period).
// CGST+SGST apply to intra-state supply, IGST to inter-state — never both
// on the same row, so CGST+SGST+IGST always equals the row's total tax.
func buildGSTRegister(from, to time.Time) *reportRows {
	var txns []models.GSTTransaction
	database.DB.Where("created_at BETWEEN ? AND ?", from, to).Order("id").Find(&txns)
	headers := []string{
		"GST Txn ID", "Financial Reference", "Supply Type", "Place of Supply",
		"Taxable Value (paise)", "GST Rate %", "CGST (paise)", "SGST (paise)", "IGST (paise)", "Total GST (paise)",
		"GSTIN", "Customer GSTIN", "Invoice Reference", "Created At",
	}
	var rows [][]string
	var totalTaxable, totalCGST, totalSGST, totalIGST int64
	for _, t := range txns {
		totalGST := t.CGSTPaise + t.SGSTPaise + t.IGSTPaise
		rows = append(rows, []string{
			itoa(t.ID), t.FinancialReference, t.SupplyType, t.PlaceOfSupply,
			paiseStr(t.TaxableValuePaise), floatStr(t.GSTRatePercent),
			paiseStr(t.CGSTPaise), paiseStr(t.SGSTPaise), paiseStr(t.IGSTPaise), paiseStr(totalGST),
			t.Gstin, t.CustomerGstin, t.InvoiceReference, t.CreatedAt.Format(time.RFC3339),
		})
		totalTaxable += t.TaxableValuePaise
		totalCGST += t.CGSTPaise
		totalSGST += t.SGSTPaise
		totalIGST += t.IGSTPaise
	}
	return &reportRows{Headers: headers, Rows: rows, Totals: map[string]string{
		"Total Taxable Value (paise)": paiseStr(totalTaxable),
		"Total CGST (paise)":          paiseStr(totalCGST),
		"Total SGST (paise)":          paiseStr(totalSGST),
		"Total IGST (paise)":          paiseStr(totalIGST),
		"Total GST (paise)":           paiseStr(totalCGST + totalSGST + totalIGST),
		"Record Count":                itoaInt(len(rows)),
	}}
}

func maskMobile(m string) string {
	if len(m) < 4 {
		return "XXXXXXXXXX"
	}
	return "XXXXXX" + m[len(m)-4:]
}
