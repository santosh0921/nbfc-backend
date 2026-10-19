package recovery

import (
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/santosh0921/nbfc-backend/internal/audit"
	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/models"
	"github.com/santosh0921/nbfc-backend/internal/notifications"
)

// createNotification now just forwards to notifications.Create, which
// also fires a real device push (internal/push) — kept as a thin wrapper
// rather than rewriting every call site in this file to the shared
// function directly.
func createNotification(role string, userID string, title, body string) {
	notifications.Create(role, userID, title, body)
}

// CasesTodayHandler handles GET /employee/recovery/cases/today
func CasesTodayHandler(c *gin.Context) {
	empID := c.GetUint("employee_id")
	var cases []models.RecoveryCase
	database.DB.Where("assigned_employee_id = ?", empID).Find(&cases)
	c.JSON(http.StatusOK, cases)
}

// SubmitReportHandler handles POST /employee/recovery/cases/:id/report
func SubmitReportHandler(c *gin.Context) {
	empID := c.GetUint("employee_id")
	id, _ := strconv.Atoi(c.Param("id"))
	var rc models.RecoveryCase
	if err := database.DB.First(&rc, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"message": "case not found"})
		return
	}
	if rc.AssignedEmployeeID != empID {
		c.JSON(http.StatusForbidden, gin.H{"message": "case not assigned to you"})
		return
	}
	var body struct {
		StatusUpdate   string `json:"statusUpdate"`
		Remarks        string `json:"remarks"`
		RiskAssessment string `json:"riskAssessment"`
		Recommendation string `json:"recommendation"`
		NextAction     string `json:"nextAction"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	report := models.RecoveryReport{
		CaseID: rc.ID, StatusUpdate: body.StatusUpdate, Remarks: body.Remarks,
		RiskAssessment: body.RiskAssessment, Recommendation: body.Recommendation, NextAction: body.NextAction,
	}
	database.DB.Create(&report)
	rc.Status = "reported"
	database.DB.Save(&rc)
	c.JSON(http.StatusCreated, report)
}

// AdminListReportsHandler handles GET /admin/recovery-reports
func AdminListReportsHandler(c *gin.Context) {
	var reports []models.RecoveryReport
	database.DB.Order("created_at desc").Find(&reports)
	c.JSON(http.StatusOK, reports)
}

// recoveryCaseRow adds the assigned employee's real name/code/city to the
// raw RecoveryCase row — previously the Collections & Recovery portal had
// no visibility into this table at all (only the Employee app read it),
// so there was nothing translating a bare AssignedEmployeeID into who that
// actually is.
type recoveryCaseRow struct {
	models.RecoveryCase
	AssignedEmployeeName string `json:"assignedEmployeeName,omitempty"`
	AssignedEmployeeCode string `json:"assignedEmployeeCode,omitempty"`
	AssignedEmployeeCity string `json:"assignedEmployeeCity,omitempty"`
}

// AdminListCasesHandler handles GET /admin/recovery/cases — requires
// COLLECTION_VIEW. The Collections & Recovery portal's real window into
// RecoveryCase (previously portal-invisible — only /admin/recovery-reports
// existed, which lists REPORTS employees filed, not the cases themselves).
// Optional ?city= / ?status= filters.
func AdminListCasesHandler(c *gin.Context) {
	q := database.DB.Model(&models.RecoveryCase{})
	if city := c.Query("city"); city != "" {
		q = q.Where("LOWER(city) = LOWER(?)", city)
	}
	if status := c.Query("status"); status != "" {
		q = q.Where("status = ?", status)
	}
	var cases []models.RecoveryCase
	q.Order("created_at desc").Limit(200).Find(&cases)

	empIDs := make([]uint, 0, len(cases))
	seen := map[uint]bool{}
	for _, rc := range cases {
		if rc.AssignedEmployeeID != 0 && !seen[rc.AssignedEmployeeID] {
			seen[rc.AssignedEmployeeID] = true
			empIDs = append(empIDs, rc.AssignedEmployeeID)
		}
	}
	var emps []models.Employee
	if len(empIDs) > 0 {
		database.DB.Where("id IN ?", empIDs).Find(&emps)
	}
	empByID := make(map[uint]models.Employee, len(emps))
	for _, e := range emps {
		empByID[e.ID] = e
	}

	out := make([]recoveryCaseRow, len(cases))
	for i, rc := range cases {
		row := recoveryCaseRow{RecoveryCase: rc}
		if e, ok := empByID[rc.AssignedEmployeeID]; ok {
			row.AssignedEmployeeName = e.Name
			row.AssignedEmployeeCode = e.Code
			row.AssignedEmployeeCity = e.BranchCity
		}
		out[i] = row
	}
	c.JSON(http.StatusOK, out)
}

// recoveryEmployeeRow adds a live open-case count to each recovery
// employee — lets the portal show workload at a glance when choosing who
// to assign a case to, rather than assigning blind.
type recoveryEmployeeRow struct {
	ID         uint   `json:"id"`
	Code       string `json:"code"`
	Name       string `json:"name"`
	BranchCity string `json:"branchCity"`
	Active     bool   `json:"active"`
	OpenCases  int64  `json:"openCases"`
}

// AdminListRecoveryEmployeesHandler handles GET /admin/recovery/employees
// — requires COLLECTION_VIEW. Every active, approved recovery-role
// employee, for the portal's city-grouped assignment picker. Optional
// ?search= matches name or employee code (case-insensitive substring);
// ?city= narrows to one branch city exactly.
func AdminListRecoveryEmployeesHandler(c *gin.Context) {
	q := database.DB.Model(&models.Employee{}).
		Where("role = ? AND active = ? AND approved = ?", "recovery", true, true)
	if search := c.Query("search"); search != "" {
		like := "%" + search + "%"
		q = q.Where("name ILIKE ? OR code ILIKE ?", like, like)
	}
	if city := c.Query("city"); city != "" {
		q = q.Where("LOWER(branch_city) = LOWER(?)", city)
	}
	var emps []models.Employee
	q.Order("branch_city asc, name asc").Find(&emps)

	out := make([]recoveryEmployeeRow, len(emps))
	for i, e := range emps {
		var openCount int64
		database.DB.Model(&models.RecoveryCase{}).
			Where("assigned_employee_id = ? AND status = ?", e.ID, "open").Count(&openCount)
		out[i] = recoveryEmployeeRow{
			ID: e.ID, Code: e.Code, Name: e.Name, BranchCity: e.BranchCity, Active: e.Active, OpenCases: openCount,
		}
	}
	c.JSON(http.StatusOK, out)
}

// AdminManualAssignCaseHandler handles POST /admin/recovery/cases/:id/assign
// — requires COLLECTION_MANAGE (a Collection Manager, single-step, no
// maker-checker — matches how the Collections portal's own case
// assignment already works). Lets a manager override/redirect the
// automatic city-match assignment (findRecoveryEmployee) to a specific
// employee — e.g. the auto-matched employee is overloaded, on leave, or
// the case needs a specific agent's attention.
func AdminManualAssignCaseHandler(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid case id"})
		return
	}
	var body struct {
		EmployeeID uint   `json:"employeeId" binding:"required"`
		Remarks    string `json:"remarks"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}

	var rc models.RecoveryCase
	if err := database.DB.First(&rc, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"message": "Case not found"})
		return
	}

	var emp models.Employee
	if err := database.DB.Where("id = ? AND role = ? AND active = ? AND approved = ?", body.EmployeeID, "recovery", true, true).First(&emp).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Target employee is not an active, approved recovery employee"})
		return
	}

	before := rc.AssignedEmployeeID
	rc.AssignedEmployeeID = emp.ID
	if err := database.DB.Save(&rc).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Failed to reassign case"})
		return
	}

	note := models.CaseNote{
		CaseID: rc.ID, Author: c.GetString("admin_username"), Action: "reassigned",
		Remarks: "Reassigned from employee #" + strconv.Itoa(int(before)) + " to " + emp.Code + " (" + emp.Name + "). " + body.Remarks,
	}
	database.DB.Create(&note)

	createNotification("employee", strconv.Itoa(int(emp.ID)), "Recovery case assigned to you",
		"Case #"+strconv.Itoa(int(rc.ID))+" ("+rc.CustomerName+") has been assigned to you.")
	audit.Record(c.GetString("admin_username"), "recovery.manual_assign", "recovery_case", strconv.Itoa(int(rc.ID)),
		"Manually assigned to "+emp.Code+" ("+emp.Name+"): "+body.Remarks)

	c.JSON(http.StatusOK, rc)
}

// findRecoveryEmployee mirrors internal/loans.findVerificationEmployee:
// exact case-insensitive city match against employees.branch_city, else
// least-loaded (fewest currently "open" cases) active+approved recovery
// employee. Only ever considers Approved employees — same reasoning as
// the verification-side lookup.
func findRecoveryEmployee(city string) *models.Employee {
	var emp models.Employee
	err := database.DB.Where("role = ? AND active = ? AND approved = ? AND LOWER(branch_city) = LOWER(?)", "recovery", true, true, city).First(&emp).Error
	if err == nil {
		return &emp
	}
	var emps []models.Employee
	if err := database.DB.Where("role = ? AND active = ? AND approved = ?", "recovery", true, true).Find(&emps).Error; err != nil {
		log.Println("findRecoveryEmployee: fallback employee query failed:", err)
		return nil
	}
	if len(emps) == 0 {
		return nil
	}
	best := emps[0]
	var bestCount int64
	database.DB.Model(&models.RecoveryCase{}).Where("assigned_employee_id = ? AND status = ?", best.ID, "open").Count(&bestCount)
	for _, e := range emps[1:] {
		var cnt int64
		database.DB.Model(&models.RecoveryCase{}).Where("assigned_employee_id = ? AND status = ?", e.ID, "open").Count(&cnt)
		if cnt < bestCount {
			best = e
			bestCount = cnt
		}
	}
	return &best
}

// AdminAssignRecoveryHandler handles POST /admin/loans/:id/assign-recovery
// — the only way a RecoveryCase is ever created outside the one-time seed
// script (internal/seed/seed.go). There is deliberately no automatic
// DPD/NPA-threshold trigger yet: that needs a real policy decision (which
// bucket, which loans are exempt, whether it re-triggers on an existing
// open case) that hasn't been made — this is the minimum viable path so
// the recovery module has ANY way to receive real cases in production
// rather than sitting permanently empty behind seed data alone.
func AdminAssignRecoveryHandler(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid loan id"})
		return
	}

	var loan models.LoanApplication
	if err := database.DB.First(&loan, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"message": "Loan not found"})
		return
	}
	if loan.Status != models.LoanStatusDisbursed {
		c.JSON(http.StatusConflict, gin.H{"message": "Only a disbursed loan can be assigned to recovery"})
		return
	}

	var existing models.RecoveryCase
	loanRef := "loan:" + strconv.Itoa(int(loan.ID))
	if err := database.DB.Where("loan_ref = ? AND status NOT IN ?", loanRef, []string{"resolved"}).First(&existing).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{"message": "This loan already has an open recovery case", "caseId": existing.ID})
		return
	}

	var overdueAmount float64
	database.DB.Model(&models.EmiInstallment{}).
		Where("loan_id = ? AND status != ? AND due_date < ?", loan.ID, models.EmiStatusPaid, time.Now()).
		Select("COALESCE(SUM(principal_component + interest_component), 0)").Scan(&overdueAmount)

	emp := findRecoveryEmployee(loan.City)
	if emp == nil {
		c.JSON(http.StatusConflict, gin.H{"message": "No active, approved recovery employee is available to assign this case to"})
		return
	}

	rc := models.RecoveryCase{
		LoanRef: loanRef, CustomerName: loan.ApplicantName, CustomerPhone: loan.ApplicantPhone,
		OverdueAmount: overdueAmount, City: loan.City, AssignedEmployeeID: emp.ID, Status: "open",
	}
	if err := database.DB.Create(&rc).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Failed to create recovery case"})
		return
	}

	createNotification("employee", strconv.Itoa(int(emp.ID)), "New recovery case",
		"Loan #"+strconv.Itoa(int(loan.ID))+" ("+loan.ApplicantName+") assigned to you for recovery.")
	audit.Record(c.GetString("admin_username"), "recovery.assign", "recovery_case", strconv.Itoa(int(rc.ID)),
		"Assigned loan #"+strconv.Itoa(int(loan.ID))+" to recovery employee "+emp.Code+" ("+emp.Name+")")

	c.JSON(http.StatusCreated, rc)
}

// AdminActionHandler handles POST /admin/recovery-reports/:id/action (id = case id)
func AdminActionHandler(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var rc models.RecoveryCase
	if err := database.DB.First(&rc, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"message": "case not found"})
		return
	}
	var body struct {
		Action  string `json:"action" binding:"required"`
		Remarks string `json:"remarks"`
		Status  string `json:"status"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	note := models.CaseNote{CaseID: rc.ID, Author: "admin", Action: body.Action, Remarks: body.Remarks}
	database.DB.Create(&note)
	if body.Status != "" {
		rc.Status = body.Status
		database.DB.Save(&rc)
	}
	createNotification("employee", strconv.Itoa(int(rc.AssignedEmployeeID)), "Admin action on case #"+strconv.Itoa(int(rc.ID)),
		body.Action+": "+body.Remarks)
	audit.Record(c.GetString("admin_username"), "recovery.action", "recovery_case", strconv.Itoa(int(rc.ID)),
		body.Action+": "+body.Remarks)
	c.JSON(http.StatusCreated, note)
}
