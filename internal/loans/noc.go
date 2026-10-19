package loans

import (
	"bytes"
	"encoding/base64"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jung-kurt/gofpdf"

	"github.com/santosh0921/nbfc-backend/internal/audit"
	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/models"
	"github.com/santosh0921/nbfc-backend/internal/notifications"
)

// DocTypeNOCPending / DocTypeNOCSent are the two states a loan's NOC
// document moves through: generated automatically but held back from the
// customer until an admin reviews and explicitly releases it. Modeled as
// two distinct DocType strings (renamed in place on release) rather than
// a separate status column — the customer-facing document list already
// filters by exact DocType string, so a pending NOC is invisible to the
// customer for free, with no extra filtering logic needed anywhere.
const (
	DocTypeNOCPending = "NOC Letter (Pending Review)"
	DocTypeNOCSent    = "NOC Letter"
)

// CheckAndGenerateNOC runs after every EMI settlement (real webhook and
// demo-mode) to see whether that payment was the loan's last remaining
// installment — if so, it issues a No Objection Certificate in
// DocTypeNOCPending state. It shows up for admins via
// AdminListNOCPendingHandler — the same "polling list" pattern this
// codebase already uses for admin action queues (see
// AdminListPendingApprovalsHandler), rather than the Notification model,
// which has no admin-facing read path (only customer/employee JWTs can
// list notifications — see GET /notifications). AdminSendNOCHandler is
// the release step that makes it visible to the customer. Idempotent:
// skips silently if a pending or already-sent NOC Document already
// exists for this loan, so a retried webhook or a race between two
// settlement paths can never produce duplicates. Best-effort like the
// receipt generator it sits next to — a failure here never rolls back
// the payment that triggered it.
func CheckAndGenerateNOC(loanID uint) error {
	var remaining int64
	if err := database.DB.Model(&models.EmiInstallment{}).
		Where("loan_id = ? AND status != ?", loanID, models.EmiStatusPaid).
		Count(&remaining).Error; err != nil {
		return err
	}
	if remaining > 0 {
		return nil
	}

	var existing int64
	database.DB.Model(&models.Document{}).
		Where("loan_id = ? AND doc_type IN ?", loanID, []string{DocTypeNOCPending, DocTypeNOCSent}).
		Count(&existing)
	if existing > 0 {
		return nil
	}

	var loan models.LoanApplication
	if err := database.DB.First(&loan, loanID).Error; err != nil {
		return err
	}

	return generateNOCLetter(loan)
}

// generateNOCLetter issues the certificate as a system-generated Document
// in DocTypeNOCPending state, same as generateLoanLetter — but an NOC is
// one-directional (the lender certifying closure to the borrower), so
// unlike Sanction/Disbursement Letters it carries only the lender's own
// signatory line and is never routed through the customer eSign flow.
func generateNOCLetter(loan models.LoanApplication) error {
	pdfBytes, err := buildNOCLetterPDF(loan)
	if err != nil {
		return err
	}

	loanID := loan.ID
	doc := models.Document{
		CustomerID:     loan.CustomerID,
		LoanID:         &loanID,
		DocType:        DocTypeNOCPending,
		FileName:       "NOC Letter - Loan " + itoa(loan.ID) + ".pdf",
		MimeType:       "application/pdf",
		DataBase64:     base64.StdEncoding.EncodeToString(pdfBytes),
		UploadedByType: "admin",
		UploadedByRef:  "system",
	}
	return database.DB.Create(&doc).Error
}

// nocCertificateNo is deterministic from the loan ID (not a separate
// sequence) so re-rendering the same loan's certificate — there should
// only ever be one, per CheckAndGenerateNOC's idempotency check — always
// produces the same number.
func nocCertificateNo(loanID uint) string {
	return "NOC-" + itoa(loanID) + "-" + strconv.Itoa(time.Now().Year())
}

func buildNOCLetterPDF(loan models.LoanApplication) ([]byte, error) {
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.AddPage()
	pdf.SetFont("Arial", "B", 16)
	pdf.SetTextColor(20, 40, 90)
	pdf.CellFormat(0, 10, lenderFullName, "", 1, "C", false, 0, "")
	pdf.SetFont("Arial", "B", 13)
	pdf.SetTextColor(0, 0, 0)
	pdf.CellFormat(0, 8, "LOAN CLOSURE / NO DUES CERTIFICATE", "", 1, "C", false, 0, "")
	pdf.Ln(4)

	var lastPaidAt time.Time
	var lastInstallment models.EmiInstallment
	if err := database.DB.Where("loan_id = ? AND status = ?", loan.ID, models.EmiStatusPaid).
		Order("installment_number desc").First(&lastInstallment).Error; err == nil && lastInstallment.PaidAt != nil {
		lastPaidAt = *lastInstallment.PaidAt
	} else {
		lastPaidAt = time.Now()
	}
	closureDate := lastPaidAt.Format("02-Jan-2006")
	loanAccountNo := "NBFC-APP-" + itoa(loan.ID)

	tableRow(pdf, "NOC / Certificate No.", nocCertificateNo(loan.ID), 55)
	tableRow(pdf, "Date", closureDate, 55)
	tableRow(pdf, "Loan Account No.", loanAccountNo, 55)
	tableRow(pdf, "Loan Type / Product", loan.Category, 55)
	pdf.Ln(4)

	pdf.SetFont("Arial", "B", 11)
	pdf.CellFormat(0, 8, "TO WHOMSOEVER IT MAY CONCERN", "", 1, "C", false, 0, "")
	pdf.Ln(2)

	address := loan.AddressLine
	if loan.City != "" {
		if address != "" {
			address += ", "
		}
		address += loan.City
	}

	pdf.SetFont("Arial", "", 10.5)
	pdf.MultiCell(0, 6, "This is to certify that Mr./Ms./M/s "+loan.ApplicantName+", having address at "+
		address+", had availed a "+loan.Category+" Loan from "+lenderFullName+" under Loan Account No. "+
		loanAccountNo+" for an original sanctioned amount of "+rupees(loan.DisbursedAmount)+
		" (Rupees "+amountInWords(loan.DisbursedAmount)+" only).", "", "L", false)
	pdf.Ln(2)

	pdf.MultiCell(0, 6, "We hereby confirm that, as per the books and records of the Company, the borrower has "+
		"fully repaid and discharged all amounts due and payable in respect of the above-mentioned loan account, "+
		"including principal, interest and applicable charges, subject to realization of all payments received.", "", "L", false)
	pdf.Ln(3)

	numberedPoint(pdf, "1.", "The above Loan Account stands closed with effect from "+closureDate+".")
	numberedPoint(pdf, "2.", "As on the date of this certificate, no amount is outstanding or payable by the "+
		"borrower in respect of the said Loan Account.")
	numberedPoint(pdf, "3.", lenderFullName+" has no objection to closure of the aforesaid loan facility.")
	numberedPoint(pdf, "4.", "Any lien, hypothecation, charge or other security created specifically in favour of "+
		"the Company for the said loan shall be released / satisfied in accordance with the loan documents and "+
		"applicable legal and regulatory requirements.")
	numberedPoint(pdf, "5.", "Where original title deeds, security documents or other original documents are held "+
		"by the Company against the said loan, the same shall be released / returned to the person entitled "+
		"thereto against proper acknowledgement and completion of applicable formalities.")
	numberedPoint(pdf, "6.", "Where applicable, the closure status of the Loan Account shall be reported / updated "+
		"with the relevant Credit Information Companies in accordance with applicable regulatory requirements.")

	pdf.Ln(2)
	pdf.MultiCell(0, 6, "This certificate is issued at the request of the borrower for record and reference "+
		"purposes and shall be read together with the terms of the applicable loan and security documents.", "", "L", false)

	pdf.Ln(10)
	pdf.SetFont("Arial", "", 11)
	pdf.CellFormat(0, 6, "For "+lenderFullName, "", 1, "L", false, 0, "")
	pdf.Ln(14)
	pdf.CellFormat(0, 6, "Authorized Signatory: ____________________", "", 1, "L", false, 0, "")
	pdf.CellFormat(0, 6, "Name: ____________________", "", 1, "L", false, 0, "")
	pdf.CellFormat(0, 6, "Designation: ____________________", "", 1, "L", false, 0, "")
	pdf.CellFormat(0, 6, "Contact No.: ____________________", "", 1, "L", false, 0, "")
	pdf.CellFormat(0, 6, "Company Seal", "", 1, "L", false, 0, "")

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func numberedPoint(pdf *gofpdf.Fpdf, num, text string) {
	pdf.SetFont("Arial", "B", 10.5)
	pdf.CellFormat(8, 6, num, "", 0, "L", false, 0, "")
	pdf.SetFont("Arial", "", 10.5)
	pdf.SetX(pdf.GetX())
	pdf.MultiCell(0, 6, text, "", "L", false)
	pdf.Ln(1)
}

var amountWordsOnes = []string{"", "One", "Two", "Three", "Four", "Five", "Six", "Seven", "Eight", "Nine",
	"Ten", "Eleven", "Twelve", "Thirteen", "Fourteen", "Fifteen", "Sixteen", "Seventeen", "Eighteen", "Nineteen"}
var amountWordsTens = []string{"", "", "Twenty", "Thirty", "Forty", "Fifty", "Sixty", "Seventy", "Eighty", "Ninety"}

// amountInWords renders the rupee (integer) portion of v in the Indian
// numbering system (lakh/crore, not thousand/million) — the standard
// convention for this figure on an Indian legal/financial document.
func amountInWords(v float64) string {
	n := int64(v)
	if n == 0 {
		return "Zero"
	}
	var parts []string
	crore := n / 10000000
	n %= 10000000
	lakh := n / 100000
	n %= 100000
	thousand := n / 1000
	n %= 1000
	hundred := n / 100
	n %= 100

	if crore > 0 {
		parts = append(parts, twoDigitWords(crore)+" Crore")
	}
	if lakh > 0 {
		parts = append(parts, twoDigitWords(lakh)+" Lakh")
	}
	if thousand > 0 {
		parts = append(parts, twoDigitWords(thousand)+" Thousand")
	}
	if hundred > 0 {
		parts = append(parts, amountWordsOnes[hundred]+" Hundred")
	}
	if n > 0 {
		parts = append(parts, twoDigitWords(n))
	}

	out := parts[0]
	for _, p := range parts[1:] {
		out += " " + p
	}
	return out
}

func twoDigitWords(n int64) string {
	if n < 20 {
		return amountWordsOnes[n]
	}
	tens := amountWordsTens[n/10]
	ones := n % 10
	if ones == 0 {
		return tens
	}
	return tens + " " + amountWordsOnes[ones]
}

// AdminListNOCPendingHandler handles GET /admin/loans/noc-pending — every
// loan whose NOC certificate has been generated but not yet reviewed and
// released to the customer.
func AdminListNOCPendingHandler(c *gin.Context) {
	var docs []models.Document
	database.DB.Where("doc_type = ?", DocTypeNOCPending).Order("created_at asc").Find(&docs)

	type pendingNOC struct {
		LoanID          uint      `json:"loanId"`
		ApplicantName   string    `json:"applicantName"`
		DocumentID      string    `json:"documentId"`
		GeneratedAt     time.Time `json:"generatedAt"`
		DisbursedAmount float64   `json:"disbursedAmount"`
	}
	out := make([]pendingNOC, 0, len(docs))
	for _, d := range docs {
		if d.LoanID == nil {
			continue
		}
		var loan models.LoanApplication
		if err := database.DB.First(&loan, *d.LoanID).Error; err != nil {
			continue
		}
		out = append(out, pendingNOC{
			LoanID:          loan.ID,
			ApplicantName:   loan.ApplicantName,
			DocumentID:      d.ID.String(),
			GeneratedAt:     d.CreatedAt,
			DisbursedAmount: loan.DisbursedAmount,
		})
	}
	c.JSON(http.StatusOK, out)
}

// AdminSendNOCHandler handles POST /admin/loans/:id/noc/send — the
// release step: renames the pending NOC Document to DocTypeNOCSent (which
// is the only state the customer's own document list actually matches),
// and notifies the customer. This is the only path that ever makes an
// NOC visible to the customer — CheckAndGenerateNOC only ever creates it
// in the pending state.
func AdminSendNOCHandler(c *gin.Context) {
	adminID, adminUsername, adminRole := currentAdmin(c)

	loanID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid loan id"})
		return
	}

	var doc models.Document
	if err := database.DB.Where("loan_id = ? AND doc_type = ?", loanID, DocTypeNOCPending).First(&doc).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"message": "No pending NOC found for this loan"})
		return
	}

	doc.DocType = DocTypeNOCSent
	if err := database.DB.Save(&doc).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Failed to release NOC"})
		return
	}

	var loan models.LoanApplication
	database.DB.First(&loan, loanID)
	notifications.Create("customer", doc.CustomerID.String(), "Loan Closure Certificate ready",
		"Your No Objection Certificate for loan #"+itoa(loan.ID)+" is now available — view it under Letters.")

	audit.RecordDetailed(c, audit.Entry{
		ActorType: "admin", ActorID: adminID, ActorRef: adminUsername, ActorRole: adminRole,
		Action: "loan.noc.send", TargetType: "loan", TargetID: itoa(loan.ID),
		Result: audit.ResultSuccess,
	})

	c.JSON(http.StatusOK, gin.H{"message": "NOC sent to customer"})
}
