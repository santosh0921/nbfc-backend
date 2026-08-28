package admin

import (
	"log"

	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/models"
)

func Migrate() {

	err := database.DB.AutoMigrate(
		&models.CustomerKYCReview{},
		&models.Agency{},
		&models.Employee{},
		&models.LoanApplication{},
		&models.LoanReference{},
		&models.RecoveryCase{},
		&models.RecoveryReport{},
		&models.CaseNote{},
		&models.Notification{},
		&models.Document{},
		&models.EmiInstallment{},
		&models.AuditLog{},
		&models.SupportThread{},
		&models.SupportMessage{},
		&models.Complaint{},
		&models.AdminUser{},
		&models.AdminSession{},
		&models.Payment{},
		&models.PaymentEvent{},
		&models.Refund{},
		&models.Disbursement{},
		&models.BankAccount{},
		&models.BankStatementImport{},
		&models.BankTransaction{},
		&models.ReconciliationRecord{},
		&models.ReconciliationException{},
		&models.CustomerKYC{},
		&models.BureauConsent{},
		&models.BureauRequest{},
		&models.CreditDecision{},
		&models.CKYCRequest{},
		&models.ScreeningRequest{},
		&models.Declaration{},
		&models.CustomerDeclarationResponse{},
		&models.LedgerAccount{},
		&models.AccountingPeriod{},
		&models.JournalEntry{},
		&models.JournalEntryLine{},
		&models.InterestAccrual{},
		&models.GSTTransaction{},
		&models.TaxInvoice{},
		&models.TDSTransaction{},
		&models.FundingSource{},
		&models.BorrowingTransaction{},
		&models.DPDRecord{},
		&models.DelinquencyBucket{},
		&models.ClassificationRule{},
		&models.LoanClassification{},
		&models.LoanClassificationHistory{},
		&models.ProvisioningRule{},
		&models.LoanProvision{},
		&models.RiskJobRun{},
		&models.RiskAlert{},
		&models.CollectionCase{},
		&models.CollectionAssignment{},
		&models.CollectionAction{},
		&models.CollectionNote{},
		&models.CollectionPromise{},
		&models.WriteOffRequest{},
		&models.RecoveryPayment{},
		&models.DataConsistencyException{},
		&models.ReportRun{},
		&models.RegulatoryReportDefinition{},
		&models.RegulatoryReportField{},
		&models.RegulatoryReportVersion{},
		&models.RegulatoryReportRun{},
		&models.RegulatoryReportReview{},
		&models.RegulatoryReportSubmission{},
		&models.RegulatoryReportException{},
		&models.CrossSystemReconciliation{},
		&models.ComplianceTask{},
		&models.ReportExport{},
		&models.RetentionPolicy{},
		&models.Vendor{},
		&models.Expense{},
		&models.TDSRule{},
		&models.TDSDeductee{},
		&models.TDSChallan{},
		&models.TDSException{},
		&models.TaxComplianceCalendarItem{},
		&models.TDSReturnFiling{},
		&models.NBFCProfile{},
		&models.ComplianceRequirement{},
		&models.ComplianceCalendarItem{},
		&models.ComplianceReminderConfig{},
		&models.RegulatoryReturnTracking{},
		&models.RegulatoryDataException{},
		&models.ComplianceEvidence{},
		&models.Policy{},
		&models.InternalAudit{},
		&models.AuditFinding{},
		&models.CAPA{},
		&models.ComplianceAlert{},

		// Phase 13 — analytics / management intelligence.
		&models.RiskSignal{},
		&models.ManagementThreshold{},
		&models.ManagementAlert{},
		&models.KPIDefinition{},
		&models.Budget{},
		&models.AnalyticsDataQualityException{},

		// Phase 16.1 — scheduled job execution records.
		&models.JobExecution{},
	)

	if err != nil {
		log.Fatal("Admin KYC review migration failed:", err)
	}

	log.Println("✅ Admin KYC review table migrated successfully")

	enforceAppendOnlyAuditLog()
}

// enforceAppendOnlyAuditLog installs a database-level trigger that rejects
// any UPDATE or DELETE against audit_logs, so tampering with the audit
// trail requires dropping the trigger itself (a schema-level operation,
// not something an application bug or a compromised admin session's API
// access can do) rather than a plain row edit. Idempotent — safe to run
// on every startup.
//
// This is a meaningful control, not a complete one: a Postgres role with
// enough privilege to run DDL (e.g. the same migration user) could still
// drop the trigger. True tamper-evidence (e.g. a hash chain, or shipping
// rows to a separate write-only store/WORM storage) is out of scope for
// this phase — see the Phase 1 report's "remaining P0 gaps" section.
func enforceAppendOnlyAuditLog() {
	stmts := []string{
		`CREATE OR REPLACE FUNCTION reject_audit_log_mutation() RETURNS trigger AS $$
		BEGIN
			RAISE EXCEPTION 'audit_logs is append-only: % is not permitted', TG_OP;
		END;
		$$ LANGUAGE plpgsql;`,
		`DROP TRIGGER IF EXISTS audit_logs_append_only ON audit_logs;`,
		`CREATE TRIGGER audit_logs_append_only
			BEFORE UPDATE OR DELETE ON audit_logs
			FOR EACH ROW EXECUTE FUNCTION reject_audit_log_mutation();`,
	}
	for _, stmt := range stmts {
		if err := database.DB.Exec(stmt).Error; err != nil {
			log.Println("⚠️  Failed to enforce append-only audit_logs trigger (audit log remains mutable at the DB level):", err)
			return
		}
	}
	log.Println("✅ audit_logs is now append-only at the database level (UPDATE/DELETE rejected by trigger)")
}
