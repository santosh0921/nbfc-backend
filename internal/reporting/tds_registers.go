// Phase 10 controlled TDS report builders — same pattern as registers.go
// (item T: every field/total server-computed from the authoritative
// tables, never client-supplied).
package reporting

import (
	"strconv"
	"time"

	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/models"
)

func buildTDSRegister(from, to time.Time) *reportRows {
	var rows []models.TDSTransaction
	database.DB.Where("transaction_date BETWEEN ? AND ?", from, to).Order("id").Find(&rows)
	headers := []string{"TDS Txn ID", "Deductee", "Section", "Rule Version", "Gross (paise)", "Threshold (paise)", "Rate %", "TDS (paise)", "Net Payable (paise)", "PAN Status", "Status", "Journal Ref"}
	var out [][]string
	var totalGross, totalTDS int64
	for _, r := range rows {
		out = append(out, []string{
			itoa(r.ID), r.Deductee, r.SectionRef, r.RuleVersion, paiseStr(r.BaseAmountPaise), paiseStr(r.ThresholdPaise),
			floatStr(r.RatePercent), paiseStr(r.TDSAmountPaise), paiseStr(r.NetPayablePaise), r.PANStatusAtCalc, r.Status, r.JournalReference,
		})
		if r.Status == models.TDSStatusDeducted {
			totalGross += r.BaseAmountPaise
			totalTDS += r.TDSAmountPaise
		}
	}
	return &reportRows{Headers: headers, Rows: out, Totals: map[string]string{
		"Total Gross Deducted-Against (paise)": paiseStr(totalGross),
		"Total TDS (paise)":                    paiseStr(totalTDS),
		"Record Count":                         itoaInt(len(out)),
	}}
}

func buildTDSChallanRegister(from, to time.Time) *reportRows {
	var rows []models.TDSChallan
	database.DB.Where("created_at BETWEEN ? AND ?", from, to).Order("id").Find(&rows)
	headers := []string{"Challan ID", "Challan Number", "Tax Year", "Quarter", "Amount (paise)", "Status", "Deposit Due Date", "Actual Deposit Date", "Bank Reference", "Journal Ref"}
	var out [][]string
	var total int64
	for _, r := range rows {
		due, actual := "", ""
		if r.DepositDueDate != nil {
			due = r.DepositDueDate.Format("2006-01-02")
		}
		if r.ActualDepositDate != nil {
			actual = r.ActualDepositDate.Format("2006-01-02")
		}
		out = append(out, []string{
			itoa(r.ID), r.ChallanNumber, r.TaxYear, r.Quarter, paiseStr(r.AmountPaise), r.Status, due, actual, r.BankReference, r.JournalReference,
		})
		if r.Status == models.ChallanStatusPaid {
			total += r.AmountPaise
		}
	}
	return &reportRows{Headers: headers, Rows: out, Totals: map[string]string{
		"Total Deposited (paise)": paiseStr(total), "Record Count": itoaInt(len(out)),
	}}
}

func buildTDSExceptionsReport(from, to time.Time) *reportRows {
	var rows []models.TDSException
	database.DB.Where("created_at BETWEEN ? AND ?", from, to).Order("id").Find(&rows)
	headers := []string{"Exception ID", "Type", "Severity", "Reference Type", "Reference ID", "Status", "Detail", "Created At"}
	var out [][]string
	openCount := 0
	for _, r := range rows {
		out = append(out, []string{
			itoa(r.ID), r.ExceptionType, r.Severity, r.ReferenceType, r.ReferenceID, r.Status, r.Detail, r.CreatedAt.Format(time.RFC3339),
		})
		if r.Status != models.TDSExceptionStatusResolved {
			openCount++
		}
	}
	return &reportRows{Headers: headers, Rows: out, Totals: map[string]string{
		"Open Exceptions": itoaInt(openCount), "Record Count": itoaInt(len(out)),
	}}
}

func floatStr(v float64) string {
	return strconv.FormatFloat(v, 'f', 2, 64)
}
