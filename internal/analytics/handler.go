package analytics

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/mis"
	"github.com/santosh0921/nbfc-backend/internal/models"
)

// resolveRange reuses Phase 12's preset vocabulary so analytics and MIS
// cannot interpret the same "?preset=this_quarter" differently.
func resolveRange(c *gin.Context) (time.Time, time.Time) {
	now := time.Now()
	if preset := c.Query("preset"); preset != "" {
		return mis.DateRangePreset(preset, now)
	}
	from, errF := time.Parse("2006-01-02", c.Query("from"))
	to, errT := time.Parse("2006-01-02", c.Query("to"))
	if errF != nil || errT != nil {
		return now.AddDate(0, -1, 0), now
	}
	return from, to.Add(24*time.Hour - time.Second)
}

func actor(c *gin.Context) string {
	if v, ok := c.Get("admin_username"); ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func AdminGrowthDashboardHandler(c *gin.Context) {
	from, to := resolveRange(c)
	c.JSON(http.StatusOK, GenerateGrowthDashboard(from, to))
}

func AdminMoMHandler(c *gin.Context) {
	c.JSON(http.StatusOK, MonthOverMonth(time.Now()))
}

func AdminYoYHandler(c *gin.Context) {
	c.JSON(http.StatusOK, YearOverYear(time.Now()))
}

func AdminGrowthQualityHandler(c *gin.Context) {
	from, to := resolveRange(c)
	c.JSON(http.StatusOK, GenerateGrowthQuality(from, to))
}

func AdminFunnelHandler(c *gin.Context) {
	from, to := resolveRange(c)
	c.JSON(http.StatusOK, GenerateFunnel(from, to))
}

func AdminProductAnalyticsHandler(c *gin.Context) {
	from, to := resolveRange(c)
	c.JSON(http.StatusOK, ProductAnalytics(from, to))
}

func AdminSegmentAnalyticsHandler(c *gin.Context) {
	from, to := resolveRange(c)
	c.JSON(http.StatusOK, SegmentAnalytics(from, to))
}

func AdminGeographicAnalyticsHandler(c *gin.Context) {
	from, to := resolveRange(c)
	c.JSON(http.StatusOK, GeographicAnalytics(from, to))
}

func AdminChannelAnalyticsHandler(c *gin.Context) {
	from, to := resolveRange(c)
	c.JSON(http.StatusOK, ChannelAnalytics(from, to))
}

func AdminPortfolioRiskHandler(c *gin.Context) {
	from, to := resolveRange(c)
	c.JSON(http.StatusOK, GeneratePortfolioRisk(from, to))
}

func AdminDPDTrendHandler(c *gin.Context) {
	months, _ := strconv.Atoi(c.DefaultQuery("months", "6"))
	c.JSON(http.StatusOK, GenerateDPDTrend(time.Now(), months))
}

func AdminCollectionRiskHandler(c *gin.Context) {
	from, to := resolveRange(c)
	c.JSON(http.StatusOK, GenerateCollectionRisk(from, to))
}

func AdminConcentrationHandler(c *gin.Context) {
	c.JSON(http.StatusOK, GenerateConcentration(time.Now()))
}

func AdminOutliersHandler(c *gin.Context) {
	from, to := resolveRange(c)
	c.JSON(http.StatusOK, DetectOutliers(from, to))
}

func AdminRevenueQualityHandler(c *gin.Context) {
	from, to := resolveRange(c)
	c.JSON(http.StatusOK, GenerateRevenueQuality(from, to))
}

func AdminExpenseEfficiencyHandler(c *gin.Context) {
	from, to := resolveRange(c)
	c.JSON(http.StatusOK, GenerateExpenseEfficiency(from, to))
}

func AdminProfitabilityHandler(c *gin.Context) {
	from, to := resolveRange(c)
	c.JSON(http.StatusOK, GenerateProfitability(from, to))
}

func AdminCohortsHandler(c *gin.Context) {
	months, _ := strconv.Atoi(c.DefaultQuery("months", "12"))
	c.JSON(http.StatusOK, GenerateCohorts(months))
}

func AdminRetentionHandler(c *gin.Context) {
	c.JSON(http.StatusOK, GenerateRetention())
}

func AdminDecisionCenterHandler(c *gin.Context) {
	from, to := resolveRange(c)
	c.JSON(http.StatusOK, GenerateDecisionCenter(from, to))
}

func AdminKPIDictionaryHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"kpis": ListKPIs()})
}

func AdminBudgetVarianceHandler(c *gin.Context) {
	from, to := resolveRange(c)
	c.JSON(http.StatusOK, GenerateBudgetVariance(from, to))
}

// --- Risk signals ---

func AdminListSignalsHandler(c *gin.Context) {
	var signals []models.RiskSignal
	q := database.DB.Order("observed_at DESC").Limit(200)
	if status := c.Query("status"); status != "" {
		q = q.Where("status = ?", status)
	}
	q.Find(&signals)
	c.JSON(http.StatusOK, gin.H{"signals": signals})
}

func AdminRunSignalDetectionHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"newSignals": RunSignalDetection(time.Now())})
}

func AdminReviewSignalHandler(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid signal id"})
		return
	}
	var body struct {
		Status string `json:"status"`
		Note   string `json:"note"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	if err := ReviewSignal(uint(id), body.Status, actor(c), body.Note); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "updated"})
}

// --- Thresholds ---

func AdminListThresholdsHandler(c *gin.Context) {
	var rows []models.ManagementThreshold
	database.DB.Order("metric_key").Find(&rows)
	c.JSON(http.StatusOK, gin.H{"thresholds": rows})
}

func AdminCreateThresholdHandler(c *gin.Context) {
	var body models.ManagementThreshold
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	if body.MetricKey == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "metricKey is required"})
		return
	}
	// A threshold may only be attached to a documented KPI, so no alert
	// can ever fire on an undefined number.
	var kpi models.KPIDefinition
	if err := database.DB.Where("key = ?", body.MetricKey).First(&kpi).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "metricKey must match a KPI in the dictionary — see GET /admin/analytics/kpi-dictionary",
		})
		return
	}
	if body.Direction != models.ThresholdDirectionAbove && body.Direction != models.ThresholdDirectionBelow {
		c.JSON(http.StatusBadRequest, gin.H{"error": "direction must be ABOVE or BELOW"})
		return
	}
	body.ID = 0
	body.CreatedBy = actor(c)
	if err := database.DB.Create(&body).Error; err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "a threshold already exists for this metric"})
		return
	}
	c.JSON(http.StatusCreated, body)
}

func AdminEvaluateThresholdsHandler(c *gin.Context) {
	from, to := resolveRange(c)
	c.JSON(http.StatusOK, gin.H{"newAlerts": EvaluateThresholds(from, to)})
}

func AdminResolveAlertHandler(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid alert id"})
		return
	}
	var body struct {
		Status string `json:"status"`
		Note   string `json:"note"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	if err := ResolveAlert(uint(id), body.Status, actor(c), body.Note); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "updated"})
}

// --- Data quality ---

func AdminListDataQualityHandler(c *gin.Context) {
	var rows []models.AnalyticsDataQualityException
	q := database.DB.Order("detected_at DESC").Limit(200)
	if status := c.Query("status"); status != "" {
		q = q.Where("status = ?", status)
	}
	q.Find(&rows)
	c.JSON(http.StatusOK, gin.H{"exceptions": rows})
}

func AdminRunDataQualityHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"newExceptions": RunDataQualityChecks(time.Now())})
}
