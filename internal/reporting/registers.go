// Controlled report-schema registry (Phase 8 item S / Phase 9 item R gap
// closure). Every function here builds headers/rows/totals directly from
// the authoritative source tables — never from client-supplied rows. The
// previous generic AdminRequestExportHandler (which accepted arbitrary
// headers/rows from the request body) is superseded for these named
// report types by AdminExportReportHandler below, which looks up one of
// these builders and ignores anything the client sent for content.
package reporting

import (
	"strconv"
	"time"

	"github.com/santosh0921/nbfc-backend/internal/accounting"
	"github.com/santosh0921/nbfc-backend/internal/analytics"
	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/models"
)

type reportRows struct {
	Headers []string
	Rows    [][]string
	Totals  map[string]string
}

// BuildRegisterReport dispatches to a named, controlled report builder.
// Returns (nil, false) for an unrecognized reportType — callers must
// treat that as "not a controlled report," never fall back to
// client-supplied content for it.
func BuildRegisterReport(reportType string, from, to time.Time) (*reportRows, bool) {
	switch reportType {
	case "GENERAL_LEDGER_TRIAL_BALANCE":
		return buildTrialBalanceReport(from, to), true
	case "DISBURSEMENT_REGISTER":
		return buildDisbursementRegister(from, to), true
	case "PAYMENT_REGISTER":
		return buildPaymentRegister(from, to), true
	case "EXPENSE_REGISTER":
		return buildExpenseRegister(from, to), true
	case "BANK_TRANSACTION_REGISTER":
		return buildBankTransactionRegister(from, to), true
	case "RECONCILIATION_EXCEPTIONS":
		return buildReconciliationExceptionsReport(from, to), true
	case "REFUND_REGISTER":
		return buildRefundRegister(from, to), true
	case "FAILED_PAYMENTS":
		return buildPaymentsByStatus(from, to, models.PaymentStatusFailed), true
	case "PENDING_PAYMENTS":
		return buildPaymentsByStatus(from, to, models.PaymentStatusPending), true
	case "TDS_REGISTER":
		return buildTDSRegister(from, to), true
	case "TDS_CHALLAN_REGISTER":
		return buildTDSChallanRegister(from, to), true
	case "TDS_EXCEPTIONS":
		return buildTDSExceptionsReport(from, to), true
	case "LOAN_REGISTER":
		return buildLoanRegister(from, to), true
	case "CUSTOMER_REGISTER":
		return buildCustomerRegister(from, to), true

	// Phase 13 analytics exports (item AD).
	case "GROWTH_DASHBOARD":
		return buildGrowthDashboardReport(from, to), true
	case "CUSTOMER_GROWTH":
		return growthSlice(from, to, func(d analytics.GrowthDashboard) []analytics.Metric { return d.Customers }), true
	case "LOAN_GROWTH":
		return growthSlice(from, to, func(d analytics.GrowthDashboard) []analytics.Metric { return d.Loans }), true
	case "DISBURSEMENT_GROWTH":
		return growthSlice(from, to, func(d analytics.GrowthDashboard) []analytics.Metric { return d.Disbursements }), true
	case "COLLECTION_GROWTH":
		return growthSlice(from, to, func(d analytics.GrowthDashboard) []analytics.Metric { return d.Collections }), true
	case "PORTFOLIO_GROWTH":
		return growthSlice(from, to, func(d analytics.GrowthDashboard) []analytics.Metric { return d.Portfolio }), true
	case "REVENUE_GROWTH":
		return growthSlice(from, to, func(d analytics.GrowthDashboard) []analytics.Metric { return d.Revenue }), true
	case "EXPENSE_GROWTH":
		return growthSlice(from, to, func(d analytics.GrowthDashboard) []analytics.Metric { return d.Expenses }), true
	case "PROFITABILITY_GROWTH":
		return growthSlice(from, to, func(d analytics.GrowthDashboard) []analytics.Metric { return d.Profitability }), true
	case "PRODUCT_ANALYTICS":
		return buildProductAnalyticsReport(from, to), true
	case "CUSTOMER_SEGMENTS":
		return buildSegmentAnalyticsReport(from, to), true
	case "GEOGRAPHIC_ANALYTICS":
		return buildGeographicAnalyticsReport(from, to), true
	case "DPD_ANALYTICS":
		return buildDPDAnalyticsReport(from, to), true
	case "CONCENTRATION_ANALYTICS":
		return buildConcentrationReport(from, to), true
	case "COLLECTION_PERFORMANCE":
		return buildCollectionPerformanceReport(from, to), true
	case "RISK_SIGNALS":
		return buildRiskSignalsReport(from, to), true
	case "MANAGEMENT_ALERTS":
		return buildManagementAlertsReport(from, to), true
	case "KPI_DICTIONARY":
		return buildKPIDictionaryReport(from, to), true

	default:
		return nil, false
	}
}

func buildTrialBalanceReport(from, to time.Time) *reportRows {
	tb := accounting.TrialBalance(from, to)
	headers := []string{"Account Code", "Account Name", "Account Type", "Opening Debit", "Opening Credit", "Period Debit", "Period Credit", "Closing Debit", "Closing Credit"}
	var rows [][]string
	for _, r := range tb.Rows {
		rows = append(rows, []string{
			r.AccountCode, r.AccountName, r.AccountType,
			paiseStr(r.OpeningDebit), paiseStr(r.OpeningCredit),
			paiseStr(r.PeriodDebit), paiseStr(r.PeriodCredit),
			paiseStr(r.ClosingDebit), paiseStr(r.ClosingCredit),
		})
	}
	return &reportRows{Headers: headers, Rows: rows, Totals: map[string]string{
		"Total Closing Debit (paise)":  paiseStr(tb.TotalClosingDebit),
		"Total Closing Credit (paise)": paiseStr(tb.TotalClosingCredit),
		"Balanced":                     boolStr(tb.Balanced),
	}}
}

func buildDisbursementRegister(from, to time.Time) *reportRows {
	var disbursements []models.Disbursement
	database.DB.Where("created_at BETWEEN ? AND ?", from, to).Order("id").Find(&disbursements)
	headers := []string{"Disbursement ID", "Loan ID", "Customer ID", "Amount (paise)", "Provider", "Provider Ref", "UTR", "Status", "Prepared By", "Approved By", "Completed At"}
	var rows [][]string
	var total int64
	for _, d := range disbursements {
		completed := ""
		if d.CompletedAt != nil {
			completed = d.CompletedAt.Format(time.RFC3339)
		}
		rows = append(rows, []string{
			itoa(d.ID), itoa(d.LoanID), d.CustomerID, paiseStr(d.AmountPaise),
			d.Provider, d.ProviderReference, d.UTR, d.Status, d.PreparedBy, d.ApprovedBy, completed,
		})
		if d.Status == models.DisbursementStatusSuccess {
			total += d.AmountPaise
		}
	}
	return &reportRows{Headers: headers, Rows: rows, Totals: map[string]string{
		"Total Successful Disbursed (paise)": paiseStr(total),
		"Record Count":                       itoaInt(len(rows)),
	}}
}

func buildPaymentRegister(from, to time.Time) *reportRows {
	var payments []models.Payment
	database.DB.Where("created_at BETWEEN ? AND ?", from, to).Order("id").Find(&payments)
	headers := []string{"Payment ID", "Loan ID", "EMI ID", "Customer ID", "Gateway", "Gateway Payment ID", "Amount (paise)", "Status", "Method", "Verified At"}
	var rows [][]string
	var total int64
	for _, p := range payments {
		verified := ""
		if p.VerifiedAt != nil {
			verified = p.VerifiedAt.Format(time.RFC3339)
		}
		rows = append(rows, []string{
			itoa(p.ID), itoa(p.LoanID), itoa(p.EmiID), p.CustomerID, p.Gateway, p.GatewayPaymentID,
			paiseStr(p.AmountPaise), p.Status, p.Method, verified,
		})
		if p.Status == models.PaymentStatusSuccess {
			total += p.AmountPaise
		}
	}
	return &reportRows{Headers: headers, Rows: rows, Totals: map[string]string{
		"Total Collected (paise)": paiseStr(total),
		"Record Count":            itoaInt(len(rows)),
	}}
}

func buildPaymentsByStatus(from, to time.Time, status string) *reportRows {
	var payments []models.Payment
	database.DB.Where("status = ? AND created_at BETWEEN ? AND ?", status, from, to).Order("id").Find(&payments)
	headers := []string{"Payment ID", "Loan ID", "Customer ID", "Amount (paise)", "Failure Reason", "Created At"}
	var rows [][]string
	for _, p := range payments {
		rows = append(rows, []string{itoa(p.ID), itoa(p.LoanID), p.CustomerID, paiseStr(p.AmountPaise), p.FailureReason, p.CreatedAt.Format(time.RFC3339)})
	}
	return &reportRows{Headers: headers, Rows: rows, Totals: map[string]string{"Record Count": itoaInt(len(rows))}}
}

func buildRefundRegister(from, to time.Time) *reportRows {
	var refunds []models.Refund
	database.DB.Where("created_at BETWEEN ? AND ?", from, to).Order("id").Find(&refunds)
	headers := []string{"Refund ID", "Payment ID", "Gateway Refund ID", "Amount (paise)", "Status", "Reason", "Created At"}
	var rows [][]string
	var total int64
	for _, r := range refunds {
		rows = append(rows, []string{
			itoa(r.ID), itoa(r.PaymentID), r.GatewayRefundID, paiseStr(r.AmountPaise), r.Status, r.Reason, r.CreatedAt.Format(time.RFC3339),
		})
		total += r.AmountPaise
	}
	return &reportRows{Headers: headers, Rows: rows, Totals: map[string]string{
		"Total Refunded (paise)": paiseStr(total), "Record Count": itoaInt(len(rows)),
	}}
}

func buildExpenseRegister(from, to time.Time) *reportRows {
	var expenses []models.Expense
	database.DB.Where("expense_date BETWEEN ? AND ?", from, to).Order("id").Find(&expenses)
	headers := []string{"Expense ID", "Date", "Category", "Vendor ID", "Gross (paise)", "Tax (paise)", "TDS (paise)", "Net Payable (paise)", "Status", "Journal Ref"}
	var rows [][]string
	var total int64
	for _, e := range expenses {
		vendorID := ""
		if e.VendorID != nil {
			vendorID = itoa(*e.VendorID)
		}
		rows = append(rows, []string{
			itoa(e.ID), e.ExpenseDate.Format("2006-01-02"), e.Category, vendorID,
			paiseStr(e.GrossAmountPaise), paiseStr(e.TaxAmountPaise), paiseStr(e.TDSAmountPaise), paiseStr(e.NetPayablePaise),
			e.Status, e.JournalReference,
		})
		if e.Status == models.ExpenseStatusPaid {
			total += e.NetPayablePaise
		}
	}
	return &reportRows{Headers: headers, Rows: rows, Totals: map[string]string{
		"Total Paid (paise)": paiseStr(total), "Record Count": itoaInt(len(rows)),
	}}
}

func buildBankTransactionRegister(from, to time.Time) *reportRows {
	var txns []models.BankTransaction
	database.DB.Where("txn_date BETWEEN ? AND ?", from, to).Order("txn_date").Find(&txns)
	headers := []string{"Bank Txn ID", "Bank Account ID", "Txn Date", "Amount (paise)", "Dr/Cr", "Reference", "Status", "Matched Entity Type", "Matched Entity ID"}
	var rows [][]string
	var credits, debits int64
	for _, t := range txns {
		rows = append(rows, []string{
			itoa(t.ID), itoa(t.BankAccountID), t.TxnDate.Format("2006-01-02"), paiseStr(t.AmountPaise),
			t.DrCr, t.Reference, t.Status, t.MatchedEntityType, itoa(t.MatchedEntityID),
		})
		if t.DrCr == "CR" {
			credits += t.AmountPaise
		} else {
			debits += t.AmountPaise
		}
	}
	return &reportRows{Headers: headers, Rows: rows, Totals: map[string]string{
		"Total Credits (paise)": paiseStr(credits), "Total Debits (paise)": paiseStr(debits), "Record Count": itoaInt(len(rows)),
	}}
}

func buildReconciliationExceptionsReport(from, to time.Time) *reportRows {
	var exceptions []models.CrossSystemReconciliation
	database.DB.Where("detected_at BETWEEN ? AND ?", from, to).Order("id").Find(&exceptions)
	headers := []string{"Exception ID", "Entity", "Reference", "Source A", "Source B", "Expected", "Actual", "Status", "Severity", "Detected At"}
	var rows [][]string
	openCount := 0
	for _, e := range exceptions {
		rows = append(rows, []string{
			itoa(e.ID), e.Entity, e.Reference, e.SourceA, e.SourceB, e.ExpectedValue, e.ActualValue,
			e.Status, e.Severity, e.DetectedAt.Format(time.RFC3339),
		})
		if e.Status != models.ReconStatusMatched {
			openCount++
		}
	}
	return &reportRows{Headers: headers, Rows: rows, Totals: map[string]string{
		"Open Exceptions": itoaInt(openCount), "Record Count": itoaInt(len(rows)),
	}}
}

func paiseStr(v int64) string  { return strconv.FormatInt(v, 10) }
func boolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}
