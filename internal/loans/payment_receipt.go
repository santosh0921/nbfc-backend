package loans

import (
	"bytes"
	"encoding/base64"
	"strconv"
	"time"

	"github.com/jung-kurt/gofpdf"

	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/models"
)

// GenerateEmiPaymentReceipt builds and stores a branded PDF receipt for one
// successfully-settled EMI installment, so a customer who pays gets an
// actual document, not just a status flip. Every figure printed comes
// straight from the loan/installment rows this payment already updated
// (installment.Amount/PrincipalComponent/InterestComponent, the loan's own
// outstanding balance after this payment) — nothing here is a separate
// re-derivation that could drift from what was actually charged.
//
// Deliberately carries NO GST line: GST applies to the NBFC's processing
// fee (a taxable service charge, see internal/loans/gst.go), not to loan
// repayment. Principal and interest are not a "supply" under Indian GST
// law, so an EMI receipt that invented a tax line here would be printing
// an incorrect statutory document, not a more complete one.
func GenerateEmiPaymentReceipt(loan models.LoanApplication, inst models.EmiInstallment, gateway string) error {
	pdfBytes, err := buildPaymentReceiptPDF(loan, inst, gateway)
	if err != nil {
		return err
	}

	loanID := loan.ID
	doc := models.Document{
		CustomerID:     loan.CustomerID,
		LoanID:         &loanID,
		DocType:        "Payment Receipt",
		FileName:       "Payment Receipt - EMI " + strconv.Itoa(inst.InstallmentNumber) + " - Loan " + itoa(loan.ID) + ".pdf",
		MimeType:       "application/pdf",
		DataBase64:     base64.StdEncoding.EncodeToString(pdfBytes),
		UploadedByType: "admin",
		UploadedByRef:  "system",
	}
	return database.DB.Create(&doc).Error
}

func buildPaymentReceiptPDF(loan models.LoanApplication, inst models.EmiInstallment, gateway string) ([]byte, error) {
	pdf := gofpdf.New("P", "mm", "A4", "")
	addLetterhead(pdf, "Payment Receipt")

	pdf.SetFont("Arial", "", 11)
	pdf.MultiCell(0, 6, "Dear "+loan.ApplicantName+",", "", "L", false)
	pdf.Ln(2)
	pdf.MultiCell(0, 6, "This confirms receipt of your EMI payment for "+loan.Category+
		" (Loan ID: #"+itoa(loan.ID)+"). Details of the payment are below.", "", "L", false)

	paidAt := time.Now()
	if inst.PaidAt != nil {
		paidAt = *inst.PaidAt
	}

	var outstanding float64
	database.DB.Model(&models.EmiInstallment{}).
		Where("loan_id = ? AND status != ?", loan.ID, models.EmiStatusPaid).
		Select("COALESCE(SUM(amount),0)").Scan(&outstanding)

	var remainingCount int64
	database.DB.Model(&models.EmiInstallment{}).
		Where("loan_id = ? AND status != ?", loan.ID, models.EmiStatusPaid).
		Count(&remainingCount)

	sectionHeading(pdf, "Payment Details")
	tableRow(pdf, "Loan ID", "#"+itoa(loan.ID)+" - "+loan.Category, 65)
	tableRow(pdf, "Installment Number", strconv.Itoa(inst.InstallmentNumber), 65)
	tableRow(pdf, "Payment Date", paidAt.Format("02-Jan-2006 15:04"), 65)
	if inst.PaymentReference != "" {
		tableRow(pdf, "Payment Reference", inst.PaymentReference, 65)
	}
	if inst.PaymentMethod != "" {
		tableRow(pdf, "Payment Method", inst.PaymentMethod, 65)
	}
	if gateway != "" {
		tableRow(pdf, "Payment Gateway", gateway, 65)
	}

	sectionHeading(pdf, "Amount Breakdown")
	tableRow(pdf, "Principal Component", rupees(inst.PrincipalComponent), 65)
	tableRow(pdf, "Interest Component", rupees(inst.InterestComponent), 65)
	tableRow(pdf, "Total Amount Paid", rupees(inst.Amount), 65)

	sectionHeading(pdf, "Loan Status After This Payment")
	tableRow(pdf, "Remaining Installments", strconv.FormatInt(remainingCount, 10), 65)
	tableRow(pdf, "Outstanding Amount", rupees(outstanding), 65)

	sectionHeading(pdf, "Important Note")
	pdf.SetFont("Arial", "", 10)
	pdf.MultiCell(0, 6, "This receipt confirms the amount above has been credited towards your loan. "+
		"GST is not applicable on loan principal or interest repayment. Please refer to the EMI Schedule "+
		"section of the app for your full installment-wise repayment status.", "", "L", false)

	sectionHeading(pdf, "Grievance Redressal")
	tableRow(pdf, "Contact Person", grievanceName, 50)
	tableRow(pdf, "Phone", grievancePhone, 50)
	tableRow(pdf, "Email", grievanceEmailStr, 50)

	pdf.Ln(8)
	pdf.SetFont("Arial", "", 11)
	pdf.CellFormat(0, 6, "Thank you for your payment.", "", 1, "L", false, 0, "")
	pdf.CellFormat(0, 6, "Best Regards,", "", 1, "L", false, 0, "")
	pdf.CellFormat(0, 6, "Team "+lenderName, "", 1, "L", false, 0, "")

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
