package loans

import (
	"log"
	"strconv"

	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/models"
)

// Every loan category now goes through employee verification (video KYC
// included) — Instant/Personal Loan used to auto-approve with no employee
// involvement at all, which made a video-KYC call for those categories
// impossible (there was never an employee assigned to place one). Kept as
// an empty, still-checked map rather than deleting the mechanism outright,
// in case a genuinely-instant product is reintroduced later.
var autoApproveCategories = map[string]bool{}

// findVerificationEmployee: exact case-insensitive city match against
// employees.branch_city; if none found, falls back to least-loaded (fewest
// current "assigned" tasks) active verification employee. Only ever
// considers Approved employees — a self-registered employee awaiting
// admin approval (see internal/employee/register_handler.go) is Active
// but cannot log in yet, so assigning a loan to one would make that loan
// invisible in the employee app to literally everyone until that specific
// employee is approved.
func findVerificationEmployee(city string) *models.Employee {
	var emp models.Employee
	err := database.DB.Where("role = ? AND active = ? AND approved = ? AND LOWER(branch_city) = LOWER(?)", "verification", true, true, city).First(&emp).Error
	if err == nil {
		return &emp
	}
	var emps []models.Employee
	if err := database.DB.Where("role = ? AND active = ? AND approved = ?", "verification", true, true).Find(&emps).Error; err != nil {
		// Previously silently swallowed — a transient DB error here (e.g.
		// during a deploy/restart) looked identical to "genuinely no
		// employees available", leaving a loan permanently unassigned
		// with zero trace of why. Logging at least makes that
		// diagnosable instead of a mystery.
		log.Println("findVerificationEmployee: fallback employee query failed:", err)
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
func submitLoanApplication(loan models.LoanApplication) models.LoanApplication {
	loan.Status = models.LoanStatusSubmitted

	if autoApproveCategories[loan.Category] {
		loan.Status = models.LoanStatusAutoVerified
		loan.AutoVerified = true
		loan.AutoVerifiedNote = "Auto-verified — no manual verification required"
		database.DB.Create(&loan)
	} else {
		database.DB.Create(&loan)
		emp := findVerificationEmployee(loan.City)
		if emp != nil {
			loan.AssignedEmployeeID = &emp.ID
			loan.Status = models.LoanStatusAssigned
			database.DB.Save(&loan)
			createNotification("employee", itoa(emp.ID), "New verification task",
				"Loan application #"+itoa(loan.ID)+" ("+loan.Category+") assigned to you for verification.")
		}
	}

	return loan
}

func createNotification(role string, userID string, title, body string) {
	n := models.Notification{Role: role, UserID: userID, Title: title, Body: body}
	database.DB.Create(&n)
}

func itoa(id uint) string {
	return strconv.Itoa(int(id))
}
