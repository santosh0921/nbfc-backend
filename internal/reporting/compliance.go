// Compliance Health Dashboard (item I) — aggregates existing signals from
// every prior phase's own tables; introduces no new source of truth.
package reporting

import (
	"time"

	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/models"
)

type SecurityHealth struct {
	FailedAdminLogins24h  int64 `json:"failedAdminLogins24h"`
	UnauthorizedAttempts24h int64 `json:"unauthorizedAttempts24h"`
	PrivilegedActions24h  int64 `json:"privilegedActions24h"`
	InactiveAdmins        int64 `json:"inactiveAdmins"` // no login in 90+ days, still ACTIVE
}

type KYCHealth struct {
	Pending  int64 `json:"pending"`
	Failures int64 `json:"failures"`
	RequiresReview int64 `json:"requiresReview"`
}

type CreditHealth struct {
	BureauFailures  int64 `json:"bureauFailures"`
	MissingConsent  int64 `json:"missingConsent"`
	ManualOverrides int64 `json:"manualOverrides"`
}

type LoansHealth struct {
	ApplicationsPending int64 `json:"applicationsPending"`
	ApprovalsPending    int64 `json:"approvalsPending"`
	DisbursementExceptions int64 `json:"disbursementExceptions"`
}

type PaymentsHealth struct {
	FailedPayments   int64 `json:"failedPayments"`
	UnmatchedPayments int64 `json:"unmatchedPayments"`
	DuplicateEvents  int64 `json:"duplicateEvents"`
	Refunds          int64 `json:"refunds"`
}

type AccountingHealth struct {
	UnreconciledTransactions int64 `json:"unreconciledTransactions"`
	ClosedPeriodExceptions   int64 `json:"closedPeriodExceptions"`
}

type RiskHealth struct {
	DPDExceptions          int64 `json:"dpdExceptions"`
	NPAChanges7d           int64 `json:"npaChanges7d"`
	ProvisioningExceptions int64 `json:"provisioningExceptions"`
}

type RegulatoryHealth struct {
	ReportsPending      int64 `json:"reportsPending"`
	ReportsWithErrors   int64 `json:"reportsWithErrors"`
	AwaitingChecker     int64 `json:"awaitingChecker"`
	SubmissionsPending  int64 `json:"submissionsPending"`
}

type ComplianceHealthDashboard struct {
	Security   SecurityHealth   `json:"security"`
	KYC        KYCHealth        `json:"kyc"`
	Credit     CreditHealth     `json:"credit"`
	Loans      LoansHealth      `json:"loans"`
	Payments   PaymentsHealth   `json:"payments"`
	Accounting AccountingHealth `json:"accounting"`
	Risk       RiskHealth       `json:"risk"`
	Regulatory RegulatoryHealth `json:"regulatory"`
	GeneratedAt time.Time       `json:"generatedAt"`
}

func GenerateComplianceHealthDashboard() ComplianceHealthDashboard {
	return ComplianceHealthDashboard{
		Security: securityHealth(), KYC: kycHealth(), Credit: creditHealth(),
		Loans: loansHealth(), Payments: paymentsHealth(), Accounting: accountingHealth(),
		Risk: riskHealth(), Regulatory: regulatoryHealth(), GeneratedAt: time.Now(),
	}
}

func securityHealth() SecurityHealth {
	var s SecurityHealth
	since := time.Now().Add(-24 * time.Hour)
	database.DB.Model(&models.AuditLog{}).
		Where("action = ? AND created_at >= ?", "admin.login.failed", since).Count(&s.FailedAdminLogins24h)
	database.DB.Model(&models.AuditLog{}).
		Where("result = ? AND created_at >= ?", "denied", since).Count(&s.UnauthorizedAttempts24h)
	database.DB.Model(&models.AuditLog{}).
		Where("actor_type = ? AND created_at >= ?", "admin", since).Count(&s.PrivilegedActions24h)
	cutoff := time.Now().AddDate(0, 0, -90)
	database.DB.Model(&models.AdminUser{}).
		Where("status = ? AND (last_login_at IS NULL OR last_login_at < ?)", models.AdminStatusActive, cutoff).
		Count(&s.InactiveAdmins)
	return s
}

func kycHealth() KYCHealth {
	var k KYCHealth
	database.DB.Model(&models.CustomerKYC{}).Where("status IN ?",
		[]string{models.KYCStatusInitiated, models.KYCStatusDocumentsPending, models.KYCStatusVerificationPending}).Count(&k.Pending)
	database.DB.Model(&models.CustomerKYC{}).Where("status = ?", models.KYCStatusRejected).Count(&k.Failures)
	database.DB.Model(&models.CustomerKYC{}).Where("status = ?", models.KYCStatusRequiresReview).Count(&k.RequiresReview)
	return k
}

func creditHealth() CreditHealth {
	var c CreditHealth
	database.DB.Model(&models.BureauRequest{}).Where("status = ?", models.BureauRequestStatusFailed).Count(&c.BureauFailures)
	database.DB.Model(&models.BureauRequest{}).Where("status = ?", models.BureauRequestStatusBlocked).Count(&c.MissingConsent)
	database.DB.Model(&models.LoanClassificationHistory{}).Where("approved_by != ''").Count(&c.ManualOverrides)
	return c
}

func loansHealth() LoansHealth {
	var l LoansHealth
	database.DB.Model(&models.LoanApplication{}).Where("status = ?", models.LoanStatusSubmitted).Count(&l.ApplicationsPending)
	database.DB.Model(&models.LoanApplication{}).Where("status = ?", models.LoanStatusPendingSanctionApproval).Count(&l.ApprovalsPending)
	database.DB.Model(&models.Disbursement{}).Where("status = ?", models.DisbursementStatusFailed).Count(&l.DisbursementExceptions)
	return l
}

func paymentsHealth() PaymentsHealth {
	var p PaymentsHealth
	since := time.Now().AddDate(0, 0, -7)
	database.DB.Model(&models.Payment{}).Where("status = ? AND created_at >= ?", models.PaymentStatusFailed, since).Count(&p.FailedPayments)
	database.DB.Model(&models.ReconciliationException{}).Where("resolved_at IS NULL").Count(&p.UnmatchedPayments)
	database.DB.Model(&models.PaymentEvent{}).Where("processed_note LIKE ?", "duplicate%").Count(&p.DuplicateEvents)
	database.DB.Model(&models.Refund{}).Count(&p.Refunds)
	return p
}

func accountingHealth() AccountingHealth {
	var a AccountingHealth
	database.DB.Model(&models.ReconciliationException{}).Where("resolved_at IS NULL").Count(&a.UnreconciledTransactions)
	database.DB.Model(&models.DataConsistencyException{}).Where("status = ?", models.ExceptionStatusOpen).Count(&a.ClosedPeriodExceptions)
	return a
}

func riskHealth() RiskHealth {
	var r RiskHealth
	database.DB.Model(&models.DataConsistencyException{}).
		Where("status = ? AND exception_type IN ?", models.ExceptionStatusOpen,
			[]string{models.ExceptionTypeNegativeOutstanding, models.ExceptionTypePaidStillOverdue}).Count(&r.DPDExceptions)
	since := time.Now().AddDate(0, 0, -7)
	database.DB.Model(&models.LoanClassificationHistory{}).
		Where("new_classification IN ? AND created_at >= ?", []string{"SUBSTANDARD", "DOUBTFUL", "LOSS"}, since).Count(&r.NPAChanges7d)
	database.DB.Model(&models.DataConsistencyException{}).
		Where("status = ? AND exception_type IN ?", models.ExceptionStatusOpen,
			[]string{models.ExceptionTypeDuplicateProvision, models.ExceptionTypeMissingJournalRef}).Count(&r.ProvisioningExceptions)
	return r
}

func regulatoryHealth() RegulatoryHealth {
	var reg RegulatoryHealth
	database.DB.Model(&models.RegulatoryReportRun{}).Where("status = ?", models.RegReportStatusGenerated).Count(&reg.ReportsPending)
	database.DB.Model(&models.RegulatoryReportRun{}).
		Where("id IN (SELECT run_id FROM regulatory_report_exceptions WHERE severity = ?)", models.ReportValidationError).
		Count(&reg.ReportsWithErrors)
	database.DB.Model(&models.RegulatoryReportRun{}).Where("status = ?", models.RegReportStatusUnderReview).Count(&reg.AwaitingChecker)
	database.DB.Model(&models.RegulatoryReportRun{}).Where("status = ?", models.RegReportStatusApproved).Count(&reg.SubmissionsPending)
	return reg
}
