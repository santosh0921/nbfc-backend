package mis

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

func resolveRange(c *gin.Context) (from, to time.Time) {
	now := time.Now()
	if preset := c.Query("preset"); preset != "" {
		return DateRangePreset(preset, now)
	}
	from = now.AddDate(0, -1, 0)
	to = now
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
	return from, to
}

func AdminExecutiveDashboardHandler(c *gin.Context) {
	from, to := resolveRange(c)
	c.JSON(http.StatusOK, GenerateExecutiveDashboard(from, to))
}

func AdminGrowthReportHandler(c *gin.Context) {
	from, to := resolveRange(c)
	c.JSON(http.StatusOK, GenerateGrowthReport(from, to))
}

func AdminFunnelReportHandler(c *gin.Context) {
	from, to := resolveRange(c)
	c.JSON(http.StatusOK, GenerateFunnelReport(from, to))
}

func AdminUnitEconomicsHandler(c *gin.Context) {
	from, to := resolveRange(c)
	c.JSON(http.StatusOK, GenerateUnitEconomics(from, to))
}

func AdminFinancialYearHandler(c *gin.Context) {
	now := time.Now()
	start, end, label := FinancialYearRange(now)
	c.JSON(http.StatusOK, gin.H{"start": start, "end": end, "label": label})
}
