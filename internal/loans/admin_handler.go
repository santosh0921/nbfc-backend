package loans

import (
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/santosh0921/nbfc-backend/internal/audit"
	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/models"
)

// AdminListLoansHandler handles GET /admin/loans
func AdminListLoansHandler(c *gin.Context) {
	category := c.Query("category")
	status := c.Query("status")
	q := database.DB.Model(&models.LoanApplication{})
	if category != "" {
		q = q.Where("category = ?", category)
	}
	if status != "" {
		q = q.Where("status = ?", status)
	}
	var loans []models.LoanApplication
	q.Order("created_at desc").Find(&loans)
	c.JSON(http.StatusOK, loans)
}

// AdminGetLoanHandler handles GET /admin/loans/:id
func AdminGetLoanHandler(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var loan models.LoanApplication
	if err := database.DB.First(&loan, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"message": "Not found"})
		return
	}
	c.JSON(http.StatusOK, withReferences(loan))
}

// AdminReassignLoanHandler handles POST /admin/loans/:id/reassign — moves
// a loan's verification assignment to a different (or first-time)
// employee. Exists mainly to recover a loan that was auto-assigned (see
// findVerificationEmployee, internal/loans/service.go) to an employee who
// turned out to be unusable for it — most notably a self-registered
// employee still awaiting approval, whose loans were invisible to
// everyone since that employee couldn't log in to act on them. Only
// allowed while the loan hasn't already moved past verification.
func AdminReassignLoanHandler(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var body struct {
		EmployeeID uint `json:"employeeId" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}

	var loan models.LoanApplication
	if err := database.DB.First(&loan, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"message": "Loan not found"})
		return
	}
	if loan.Status != models.LoanStatusSubmitted && loan.Status != models.LoanStatusAssigned {
		c.JSON(http.StatusConflict, gin.H{"message": "Loan has already moved past verification and can no longer be reassigned"})
		return
	}

	var emp models.Employee
	if err := database.DB.First(&emp, body.EmployeeID).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Employee not found"})
		return
	}
	if !emp.Active || !emp.Approved {
		c.JSON(http.StatusBadRequest, gin.H{"message": "That employee is not active/approved and cannot take this assignment"})
		return
	}

	loan.AssignedEmployeeID = &emp.ID
	loan.Status = models.LoanStatusAssigned
	database.DB.Save(&loan)
	createNotification("employee", itoa(emp.ID), "New verification task",
		"Loan application #"+itoa(loan.ID)+" ("+loan.Category+") assigned to you for verification.")
	audit.Record(c.GetString("admin_username"), "loan.reassign", "loan", strconv.Itoa(int(loan.ID)),
		"Reassigned loan #"+itoa(loan.ID)+" to employee "+emp.Code+" ("+emp.Name+")")

	c.JSON(http.StatusOK, withReferences(loan))
}

// AdminDecisionHandler handles POST /admin/loans/:id/decision
func AdminDecisionHandler(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var loan models.LoanApplication
	if err := database.DB.First(&loan, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"message": "Not found"})
		return
	}
	var body struct {
		Action              string   `json:"action" binding:"required"` // sanction | reject
		Remarks             string   `json:"remarks"`
		TenureMonths        *int     `json:"tenureMonths"`
		InterestRatePercent *float64 `json:"interestRatePercent"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	var cibilRateNote string
	switch body.Action {
	case "sanction":
		if body.TenureMonths == nil || *body.TenureMonths <= 0 || body.InterestRatePercent == nil || *body.InterestRatePercent <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"message": "tenureMonths and interestRatePercent are required and must be positive when sanctioning"})
			return
		}

		// CIBIL-driven decision rules. Tenure stays fully admin-controlled;
		// only the interest rate (and, below 650, the ability to sanction
		// at all) is rule-driven.
		if loan.CibilScore > 0 && loan.CibilScore < models.CibilRejectThreshold {
			c.JSON(http.StatusBadRequest, gin.H{
				"message": "Cannot sanction: applicant's CIBIL score (" + strconv.Itoa(loan.CibilScore) +
					") is below the minimum threshold of " + strconv.Itoa(models.CibilRejectThreshold) +
					". This application must be rejected.",
			})
			return
		}

		effectiveRate := *body.InterestRatePercent
		switch {
		case loan.CibilScore > models.CibilPrimeThreshold:
			effectiveRate = models.CibilMinRatePercent
			cibilRateNote = "CIBIL " + strconv.Itoa(loan.CibilScore) + " > " + strconv.Itoa(models.CibilPrimeThreshold) +
				" — rate forced to minimum " + strconv.FormatFloat(models.CibilMinRatePercent, 'f', 2, 64) + "%"
		case loan.CibilScore >= models.CibilRejectThreshold:
			effectiveRate = models.CibilMidRatePercent
			cibilRateNote = "CIBIL " + strconv.Itoa(loan.CibilScore) + " in [" + strconv.Itoa(models.CibilRejectThreshold) +
				"-" + strconv.Itoa(models.CibilPrimeThreshold) + "] — rate forced to " +
				strconv.FormatFloat(models.CibilMidRatePercent, 'f', 2, 64) + "%"
		default:
			cibilRateNote = "CIBIL score not on file — admin-submitted rate used as-is"
		}

		loan.TenureMonths = body.TenureMonths
		loan.InterestRatePercent = &effectiveRate
		loan.Status = models.LoanStatusSanctioned
	case "reject":
		loan.Status = models.LoanStatusRejected
	default:
		c.JSON(http.StatusBadRequest, gin.H{"message": "action must be sanction or reject"})
		return
	}
	now := time.Now()
	loan.DecisionRemarks = body.Remarks
	loan.DecidedBy = c.GetString("admin_username")
	loan.DecidedAt = &now
	database.DB.Save(&loan)

	title := "Loan " + loan.Status
	notifBody := "Your " + loan.Category + " application #" + itoa(loan.ID) + " has been " + loan.Status + ". Remarks: " + body.Remarks
	if body.Action == "sanction" {
		notifBody += " Please review, sign, and re-upload your Sanction Letter."
	}
	createNotification("customer", loan.CustomerID.String(), title, notifBody)

	switch body.Action {
	case "sanction":
		details := "Sanctioned loan #" + itoa(loan.ID) + " — tenure " + strconv.Itoa(*loan.TenureMonths) + "mo @ " +
			strconv.FormatFloat(*loan.InterestRatePercent, 'f', 2, 64) + "%. " + cibilRateNote + ". Remarks: " + body.Remarks
		audit.Record(loan.DecidedBy, "loan.sanction", "loan", itoa(loan.ID), details)

		if err := generateLoanLetter(loan, "Sanction Letter"); err != nil {
			log.Println("failed to generate sanction letter:", err)
		}
	case "reject":
		audit.Record(loan.DecidedBy, "loan.reject", "loan", itoa(loan.ID), "Rejected: "+body.Remarks)
	}

	c.JSON(http.StatusOK, withReferences(loan))
}

// AdminDisburseHandler handles POST /admin/loans/:id/disburse
func AdminDisburseHandler(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var loan models.LoanApplication
	if err := database.DB.First(&loan, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"message": "Not found"})
		return
	}
	if loan.Status != models.LoanStatusSanctioned {
		c.JSON(http.StatusBadRequest, gin.H{"message": "loan must be sanctioned before disbursement"})
		return
	}
	var body struct {
		Amount float64 `json:"amount"`
	}
	c.ShouldBindJSON(&body)
	amt := body.Amount
	if amt == 0 {
		amt = loan.AmountRequested
	}
	if loan.TenureMonths == nil || *loan.TenureMonths <= 0 || loan.InterestRatePercent == nil || *loan.InterestRatePercent <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"message": "loan is missing tenureMonths/interestRatePercent from sanction — cannot generate EMI schedule"})
		return
	}

	now := time.Now()
	loan.Status = models.LoanStatusDisbursed
	loan.DisbursedAmount = amt
	loan.DisbursedAt = &now
	database.DB.Save(&loan)

	if err := generateEmiSchedule(&loan); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "failed to generate EMI schedule: " + err.Error()})
		return
	}

	notifyDisbursement(loan, amt, now)

	audit.Record(c.GetString("admin_username"), "loan.disburse", "loan", itoa(loan.ID),
		"Disbursed Rs."+strconv.FormatFloat(amt, 'f', 2, 64)+" on "+now.Format("2006-01-02"))

	if err := generateLoanLetter(loan, "Disbursement Letter"); err != nil {
		log.Println("failed to generate disbursement letter:", err)
	}

	c.JSON(http.StatusOK, withReferences(loan))
}

// letterNotificationMergeWindow: how recently the sanction notification for
// this loan must have been created for the disbursement notification to
// merge into it instead of creating a second entry. Sanction and
// disbursement are genuinely separate real-world events (disbursement
// often happens days after sanction, once the signed sanction letter comes
// back), so they should normally each get their own bell entry — this only
// collapses the two when they happen in quick succession (e.g. same
// admin session), where two near-identical "please sign a letter" entries
// for the same loan is just noise rather than useful separate information.
const letterNotificationMergeWindow = 24 * time.Hour

// notifyDisbursement sends the "your loan was disbursed" notification,
// merging into the still-unread sanction notification for this loan (if one
// exists within letterNotificationMergeWindow) rather than always adding a
// new bell entry.
func notifyDisbursement(loan models.LoanApplication, amt float64, disbursedAt time.Time) {
	amountText := "Rs." + strconv.FormatFloat(amt, 'f', 2, 64)
	loanRef := "#" + itoa(loan.ID)

	// A plain `LIKE '%#53%'` would also match an unrelated loan's
	// notification body containing "#530" or "#153" — the regex requires
	// a non-digit (or end of string) right after the loan number so only
	// an exact "#53" reference matches.
	var existing models.Notification
	err := database.DB.
		Where("role = ? AND user_id = ? AND read = ? AND body ~ ? AND body LIKE ? AND created_at >= ?",
			"customer", loan.CustomerID.String(), false, "#"+itoa(loan.ID)+"([^0-9]|$)", "%Sanction Letter%",
			time.Now().Add(-letterNotificationMergeWindow)).
		Order("created_at desc").
		First(&existing).Error

	if err == nil {
		existing.Title = "Loan sanctioned and disbursed"
		existing.Body = "Your " + loan.Category + " application " + loanRef + " has been sanctioned and disbursed. Amount " +
			amountText + " disbursed on " + disbursedAt.Format("2006-01-02") +
			". Please review, sign, and re-upload your Sanction Letter and Disbursement Letter."
		database.DB.Save(&existing)
		return
	}

	createNotification("customer", loan.CustomerID.String(), "Loan disbursed",
		"Amount "+amountText+" for loan "+loanRef+" disbursed on "+disbursedAt.Format("2006-01-02")+
			". Please review, sign, and re-upload your Disbursement Letter.")
}

// AdminRestructureLoanHandler handles POST /admin/loans/:id/restructure —
// re-amortizes the remaining principal of a disbursed loan's EMI schedule
// under new tenure/rate terms. Already-paid installments are left untouched
// as history; pending installments are replaced with a fresh schedule.
func AdminRestructureLoanHandler(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var loan models.LoanApplication
	if err := database.DB.First(&loan, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"message": "Not found"})
		return
	}
	if loan.Status != models.LoanStatusDisbursed {
		c.JSON(http.StatusBadRequest, gin.H{"message": "loan must be disbursed to be restructured"})
		return
	}

	var body struct {
		NewTenureMonths        int     `json:"newTenureMonths" binding:"required"`
		NewInterestRatePercent float64 `json:"newInterestRatePercent" binding:"required"`
		Reason                 string  `json:"reason"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	if body.NewTenureMonths <= 0 || body.NewInterestRatePercent <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"message": "newTenureMonths and newInterestRatePercent must be positive"})
		return
	}

	var installments []models.EmiInstallment
	database.DB.Where("loan_id = ?", loan.ID).Order("installment_number asc").Find(&installments)
	if len(installments) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"message": "loan has no existing EMI schedule to restructure"})
		return
	}

	var remainingPrincipal float64
	var maxPaidInstallmentNumber int
	pendingIDs := make([]uint, 0, len(installments))
	for _, inst := range installments {
		if inst.Status == models.EmiStatusPending {
			remainingPrincipal += inst.PrincipalComponent
			pendingIDs = append(pendingIDs, inst.ID)
		} else if inst.InstallmentNumber > maxPaidInstallmentNumber {
			maxPaidInstallmentNumber = inst.InstallmentNumber
		}
	}
	if len(pendingIDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"message": "loan has no pending installments left to restructure"})
		return
	}
	remainingPrincipal = round2(remainingPrincipal)

	oldTenure := 0
	if loan.TenureMonths != nil {
		oldTenure = *loan.TenureMonths
	}
	oldRate := 0.0
	if loan.InterestRatePercent != nil {
		oldRate = *loan.InterestRatePercent
	}

	if err := database.DB.Where("id IN ?", pendingIDs).Delete(&models.EmiInstallment{}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "failed to clear pending installments: " + err.Error()})
		return
	}

	if err := generateEmiInstallments(loan.ID, remainingPrincipal, body.NewTenureMonths, body.NewInterestRatePercent, time.Now(), maxPaidInstallmentNumber+1); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "failed to generate restructured schedule: " + err.Error()})
		return
	}

	newTenure := body.NewTenureMonths
	newRate := body.NewInterestRatePercent
	loan.TenureMonths = &newTenure
	loan.InterestRatePercent = &newRate
	database.DB.Save(&loan)

	details := "Restructured loan #" + itoa(loan.ID) + " — tenure " + strconv.Itoa(oldTenure) + "mo @ " +
		strconv.FormatFloat(oldRate, 'f', 2, 64) + "% -> " + strconv.Itoa(newTenure) + "mo @ " +
		strconv.FormatFloat(newRate, 'f', 2, 64) + "%. Remaining principal: Rs." +
		strconv.FormatFloat(remainingPrincipal, 'f', 2, 64) + ". Reason: " + body.Reason
	audit.Record(c.GetString("admin_username"), "loan.restructure", "loan", itoa(loan.ID), details)

	var newInstallments []models.EmiInstallment
	database.DB.Where("loan_id = ?", loan.ID).Order("installment_number asc").Find(&newInstallments)
	c.JSON(http.StatusOK, buildScheduleResponse(loan, newInstallments))
}
