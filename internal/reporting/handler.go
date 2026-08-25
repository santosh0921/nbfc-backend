package reporting

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/santosh0921/nbfc-backend/internal/audit"
	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/models"
	"github.com/santosh0921/nbfc-backend/internal/storage"
)

func adminActor(c *gin.Context) string {
	if v := c.GetString("admin_username"); v != "" {
		return v
	}
	return "system"
}
func adminIDFrom(c *gin.Context) uint {
	if v, ok := c.Get("admin_id"); ok {
		if id, ok := v.(uint); ok {
			return id
		}
	}
	return 0
}

func parseDateRange(c *gin.Context) (from, to *time.Time) {
	if v := c.Query("from"); v != "" {
		if t, err := time.Parse("2006-01-02", v); err == nil {
			from = &t
		}
	}
	if v := c.Query("to"); v != "" {
		if t, err := time.Parse("2006-01-02", v); err == nil {
			end := t.Add(24*time.Hour - time.Second)
			to = &end
		}
	}
	return
}

// --- MIS dashboard (item B) ---

func AdminMISDashboardHandler(c *gin.Context) {
	from, to := parseDateRange(c)
	dashboard := GenerateMISDashboard(MISFilters{From: from, To: to, Product: c.Query("product"), State: c.Query("state")})
	c.JSON(http.StatusOK, dashboard)
}

// --- Compliance health dashboard (item I) ---

func AdminComplianceHealthHandler(c *gin.Context) {
	c.JSON(http.StatusOK, GenerateComplianceHealthDashboard())
}

// --- Audit Center (item G/H) — read-only, no write handler exists for
// any of these routes by construction. ---

func AdminLoanTimelineHandler(c *gin.Context) {
	loanID, err := strconv.Atoi(c.Param("loanId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid loan id"})
		return
	}
	c.JSON(http.StatusOK, LoanTimeline(uint(loanID)))
}

func AdminCustomerTimelineHandler(c *gin.Context) {
	c.JSON(http.StatusOK, CustomerTimeline(c.Param("customerId")))
}

// --- Reconciliation dashboard (item F) ---

func AdminListReconciliationHandler(c *gin.Context) {
	q := database.DB.Model(&models.CrossSystemReconciliation{})
	if status := c.Query("status"); status != "" {
		q = q.Where("status = ?", status)
	} else {
		q = q.Where("status != ?", models.ReconStatusMatched)
	}
	var rows []models.CrossSystemReconciliation
	q.Order("detected_at desc").Limit(200).Find(&rows)
	c.JSON(http.StatusOK, rows)
}

func AdminRunReconciliationChecksHandler(c *gin.Context) {
	found := RunCrossSystemChecks()
	c.JSON(http.StatusOK, gin.H{"findingsChecked": found})
}

func AdminResolveReconciliationHandler(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var body struct {
		Note string `json:"note"`
	}
	c.ShouldBindJSON(&body)
	if err := ResolveReconciliation(uint(id), adminActor(c), body.Note); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "resolved"})
}

// --- Regulatory reporting framework (item C/D/E/N/P) ---

func AdminListDefinitionsHandler(c *gin.Context) {
	var defs []models.RegulatoryReportDefinition
	database.DB.Order("id desc").Find(&defs)
	c.JSON(http.StatusOK, defs)
}

func AdminCreateDefinitionHandler(c *gin.Context) {
	var body struct {
		ReportType  string `json:"reportType" binding:"required"`
		Authority   string `json:"reportingAuthority"`
		Description string `json:"description"`
		Frequency   string `json:"frequency"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	def, err := CreateDefinition(body.ReportType, body.Authority, body.Description, body.Frequency, adminActor(c))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, def)
}

func AdminUpdateDefinitionStatusHandler(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var body struct {
		Status string `json:"status" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	if err := UpdateDefinitionStatus(uint(id), body.Status, adminActor(c)); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "updated"})
}

func AdminCreateVersionHandler(c *gin.Context) {
	definitionID, _ := strconv.Atoi(c.Param("id"))
	var body struct {
		VersionLabel string                          `json:"versionLabel" binding:"required"`
		Notes        string                          `json:"notes"`
		Fields       []models.RegulatoryReportField   `json:"fields"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	version, err := CreateVersion(uint(definitionID), body.VersionLabel, body.Notes, adminActor(c), body.Fields)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, version)
}

func AdminListRunsHandler(c *gin.Context) {
	q := database.DB.Model(&models.RegulatoryReportRun{})
	if definitionID := c.Query("definitionId"); definitionID != "" {
		q = q.Where("definition_id = ?", definitionID)
	}
	var runs []models.RegulatoryReportRun
	q.Order("id desc").Limit(200).Find(&runs)
	c.JSON(http.StatusOK, runs)
}

func AdminGetRunHandler(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var run models.RegulatoryReportRun
	if err := database.DB.First(&run, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"message": "Run not found"})
		return
	}
	var exceptions []models.RegulatoryReportException
	database.DB.Where("run_id = ?", id).Find(&exceptions)
	var reviews []models.RegulatoryReportReview
	database.DB.Where("run_id = ?", id).Order("created_at desc").Find(&reviews)
	c.JSON(http.StatusOK, gin.H{"run": run, "exceptions": exceptions, "reviews": reviews})
}

// AdminGenerateRunHandler — requires REG_REPORT_MANAGE.
func AdminGenerateRunHandler(c *gin.Context) {
	var body struct {
		DefinitionID    uint   `json:"definitionId" binding:"required"`
		VersionID       uint   `json:"versionId" binding:"required"`
		ReportingPeriod string `json:"reportingPeriod" binding:"required"`
		Parameters      string `json:"parameters"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	run, err := GenerateRun(body.DefinitionID, body.VersionID, body.ReportingPeriod, body.Parameters, adminActor(c), adminIDFrom(c))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, run)
}

func AdminSubmitForReviewHandler(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	if err := SubmitForReview(uint(id), adminActor(c)); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "submitted for review"})
}

// AdminReviewRunHandler — requires REG_REPORT_APPROVE. Refuses the
// preparer reviewing their own run (item P).
func AdminReviewRunHandler(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var body struct {
		Decision string `json:"decision" binding:"required"` // APPROVED | REJECTED
		Reason   string `json:"reason"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	run, err := ReviewRun(uint(id), adminIDFrom(c), adminActor(c), body.Decision, body.Reason)
	if err != nil {
		status := http.StatusBadRequest
		if err == ErrSameActor {
			status = http.StatusForbidden
		}
		if err == ErrBlockingErrors {
			status = http.StatusUnprocessableEntity
		}
		c.JSON(status, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, run)
}

func AdminSubmitRunHandler(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var body struct {
		SubmissionReference string `json:"submissionReference"`
		Notes                string `json:"notes"`
	}
	c.ShouldBindJSON(&body)
	run, err := SubmitRun(uint(id), adminIDFrom(c), adminActor(c), body.SubmissionReference, body.Notes)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, run)
}

func AdminCloseRunHandler(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	if err := CloseRun(uint(id), adminActor(c)); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "closed"})
}

func AdminRevalidateRunHandler(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	findings := RunValidation(uint(id))
	c.JSON(http.StatusOK, findings)
}

// --- Compliance tasks (item J/K) ---

func AdminListTasksHandler(c *gin.Context) {
	q := database.DB.Model(&models.ComplianceTask{})
	if category := c.Query("category"); category != "" {
		q = q.Where("category = ?", category)
	}
	if status := c.Query("status"); status != "" {
		q = q.Where("status = ?", status)
	} else {
		q = q.Where("status NOT IN ?", []string{models.TaskStatusResolved, models.TaskStatusClosed})
	}
	var tasks []models.ComplianceTask
	q.Order("severity desc, id desc").Limit(200).Find(&tasks)
	c.JSON(http.StatusOK, tasks)
}

func AdminCreateTaskHandler(c *gin.Context) {
	var body struct {
		Category    string     `json:"category" binding:"required"`
		Severity    string     `json:"severity" binding:"required"`
		Description string     `json:"description" binding:"required"`
		Reference   string     `json:"reference"`
		DueDate     *time.Time `json:"dueDate"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	task, err := CreateTask(body.Category, body.Severity, body.Description, body.Reference, adminActor(c), body.DueDate)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, task)
}

func AdminAssignTaskHandler(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var body struct {
		AssignToAdminID uint   `json:"assignToAdminId" binding:"required"`
		AssignToName    string `json:"assignToName" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	if err := AssignTask(uint(id), body.AssignToAdminID, body.AssignToName, adminActor(c)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "assigned"})
}

func AdminUpdateTaskStatusHandler(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var body struct {
		Status     string `json:"status" binding:"required"`
		Resolution string `json:"resolution"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	if err := UpdateTaskStatus(uint(id), body.Status, body.Resolution, adminActor(c)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "updated"})
}

func AdminRunPeriodicChecksHandler(c *gin.Context) {
	created := RunPeriodicComplianceChecks()
	c.JSON(http.StatusOK, gin.H{"tasksCreated": created})
}

// --- Exports (item L) — requires REPORT_EXPORT. This generic endpoint
// exports whatever tabular data the caller already has visibility into
// via another report endpoint; it does not itself decide what's
// sensitive — the RBAC gate on the underlying data view is what already
// controls that. ---

func AdminRequestExportHandler(c *gin.Context) {
	var body struct {
		ReportType string     `json:"reportType" binding:"required"`
		Format     string     `json:"format" binding:"required"`
		Parameters string     `json:"parameters"`
		Headers    []string   `json:"headers"`
		Rows       [][]string `json:"rows"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	export, err := RequestExport(body.ReportType, body.Format, body.Parameters, adminIDFrom(c), adminActor(c), body.Headers, body.Rows)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusAccepted, export)
}

// AdminExportControlledReportHandler (Phase 8/9 gap closure) — exports
// one of the named, controlled report types (registers.go) built
// server-side from the authoritative source, never from client-supplied
// rows. reportType is a path param, from/to are query params (default:
// trailing 30 days). This is the endpoint every Finance/Payments admin
// screen's "Export" button should call for the report types listed in
// BuildRegisterReport — the older generic AdminRequestExportHandler
// remains for ad hoc admin-computed views that don't have (yet) a
// controlled builder.
func AdminExportControlledReportHandler(c *gin.Context) {
	reportType := c.Param("reportType")
	from := time.Now().AddDate(0, 0, -30)
	to := time.Now()
	if v := c.Query("from"); v != "" {
		if t, err := time.Parse("2006-01-02", v); err == nil {
			from = t
		}
	}
	if v := c.Query("to"); v != "" {
		if t, err := time.Parse("2006-01-02", v); err == nil {
			to = t
		}
	}
	format := c.DefaultQuery("format", models.ExportFormatExcel)

	report, ok := BuildRegisterReport(reportType, from, to)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"message": "Unknown or unsupported report type: " + reportType})
		return
	}

	meta := ExportMetadata{
		ReportName: reportType, GeneratedBy: adminActor(c),
		ReportingPeriod: from.Format("2006-01-02") + " to " + to.Format("2006-01-02"),
		Filters:         "from=" + from.Format("2006-01-02") + ", to=" + to.Format("2006-01-02"),
		RecordCount:     len(report.Rows), Totals: report.Totals,
	}
	export, err := RequestExportWithMetadata(reportType, format, meta.Filters, adminIDFrom(c), adminActor(c), report.Headers, report.Rows, meta)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusAccepted, export)
}

// AdminGatewayReconciliationSummaryHandler — Phase 9 item I.
func AdminGatewayReconciliationSummaryHandler(c *gin.Context) {
	from := time.Now().AddDate(0, 0, -30)
	to := time.Now()
	if v := c.Query("from"); v != "" {
		if t, err := time.Parse("2006-01-02", v); err == nil {
			from = t
		}
	}
	if v := c.Query("to"); v != "" {
		if t, err := time.Parse("2006-01-02", v); err == nil {
			to = t
		}
	}
	c.JSON(http.StatusOK, GatewayReconciliationSummary(from, to))
}

func AdminGetExportHandler(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var export models.ReportExport
	if err := database.DB.First(&export, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"message": "Export not found"})
		return
	}
	c.JSON(http.StatusOK, export)
}

func AdminListExportsHandler(c *gin.Context) {
	var exports []models.ReportExport
	database.DB.Order("id desc").Limit(100).Find(&exports)
	c.JSON(http.StatusOK, exports)
}

// --- Retention policy (item M) ---

func AdminListRetentionHandler(c *gin.Context) {
	c.JSON(http.StatusOK, ListRetentionPolicies())
}

func AdminConfigureRetentionHandler(c *gin.Context) {
	var body struct {
		RecordCategory string `json:"recordCategory" binding:"required"`
		RetentionYears int    `json:"retentionYears"`
		LegalBasisNote string `json:"legalBasisNote"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	policy, err := ConfigureRetentionPolicy(body.RecordCategory, body.RetentionYears, body.LegalBasisNote, adminActor(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, policy)
}

func AdminValidateRetentionHandler(c *gin.Context) {
	category := c.Param("category")
	if err := ValidateRetentionPolicy(category, adminActor(c)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "validated"})
}

// AdminDownloadExportHandler streams a completed export's bytes to the
// caller (item — admin panel "downloadable" reports). Kept separate from
// AdminGetExportHandler (status/metadata only) so the JSON status-poll
// path and the byte-download path stay distinct — a client polls status
// first, then downloads once READY, rather than every status check
// re-reading a potentially large file from storage.
func AdminDownloadExportHandler(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var export models.ReportExport
	if err := database.DB.First(&export, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"message": "Export not found"})
		return
	}
	if export.Status != models.ExportStatusReady {
		c.JSON(http.StatusConflict, gin.H{"message": "Export is not ready yet", "status": export.Status})
		return
	}
	if export.StorageReference == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Export has no stored file"})
		return
	}
	provider := storage.Active()
	if provider == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "No storage provider configured"})
		return
	}
	data, err := provider.Retrieve(export.StorageReference)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Failed to retrieve export: " + err.Error()})
		return
	}

	ext := map[string]string{models.ExportFormatExcel: "xlsx", models.ExportFormatCSV: "csv", models.ExportFormatPDF: "pdf"}[export.Format]
	mime := map[string]string{
		models.ExportFormatExcel: "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
		models.ExportFormatCSV:   "text/csv",
		models.ExportFormatPDF:   "application/pdf",
	}[export.Format]
	if ext == "" {
		ext = "bin"
	}
	if mime == "" {
		mime = "application/octet-stream"
	}
	filename := fmt.Sprintf("%s_%s.%s", export.ReportType, export.RequestedAt.Format("20060102_150405"), ext)

	// Every download of a completed export is itself an auditable event
	// — who pulled which report's bytes and when.
	audit.RecordDetailed(nil, audit.Entry{
		ActorType: "admin", ActorID: adminIDFrom(c), ActorRef: adminActor(c),
		Action: "report.export.downloaded", TargetType: "report_export", TargetID: itoa(export.ID),
		Result: audit.ResultSuccess, Details: export.ReportType,
	})

	c.Header("Content-Disposition", "attachment; filename=\""+filename+"\"")
	c.Data(http.StatusOK, mime, data)
}

// AdminPreviewReportHandler returns a JSON preview of a controlled
// report — headers, totals, record count, and the first 25 rows — so an
// admin panel can show what a report contains before committing to a
// full export/download. Deliberately separate from the export pipeline:
// this never touches storage or the async export/audit path, since a
// preview is not itself a data export.
func AdminPreviewReportHandler(c *gin.Context) {
	reportType := c.Param("reportType")
	from := time.Now().AddDate(0, 0, -30)
	to := time.Now()
	if v := c.Query("from"); v != "" {
		if t, err := time.Parse("2006-01-02", v); err == nil {
			from = t
		}
	}
	if v := c.Query("to"); v != "" {
		if t, err := time.Parse("2006-01-02", v); err == nil {
			to = t
		}
	}

	report, ok := BuildRegisterReport(reportType, from, to)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"message": "Unknown or unsupported report type: " + reportType})
		return
	}

	const sampleLimit = 25
	sample := report.Rows
	truncated := false
	if len(sample) > sampleLimit {
		sample = sample[:sampleLimit]
		truncated = true
	}

	c.JSON(http.StatusOK, gin.H{
		"reportType":  reportType,
		"periodFrom":  from.Format("2006-01-02"),
		"periodTo":    to.Format("2006-01-02"),
		"headers":     report.Headers,
		"totals":      report.Totals,
		"recordCount": len(report.Rows),
		"sampleRows":  sample,
		"truncated":   truncated,
	})
}
