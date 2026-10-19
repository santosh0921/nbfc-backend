package loans

import (
	"log"
	"strconv"

	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/models"
	"github.com/santosh0921/nbfc-backend/internal/notifications"
)

// Every loan category now goes through employee verification (video KYC
// included) — Instant/Personal Loan used to auto-approve with no employee
// involvement at all, which made a video-KYC call for those categories
// impossible (there was never an employee assigned to place one). Kept as
// an empty, still-checked map rather than deleting the mechanism outright,
// in case a genuinely-instant product is reintroduced later.
var autoApproveCategories = map[string]bool{}

// findVerificationEmployeesInCity: every active, approved verification
// employee whose branch_city exact-case-insensitive-matches city. Only
// ever considers Approved employees — a self-registered employee awaiting
// admin approval (see internal/employee/register_handler.go) is Active
// but cannot log in yet, so broadcasting a loan to one would make that
// loan invisible to everyone until that specific employee is approved.
func findVerificationEmployeesInCity(city string) []models.Employee {
	var emps []models.Employee
	if err := database.DB.Where("role = ? AND active = ? AND approved = ? AND LOWER(branch_city) = LOWER(?)", "verification", true, true, city).Find(&emps).Error; err != nil {
		log.Println("findVerificationEmployeesInCity: query failed:", err)
		return nil
	}
	return emps
}

// findAnyVerificationEmployee is the fallback used only when a loan's
// city has literally no approved verification employee at all — least-
// loaded (fewest current "assigned" tasks) active verification employee,
// regardless of city, so the loan is never left permanently unassigned
// just because its city has no local coverage yet.
func findAnyVerificationEmployee() *models.Employee {
	var emps []models.Employee
	if err := database.DB.Where("role = ? AND active = ? AND approved = ?", "verification", true, true).Find(&emps).Error; err != nil {
		log.Println("findAnyVerificationEmployee: query failed:", err)
		return nil
	}
	if len(emps) == 0 {
		return nil
	}
	best := emps[0]
	var bestCount int64
	database.DB.Model(&models.LoanApplication{}).Where("assigned_employee_id = ? AND status = ?", best.ID, models.LoanStatusAssigned).Count(&bestCount)
	for _, e := range emps[1:] {
		var cnt int64
		database.DB.Model(&models.LoanApplication{}).Where("assigned_employee_id = ? AND status = ?", e.ID, models.LoanStatusAssigned).Count(&cnt)
		if cnt < bestCount {
			best = e
			bestCount = cnt
		}
	}
	return &best
}

// submitLoanApplication runs the shared auto-approval / city-assignment
// pipeline used by both a normal application (SubmitLoanHandler) and a
// customer-initiated top-up (TopUpLoanHandler). It creates the
// LoanApplication row, applying the same auto-verify-or-assign logic either
// way, and returns the persisted loan.
//
// A loan needing verification is broadcast to every approved employee in
// its city (LoanStatusPendingClaim, AssignedEmployeeID left nil) rather
// than auto-assigned to one — first employee to call
// POST /employee/loans/:id/claim gets it. Only falls back to the old
// immediate single-employee assignment when the city has no employee at
// all, so a loan is never stranded unassigned just for lack of local
// coverage.
func submitLoanApplication(loan models.LoanApplication) models.LoanApplication {
	loan.Status = models.LoanStatusSubmitted

	if autoApproveCategories[loan.Category] {
		loan.Status = models.LoanStatusAutoVerified
		loan.AutoVerified = true
		loan.AutoVerifiedNote = "Auto-verified — no manual verification required"
		database.DB.Create(&loan)
		return loan
	}

	// Two-stage KYC review: a freshly submitted loan is no longer
	// broadcast to field employees immediately. It sits here, visible to
	// KYC with every document the customer uploaded, until a KYC officer
	// explicitly approves the documents — see
	// AdminApproveLoanKYCDocumentsHandler, which is what now calls
	// NotifyEligibleEmployees (moved out of this function).
	loan.Status = models.LoanStatusPendingKYCDocReview
	database.DB.Create(&loan)
	return loan
}

// NotifyEligibleEmployees broadcasts a loan to every approved verification
// employee in its city (first-come-first-served claim), falling back to
// the old single-employee least-loaded assignment when no employee exists
// in that city at all. Exported because it now runs from
// AdminApproveLoanKYCDocumentsHandler (internal/loans/kyc_review_handler.go)
// rather than from submitLoanApplication — this is the exact code that used
// to run inline there, unchanged, just moved to fire after KYC document
// approval instead of at submission time.
func NotifyEligibleEmployees(loan *models.LoanApplication) {
	cityEmps := findVerificationEmployeesInCity(loan.City)
	if len(cityEmps) > 0 {
		loan.Status = models.LoanStatusPendingClaim
		database.DB.Save(loan)
		for _, emp := range cityEmps {
			createNotification("employee", itoa(emp.ID), "New verification task available",
				"Loan application #"+itoa(loan.ID)+" ("+loan.Category+") in "+loan.City+" is available to claim — first come, first served.")
		}
		return
	}

	// No employee at all in this city — fall back to the old behavior
	// rather than leaving the loan permanently invisible to everyone.
	emp := findAnyVerificationEmployee()
	if emp != nil {
		loan.AssignedEmployeeID = &emp.ID
		loan.Status = models.LoanStatusAssigned
		database.DB.Save(loan)
		createNotification("employee", itoa(emp.ID), "New verification task",
			"Loan application #"+itoa(loan.ID)+" ("+loan.Category+") assigned to you for verification — no employee found in "+loan.City+".")
	}
}

// createNotification forwards to notifications.Create, which also fires
// a real device push (internal/push) — kept as a thin wrapper rather
// than rewriting every call site across this package to the shared
// function directly.
func createNotification(role string, userID string, title, body string) {
	notifications.Create(role, userID, title, body)
}

func itoa(id uint) string {
	return strconv.Itoa(int(id))
}
