package risk

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/santosh0921/nbfc-backend/internal/customerinfo"
	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/models"
)

// riskAlertRow adds the loan applicant's real name to the raw RiskAlert
// row. RiskAlert only references a LoanID (not a customer ID directly), so
// this looks up LoanApplication.ApplicantName — already a plain stored
// field, no further join needed. Omitted when LoanID is nil (a
// non-loan-specific alert, e.g. a job-failure alert) or the loan can't be
// found — never fabricated.
type riskAlertRow struct {
	models.RiskAlert
	ApplicantName string `json:"applicantName,omitempty"`
}

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

// --- Delinquency buckets (item C) ---

func AdminListBucketsHandler(c *gin.Context) {
	var buckets []models.DelinquencyBucket
	database.DB.Order("priority").Find(&buckets)
	c.JSON(http.StatusOK, buckets)
}

func AdminCreateBucketHandler(c *gin.Context) {
	var body models.DelinquencyBucket
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	if body.Name == "" || body.RuleVersion == "" {
		c.JSON(http.StatusBadRequest, gin.H{"message": "name and ruleVersion are required"})
		return
	}
	if body.Status == "" {
		body.Status = models.BucketStatusActive
	}
	if body.EffectiveFrom.IsZero() {
		body.EffectiveFrom = time.Now()
	}
	database.DB.Create(&body)
	c.JSON(http.StatusCreated, body)
}

// --- Classification rules (item D/S) — maker-checker ---

func AdminListClassificationRulesHandler(c *gin.Context) {
	var rules []models.ClassificationRule
	database.DB.Order("id desc").Find(&rules)
	c.JSON(http.StatusOK, rules)
}

// AdminDraftClassificationRuleHandler — requires RISK_RULE_MANAGE. Creates
// a DRAFT rule, unusable by ClassifyLoan until approved (item D/S/U#7).
func AdminDraftClassificationRuleHandler(c *gin.Context) {
	var body models.ClassificationRule
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	if body.RuleVersion == "" || body.Classification == "" {
		c.JSON(http.StatusBadRequest, gin.H{"message": "ruleVersion and classification are required"})
		return
	}
	body.Status = models.ClassificationStatusDraft
	body.CreatedBy = adminActor(c)
	body.ApprovedBy = ""
	body.ApprovalDate = nil
	if body.EffectiveDate.IsZero() {
		body.EffectiveDate = time.Now()
	}
	database.DB.Create(&body)
	c.JSON(http.StatusCreated, body)
}

// AdminApproveClassificationRuleHandler — requires RISK_RULE_APPROVE.
// Maker != checker enforced here (item D/S).
func AdminApproveClassificationRuleHandler(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var rule models.ClassificationRule
	if err := database.DB.First(&rule, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"message": "Rule not found"})
		return
	}
	if rule.Status != models.ClassificationStatusDraft {
		c.JSON(http.StatusBadRequest, gin.H{"message": "only a DRAFT rule can be approved"})
		return
	}
	actor := adminActor(c)
	if rule.CreatedBy == actor {
		c.JSON(http.StatusForbidden, gin.H{"message": "the admin who drafted this rule cannot also approve it"})
		return
	}
	now := time.Now()
	rule.Status = models.ClassificationStatusApproved
	rule.ApprovedBy = actor
	rule.ApprovalDate = &now
	database.DB.Save(&rule)
	c.JSON(http.StatusOK, rule)
}

// --- Provisioning rules (item G/S) — same maker-checker pattern ---

func AdminListProvisioningRulesHandler(c *gin.Context) {
	var rules []models.ProvisioningRule
	database.DB.Order("id desc").Find(&rules)
	c.JSON(http.StatusOK, rules)
}

func AdminDraftProvisioningRuleHandler(c *gin.Context) {
	var body models.ProvisioningRule
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	if body.RuleVersion == "" || body.Classification == "" {
		c.JSON(http.StatusBadRequest, gin.H{"message": "ruleVersion and classification are required"})
		return
	}
	body.Status = models.ClassificationStatusDraft
	body.CreatedBy = adminActor(c)
	body.ApprovedBy = ""
	body.ApprovalDate = nil
	if body.EffectiveDate.IsZero() {
		body.EffectiveDate = time.Now()
	}
	database.DB.Create(&body)
	c.JSON(http.StatusCreated, body)
}

func AdminApproveProvisioningRuleHandler(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var rule models.ProvisioningRule
	if err := database.DB.First(&rule, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"message": "Rule not found"})
		return
	}
	if rule.Status != models.ClassificationStatusDraft {
		c.JSON(http.StatusBadRequest, gin.H{"message": "only a DRAFT rule can be approved"})
		return
	}
	actor := adminActor(c)
	if rule.CreatedBy == actor {
		c.JSON(http.StatusForbidden, gin.H{"message": "the admin who drafted this rule cannot also approve it"})
		return
	}
	now := time.Now()
	rule.Status = models.ClassificationStatusApproved
	rule.ApprovedBy = actor
	rule.ApprovalDate = &now
	database.DB.Save(&rule)
	c.JSON(http.StatusOK, rule)
}

// --- Loan risk detail (item N) ---

func AdminLoanRiskHandler(c *gin.Context) {
	loanID, _ := strconv.Atoi(c.Param("loanId"))

	var latestDPD models.DPDRecord
	database.DB.Where("loan_id = ?", loanID).Order("calculation_date desc").First(&latestDPD)

	var dpdHistory []models.DPDRecord
	database.DB.Where("loan_id = ?", loanID).Order("calculation_date desc").Limit(90).Find(&dpdHistory)

	var classification models.LoanClassification
	database.DB.First(&classification, "loan_id = ?", loanID)

	var classificationHistory []models.LoanClassificationHistory
	database.DB.Where("loan_id = ?", loanID).Order("created_at desc").Find(&classificationHistory)

	var provisions []models.LoanProvision
	database.DB.Where("loan_id = ?", loanID).Order("calculation_date desc").Find(&provisions)

	var cases []models.CollectionCase
	database.DB.Where("loan_id = ?", loanID).Order("id desc").Find(&cases)

	currentDPD := 0
	if latestDPD.ID != 0 {
		var maxDPD int
		database.DB.Model(&models.DPDRecord{}).
			Where("loan_id = ? AND calculation_date = ?", loanID, latestDPD.CalculationDate).
			Select("COALESCE(MAX(dpd),0)").Scan(&maxDPD)
		currentDPD = maxDPD
	}

	c.JSON(http.StatusOK, gin.H{
		"currentDpd":             currentDPD,
		"dpdHistory":             dpdHistory,
		"classification":         classification,
		"classificationHistory":  classificationHistory,
		"provisions":             provisions,
		"collectionCases":        cases,
	})
}

// AdminOverrideClassificationHandler — requires CLASSIFICATION_OVERRIDE
// (item D). approvedBy is required in the body — a second admin's name
// confirming the override, since this endpoint alone can't enforce a
// second login the way the rule maker-checker flow does.
func AdminOverrideClassificationHandler(c *gin.Context) {
	loanID, _ := strconv.Atoi(c.Param("loanId"))
	var body struct {
		Classification string `json:"classification" binding:"required"`
		Reason         string `json:"reason" binding:"required"`
		ApprovedBy     string `json:"approvedBy" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	if err := ManualOverrideClassification(uint(loanID), body.Classification, body.Reason, adminActor(c), body.ApprovedBy); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "classification overridden"})
}

// --- Dashboard (item M) ---

func AdminPortfolioDashboardHandler(c *gin.Context) {
	f := DashboardFilters{Product: c.Query("product"), State: c.Query("state")}
	if from := c.Query("from"); from != "" {
		if t, err := time.Parse("2006-01-02", from); err == nil {
			f.From = &t
		}
	}
	if to := c.Query("to"); to != "" {
		if t, err := time.Parse("2006-01-02", to); err == nil {
			f.To = &t
		}
	}
	c.JSON(http.StatusOK, PortfolioDashboard(f))
}

// --- Daily job (item F) — admin-triggered; no cron infra exists in this
// codebase (see the Phase 5 report), so this endpoint is what an external
// scheduler (or an admin, manually) calls once a day. ---

func AdminRunDailyJobHandler(c *gin.Context) {
	asOf := time.Now()
	if dateStr := c.Query("date"); dateStr != "" {
		if t, err := time.Parse("2006-01-02", dateStr); err == nil {
			asOf = t
		}
	}
	run, err := RunDailyRiskJob(asOf)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, run)
}

func AdminListJobRunsHandler(c *gin.Context) {
	var runs []models.RiskJobRun
	database.DB.Order("run_date desc").Limit(90).Find(&runs)
	c.JSON(http.StatusOK, runs)
}

// --- Alerts (item O) ---

func AdminListAlertsHandler(c *gin.Context) {
	q := database.DB.Model(&models.RiskAlert{})
	if c.Query("all") != "1" {
		q = q.Where("status = ?", models.AlertStatusOpen)
	}
	var alerts []models.RiskAlert
	q.Order("id desc").Limit(200).Find(&alerts)

	loanIDs := make([]uint, 0, len(alerts))
	for _, a := range alerts {
		if a.LoanID != nil {
			loanIDs = append(loanIDs, *a.LoanID)
		}
	}
	names := customerinfo.LoanApplicantNames(loanIDs)

	out := make([]riskAlertRow, len(alerts))
	for i, a := range alerts {
		row := riskAlertRow{RiskAlert: a}
		if a.LoanID != nil {
			row.ApplicantName = names[*a.LoanID]
		}
		out[i] = row
	}
	c.JSON(http.StatusOK, out)
}

func AdminAcknowledgeAlertHandler(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	now := time.Now()
	database.DB.Model(&models.RiskAlert{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status": models.AlertStatusAcknowledged, "acknowledged_by": adminActor(c), "acknowledged_at": now,
	})
	c.JSON(http.StatusOK, gin.H{"message": "acknowledged"})
}

// --- Exceptions (item T) ---

func AdminListExceptionsHandler(c *gin.Context) {
	q := database.DB.Model(&models.DataConsistencyException{})
	if c.Query("all") != "1" {
		q = q.Where("status = ?", models.ExceptionStatusOpen)
	}
	var exceptions []models.DataConsistencyException
	q.Order("id desc").Limit(200).Find(&exceptions)
	c.JSON(http.StatusOK, exceptions)
}

func AdminResolveExceptionHandler(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var body struct {
		Note string `json:"note"`
	}
	c.ShouldBindJSON(&body)
	now := time.Now()
	database.DB.Model(&models.DataConsistencyException{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status": models.ExceptionStatusResolved, "resolved_by": adminActor(c), "resolved_at": now, "resolution_note": body.Note,
	})
	c.JSON(http.StatusOK, gin.H{"message": "resolved"})
}

func AdminRunConsistencyChecksHandler(c *gin.Context) {
	found := RunConsistencyChecks(time.Now())
	c.JSON(http.StatusOK, gin.H{"newExceptionsChecked": found})
}
