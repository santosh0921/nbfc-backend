package loans

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/santosh0921/nbfc-backend/internal/audit"
	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/models"
	"github.com/santosh0921/nbfc-backend/internal/pagination"
)

// AdminListLoansHandler handles GET /admin/loans
//
// PHASE 15: this previously returned EVERY matching loan with no limit.
// The loan table grows with the business, so at 100k+ loans an unfiltered
// call would serialize the entire book into one response. Now paginated
// via the shared bounded contract (internal/pagination).
//
// The response shape changed from a bare JSON array to an object with
// "loans" plus a "pagination" block — a breaking change for any client
// that assumed the array, and a deliberate one: silently truncating to a
// page while still returning a bare array would have been worse, because
// a caller could not tell a full result from a first page.
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

	var total int64
	q.Count(&total)

	p := pagination.FromQuery(c)
	var loans []models.LoanApplication
	q.Order("created_at desc").Limit(p.Limit).Offset(p.Offset()).Find(&loans)

	c.JSON(http.StatusOK, gin.H{
		"loans":      loans,
		"pagination": p.Meta(total, len(loans)),
	})
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

// AdminDecisionHandler handles POST /admin/loans/:id/decision — as of the
// Phase 1 maker-checker rework this ONLY supports outright rejection
// (pre-recommendation; a reviewer who never wants to recommend the loan
// for sanction at all). "sanction" is no longer accepted here — it used
// to let a single admin unilaterally sanction a loan in one call; that
// path is now split into two independently-authorized steps, see
// AdminRecommendSanctionHandler + AdminApproveSanctionHandler in
// internal/loans/maker_checker_handler.go.
func AdminDecisionHandler(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var loan models.LoanApplication
	if err := database.DB.First(&loan, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"message": "Not found"})
		return
	}
	var body struct {
		Action  string `json:"action" binding:"required"`
		Remarks string `json:"remarks"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	if body.Action == "sanction" {
		c.JSON(http.StatusGone, gin.H{"message": "Single-admin sanction has been disabled. Use POST /admin/loans/:id/recommend to recommend sanction terms, then a different authorized admin must call POST /admin/loans/:id/approve-sanction."})
		return
	}
	if body.Action != "reject" {
		c.JSON(http.StatusBadRequest, gin.H{"message": "action must be reject"})
		return
	}

	before := audit.Snapshot(loan)
	now := time.Now()
	loan.Status = models.LoanStatusRejected
	loan.DecisionRemarks = body.Remarks
	loan.DecidedBy = c.GetString("admin_username")
	if v, ok := c.Get("admin_id"); ok {
		if id, ok := v.(uint); ok {
			loan.DecidedByAdminID = &id
		}
	}
	loan.DecidedAt = &now
	database.DB.Save(&loan)

	createNotification("customer", loan.CustomerID.String(), "Loan rejected",
		"Your "+loan.Category+" application #"+itoa(loan.ID)+" has been rejected. Remarks: "+body.Remarks)

	role, _ := c.Get("admin_role")
	audit.RecordDetailed(c, audit.Entry{
		ActorType: "admin", ActorID: toAdminID(c), ActorRef: loan.DecidedBy, ActorRole: toStr(role),
		Action: "loan.reject", TargetType: "loan", TargetID: itoa(loan.ID),
		Result: audit.ResultSuccess, Reason: body.Remarks, Details: before,
	})

	c.JSON(http.StatusOK, withReferences(loan))
}

func toAdminID(c *gin.Context) uint {
	if v, ok := c.Get("admin_id"); ok {
		if id, ok := v.(uint); ok {
			return id
		}
	}
	return 0
}

func toStr(v interface{}) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

// AdminDisburseHandler handles POST /admin/loans/:id/disburse — disabled
// as of the Phase 1 maker-checker rework. A single admin could previously
// disburse real funds in one call the moment a loan was sanctioned; that
// is now split into AdminPrepareDisbursementHandler (maker) +
// AdminApproveDisbursementHandler (checker), see
// internal/loans/maker_checker_handler.go.
func AdminDisburseHandler(c *gin.Context) {
	c.JSON(http.StatusGone, gin.H{"message": "Single-admin disbursement has been disabled. Use POST /admin/loans/:id/prepare-disbursement, then a different authorized admin must call POST /admin/loans/:id/approve-disbursement."})
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
