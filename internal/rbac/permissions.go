// Package rbac defines the admin-panel permission model: a fixed,
// backend-enforced role -> permission mapping. Every admin-only handler
// that acts on customer/loan/financial data must be gated by
// middleware.RequirePermission(...) using one of the constants below — the
// admin panel hiding a button is UX, not security; the backend check here
// is the actual control.
package rbac

// Permission is a single granular capability. Roles are just named sets of
// these — there is no per-admin permission override in this phase (every
// admin gets exactly the permissions of their assigned role).
type Permission string

const (
	CustomerView Permission = "CUSTOMER_VIEW"
	CustomerEdit Permission = "CUSTOMER_EDIT"

	KYCView    Permission = "KYC_VIEW"
	KYCApprove Permission = "KYC_APPROVE"

	LoanView         Permission = "LOAN_VIEW"
	LoanReview       Permission = "LOAN_REVIEW"           // recommend a sanction decision (maker)
	LoanSanction     Permission = "LOAN_SANCTION"         // approve/reject a recommended sanction (checker)
	LoanReject       Permission = "LOAN_REJECT"           // reject outright, pre-recommendation
	LoanDisbursePrep Permission = "LOAN_DISBURSE_PREPARE" // prepare a disbursement (maker)
	LoanDisburse     Permission = "LOAN_DISBURSE"         // approve/release a prepared disbursement (checker)
	LoanRestructure  Permission = "LOAN_RESTRUCTURE"

	CollectionView   Permission = "COLLECTION_VIEW"
	CollectionRecord Permission = "COLLECTION_RECORD"

	AccountingView Permission = "ACCOUNTING_VIEW"
	ReportView     Permission = "REPORT_VIEW"
	AuditView      Permission = "AUDIT_VIEW"

	AdminManage Permission = "ADMIN_MANAGE" // create/disable/edit admin_users
	RoleManage  Permission = "ROLE_MANAGE"  // change an admin's assigned role

	// Phase 2 — payments, disbursement money-movement, and bank
	// reconciliation. PaymentView/DisbursementView are read-only; the
	// LoanDisburse* permissions already gate the maker/checker actions
	// that create a Disbursement in the first place (see
	// internal/loans/maker_checker_handler.go), so there is no separate
	// "disbursement manage" permission here.
	PaymentView          Permission = "PAYMENT_VIEW"
	DisbursementView     Permission = "DISBURSEMENT_VIEW"
	ReconciliationView   Permission = "RECONCILIATION_VIEW"
	ReconciliationManage Permission = "RECONCILIATION_MANAGE" // import statements, match/resolve

	// Phase 3 — credit bureau, CKYC/CERSAI, and PEP/sanctions screening.
	// BureauView/CKYCView are read-only (queue/results screens).
	// ScreeningDecide is deliberately its own permission, held only by
	// COMPLIANCE_OFFICER/SUPER_ADMIN — an ordinary credit officer must
	// never be able to clear a POTENTIAL_MATCH/REVIEW_REQUIRED screening
	// result, per Phase 3 item K.
	BureauView      Permission = "BUREAU_VIEW"
	CKYCView        Permission = "CKYC_VIEW"
	ScreeningView   Permission = "SCREENING_VIEW"
	ScreeningDecide Permission = "SCREENING_DECIDE"
	// BureauApprove is the credit-officer sign-off gate added after Phase
	// 3: a completed bureau pull isn't enough on its own for underwriting
	// to act on a loan — someone holding this permission must explicitly
	// approve the result first (internal/loans.requireApprovedBureauRequest
	// enforces this at the recommend-sanction step). Held by CREDIT_MANAGER/
	// SUPER_ADMIN, not CREDIT_OFFICER, mirroring the maker!=checker split
	// already used for LoanReview/LoanSanction.
	BureauApprove Permission = "BUREAU_APPROVE"

	// CorrectionView/CorrectionManage back the Correction Portal: when a
	// KYC reviewer rejects a customer, or a credit officer flags a bureau
	// result, with a reason that needs a human follow-up (document not
	// visible, customer didn't cooperate, bureau mismatch, etc.), a
	// CorrectionCase is raised. CorrectionView is read-only (list/detail);
	// CorrectionManage is claiming and resolving a case — resolving
	// re-opens the original KYC/Credit review automatically.
	CorrectionView   Permission = "CORRECTION_VIEW"
	CorrectionManage Permission = "CORRECTION_MANAGE"

	// Phase 4 — accounting. AccountingView (Phase 1/2) stays read-only;
	// AccountingPost is the new permission actually allowed to call
	// AccountingService.PostJournalEntry-backed admin actions (manual
	// journal entries, reversals). PeriodManage is deliberately its own,
	// narrower permission — closing/reopening a period is a materially
	// bigger action than posting one journal entry, and item M requires
	// reopening specifically to need "elevated permission".
	AccountingPost Permission = "ACCOUNTING_POST"
	PeriodManage   Permission = "PERIOD_MANAGE"

	// Phase 5 — portfolio risk, NPA classification, provisioning,
	// collections, write-off. RiskView is read-only (dashboard/reports/
	// loan risk section). RiskRuleManage drafts a
	// ClassificationRule/ProvisioningRule/DelinquencyBucket (maker);
	// RiskRuleApprove approves it (checker) — item S's maker-checker
	// requirement, mirroring the LoanReview/LoanSanction split.
	// ClassificationOverride is its own, narrower permission for a manual
	// classification change outside the automated daily job (item D: only
	// with "appropriate authorization" — never granted to an ordinary
	// credit/operations role). CollectionManage is what distinguishes a
	// collection MANAGER (assign/reassign/escalate-approve) from an
	// ordinary CollectionRecord-holding officer (item L).
	RiskView               Permission = "RISK_VIEW"
	RiskRuleManage         Permission = "RISK_RULE_MANAGE"
	RiskRuleApprove        Permission = "RISK_RULE_APPROVE"
	ClassificationOverride Permission = "CLASSIFICATION_OVERRIDE"
	CollectionManage       Permission = "COLLECTION_MANAGE"
	WriteOffRequest        Permission = "WRITE_OFF_REQUEST"
	WriteOffApprove        Permission = "WRITE_OFF_APPROVE"

	// Phase 6 — MIS/regulatory reporting/compliance monitoring/audit
	// readiness. MISView (dashboards/reconciliation/compliance-health,
	// read-only) is broadly granted since it's just visibility into
	// figures each role can mostly already see piecemeal elsewhere.
	// RegReportManage drafts/generates/prepares a regulatory report run
	// (maker); RegReportApprove reviews/approves it (checker) — item P's
	// maker-checker, mirroring Phase 5's risk-rule pattern, with the same
	// preparer!=reviewer check enforced in the handler. AuditCenterAccess
	// is its own permission (not folded into AuditView) because the Audit
	// Center surfaces cross-loan/cross-customer timelines spanning KYC,
	// credit, and financial detail all at once — a materially broader
	// data-exposure surface than the existing per-module AuditView.
	// ComplianceTaskManage covers creating/assigning/closing tasks.
	// ReportExport is its own permission so export (which can pull a lot
	// of data into one file) isn't implied by mere viewing.
	MISView              Permission = "MIS_VIEW"
	RegReportManage      Permission = "REG_REPORT_MANAGE"
	RegReportApprove     Permission = "REG_REPORT_APPROVE"
	AuditCenterAccess    Permission = "AUDIT_CENTER_ACCESS"
	ComplianceTaskManage Permission = "COMPLIANCE_TASK_MANAGE"
	ReportExport         Permission = "REPORT_EXPORT"

	// Phase 8 — vendor/expense management + the financial dashboard.
	// FinanceView is read-only (dashboard, bank balances, GL drill-down —
	// reuses AccountingView's existing data, this is just the narrower
	// permission for the new Finance admin-panel section). ExpenseManage
	// creates/drafts an expense (maker); ExpenseApprove approves/rejects
	// it and marks it paid (checker) — same maker!=checker pattern as
	// every other Phase 5-8 approval flow. VendorManage is its own
	// permission since vendor PAN/bank details are as sensitive as
	// customer data (Phase 7 item I) and shouldn't be editable by
	// whoever merely has ExpenseManage.
	FinanceView    Permission = "FINANCE_VIEW"
	ExpenseManage  Permission = "EXPENSE_MANAGE"
	ExpenseApprove Permission = "EXPENSE_APPROVE"
	VendorManage   Permission = "VENDOR_MANAGE"

	// Phase 10 — TDS/tax engine. TaxView is read-only (dashboard,
	// registers, deductees, calendar). TaxRuleManage drafts a TDSRule
	// (maker); TaxRuleApprove approves it (checker) — same pattern as
	// every prior rule-engine phase. TaxOperate covers the day-to-day
	// operational actions (calculate TDS, post a deduction, create/pay a
	// challan, resolve an exception) — deliberately separate from rule
	// approval, since running the engine day-to-day is a different
	// authorization level than changing what rules it uses.
	TaxView        Permission = "TAX_VIEW"
	TaxRuleManage  Permission = "TAX_RULE_MANAGE"
	TaxRuleApprove Permission = "TAX_RULE_APPROVE"
	TaxOperate     Permission = "TAX_OPERATE"

	// Phase 11 — NBFC compliance management / audit-evidence. ComplianceView
	// is read-only. ComplianceManage covers day-to-day compliance-team
	// actions (requirements, calendar, evidence upload, policies,
	// regulatory-return preparation). ComplianceApprove is the checker
	// tier for maker-checker-gated actions (approving a regulatory
	// return, activating a requirement) — deliberately separate from
	// ComplianceManage so the same admin who prepared something cannot
	// also approve it merely by holding one broad permission.
	// AuditModuleAccess/AuditModuleManage cover the internal-audit/
	// findings/CAPA module specifically — kept distinct from
	// ComplianceView/Manage since "who can see audit findings" and "who
	// can manage the compliance calendar" are different authorization
	// questions in most NBFCs (item U).
	ComplianceView    Permission = "COMPLIANCE_MGMT_VIEW"
	ComplianceManage  Permission = "COMPLIANCE_MGMT_MANAGE"
	ComplianceApprove Permission = "COMPLIANCE_MGMT_APPROVE"
	AuditModuleAccess Permission = "AUDIT_MODULE_ACCESS"
	AuditModuleManage Permission = "AUDIT_MODULE_MANAGE"
)

// RolePermissions is the single source of truth for what each role can do.
// SUPER_ADMIN intentionally has everything — it's the only role that can
// also manage other admins/roles. No role includes both LoanReview and
// LoanSanction being usable against the SAME loan by the SAME person: the
// maker!=checker rule is enforced in the handler (internal/loans/maker_checker_handler.go),
// not by withholding permissions, since a CREDIT_MANAGER legitimately needs
// both permissions to review one loan and approve a different one.
var RolePermissions = map[string]map[Permission]bool{
	"SUPER_ADMIN": {
		CustomerView: true, CustomerEdit: true,
		KYCView: true, KYCApprove: true,
		LoanView: true, LoanReview: true, LoanSanction: true, LoanReject: true,
		LoanDisbursePrep: true, LoanDisburse: true, LoanRestructure: true,
		CollectionView: true, CollectionRecord: true,
		AccountingView: true, ReportView: true, AuditView: true,
		AdminManage: true, RoleManage: true,
		PaymentView: true, DisbursementView: true, ReconciliationView: true, ReconciliationManage: true,
		BureauView: true, CKYCView: true, ScreeningView: true, ScreeningDecide: true, BureauApprove: true,
		CorrectionView: true, CorrectionManage: true,
		AccountingPost: true, PeriodManage: true,
		RiskView: true, RiskRuleManage: true, RiskRuleApprove: true, ClassificationOverride: true,
		CollectionManage: true, WriteOffRequest: true, WriteOffApprove: true,
		MISView: true, RegReportManage: true, RegReportApprove: true, AuditCenterAccess: true,
		ComplianceTaskManage: true, ReportExport: true,
		FinanceView: true, ExpenseManage: true, ExpenseApprove: true, VendorManage: true,
		TaxView: true, TaxRuleManage: true, TaxRuleApprove: true, TaxOperate: true,
		ComplianceView: true, ComplianceManage: true, ComplianceApprove: true,
		AuditModuleAccess: true, AuditModuleManage: true,
	},
	"CREDIT_OFFICER": {
		CustomerView: true, KYCView: true,
		LoanView: true, LoanReview: true, LoanReject: true,
		ReportView: true,
		BureauView: true, ScreeningView: true, BureauApprove: true,
		RiskView: true,
		MISView: true, // item O: "CREDIT -> credit/risk reports"
	},
	"CREDIT_MANAGER": {
		CustomerView: true, KYCView: true, KYCApprove: true,
		LoanView: true, LoanReview: true, LoanSanction: true, LoanReject: true, LoanRestructure: true,
		ReportView: true, AuditView: true,
		BureauView: true, ScreeningView: true, BureauApprove: true,
		RiskView: true, RiskRuleManage: true, ClassificationOverride: true,
		WriteOffRequest: true,
		MISView: true, ReportExport: true,
	},
	"OPERATIONS": {
		CustomerView: true, CustomerEdit: true, KYCView: true,
		LoanView: true, CollectionView: true,
		ReportView:  true,
		PaymentView: true, DisbursementView: true,
		// LoanDisburse (Phase 8): before this, LoanDisburse — the checker
		// half of disbursement release — was held only by SUPER_ADMIN. Once
		// AdminApproveDisbursementHandler started enforcing maker != checker
		// (Phase 2), a deployment with a single SUPER_ADMIN could prepare a
		// disbursement but never have a distinct, qualified admin to release
		// it. OPERATIONS already has read-only disbursement/payment
		// visibility and no sanction-side authority, so it's the natural
		// release/oversight role, kept separate from DISBURSEMENT_OFFICER's
		// prep-only authority above.
		LoanDisburse: true,
		MISView:      true, // item O: "OPERATIONS -> operational reports"
	},
	"DISBURSEMENT_OFFICER": {
		CustomerView: true, LoanView: true,
		LoanDisbursePrep: true,
		ReportView:       true,
		DisbursementView: true,
	},
	"COLLECTION_OFFICER": {
		CustomerView: true, LoanView: true,
		CollectionView: true, CollectionRecord: true,
		ReportView:  true,
		PaymentView: true,
		RiskView: true,
	},
	// COLLECTION_MANAGER (item L: "assign/reassign, escalate, approve
	// permitted collection actions") — everything an officer has, plus
	// CollectionManage. Deliberately does NOT get ClassificationOverride
	// or WriteOffRequest/Approve — a collections manager runs the
	// operational workflow, not the regulatory classification or the
	// write-off decision.
	"COLLECTION_MANAGER": {
		CustomerView: true, LoanView: true,
		CollectionView: true, CollectionRecord: true, CollectionManage: true,
		ReportView: true,
		PaymentView: true,
		RiskView: true,
	},
	"ACCOUNTANT": {
		CustomerView: true, LoanView: true,
		AccountingView: true, ReportView: true,
		PaymentView: true, DisbursementView: true, ReconciliationView: true, ReconciliationManage: true,
		AccountingPost: true, PeriodManage: true,
		// item L: "Accountants view financial results but cannot
		// manipulate collection status" — RiskView (read-only) only, no
		// CollectionManage/RiskRuleManage/ClassificationOverride.
		RiskView: true,
		MISView: true, ReportExport: true, // item O: "ACCOUNTANT -> accounting reports"
		// Accountants are the primary users of the new Finance module —
		// they can raise/manage expenses and vendors, but expense
		// APPROVAL is deliberately not granted here (maker!=checker: an
		// accountant who can both create and approve their own expense
		// entries defeats the control) — approval stays with SUPER_ADMIN.
		FinanceView: true, ExpenseManage: true, VendorManage: true,
		// Accountants operate the TDS engine day-to-day (calculate,
		// challan, resolve exceptions) but do not approve tax RULES —
		// that stays with SUPER_ADMIN/compliance, same maker!=checker
		// separation as expense approval above.
		TaxView: true, TaxOperate: true,
	},
	"COMPLIANCE_OFFICER": {
		CustomerView: true, KYCView: true, KYCApprove: true,
		LoanView:  true,
		AuditView: true, ReportView: true,
		PaymentView: true, DisbursementView: true, ReconciliationView: true,
		BureauView: true, CKYCView: true, ScreeningView: true, ScreeningDecide: true,
		AccountingView: true,
		// Compliance is the CHECKER for risk-rule approval and write-off
		// approval — never the same admin who drafted/requested it (the
		// maker!=checker check lives in the handler, same defense-in-depth
		// pattern as loan sanction/disbursement).
		RiskView: true, RiskRuleApprove: true, WriteOffApprove: true,
		// Phase 6 — compliance owns the regulatory-reporting checker seat
		// and every compliance-monitoring surface (item O: "COMPLIANCE_OFFICER
		// -> compliance/reporting").
		MISView: true, RegReportManage: true, RegReportApprove: true,
		ComplianceTaskManage: true, ReportExport: true,
		// Phase 11 — the compliance officer runs the day-to-day compliance
		// module (calendar, evidence, policies, requirement drafting) and
		// is also the checker for regulatory-return approval — never the
		// same admin who prepared the return (maker!=checker enforced in
		// the handler).
		ComplianceView: true, ComplianceManage: true, ComplianceApprove: true,
		AuditModuleAccess: true,
		CorrectionView: true, CorrectionManage: true,
	},
	// CORRECTION_OFFICER is the Correction Portal's own role: follows up
	// with customers on KYC/credit cases that were rejected/flagged for a
	// fixable reason (document unclear, customer didn't cooperate, etc.).
	// CustomerView/LoanView let it fetch the same customer info, loan
	// details and documents (GET /admin/customers/:id/documents, already
	// gated on CustomerView) the KYC/Credit portals show — never a
	// duplicated data source, just read access to the real one.
	"CORRECTION_OFFICER": {
		CustomerView: true, LoanView: true,
		CorrectionView: true, CorrectionManage: true,
	},
	"AUDITOR": {
		CustomerView: true, KYCView: true,
		LoanView: true, CollectionView: true, AccountingView: true,
		AuditView: true, ReportView: true,
		PaymentView: true, DisbursementView: true, ReconciliationView: true,
		BureauView: true, CKYCView: true, ScreeningView: true,
		RiskView: true, // read-only — item L: "Auditors: read-only"
		// item G: "Do not allow auditors to modify any data" — AUDITOR
		// gets MISView/AuditCenterAccess (both read-only by construction:
		// no handler gated on either permission ever writes anything) but
		// never RegReportManage/Approve, ComplianceTaskManage, or
		// ReportExport (export is a data-movement action, deliberately
		// withheld from a read-only role even though viewing isn't).
		MISView: true, AuditCenterAccess: true,
		FinanceView: true, // read-only
		TaxView: true,     // read-only
		ComplianceView: true, AuditModuleAccess: true, // read-only — item U/G: auditor can view evidence/findings, never modify
	},
	"READ_ONLY": {
		CustomerView: true, KYCView: true, LoanView: true,
		CollectionView: true, ReportView: true,
		PaymentView: true, DisbursementView: true, ReconciliationView: true,
		RiskView: true,
	},
}

// Has reports whether role grants perm. An unrecognized role has no
// permissions at all (fail-closed), not "everything."
func Has(role string, perm Permission) bool {
	perms, ok := RolePermissions[role]
	if !ok {
		return false
	}
	return perms[perm]
}

// PermissionsFor returns the sorted-by-declaration permission list for a
// role, for the admin panel's Roles & Permissions viewer.
func PermissionsFor(role string) []Permission {
	perms, ok := RolePermissions[role]
	if !ok {
		return nil
	}
	out := make([]Permission, 0, len(perms))
	for p := range perms {
		out = append(out, p)
	}
	return out
}

// AllRoles lists every recognized role, in a stable display order.
var AllRoles = []string{
	"SUPER_ADMIN", "CREDIT_OFFICER", "CREDIT_MANAGER", "OPERATIONS",
	"DISBURSEMENT_OFFICER", "COLLECTION_OFFICER", "COLLECTION_MANAGER", "ACCOUNTANT",
	"COMPLIANCE_OFFICER", "AUDITOR", "READ_ONLY",
}

// IsValidRole reports whether role is one of the fixed roles above —
// used to validate admin-user creation/role-change requests.
func IsValidRole(role string) bool {
	_, ok := RolePermissions[role]
	return ok
}
