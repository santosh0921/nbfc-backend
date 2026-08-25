package routes

import (
	"github.com/gin-gonic/gin"

	"github.com/santosh0921/nbfc-backend/internal/accounting"
	"github.com/santosh0921/nbfc-backend/internal/admin"
	"github.com/santosh0921/nbfc-backend/internal/agency"
	"github.com/santosh0921/nbfc-backend/internal/alerts"
	"github.com/santosh0921/nbfc-backend/internal/analytics"
	"github.com/santosh0921/nbfc-backend/internal/audit"
	"github.com/santosh0921/nbfc-backend/internal/auth"
	"github.com/santosh0921/nbfc-backend/internal/bureau"
	"github.com/santosh0921/nbfc-backend/internal/ckyc"
	"github.com/santosh0921/nbfc-backend/internal/collections"
	"github.com/santosh0921/nbfc-backend/internal/compliance"
	"github.com/santosh0921/nbfc-backend/internal/documents"
	"github.com/santosh0921/nbfc-backend/internal/employee"
	"github.com/santosh0921/nbfc-backend/internal/finance"
	"github.com/santosh0921/nbfc-backend/internal/integrity"
	"github.com/santosh0921/nbfc-backend/internal/jobs"
	"github.com/santosh0921/nbfc-backend/internal/kyc"
	"github.com/santosh0921/nbfc-backend/internal/loans"
	"github.com/santosh0921/nbfc-backend/internal/middleware"
	"github.com/santosh0921/nbfc-backend/internal/mis"
	"github.com/santosh0921/nbfc-backend/internal/notifications"
	"github.com/santosh0921/nbfc-backend/internal/otp"
	"github.com/santosh0921/nbfc-backend/internal/payments"
	"github.com/santosh0921/nbfc-backend/internal/profile"
	"github.com/santosh0921/nbfc-backend/internal/rbac"
	"github.com/santosh0921/nbfc-backend/internal/reconciliation"
	"github.com/santosh0921/nbfc-backend/internal/recovery"
	"github.com/santosh0921/nbfc-backend/internal/reporting"
	"github.com/santosh0921/nbfc-backend/internal/risk"
	"github.com/santosh0921/nbfc-backend/internal/screening"
	"github.com/santosh0921/nbfc-backend/internal/signaling"
	"github.com/santosh0921/nbfc-backend/internal/support"
	taxpkg "github.com/santosh0921/nbfc-backend/internal/tax"
	"github.com/santosh0921/nbfc-backend/internal/writeoff"
)

// perm is a short alias purely to keep the route table below readable —
// every admin-authenticated route is explicitly gated by exactly one
// backend permission check (internal/rbac), never left to the admin
// panel's own judgement about which buttons to show.
func perm(p rbac.Permission) gin.HandlerFunc {
	return middleware.RequirePermission(p)
}

func RegisterRoutes(router *gin.Engine) {

	// Rate limiters (Phase 7 item E): endpoint-specific, not one shared
	// global bucket — a burst of OTP requests should never eat into a
	// different customer's login attempts, and each limiter's own
	// violation is now audit-logged (internal/middleware/rate_limit.go's
	// NamedRateLimit) for security monitoring. Limits below are
	// conservative starting points, not values validated against real
	// production traffic — tune once real usage patterns are known.
	sendOTPLimit := middleware.NamedRateLimit("otp.send", 1, 5)
	verifyOTPLimit := middleware.NamedRateLimit("otp.verify", 1, 8) // slightly higher burst: legitimate typo retries shouldn't lock a customer out
	mpinLimit := middleware.NamedRateLimit("auth.create_mpin", 0.5, 3)
	loginLimit := middleware.NamedRateLimit("auth.login", 1, 5)
	adminLoginLimit := middleware.NamedRateLimit("admin.login", 0.5, 15)
	webhookLimit := middleware.NamedRateLimit("webhook.razorpay", 20, 40) // gateway can burst legitimately; this guards against abuse of the endpoint itself, not normal traffic
	kycLimit := middleware.NamedRateLimit("kyc.initiate", 1, 5)
	documentUploadLimit := middleware.NamedRateLimit("document.upload", 1, 10)
	loanSubmitLimit := middleware.NamedRateLimit("loan.submit", 0.5, 3)
	paymentOrderLimit := middleware.NamedRateLimit("payment.create", 1, 5)
	exportLimit := middleware.NamedRateLimit("report.export", 0.2, 3)

	// Public Routes
	//
	// verify-otp and create-mpin used to be the only two public auth
	// endpoints WITHOUT rate limiting — verify-otp guards OTP brute-force
	// (otp.VerifyOTP's 5-attempt cap was otherwise trivially bypassed by
	// just calling send-otp again for a fresh counter, since nothing
	// throttled how often that loop could repeat), and create-mpin is
	// effectively a password-reset endpoint for an existing account, which
	// should never be unthrottled regardless of what credential it checks.
	router.POST("/auth/send-otp", sendOTPLimit, otp.SendOTPHandler)
	router.POST("/auth/verify-otp", verifyOTPLimit, otp.VerifyOTPHandler)
	router.POST("/auth/create-mpin", mpinLimit, auth.CreateMPINHandler)
	router.POST("/auth/login", loginLimit, auth.LoginHandler)

	// Payment gateway webhook — deliberately outside any JWT-authenticated
	// group (the caller is the gateway, not a logged-in user); its own
	// request is proven authentic via HMAC signature verification inside
	// the handler itself, not a bearer token. See
	// internal/payments/webhook_handler.go. Rate-limited too — a
	// legitimate gateway is well within these limits; this guards against
	// someone hammering the public endpoint directly.
	router.POST("/webhooks/razorpay", webhookLimit, payments.RazorpayWebhookHandler)

	// Protected Routes
	authorized := router.Group("/auth")
	authorized.Use(middleware.AuthMiddleware())

	authorized.GET("/profile", auth.GetProfile)
	authorized.POST("/profile", profile.CreateProfileHandler)
	authorized.POST("/address", profile.CreateAddressHandler)
	authorized.POST("/pan", kycLimit, profile.CreatePANHandler)
	authorized.POST("/aadhaar", kycLimit, profile.CreateAadhaarHandler)
	authorized.POST("/selfie", profile.CreateSelfieHandler)
	authorized.GET("/kyc/status", profile.GetKYCStatusHandler)
	authorized.GET("/kyc/v2/status", kyc.CustomerStatusHandler)
	authorized.POST("/employment", profile.CreateEmploymentHandler)

	authorized.PUT("/biometric", auth.ToggleBiometric)

	authorized.POST("/documents", documentUploadLimit, documents.UploadCustomerDocumentHandler)
	authorized.GET("/documents", documents.ListCustomerDocumentsHandler)

	// Admin auth routes — no shared credential anymore, see internal/admin.
	router.POST("/admin/login", adminLoginLimit, admin.LoginHandler)
	router.POST("/admin/refresh", adminLoginLimit, admin.RefreshHandler)

	// General admin-API ceiling (item E: "Admin APIs") — a much higher
	// limit than the auth-specific ones above, since legitimate admin
	// panel usage (dashboards polling several endpoints) is bursty; this
	// exists to catch scripted abuse of an already-authenticated session,
	// not to throttle normal admin panel usage.
	adminAPILimit := middleware.NamedRateLimit("admin.api", 20, 60)

	adminGroup := router.Group("/admin")
	adminGroup.Use(middleware.AdminAuthMiddleware())
	adminGroup.Use(adminAPILimit)

	adminGroup.POST("/logout", admin.LogoutHandler)

	// Admin identity & RBAC management — SUPER_ADMIN only in practice
	// (only SUPER_ADMIN's role carries ADMIN_MANAGE/ROLE_MANAGE).
	adminGroup.GET("/admin-users", perm(rbac.AdminManage), admin.AdminListAdminUsersHandler)
	adminGroup.POST("/admin-users", perm(rbac.AdminManage), admin.AdminCreateAdminUserHandler)
	adminGroup.PUT("/admin-users/:id/role", perm(rbac.RoleManage), admin.AdminUpdateAdminUserRoleHandler)
	adminGroup.PUT("/admin-users/:id/status", perm(rbac.AdminManage), admin.AdminSetAdminUserStatusHandler)
	adminGroup.GET("/roles", perm(rbac.AdminManage), admin.AdminListRolesHandler)

	adminGroup.GET("/dashboard", perm(rbac.ReportView), admin.DashboardHandler)
	adminGroup.GET("/users", perm(rbac.CustomerView), admin.ListUsersHandler)
	adminGroup.GET("/users/:id", perm(rbac.CustomerView), admin.GetUserDetailHandler)
	adminGroup.POST("/users/:id/kyc/approve", perm(rbac.KYCApprove), admin.ApproveKYCHandler)
	adminGroup.POST("/users/:id/kyc/reject", perm(rbac.KYCApprove), admin.RejectKYCHandler)

	// KYC state machine (Phase 3) — additive alongside the routes above,
	// which continue to write CustomerKYCReview directly. These wrap the
	// same write with Recompute so internal/kyc.CustomerKYC never drifts
	// out of sync with the review table.
	adminGroup.GET("/kyc/queue", perm(rbac.KYCView), kyc.AdminQueueHandler)
	adminGroup.GET("/kyc/:customerId", perm(rbac.KYCView), kyc.AdminDetailHandler)
	adminGroup.POST("/kyc/:customerId/approve", perm(rbac.KYCApprove), kyc.AdminApproveHandler)
	adminGroup.POST("/kyc/:customerId/reject", perm(rbac.KYCApprove), kyc.AdminRejectHandler)
	adminGroup.POST("/kyc/:customerId/require-review", perm(rbac.KYCApprove), kyc.AdminRequireReviewHandler)

	// Credit bureau (read-only admin visibility over BureauRequest rows).
	adminGroup.GET("/bureau/requests", perm(rbac.BureauView), bureau.AdminListHandler)

	// CKYC/CERSAI — see internal/ckyc's doc comment: no provider is
	// configured, so search always resolves to a BLOCKED CKYCRequest today.
	adminGroup.GET("/ckyc", perm(rbac.CKYCView), ckyc.AdminListHandler)
	adminGroup.POST("/ckyc/:customerId/search", perm(rbac.CKYCView), ckyc.AdminSearchHandler)

	// PEP/sanctions screening — running a check and viewing results only
	// needs SCREENING_VIEW; resolving a POTENTIAL_MATCH/REVIEW_REQUIRED
	// result requires the separate SCREENING_DECIDE permission, held only
	// by compliance-tier roles (see internal/rbac/permissions.go).
	adminGroup.GET("/screening", perm(rbac.ScreeningView), screening.AdminListHandler)
	// NOTE: these two previously collided as /screening/:customerId/run and
	// /screening/:requestId/decide — Gin panics at startup when two
	// routes need different wildcard names at the same path segment, so
	// the server could never actually boot with both registered. Found by
	// Phase 7's security test suite (the first test to exercise the real
	// production route table end-to-end) and fixed by giving the decide
	// route its own static prefix rather than sharing the bare
	// /screening/:id segment.
	adminGroup.POST("/screening/:customerId/run", perm(rbac.ScreeningView), screening.AdminRunHandler)
	adminGroup.POST("/screening/requests/:requestId/decide", perm(rbac.ScreeningDecide), screening.AdminDecideHandler)

	// Accounting (Phase 4). AccountingView is read-only (reports/COA/GL
	// browsing); AccountingPost is required for anything that writes a
	// journal entry or a funding drawdown; PeriodManage is its own,
	// narrower permission for closing/reopening a period.
	acctView := perm(rbac.AccountingView)
	acctPost := perm(rbac.AccountingPost)
	periodManage := perm(rbac.PeriodManage)

	adminGroup.GET("/accounting/accounts", acctView, accounting.AdminListAccountsHandler)
	adminGroup.POST("/accounting/accounts", acctPost, accounting.AdminCreateAccountHandler)
	adminGroup.PUT("/accounting/accounts/:id", acctPost, accounting.AdminUpdateAccountHandler)

	adminGroup.GET("/accounting/journal", acctView, accounting.AdminListJournalEntriesHandler)
	adminGroup.GET("/accounting/journal/:id", acctView, accounting.AdminGetJournalEntryHandler)
	adminGroup.POST("/accounting/journal", acctPost, accounting.AdminPostManualJournalHandler)
	adminGroup.POST("/accounting/journal/:id/reverse", acctPost, accounting.AdminReverseJournalHandler)

	adminGroup.GET("/accounting/periods", acctView, accounting.AdminListPeriodsHandler)
	adminGroup.POST("/accounting/periods/:id/close", periodManage, accounting.AdminClosePeriodHandler)
	adminGroup.POST("/accounting/periods/:id/reopen", periodManage, accounting.AdminReopenPeriodHandler)

	adminGroup.GET("/accounting/reports/trial-balance", acctView, accounting.AdminTrialBalanceHandler)
	adminGroup.GET("/accounting/reports/profit-and-loss", acctView, accounting.AdminProfitAndLossHandler)
	adminGroup.GET("/accounting/reports/balance-sheet", acctView, accounting.AdminBalanceSheetHandler)
	adminGroup.GET("/accounting/reports/general-ledger", acctView, accounting.AdminGeneralLedgerHandler)

	adminGroup.GET("/accounting/gst", acctView, accounting.AdminListGSTTransactionsHandler)
	adminGroup.GET("/accounting/invoices", acctView, accounting.AdminListInvoicesHandler)
	adminGroup.GET("/accounting/tds", acctView, accounting.AdminListTDSHandler)

	adminGroup.GET("/accounting/funding", acctView, accounting.AdminListFundingSourcesHandler)
	adminGroup.POST("/accounting/funding", acctPost, accounting.AdminCreateFundingSourceHandler)

	// Phase 8 — vendor/expense management, bank balances, financial
	// dashboard. financeView is read-only; expenseManage drafts an
	// expense (maker); expenseApprove approves/rejects/marks-paid
	// (checker) — the handler additionally refuses the same admin doing
	// both. vendorManage is its own permission (vendor PAN/bank data is
	// as sensitive as customer data, Phase 7 item I).
	financeView := perm(rbac.FinanceView)
	expenseManage := perm(rbac.ExpenseManage)
	expenseApprove := perm(rbac.ExpenseApprove)
	vendorManage := perm(rbac.VendorManage)

	adminGroup.GET("/finance/vendors", financeView, finance.AdminListVendorsHandler)
	adminGroup.POST("/finance/vendors", vendorManage, finance.AdminCreateVendorHandler)
	adminGroup.PUT("/finance/vendors/:id/status", vendorManage, finance.AdminSetVendorStatusHandler)

	adminGroup.GET("/finance/expenses", financeView, finance.AdminListExpensesHandler)
	adminGroup.POST("/finance/expenses", expenseManage, finance.AdminCreateExpenseHandler)
	adminGroup.POST("/finance/expenses/:id/approve", expenseApprove, finance.AdminApproveExpenseHandler)
	adminGroup.POST("/finance/expenses/:id/reject", expenseApprove, finance.AdminRejectExpenseHandler)
	adminGroup.POST("/finance/expenses/:id/mark-paid", expenseApprove, finance.AdminMarkExpensePaidHandler)
	adminGroup.POST("/finance/expenses/:id/reverse", expenseApprove, finance.AdminReverseExpenseHandler)

	adminGroup.GET("/finance/bank-accounts/balances", financeView, finance.AdminBankBalancesHandler)
	adminGroup.GET("/finance/bank-accounts/:id/balance", financeView, finance.AdminBankBalanceHandler)

	adminGroup.GET("/finance/dashboard", financeView, finance.AdminFinancialDashboardHandler)

	// Phase 10 — TDS/tax engine. taxView is read-only; taxRuleManage/
	// taxRuleApprove are the maker/checker pair for TDSRule configuration
	// (item B/O); taxOperate covers day-to-day operational actions
	// (calculate, post deduction, challan, resolve exception) — a
	// separate, lower-tier authorization than approving a tax rule.
	taxView := perm(rbac.TaxView)
	taxRuleManage := perm(rbac.TaxRuleManage)
	taxRuleApprove := perm(rbac.TaxRuleApprove)
	taxOperate := perm(rbac.TaxOperate)

	adminGroup.GET("/tax/rules", taxView, taxpkg.AdminListRulesHandler)
	adminGroup.POST("/tax/rules", taxRuleManage, taxpkg.AdminDraftRuleHandler)
	adminGroup.POST("/tax/rules/:id/approve", taxRuleApprove, taxpkg.AdminApproveRuleHandler)
	adminGroup.POST("/tax/rules/:id/retire", taxRuleApprove, taxpkg.AdminRetireRuleHandler)

	adminGroup.GET("/tax/transactions", taxView, taxpkg.AdminListTransactionsHandler)
	adminGroup.POST("/tax/transactions/calculate", taxOperate, taxpkg.AdminCalculateTDSHandler)
	adminGroup.POST("/tax/transactions/:id/override", taxOperate, taxpkg.AdminOverrideTDSHandler)
	adminGroup.POST("/tax/transactions/:id/post-deduction", taxOperate, taxpkg.AdminPostDeductionHandler)
	adminGroup.POST("/tax/transactions/:id/reverse", taxOperate, taxpkg.AdminReverseDeductionHandler)

	adminGroup.GET("/tax/challans", taxView, taxpkg.AdminListChallansHandler)
	adminGroup.POST("/tax/challans", taxOperate, taxpkg.AdminCreateChallanHandler)
	adminGroup.POST("/tax/challans/link", taxOperate, taxpkg.AdminLinkChallanHandler)
	adminGroup.POST("/tax/challans/:id/mark-paid", taxOperate, taxpkg.AdminMarkChallanPaidHandler)

	adminGroup.GET("/tax/deductees", taxView, taxpkg.AdminListDeducteesHandler)
	adminGroup.POST("/tax/deductees", taxOperate, taxpkg.AdminCreateDeducteeHandler)
	adminGroup.PUT("/tax/deductees/:id/pan-status", taxOperate, taxpkg.AdminSetPANStatusHandler)

	adminGroup.GET("/tax/exceptions", taxView, taxpkg.AdminListExceptionsHandler)
	adminGroup.POST("/tax/exceptions/run-checks", taxOperate, taxpkg.AdminRunReconciliationHandler)
	adminGroup.POST("/tax/exceptions/:id/resolve", taxOperate, taxpkg.AdminResolveExceptionHandler)

	adminGroup.GET("/tax/calendar", taxView, taxpkg.AdminListCalendarHandler)
	adminGroup.POST("/tax/calendar", taxOperate, taxpkg.AdminCreateCalendarItemHandler)
	adminGroup.POST("/tax/calendar/:id/complete", taxOperate, taxpkg.AdminCompleteCalendarItemHandler)

	adminGroup.GET("/tax/dashboard", taxView, taxpkg.AdminTaxDashboardHandler)

	// Phase 11 — NBFC compliance management. complianceView is read-only;
	// complianceManage covers day-to-day compliance-team actions;
	// complianceApprove is the checker tier (regulatory-return approval,
	// requirement activation); auditAccess/auditManage gate the internal-
	// audit/findings/CAPA module separately (item U).
	complianceView := perm(rbac.ComplianceView)
	complianceManage := perm(rbac.ComplianceManage)
	complianceApprove := perm(rbac.ComplianceApprove)
	auditAccess := perm(rbac.AuditModuleAccess)
	auditManage := perm(rbac.AuditModuleManage)

	adminGroup.GET("/compliance/profile", complianceView, compliance.AdminGetProfileHandler)
	adminGroup.PUT("/compliance/profile", complianceManage, compliance.AdminUpdateProfileHandler)
	adminGroup.POST("/compliance/profile/confirm", complianceApprove, compliance.AdminConfirmProfileHandler)

	adminGroup.GET("/compliance/requirements", complianceView, compliance.AdminListRequirementsHandler)
	adminGroup.POST("/compliance/requirements", complianceManage, compliance.AdminCreateRequirementHandler)
	adminGroup.POST("/compliance/requirements/:id/activate", complianceApprove, compliance.AdminActivateRequirementHandler)
	adminGroup.PUT("/compliance/requirements/:id/applicability", complianceManage, compliance.AdminSetApplicabilityHandler)

	adminGroup.GET("/compliance/calendar", complianceView, compliance.AdminListCalendarHandler)
	adminGroup.POST("/compliance/calendar", complianceManage, compliance.AdminCreateCalendarItemHandler)
	adminGroup.PUT("/compliance/calendar/:id/status", complianceManage, compliance.AdminUpdateCalendarStatusHandler)

	adminGroup.GET("/compliance/returns", complianceView, compliance.AdminListReturnsHandler)
	adminGroup.POST("/compliance/returns", complianceManage, compliance.AdminCreateReturnHandler)
	adminGroup.POST("/compliance/returns/:id/approve", complianceApprove, compliance.AdminApproveReturnHandler)
	adminGroup.POST("/compliance/returns/:id/submit", complianceApprove, compliance.AdminSubmitReturnHandler)
	adminGroup.POST("/compliance/returns/:id/correct", complianceManage, compliance.AdminCorrectReturnHandler)

	adminGroup.GET("/compliance/evidence", complianceView, compliance.AdminListEvidenceHandler)
	adminGroup.POST("/compliance/evidence", complianceManage, compliance.AdminUploadEvidenceHandler)
	adminGroup.GET("/compliance/evidence/:id/download", complianceView, compliance.AdminDownloadEvidenceHandler)

	adminGroup.GET("/compliance/policies", complianceView, compliance.AdminListPoliciesHandler)
	adminGroup.POST("/compliance/policies", complianceManage, compliance.AdminCreatePolicyHandler)
	adminGroup.PUT("/compliance/policies/:id/status", complianceApprove, compliance.AdminAdvancePolicyStatusHandler)

	adminGroup.GET("/compliance/audits", auditAccess, compliance.AdminListAuditsHandler)
	adminGroup.POST("/compliance/audits", auditManage, compliance.AdminCreateAuditHandler)
	adminGroup.GET("/compliance/findings", auditAccess, compliance.AdminListFindingsHandler)
	adminGroup.POST("/compliance/findings", auditManage, compliance.AdminCreateFindingHandler)
	adminGroup.POST("/compliance/findings/:id/management-response", auditManage, compliance.AdminManagementResponseHandler)
	adminGroup.POST("/compliance/findings/:id/close", auditManage, compliance.AdminCloseFindingHandler)
	adminGroup.GET("/compliance/capas", auditAccess, compliance.AdminListCAPAsHandler)
	adminGroup.POST("/compliance/capas", auditManage, compliance.AdminCreateCAPAHandler)
	adminGroup.PUT("/compliance/capas/:id/status", auditManage, compliance.AdminUpdateCAPAStatusHandler)

	adminGroup.GET("/compliance/alerts", complianceView, compliance.AdminListAlertsHandler)
	adminGroup.POST("/compliance/alerts/:id/resolve", complianceManage, compliance.AdminResolveAlertHandler)
	adminGroup.POST("/compliance/sweep", complianceManage, compliance.AdminRunSweepHandler)

	adminGroup.GET("/compliance/dashboard", complianceView, compliance.AdminDashboardHandler)

	// Phase 5 — portfolio risk / DPD / NPA classification / provisioning /
	// collections / write-off. riskView is read-only (dashboard, loan risk
	// section, rule listings). riskManage/riskApprove are the maker/checker
	// pair for classification+provisioning rule configuration (item S).
	riskView := perm(rbac.RiskView)
	riskManage := perm(rbac.RiskRuleManage)
	riskApprove := perm(rbac.RiskRuleApprove)
	classOverride := perm(rbac.ClassificationOverride)

	adminGroup.GET("/risk/buckets", riskView, risk.AdminListBucketsHandler)
	adminGroup.POST("/risk/buckets", riskManage, risk.AdminCreateBucketHandler)

	adminGroup.GET("/risk/classification-rules", riskView, risk.AdminListClassificationRulesHandler)
	adminGroup.POST("/risk/classification-rules", riskManage, risk.AdminDraftClassificationRuleHandler)
	adminGroup.POST("/risk/classification-rules/:id/approve", riskApprove, risk.AdminApproveClassificationRuleHandler)

	adminGroup.GET("/risk/provisioning-rules", riskView, risk.AdminListProvisioningRulesHandler)
	adminGroup.POST("/risk/provisioning-rules", riskManage, risk.AdminDraftProvisioningRuleHandler)
	adminGroup.POST("/risk/provisioning-rules/:id/approve", riskApprove, risk.AdminApproveProvisioningRuleHandler)

	adminGroup.GET("/risk/loans/:loanId", riskView, risk.AdminLoanRiskHandler)
	adminGroup.POST("/risk/loans/:loanId/override-classification", classOverride, risk.AdminOverrideClassificationHandler)

	adminGroup.GET("/risk/dashboard", riskView, risk.AdminPortfolioDashboardHandler)

	adminGroup.POST("/risk/jobs/run", riskManage, risk.AdminRunDailyJobHandler)
	adminGroup.GET("/risk/jobs", riskView, risk.AdminListJobRunsHandler)

	adminGroup.GET("/risk/alerts", riskView, risk.AdminListAlertsHandler)
	adminGroup.POST("/risk/alerts/:id/acknowledge", riskView, risk.AdminAcknowledgeAlertHandler)

	adminGroup.GET("/risk/exceptions", riskView, risk.AdminListExceptionsHandler)
	adminGroup.POST("/risk/exceptions/:id/resolve", riskManage, risk.AdminResolveExceptionHandler)
	adminGroup.POST("/risk/exceptions/run-checks", riskManage, risk.AdminRunConsistencyChecksHandler)

	// Collections (item I/J/K/L). collectionView/collectionRecord already
	// existed pre-Phase-5; collectionManage is new (assign/escalate).
	collView := perm(rbac.CollectionView)
	collRecord := perm(rbac.CollectionRecord)
	collManage := perm(rbac.CollectionManage)

	adminGroup.GET("/collections/cases", collView, collections.AdminListCasesHandler)
	adminGroup.GET("/collections/cases/:id", collView, collections.AdminGetCaseHandler)
	adminGroup.POST("/collections/cases/:id/assign", collManage, collections.AdminAssignCaseHandler)
	adminGroup.POST("/collections/cases/:id/escalate", collRecord, collections.AdminEscalateCaseHandler)
	adminGroup.POST("/collections/cases/:id/actions", collRecord, collections.AdminRecordActionHandler)
	adminGroup.POST("/collections/cases/:id/notes", collRecord, collections.AdminAddNoteHandler)
	adminGroup.POST("/collections/cases/:id/promises", collRecord, collections.AdminCreatePromiseHandler)

	// Write-off (item Q) — requester and approver are deliberately
	// different permissions, and the service layer additionally refuses
	// the same admin doing both (maker != checker).
	adminGroup.GET("/writeoffs", perm(rbac.WriteOffRequest), writeoff.AdminListWriteOffsHandler)
	adminGroup.POST("/writeoffs", perm(rbac.WriteOffRequest), writeoff.AdminRequestWriteOffHandler)
	adminGroup.POST("/writeoffs/:id/approve", perm(rbac.WriteOffApprove), writeoff.AdminApproveWriteOffHandler)
	adminGroup.POST("/writeoffs/:id/reject", perm(rbac.WriteOffApprove), writeoff.AdminRejectWriteOffHandler)
	adminGroup.POST("/writeoffs/:id/recovery", perm(rbac.WriteOffApprove), writeoff.AdminRecordRecoveryHandler)

	// Phase 6 — MIS / regulatory reporting / compliance monitoring / audit
	// readiness.
	misView := perm(rbac.MISView)
	regManage := perm(rbac.RegReportManage)
	regApprove := perm(rbac.RegReportApprove)
	auditCenter := perm(rbac.AuditCenterAccess)
	taskManage := perm(rbac.ComplianceTaskManage)
	exportPerm := perm(rbac.ReportExport)

	// Phase 16 — operational surfaces. Both reuse misView (read-only
	// operational visibility, same tier as the other dashboards); no new
	// permission was invented. Deliberately admin-gated rather than
	// public: metrics and integrity results reveal traffic shape and
	// financial state.
	adminGroup.GET("/ops/metrics", misView, integrity.AdminMetricsHandler)
	adminGroup.GET("/ops/integrity", misView, integrity.AdminIntegrityCheckHandler)
	adminGroup.GET("/ops/reconciliation", misView, integrity.AdminReconciliationHandler)
	adminGroup.GET("/ops/backup-status", misView, integrity.AdminBackupStatusHandler)
	adminGroup.GET("/ops/sync-status", misView, integrity.AdminSyncStatusHandler)

	// Phase 16.1 — scheduled job execution. The RUN endpoint requires a
	// manage-tier permission because these jobs change financial state;
	// the read endpoints reuse misView. An external scheduler
	// authenticates as a dedicated admin holding only RiskRuleManage.
	adminGroup.GET("/ops/jobs", misView, jobs.AdminListJobsHandler)
	adminGroup.GET("/ops/jobs/executions", misView, jobs.AdminJobExecutionsHandler)
	adminGroup.GET("/ops/jobs/stale", misView, jobs.AdminStaleJobsHandler)
	adminGroup.POST("/ops/jobs/:name/run", perm(rbac.RiskRuleManage), jobs.AdminRunJobHandler)

	adminGroup.GET("/reporting/mis", misView, reporting.AdminMISDashboardHandler)
	adminGroup.GET("/reporting/compliance-health", misView, reporting.AdminComplianceHealthHandler)

	// Phase 12 — executive MIS. Reuses this same misView permission (no
	// new permission needed — this module is purely a composition/
	// reporting layer over data every other module already gates).
	adminGroup.GET("/mis/executive", misView, mis.AdminExecutiveDashboardHandler)
	adminGroup.GET("/mis/growth", misView, mis.AdminGrowthReportHandler)
	adminGroup.GET("/mis/funnel", misView, mis.AdminFunnelReportHandler)
	adminGroup.GET("/mis/unit-economics", misView, mis.AdminUnitEconomicsHandler)
	adminGroup.GET("/mis/financial-year", misView, mis.AdminFinancialYearHandler)

	// Phase 13 — management intelligence. Read endpoints reuse misView
	// (same composed-reporting visibility as Phase 12); the risk-signal
	// and data-quality surfaces reuse riskView because they concern
	// portfolio/operational risk. Actions that change state (reviewing a
	// signal, configuring a threshold, resolving an alert) require the
	// corresponding manage permission — no new permission was invented.
	adminGroup.GET("/analytics/growth", misView, analytics.AdminGrowthDashboardHandler)
	adminGroup.GET("/analytics/growth/mom", misView, analytics.AdminMoMHandler)
	adminGroup.GET("/analytics/growth/yoy", misView, analytics.AdminYoYHandler)
	adminGroup.GET("/analytics/growth-quality", misView, analytics.AdminGrowthQualityHandler)
	adminGroup.GET("/analytics/funnel", misView, analytics.AdminFunnelHandler)
	adminGroup.GET("/analytics/products", misView, analytics.AdminProductAnalyticsHandler)
	adminGroup.GET("/analytics/segments", misView, analytics.AdminSegmentAnalyticsHandler)
	adminGroup.GET("/analytics/geography", misView, analytics.AdminGeographicAnalyticsHandler)
	adminGroup.GET("/analytics/channels", misView, analytics.AdminChannelAnalyticsHandler)
	adminGroup.GET("/analytics/revenue-quality", misView, analytics.AdminRevenueQualityHandler)
	adminGroup.GET("/analytics/expense-efficiency", misView, analytics.AdminExpenseEfficiencyHandler)
	adminGroup.GET("/analytics/profitability", misView, analytics.AdminProfitabilityHandler)
	adminGroup.GET("/analytics/cohorts", misView, analytics.AdminCohortsHandler)
	adminGroup.GET("/analytics/retention", misView, analytics.AdminRetentionHandler)
	adminGroup.GET("/analytics/decision-center", misView, analytics.AdminDecisionCenterHandler)
	adminGroup.GET("/analytics/kpi-dictionary", misView, analytics.AdminKPIDictionaryHandler)
	adminGroup.GET("/analytics/budget-variance", misView, analytics.AdminBudgetVarianceHandler)

	adminGroup.GET("/analytics/portfolio-risk", riskView, analytics.AdminPortfolioRiskHandler)
	adminGroup.GET("/analytics/dpd-trend", riskView, analytics.AdminDPDTrendHandler)
	adminGroup.GET("/analytics/collection-risk", riskView, analytics.AdminCollectionRiskHandler)
	adminGroup.GET("/analytics/concentration", riskView, analytics.AdminConcentrationHandler)
	adminGroup.GET("/analytics/outliers", riskView, analytics.AdminOutliersHandler)
	adminGroup.GET("/analytics/signals", riskView, analytics.AdminListSignalsHandler)
	adminGroup.GET("/analytics/data-quality", riskView, analytics.AdminListDataQualityHandler)

	adminGroup.POST("/analytics/signals/run-detection", riskManage, analytics.AdminRunSignalDetectionHandler)
	adminGroup.POST("/analytics/signals/:id/review", riskManage, analytics.AdminReviewSignalHandler)
	adminGroup.POST("/analytics/data-quality/run-checks", riskManage, analytics.AdminRunDataQualityHandler)
	adminGroup.GET("/analytics/thresholds", misView, analytics.AdminListThresholdsHandler)
	adminGroup.POST("/analytics/thresholds", riskManage, analytics.AdminCreateThresholdHandler)
	adminGroup.POST("/analytics/thresholds/evaluate", riskManage, analytics.AdminEvaluateThresholdsHandler)
	adminGroup.POST("/analytics/alerts/:id/resolve", riskManage, analytics.AdminResolveAlertHandler)

	adminGroup.GET("/reporting/audit-center/loans/:loanId", auditCenter, reporting.AdminLoanTimelineHandler)
	adminGroup.GET("/reporting/audit-center/customers/:customerId", auditCenter, reporting.AdminCustomerTimelineHandler)

	adminGroup.GET("/reporting/reconciliation", misView, reporting.AdminListReconciliationHandler)
	adminGroup.POST("/reporting/reconciliation/run-checks", taskManage, reporting.AdminRunReconciliationChecksHandler)
	adminGroup.GET("/reporting/reconciliation/summary", misView, reporting.AdminGatewayReconciliationSummaryHandler)
	adminGroup.POST("/reporting/reconciliation/:id/resolve", taskManage, reporting.AdminResolveReconciliationHandler)

	adminGroup.GET("/reporting/definitions", misView, reporting.AdminListDefinitionsHandler)
	adminGroup.POST("/reporting/definitions", regManage, reporting.AdminCreateDefinitionHandler)
	adminGroup.PUT("/reporting/definitions/:id/status", regApprove, reporting.AdminUpdateDefinitionStatusHandler)
	adminGroup.POST("/reporting/definitions/:id/versions", regManage, reporting.AdminCreateVersionHandler)

	adminGroup.GET("/reporting/runs", misView, reporting.AdminListRunsHandler)
	adminGroup.GET("/reporting/runs/:id", misView, reporting.AdminGetRunHandler)
	adminGroup.POST("/reporting/runs", regManage, reporting.AdminGenerateRunHandler)
	adminGroup.POST("/reporting/runs/:id/submit-for-review", regManage, reporting.AdminSubmitForReviewHandler)
	adminGroup.POST("/reporting/runs/:id/review", regApprove, reporting.AdminReviewRunHandler)
	adminGroup.POST("/reporting/runs/:id/submit", regApprove, reporting.AdminSubmitRunHandler)
	adminGroup.POST("/reporting/runs/:id/close", regApprove, reporting.AdminCloseRunHandler)
	adminGroup.POST("/reporting/runs/:id/revalidate", regManage, reporting.AdminRevalidateRunHandler)

	adminGroup.GET("/reporting/tasks", misView, reporting.AdminListTasksHandler)
	adminGroup.POST("/reporting/tasks", taskManage, reporting.AdminCreateTaskHandler)
	adminGroup.POST("/reporting/tasks/:id/assign", taskManage, reporting.AdminAssignTaskHandler)
	adminGroup.POST("/reporting/tasks/:id/status", taskManage, reporting.AdminUpdateTaskStatusHandler)
	adminGroup.POST("/reporting/tasks/run-periodic-checks", taskManage, reporting.AdminRunPeriodicChecksHandler)

	adminGroup.POST("/reporting/exports", exportPerm, exportLimit, reporting.AdminRequestExportHandler)
	adminGroup.POST("/reporting/exports/:reportType", exportPerm, exportLimit, reporting.AdminExportControlledReportHandler)
	adminGroup.GET("/reporting/exports", exportPerm, reporting.AdminListExportsHandler)
	adminGroup.GET("/reporting/exports/:id", exportPerm, reporting.AdminGetExportHandler)
	adminGroup.GET("/reporting/exports/:id/download", exportPerm, reporting.AdminDownloadExportHandler)
	adminGroup.GET("/reporting/preview/:reportType", exportPerm, reporting.AdminPreviewReportHandler)

	adminGroup.GET("/reporting/retention", misView, reporting.AdminListRetentionHandler)
	adminGroup.POST("/reporting/retention", taskManage, reporting.AdminConfigureRetentionHandler)
	adminGroup.POST("/reporting/retention/:category/validate", taskManage, reporting.AdminValidateRetentionHandler)

	adminGroup.GET("/loans", perm(rbac.LoanView), loans.AdminListLoansHandler)
	adminGroup.GET("/loans/:id", perm(rbac.LoanView), loans.AdminGetLoanHandler)
	adminGroup.GET("/loans/pending-approvals", perm(rbac.LoanView), loans.AdminListPendingApprovalsHandler)

	// Maker-checker: sanction. LoanReview (maker) != LoanSanction (checker)
	// as PERMISSIONS, and the handlers additionally enforce that the two
	// steps can't be performed by the same PERSON even if one admin
	// happened to hold both permissions.
	adminGroup.POST("/loans/:id/decision", perm(rbac.LoanReject), loans.AdminDecisionHandler) // reject-only now
	adminGroup.POST("/loans/:id/recommend", perm(rbac.LoanReview), loans.AdminRecommendSanctionHandler)
	adminGroup.POST("/loans/:id/approve-sanction", perm(rbac.LoanSanction), loans.AdminApproveSanctionHandler)

	// Maker-checker: disbursement.
	adminGroup.POST("/loans/:id/prepare-disbursement", perm(rbac.LoanDisbursePrep), loans.AdminPrepareDisbursementHandler)
	adminGroup.POST("/loans/:id/approve-disbursement", perm(rbac.LoanDisburse), loans.AdminApproveDisbursementHandler)
	adminGroup.POST("/loans/:id/disburse", perm(rbac.LoanDisburse), loans.AdminDisburseHandler) // disabled — returns 410

	adminGroup.POST("/loans/:id/reassign", perm(rbac.LoanReview), loans.AdminReassignLoanHandler)
	adminGroup.POST("/loans/:id/restructure", perm(rbac.LoanRestructure), loans.AdminRestructureLoanHandler)
	adminGroup.GET("/loans/:id/documents", perm(rbac.LoanView), documents.AdminListLoanDocumentsHandler)
	adminGroup.GET("/loans/:id/emi-schedule", perm(rbac.LoanView), loans.AdminEmiScheduleHandler)
	adminGroup.GET("/customers/:id/documents", perm(rbac.CustomerView), documents.AdminListCustomerDocumentsHandler)

	adminGroup.GET("/recovery-reports", perm(rbac.CollectionView), recovery.AdminListReportsHandler)
	adminGroup.POST("/recovery-reports/:id/action", perm(rbac.CollectionRecord), recovery.AdminActionHandler)

	adminGroup.GET("/agencies", perm(rbac.AdminManage), agency.ListHandler)
	adminGroup.POST("/agencies", perm(rbac.AdminManage), agency.CreateHandler)
	adminGroup.DELETE("/agencies/:id", perm(rbac.AdminManage), agency.DeleteHandler)

	adminGroup.GET("/employees", perm(rbac.AdminManage), employee.AdminListHandler)
	adminGroup.POST("/employees", perm(rbac.AdminManage), employee.AdminCreateHandler)
	adminGroup.POST("/employees/:id/approve", perm(rbac.AdminManage), employee.AdminApproveHandler)
	adminGroup.DELETE("/employees/:id", perm(rbac.AdminManage), employee.AdminDeleteHandler)

	adminGroup.GET("/audit-log", perm(rbac.AuditView), audit.AdminListHandler)
	adminGroup.GET("/sla-alerts", perm(rbac.ReportView), alerts.AdminSlaAlertsHandler)

	// Payments (Phase 2)
	adminGroup.GET("/payments", perm(rbac.PaymentView), payments.AdminListPaymentsHandler)
	adminGroup.GET("/payments/:id", perm(rbac.PaymentView), payments.AdminGetPaymentHandler)

	// Bank reconciliation (Phase 2)
	adminGroup.GET("/reconciliation/bank-accounts", perm(rbac.ReconciliationView), reconciliation.AdminListBankAccountsHandler)
	adminGroup.POST("/reconciliation/bank-accounts", perm(rbac.ReconciliationManage), reconciliation.AdminCreateBankAccountHandler)
	adminGroup.POST("/reconciliation/bank-accounts/:id/import", perm(rbac.ReconciliationManage), reconciliation.AdminImportBankStatementHandler)
	adminGroup.GET("/reconciliation/transactions", perm(rbac.ReconciliationView), reconciliation.AdminListBankTransactionsHandler)
	adminGroup.POST("/reconciliation/transactions/:id/match", perm(rbac.ReconciliationManage), reconciliation.AdminManualMatchHandler)
	adminGroup.GET("/reconciliation/exceptions", perm(rbac.ReconciliationView), reconciliation.AdminListReconciliationExceptionsHandler)
	adminGroup.POST("/reconciliation/exceptions/:id/resolve", perm(rbac.ReconciliationManage), reconciliation.AdminResolveExceptionHandler)

	adminGroup.GET("/support/threads", perm(rbac.CustomerView), support.AdminListThreadsHandler)
	adminGroup.GET("/support/threads/:id/messages", perm(rbac.CustomerView), support.AdminThreadMessagesHandler)
	adminGroup.POST("/support/threads/:id/messages", perm(rbac.CustomerView), support.AdminReplyHandler)
	adminGroup.POST("/support/threads/:id/close", perm(rbac.CustomerView), support.AdminCloseThreadHandler)

	// Customer loan routes (reuse existing customer JWT middleware)
	authorized.POST("/loans/apply", loanSubmitLimit, loans.SubmitLoanHandler)
	authorized.GET("/loans/mine", loans.ListMyLoansHandler)
	authorized.GET("/loans/:id", loans.GetMyLoanHandler)
	authorized.GET("/loans/:id/emi-schedule", loans.CustomerEmiScheduleHandler)
	authorized.POST("/loans/:id/emi/:installmentId/pay", loans.CustomerPayEmiHandler) // disabled — returns 410
	authorized.POST("/loans/:id/emi/:installmentId/create-order", paymentOrderLimit, payments.CustomerCreatePaymentOrderHandler)
	authorized.GET("/payments/:paymentId", payments.CustomerGetPaymentStatusHandler)
	authorized.GET("/dashboard/emi-summary", loans.CustomerEmiSummaryHandler)
	authorized.GET("/credit-score", loans.CustomerCreditScoreHandler)
	authorized.POST("/loans/:id/topup", loans.TopUpLoanHandler)
	authorized.POST("/loans/:id/esign", loans.CustomerEsignLetterHandler)
	authorized.GET("/transactions", loans.CustomerTransactionsHandler)

	authorized.POST("/support/messages", support.PostMessageHandler)
	authorized.GET("/support/messages", support.ListMessagesHandler)

	// Employee routes (new employee JWT middleware)
	router.POST("/employee/login", loginLimit, employee.LoginHandler)
	router.POST("/employee/register", loginLimit, employee.RegisterHandler)

	employeeGroup := router.Group("/employee")
	employeeGroup.Use(middleware.EmployeeAuthMiddleware())

	employeeGroup.GET("/tasks/today", loans.EmployeeTasksTodayHandler)
	employeeGroup.POST("/loans/:id/verify", loans.EmployeeVerifyLoanHandler)
	employeeGroup.POST("/loans/:id/documents", documents.UploadEmployeeLoanDocumentHandler)
	employeeGroup.GET("/recovery/cases/today", recovery.CasesTodayHandler)
	employeeGroup.POST("/recovery/cases/:id/report", recovery.SubmitReportHandler)

	// Notifications: accepts either a customer JWT or an employee JWT,
	// inferring the recipient (role + id) from whichever token validates.
	router.GET("/notifications", middleware.CustomerOrEmployeeAuthMiddleware(), notifications.ListHandler)
	// WebRTC signaling relay for employee<->customer video calls — see
	// internal/signaling/handler.go's doc comment for what this endpoint
	// does and doesn't do.
	router.GET("/ws/call/:loanId", middleware.CustomerOrEmployeeAuthMiddleware(), signaling.CallSignalHandler)
	router.POST("/notifications/:id/read", middleware.CustomerOrEmployeeAuthMiddleware(), notifications.MarkReadHandler)
}
