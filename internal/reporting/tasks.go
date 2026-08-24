// Compliance task management (item J) + periodic compliance checks (item
// K). Periodic checks create tasks/alerts for a human to act on — they
// never silently fix a record themselves (item K: "Create alerts and
// compliance tasks rather than silently fixing records").
package reporting

import (
	"time"

	"github.com/santosh0921/nbfc-backend/internal/audit"
	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/models"
)

func CreateTask(category, severity, description, reference, createdBy string, dueDate *time.Time) (*models.ComplianceTask, error) {
	task := models.ComplianceTask{
		Category: category, Severity: severity, Description: description, Reference: reference,
		Status: models.TaskStatusOpen, CreatedBy: createdBy, DueDate: dueDate,
	}
	if err := database.DB.Create(&task).Error; err != nil {
		return nil, err
	}
	audit.RecordDetailed(nil, audit.Entry{
		ActorType: actorTypeFor(createdBy), ActorRef: createdBy,
		Action: "compliance.task.created", TargetType: "compliance_task", TargetID: itoa(task.ID),
		Result: audit.ResultSuccess, Details: category + ": " + description,
	})
	return &task, nil
}

func AssignTask(taskID, assignToAdminID uint, assignToName, actor string) error {
	err := database.DB.Model(&models.ComplianceTask{}).Where("id = ?", taskID).
		Updates(map[string]interface{}{"assigned_to_admin_id": assignToAdminID, "assigned_to_name": assignToName, "status": models.TaskStatusAssigned}).Error
	if err != nil {
		return err
	}
	audit.RecordDetailed(nil, audit.Entry{
		ActorType: "admin", ActorRef: actor,
		Action: "compliance.task.assigned", TargetType: "compliance_task", TargetID: itoa(taskID),
		Result: audit.ResultSuccess, Details: "assigned to " + assignToName,
	})
	return nil
}

func UpdateTaskStatus(taskID uint, status, resolution, actor string) error {
	updates := map[string]interface{}{"status": status}
	if resolution != "" {
		updates["resolution"] = resolution
	}
	if status == models.TaskStatusClosed || status == models.TaskStatusResolved {
		now := time.Now()
		updates["closed_at"] = now
	}
	if err := database.DB.Model(&models.ComplianceTask{}).Where("id = ?", taskID).Updates(updates).Error; err != nil {
		return err
	}
	audit.RecordDetailed(nil, audit.Entry{
		ActorType: "admin", ActorRef: actor,
		Action: "compliance.task.status_changed", TargetType: "compliance_task", TargetID: itoa(taskID),
		Result: audit.ResultSuccess, Details: status, Reason: resolution,
	})
	return nil
}

// RunPeriodicComplianceChecks (item K) — creates a ComplianceTask for
// every newly-detected issue across the categories item K lists. Each
// check is deduplicated against any still-OPEN task with the same
// category+reference, so re-running (e.g. daily) doesn't spam duplicate
// tasks for a condition already being worked.
func RunPeriodicComplianceChecks() int {
	created := 0
	created += checkMissingKYC()
	created += checkFailedBureauRequests()
	created += checkInactiveAdmins()
	created += checkUnmatchedBankTransactions()
	created += checkOverdueReconciliation()
	created += checkReportingErrors()
	return created
}

func taskExistsFor(category, reference string) bool {
	var count int64
	database.DB.Model(&models.ComplianceTask{}).
		Where("category = ? AND reference = ? AND status NOT IN ?", category, reference,
			[]string{models.TaskStatusResolved, models.TaskStatusClosed}).Count(&count)
	return count > 0
}

func checkMissingKYC() int {
	var loanIDs []uint
	database.DB.Raw(`
		SELECT la.id FROM loan_applications la
		WHERE la.status = 'disbursed'
		AND NOT EXISTS (SELECT 1 FROM customer_kycs ck WHERE ck.customer_id = la.customer_id AND ck.status = 'VERIFIED')
	`).Scan(&loanIDs)
	created := 0
	for _, id := range loanIDs {
		ref := "loan:" + itoa(id)
		if taskExistsFor(models.TaskCategoryKYC, ref) {
			continue
		}
		CreateTask(models.TaskCategoryKYC, models.TaskSeverityHigh,
			"Loan disbursed without a VERIFIED KYC record on file", ref, "system", nil)
		created++
	}
	return created
}

func checkFailedBureauRequests() int {
	var ids []uint
	database.DB.Model(&models.BureauRequest{}).Where("status = ?", models.BureauRequestStatusFailed).
		Where("created_at >= ?", time.Now().AddDate(0, 0, -7)).Pluck("id", &ids)
	created := 0
	for _, id := range ids {
		ref := "bureau_request:" + itoa(id)
		if taskExistsFor(models.TaskCategoryCredit, ref) {
			continue
		}
		CreateTask(models.TaskCategoryCredit, models.TaskSeverityMedium, "Bureau request failed", ref, "system", nil)
		created++
	}
	return created
}

func checkInactiveAdmins() int {
	var ids []uint
	cutoff := time.Now().AddDate(0, 0, -90)
	database.DB.Model(&models.AdminUser{}).
		Where("status = ? AND (last_login_at IS NULL OR last_login_at < ?)", models.AdminStatusActive, cutoff).
		Pluck("id", &ids)
	created := 0
	for _, id := range ids {
		ref := "admin_user:" + itoa(id)
		if taskExistsFor(models.TaskCategorySecurity, ref) {
			continue
		}
		CreateTask(models.TaskCategorySecurity, models.TaskSeverityMedium,
			"Admin account has had no login in 90+ days and is still ACTIVE — review for access deactivation", ref, "system", nil)
		created++
	}
	return created
}

func checkUnmatchedBankTransactions() int {
	var ids []uint
	database.DB.Model(&models.ReconciliationException{}).Where("resolved_at IS NULL").
		Where("created_at <= ?", time.Now().AddDate(0, 0, -3)).Pluck("id", &ids)
	created := 0
	for _, id := range ids {
		ref := "reconciliation_exception:" + itoa(id)
		if taskExistsFor(models.TaskCategoryAccounting, ref) {
			continue
		}
		CreateTask(models.TaskCategoryAccounting, models.TaskSeverityMedium,
			"Bank reconciliation exception unresolved for 3+ days", ref, "system", nil)
		created++
	}
	return created
}

func checkOverdueReconciliation() int {
	var ids []uint
	database.DB.Model(&models.DataConsistencyException{}).Where("status = ?", models.ExceptionStatusOpen).
		Where("detected_at <= ?", time.Now().AddDate(0, 0, -7)).Pluck("id", &ids)
	created := 0
	for _, id := range ids {
		ref := "data_consistency_exception:" + itoa(id)
		if taskExistsFor(models.TaskCategoryDataQuality, ref) {
			continue
		}
		CreateTask(models.TaskCategoryDataQuality, models.TaskSeverityHigh,
			"Data consistency exception open for 7+ days", ref, "system", nil)
		created++
	}
	return created
}

func checkReportingErrors() int {
	var runIDs []uint
	database.DB.Model(&models.RegulatoryReportRun{}).
		Where("id IN (SELECT run_id FROM regulatory_report_exceptions WHERE severity = ?) AND status = ?",
			models.ReportValidationError, models.RegReportStatusGenerated).Pluck("id", &runIDs)
	created := 0
	for _, id := range runIDs {
		ref := "regulatory_report_run:" + itoa(id)
		if taskExistsFor(models.TaskCategoryRegReporting, ref) {
			continue
		}
		CreateTask(models.TaskCategoryRegReporting, models.TaskSeverityCritical,
			"Regulatory report run has blocking validation errors", ref, "system", nil)
		created++
	}
	return created
}
