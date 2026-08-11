package loans

import (
	"bytes"
	"encoding/base64"
	"math"
	"strconv"
	"time"

	"github.com/jung-kurt/gofpdf"

	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/models"
)

const (
	lenderName        = "Jayashri Capital"
	lenderFullName    = "Jayashri Capital Private Limited"
	lenderOfficeLine  = "Registered Office: Jayashri Capital, Mumbai, Maharashtra, India"
	grievanceName     = "Customer Support"
	grievancePhone    = "+91 8657823643"
	grievanceEmailStr = "support@jayashricapital.com"
)

// generateLoanLetter builds the initial (unsigned) Jayashri Capital-branded
// letterhead PDF for the given loan and stores it as a system-generated
// Document row, so it shows up in GET /admin/loans/:id/documents alongside
// customer/employee uploads. docType is exactly "Sanction Letter" or
// "Disbursement Letter" so the Flutter apps can find it by name.
func generateLoanLetter(loan models.LoanApplication, docType string) error {
	pdfBytes, err := buildLetterPDF(loan, docType, nil, "")
	if err != nil {
		return err
	}

	loanID := loan.ID
	doc := models.Document{
		CustomerID:     loan.CustomerID,
		LoanID:         &loanID,
		DocType:        docType,
		FileName:       docType + " - Loan " + itoa(loan.ID) + ".pdf",
		MimeType:       "application/pdf",
		DataBase64:     base64.StdEncoding.EncodeToString(pdfBytes),
		UploadedByType: "admin",
		UploadedByRef:  "system",
	}
	return database.DB.Create(&doc).Error
}

// buildLetterPDF renders the letter. When signaturePNG is non-nil, the
// borrower's signature image is embedded directly into the document at the
// signature line ("eSigned in the letter itself"), and signedAtLabel (a
// pre-formatted timestamp) is printed underneath as an audit trail.
func buildLetterPDF(loan models.LoanApplication, docType string, signaturePNG []byte, signedAtLabel string) ([]byte, error) {
	if docType == "Disbursement Letter" {
		return buildDisbursementLetterPDF(loan, signaturePNG, signedAtLabel)
	}
	return buildSanctionLetterPDF(loan, signaturePNG, signedAtLabel)
}

func addLetterhead(pdf *gofpdf.Fpdf, title string) {
	pdf.AddPage()
	pdf.SetFont("Arial", "B", 18)
	pdf.SetTextColor(20, 40, 90)
	pdf.CellFormat(0, 10, lenderName, "", 1, "C", false, 0, "")
	pdf.SetFont("Arial", "", 9)
	pdf.SetTextColor(90, 90, 90)
	pdf.CellFormat(0, 5, lenderOfficeLine, "", 1, "C", false, 0, "")
	pdf.CellFormat(0, 5, "RBI-Registered Non-Banking Financial Company (Demo)", "", 1, "C", false, 0, "")
	pdf.Ln(3)
	pdf.SetDrawColor(20, 40, 90)
	pdf.SetLineWidth(0.6)
	y := pdf.GetY()
	pdf.Line(15, y, 195, y)
	pdf.Ln(7)

	pdf.SetTextColor(0, 0, 0)
	pdf.SetFont("Arial", "B", 14)
	pdf.CellFormat(0, 8, title, "", 1, "C", false, 0, "")
	pdf.Ln(1)
	pdf.SetFont("Arial", "", 10)
	pdf.SetTextColor(90, 90, 90)
	pdf.CellFormat(0, 6, "Date: "+time.Now().Format("02-Jan-2006"), "", 1, "C", false, 0, "")
	pdf.SetTextColor(0, 0, 0)
	pdf.Ln(4)
}

func tableRow(pdf *gofpdf.Fpdf, label, value string, labelW float64) {
	pdf.SetFont("Arial", "B", 10)
	pdf.CellFormat(labelW, 8, label, "1", 0, "L", false, 0, "")
	pdf.SetFont("Arial", "", 10)
	pdf.MultiCell(0, 8, value, "1", "L", false)
}

func sectionHeading(pdf *gofpdf.Fpdf, text string) {
	pdf.Ln(3)
	pdf.SetFont("Arial", "B", 12)
	pdf.SetTextColor(20, 40, 90)
	pdf.CellFormat(0, 8, text, "", 1, "L", false, 0, "")
	pdf.SetTextColor(0, 0, 0)
	pdf.Ln(1)
}

func calcMonthlyEmi(principal, annualRatePercent float64, tenureMonths int) float64 {
	if tenureMonths <= 0 {
		return 0
	}
	if annualRatePercent <= 0 {
		return principal / float64(tenureMonths)
	}
	r := annualRatePercent / 12 / 100
	factor := math.Pow(1+r, float64(tenureMonths))
	return principal * r * factor / (factor - 1)
}

func rupees(v float64) string {
	return "Rs. " + strconv.FormatFloat(v, 'f', 2, 64)
}

// signatureBlock draws the "Authorized Signatory / Borrower's Signature"
// footer. When signaturePNG is provided, the actual signature image is
// embedded above the borrower's line and a "digitally signed" audit
// caption is printed underneath — this makes the letter itself carry the
// eSign, rather than requiring a separately uploaded signed copy.
func signatureBlock(pdf *gofpdf.Fpdf, loan models.LoanApplication, signaturePNG []byte, signedAtLabel string) {
	pdf.Ln(10)

	// The image, labels, and caption must all land on the same page — draw
	// them as one unit, forcing a page break first if the block wouldn't
	// otherwise fit, rather than letting gofpdf auto-break mid-block (which
	// would strand the signature image on the previous page).
	const blockHeight = 34.0
	_, pageHeight := pdf.GetPageSize()
	_, _, _, bottomMargin := pdf.GetMargins()
	if pdf.GetY()+blockHeight > pageHeight-bottomMargin {
		pdf.AddPage()
	}

	sigTop := pdf.GetY()

	if signaturePNG != nil {
		pdf.RegisterImageOptionsReader("borrower-signature", gofpdf.ImageOptions{ImageType: "PNG"}, bytes.NewReader(signaturePNG))
		pdf.ImageOptions("borrower-signature", 115, sigTop-2, 55, 18, false, gofpdf.ImageOptions{ImageType: "PNG"}, 0, "")
	}

	pdf.SetY(sigTop + 16)
	pdf.SetFont("Arial", "", 11)
	pdf.CellFormat(95, 6, "Authorized Signatory", "", 0, "L", false, 0, "")
	pdf.CellFormat(0, 6, "Borrower's Signature", "", 1, "L", false, 0, "")

	if signaturePNG != nil {
		pdf.SetFont("Arial", "I", 8)
		pdf.SetTextColor(20, 130, 60)
		pdf.CellFormat(95, 5, "", "", 0, "L", false, 0, "")
		pdf.CellFormat(0, 5, "eSigned by "+loan.ApplicantName+" on "+signedAtLabel, "", 1, "L", false, 0, "")
		pdf.SetTextColor(0, 0, 0)
	}
}

// buildSanctionLetterPDF follows a Key-Fact-Statement style layout: a
// summary table of loan terms/charges, then borrower acknowledgement and
// grievance-contact sections, closing with a sign-and-submit request.
func buildSanctionLetterPDF(loan models.LoanApplication, signaturePNG []byte, signedAtLabel string) ([]byte, error) {
	pdf := gofpdf.New("P", "mm", "A4", "")
	addLetterhead(pdf, "Sanction Letter")

	pdf.SetFont("Arial", "", 11)
	pdf.MultiCell(0, 6, "Dear "+loan.ApplicantName+",", "", "L", false)
	pdf.Ln(2)
	pdf.MultiCell(0, 6, "We are pleased to inform you that your "+loan.Category+" application "+
		"(Loan ID: #"+itoa(loan.ID)+") has been sanctioned as per the Key Fact Statement below.", "", "L", false)

	rate := 0.0
	if loan.InterestRatePercent != nil {
		rate = *loan.InterestRatePercent
	}
	tenure := 0
	if loan.TenureMonths != nil {
		tenure = *loan.TenureMonths
	}
	monthlyEmi := calcMonthlyEmi(loan.AmountRequested, rate, tenure)
	totalRepayment := monthlyEmi * float64(tenure)

	sectionHeading(pdf, "Key Fact Statement")
	tableRow(pdf, "Loan ID / Type", "#"+itoa(loan.ID)+" - "+loan.Category, 65)
	tableRow(pdf, "Sanctioned Amount", rupees(loan.AmountRequested), 65)
	tableRow(pdf, "Loan Tenure", strconv.Itoa(tenure)+" months", 65)
	tableRow(pdf, "Interest Rate (Annual, Fixed)", strconv.FormatFloat(rate, 'f', 2, 64)+"% per annum", 65)
	tableRow(pdf, "Estimated Monthly EMI", rupees(monthlyEmi), 65)
	tableRow(pdf, "Estimated Total Repayment", rupees(totalRepayment)+" (principal + interest over full tenure)", 65)
	tableRow(pdf, "CIBIL Score Considered", cibilLabel(loan.CibilScore), 65)
	tableRow(pdf, "Overdue Charges (Late Payment)", "2% per month on overdue installment amount", 65)
	tableRow(pdf, "Foreclosure Charges", "Nil", 65)

	sectionHeading(pdf, "Borrower Acknowledgement")
	pdf.SetFont("Arial", "", 10)
	pdf.MultiCell(0, 6, "By signing and submitting this letter, the Borrower confirms having read, understood, and accepted "+
		"the loan amount, interest rate, tenure, EMI schedule, and applicable charges disclosed above, and consents to "+
		lenderFullName+" sharing loan and repayment information with credit information companies (including CIBIL) as permitted by law.", "", "L", false)

	sectionHeading(pdf, "Grievance Redressal")
	tableRow(pdf, "Contact Person", grievanceName, 50)
	tableRow(pdf, "Phone", grievancePhone, 50)
	tableRow(pdf, "Email", grievanceEmailStr, 50)

	pdf.Ln(6)
	pdf.SetFont("Arial", "B", 11)
	if signaturePNG != nil {
		pdf.MultiCell(0, 6, "This letter has been electronically signed by the Borrower to confirm acceptance and complete loan formalities.", "", "L", false)
	} else {
		pdf.MultiCell(0, 6, "Please sign and submit this letter to confirm acceptance and complete your loan formalities.", "", "L", false)
	}
	pdf.SetFont("Arial", "", 11)
	pdf.CellFormat(0, 6, "For "+lenderFullName, "", 1, "L", false, 0, "")

	signatureBlock(pdf, loan, signaturePNG, signedAtLabel)

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// buildDisbursementLetterPDF mirrors a disbursement-confirmation notice:
// a short thank-you note, then a loan-terms summary table drawn from the
// actual generated EMI schedule where available.
func buildDisbursementLetterPDF(loan models.LoanApplication, signaturePNG []byte, signedAtLabel string) ([]byte, error) {
	pdf := gofpdf.New("P", "mm", "A4", "")
	addLetterhead(pdf, "Disbursement Letter")

	pdf.SetFont("Arial", "", 11)
	pdf.MultiCell(0, 6, "Dear "+loan.ApplicantName+",", "", "L", false)
	pdf.Ln(2)
	pdf.MultiCell(0, 6, "Thank you for choosing "+lenderName+". We are pleased to confirm that your "+
		loan.Category+" (Loan ID: #"+itoa(loan.ID)+") has been disbursed. Please review the loan terms and "+
		"repayment schedule below.", "", "L", false)

	rate := 0.0
	if loan.InterestRatePercent != nil {
		rate = *loan.InterestRatePercent
	}
	tenure := 0
	if loan.TenureMonths != nil {
		tenure = *loan.TenureMonths
	}

	var count int64
	var firstDue time.Time
	var emiAmount float64
	database.DB.Model(&models.EmiInstallment{}).Where("loan_id = ?", loan.ID).Count(&count)
	var firstInstallment models.EmiInstallment
	if err := database.DB.Where("loan_id = ?", loan.ID).Order("installment_number asc").First(&firstInstallment).Error; err == nil {
		firstDue = firstInstallment.DueDate
		emiAmount = firstInstallment.Amount
	}
	var totalRepayment float64
	database.DB.Model(&models.EmiInstallment{}).Where("loan_id = ?", loan.ID).Select("COALESCE(SUM(amount),0)").Scan(&totalRepayment)

	sectionHeading(pdf, "Loan Terms")
	tableRow(pdf, "Name", loan.ApplicantName, 65)
	tableRow(pdf, "Loan ID", "#"+itoa(loan.ID), 65)
	tableRow(pdf, "Disbursed Amount", rupees(loan.DisbursedAmount), 65)
	tableRow(pdf, "Rate of Interest", strconv.FormatFloat(rate, 'f', 2, 64)+"% per annum", 65)
	tableRow(pdf, "Tenure", strconv.Itoa(tenure)+" months ("+strconv.FormatInt(count, 10)+" EMIs)", 65)
	if !firstDue.IsZero() {
		tableRow(pdf, "First EMI Due Date", firstDue.Format("02-Jan-2006"), 65)
	}
	if emiAmount > 0 {
		tableRow(pdf, "Monthly EMI Amount", rupees(emiAmount), 65)
	}
	if totalRepayment > 0 {
		tableRow(pdf, "Total Repayment Amount", rupees(totalRepayment), 65)
	}
	tableRow(pdf, "Overdue Charges (Late Payment)", "2% per month on overdue installment amount", 65)

	sectionHeading(pdf, "Important Note")
	pdf.SetFont("Arial", "", 10)
	pdf.MultiCell(0, 6, "Non-payment of EMIs on time will affect your credit score and your eligibility for future loans. "+
		"Please refer to the EMI Schedule section of the app for the full installment-wise breakdown.", "", "L", false)

	sectionHeading(pdf, "Grievance Redressal")
	tableRow(pdf, "Contact Person", grievanceName, 50)
	tableRow(pdf, "Phone", grievancePhone, 50)
	tableRow(pdf, "Email", grievanceEmailStr, 50)

	pdf.Ln(6)
	pdf.SetFont("Arial", "B", 11)
	if signaturePNG != nil {
		pdf.MultiCell(0, 6, "This letter has been electronically signed by the Borrower to acknowledge receipt of the disbursed loan amount and confirm the above terms.", "", "L", false)
	} else {
		pdf.MultiCell(0, 6, "Please sign and submit this letter to acknowledge receipt of the disbursed loan amount and confirm the above terms.", "", "L", false)
	}

	signatureBlock(pdf, loan, signaturePNG, signedAtLabel)

	pdf.Ln(4)
	pdf.SetFont("Arial", "", 11)
	pdf.CellFormat(0, 6, "Best Regards,", "", 1, "L", false, 0, "")
	pdf.CellFormat(0, 6, "Team "+lenderName, "", 1, "L", false, 0, "")

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func cibilLabel(score int) string {
	if score <= 0 {
		return "Not Available"
	}
	return strconv.Itoa(score)
}
