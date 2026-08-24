package accounting

import (
	"errors"
	"strconv"
	"time"

	"github.com/santosh0921/nbfc-backend/internal/audit"
	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/models"
)

var (
	ErrPeriodNotFound     = errors.New("accounting: period not found")
	ErrPeriodAlreadyClosed = errors.New("accounting: period is already closed")
	ErrPeriodNotClosed    = errors.New("accounting: only a closed period can be reopened")
)

// ClosePeriod (item M) — the ONLY way a period's Status becomes CLOSED.
// Once closed, PostJournalEntry refuses any new posting dated within it
// (see getOrCreatePeriodForDate/PostJournalEntry in service.go). Caller
// (the HTTP handler) is responsible for enforcing the accounting-period
// management permission before calling this.
func ClosePeriod(periodID uint, closedBy string) (*models.AccountingPeriod, error) {
	var period models.AccountingPeriod
	if err := database.DB.First(&period, periodID).Error; err != nil {
		return nil, ErrPeriodNotFound
	}
	if period.Status == models.PeriodStatusClosed {
		return nil, ErrPeriodAlreadyClosed
	}
	before := audit.Snapshot(period)
	now := time.Now()
	period.Status = models.PeriodStatusClosed
	period.ClosedBy = closedBy
	period.ClosedAt = &now
	if err := database.DB.Save(&period).Error; err != nil {
		return nil, err
	}
	audit.RecordDetailed(nil, audit.Entry{
		ActorType: "admin", ActorRef: closedBy,
		Action: "accounting.period.closed", TargetType: "accounting_period", TargetID: strconv.Itoa(int(period.ID)),
		Result: audit.ResultSuccess, Details: before, After: period,
	})
	return &period, nil
}

// ReopenPeriod requires an elevated permission (enforced by the caller —
// see rbac.PeriodManage / the admin route) and an explicit reason, always
// audited (item M: "reopening must generate an audit event and reason").
func ReopenPeriod(periodID uint, reopenedBy, reason string) (*models.AccountingPeriod, error) {
	if reason == "" {
		return nil, errors.New("accounting: a reason is required to reopen a closed period")
	}
	var period models.AccountingPeriod
	if err := database.DB.First(&period, periodID).Error; err != nil {
		return nil, ErrPeriodNotFound
	}
	if period.Status != models.PeriodStatusClosed {
		return nil, ErrPeriodNotClosed
	}
	before := audit.Snapshot(period)
	now := time.Now()
	period.Status = models.PeriodStatusReopened
	period.ReopenedBy = reopenedBy
	period.ReopenedAt = &now
	period.ReopenReason = reason
	if err := database.DB.Save(&period).Error; err != nil {
		return nil, err
	}
	audit.RecordDetailed(nil, audit.Entry{
		ActorType: "admin", ActorRef: reopenedBy,
		Action: "accounting.period.reopened", TargetType: "accounting_period", TargetID: strconv.Itoa(int(period.ID)),
		Result: audit.ResultSuccess, Reason: reason, Details: before, After: period,
	})
	return &period, nil
}
