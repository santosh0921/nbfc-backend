package alerts

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/models"
)

const (
	staleVerificationDays = 3
	staleRecoveryDays     = 5
)

type staleVerification struct {
	models.LoanApplication
	DaysSinceAssigned int `json:"daysSinceAssigned"`
}

type staleRecoveryCase struct {
	models.RecoveryCase
	DaysSinceLastAction int `json:"daysSinceLastAction"`
}

// AdminSlaAlertsHandler handles GET /admin/sla-alerts
func AdminSlaAlertsHandler(c *gin.Context) {
	now := time.Now()

	var assignedLoans []models.LoanApplication
	database.DB.Where("status = ?", models.LoanStatusAssigned).Find(&assignedLoans)

	staleVerifications := make([]staleVerification, 0)
	for _, loan := range assignedLoans {
		days := int(now.Sub(loan.UpdatedAt).Hours() / 24)
		if days > staleVerificationDays {
			staleVerifications = append(staleVerifications, staleVerification{LoanApplication: loan, DaysSinceAssigned: days})
		}
	}

	var cases []models.RecoveryCase
	database.DB.Where("status != ?", "resolved").Find(&cases)

	staleRecoveryCases := make([]staleRecoveryCase, 0)
	for _, rc := range cases {
		lastAction := rc.CreatedAt

		var lastReport models.RecoveryReport
		if err := database.DB.Where("case_id = ?", rc.ID).Order("created_at desc").First(&lastReport).Error; err == nil {
			if lastReport.CreatedAt.After(lastAction) {
				lastAction = lastReport.CreatedAt
			}
		}
		var lastNote models.CaseNote
		if err := database.DB.Where("case_id = ?", rc.ID).Order("created_at desc").First(&lastNote).Error; err == nil {
			if lastNote.CreatedAt.After(lastAction) {
				lastAction = lastNote.CreatedAt
			}
		}

		days := int(now.Sub(lastAction).Hours() / 24)
		if days > staleRecoveryDays {
			staleRecoveryCases = append(staleRecoveryCases, staleRecoveryCase{RecoveryCase: rc, DaysSinceLastAction: days})
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"staleVerifications":  staleVerifications,
		"staleRecoveryCases": staleRecoveryCases,
	})
}
