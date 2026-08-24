# Phase 16 — Complete Route Inventory (item AH)

Generated from the REAL production route table (`routes.RegisterRoutes`) by
`TestGenerateRouteInventory` — not hand-maintained, so it cannot drift from
the code.

**IMPORTANT — inventory is not testing.** This document establishes 100%
route INVENTORY. It does NOT claim these routes were security-tested.
Phase 14 security-tested a representative subset; the Phase 16 report keeps
INVENTORIED, SECURITY-TESTED and NOT-TESTED as separate counts.

## Totals

| Tier | Routes |
|---|---|
| Public (unauthenticated) | 10 |
| Customer JWT | 26 |
| Employee JWT | 7 |
| Admin JWT + RBAC permission | 248 |
| **TOTAL** | **291** |

## Public routes (10)

Unauthenticated by design. Each is individually rate-limited (see `internal/routes/routes.go`); the Razorpay webhook additionally verifies an HMAC signature inside its handler.

| Method | Path |
|---|---|
| POST | `/admin/login` |
| POST | `/admin/refresh` |
| POST | `/auth/create-mpin` |
| POST | `/auth/login` |
| POST | `/auth/send-otp` |
| POST | `/auth/verify-otp` |
| GET | `/notifications` |
| POST | `/notifications/:id/read` |
| POST | `/webhooks/razorpay` |
| GET | `/ws/call/:loanId` |

## Customer-authenticated routes (26)

Require a valid customer JWT (`AuthMiddleware`). Resource ownership is enforced inside each handler.

| Method | Path |
|---|---|
| POST | `/auth/aadhaar` |
| POST | `/auth/address` |
| PUT | `/auth/biometric` |
| GET | `/auth/credit-score` |
| GET | `/auth/dashboard/emi-summary` |
| GET | `/auth/documents` |
| POST | `/auth/documents` |
| POST | `/auth/employment` |
| GET | `/auth/kyc/status` |
| GET | `/auth/kyc/v2/status` |
| GET | `/auth/loans/:id` |
| GET | `/auth/loans/:id/emi-schedule` |
| POST | `/auth/loans/:id/emi/:installmentId/create-order` |
| POST | `/auth/loans/:id/emi/:installmentId/pay` |
| POST | `/auth/loans/:id/esign` |
| POST | `/auth/loans/:id/topup` |
| POST | `/auth/loans/apply` |
| GET | `/auth/loans/mine` |
| POST | `/auth/pan` |
| GET | `/auth/payments/:paymentId` |
| GET | `/auth/profile` |
| POST | `/auth/profile` |
| POST | `/auth/selfie` |
| GET | `/auth/support/messages` |
| POST | `/auth/support/messages` |
| GET | `/auth/transactions` |

## Employee-authenticated routes (7)

Require a valid employee JWT (`EmployeeAuthMiddleware`).

| Method | Path |
|---|---|
| POST | `/employee/loans/:id/documents` |
| POST | `/employee/loans/:id/verify` |
| POST | `/employee/login` |
| POST | `/employee/recovery/cases/:id/report` |
| GET | `/employee/recovery/cases/today` |
| POST | `/employee/register` |
| GET | `/employee/tasks/today` |

## Admin routes (248)

Require a valid admin JWT (`AdminAuthMiddleware`, which also re-checks session revocation and account status on every request), the blanket `admin.api` rate limit, AND an explicit `rbac.Permission` per route.

| Method | Path |
|---|---|
| GET | `/admin/accounting/accounts` |
| POST | `/admin/accounting/accounts` |
| PUT | `/admin/accounting/accounts/:id` |
| GET | `/admin/accounting/funding` |
| POST | `/admin/accounting/funding` |
| GET | `/admin/accounting/gst` |
| GET | `/admin/accounting/invoices` |
| GET | `/admin/accounting/journal` |
| POST | `/admin/accounting/journal` |
| GET | `/admin/accounting/journal/:id` |
| POST | `/admin/accounting/journal/:id/reverse` |
| GET | `/admin/accounting/periods` |
| POST | `/admin/accounting/periods/:id/close` |
| POST | `/admin/accounting/periods/:id/reopen` |
| GET | `/admin/accounting/reports/balance-sheet` |
| GET | `/admin/accounting/reports/general-ledger` |
| GET | `/admin/accounting/reports/profit-and-loss` |
| GET | `/admin/accounting/reports/trial-balance` |
| GET | `/admin/accounting/tds` |
| GET | `/admin/admin-users` |
| POST | `/admin/admin-users` |
| PUT | `/admin/admin-users/:id/role` |
| PUT | `/admin/admin-users/:id/status` |
| GET | `/admin/agencies` |
| POST | `/admin/agencies` |
| DELETE | `/admin/agencies/:id` |
| POST | `/admin/analytics/alerts/:id/resolve` |
| GET | `/admin/analytics/budget-variance` |
| GET | `/admin/analytics/channels` |
| GET | `/admin/analytics/cohorts` |
| GET | `/admin/analytics/collection-risk` |
| GET | `/admin/analytics/concentration` |
| GET | `/admin/analytics/data-quality` |
| POST | `/admin/analytics/data-quality/run-checks` |
| GET | `/admin/analytics/decision-center` |
| GET | `/admin/analytics/dpd-trend` |
| GET | `/admin/analytics/expense-efficiency` |
| GET | `/admin/analytics/funnel` |
| GET | `/admin/analytics/geography` |
| GET | `/admin/analytics/growth` |
| GET | `/admin/analytics/growth-quality` |
| GET | `/admin/analytics/growth/mom` |
| GET | `/admin/analytics/growth/yoy` |
| GET | `/admin/analytics/kpi-dictionary` |
| GET | `/admin/analytics/outliers` |
| GET | `/admin/analytics/portfolio-risk` |
| GET | `/admin/analytics/products` |
| GET | `/admin/analytics/profitability` |
| GET | `/admin/analytics/retention` |
| GET | `/admin/analytics/revenue-quality` |
| GET | `/admin/analytics/segments` |
| GET | `/admin/analytics/signals` |
| POST | `/admin/analytics/signals/:id/review` |
| POST | `/admin/analytics/signals/run-detection` |
| GET | `/admin/analytics/thresholds` |
| POST | `/admin/analytics/thresholds` |
| POST | `/admin/analytics/thresholds/evaluate` |
| GET | `/admin/audit-log` |
| GET | `/admin/bureau/requests` |
| GET | `/admin/ckyc` |
| POST | `/admin/ckyc/:customerId/search` |
| GET | `/admin/collections/cases` |
| GET | `/admin/collections/cases/:id` |
| POST | `/admin/collections/cases/:id/actions` |
| POST | `/admin/collections/cases/:id/assign` |
| POST | `/admin/collections/cases/:id/escalate` |
| POST | `/admin/collections/cases/:id/notes` |
| POST | `/admin/collections/cases/:id/promises` |
| GET | `/admin/compliance/alerts` |
| POST | `/admin/compliance/alerts/:id/resolve` |
| GET | `/admin/compliance/audits` |
| POST | `/admin/compliance/audits` |
| GET | `/admin/compliance/calendar` |
| POST | `/admin/compliance/calendar` |
| PUT | `/admin/compliance/calendar/:id/status` |
| GET | `/admin/compliance/capas` |
| POST | `/admin/compliance/capas` |
| PUT | `/admin/compliance/capas/:id/status` |
| GET | `/admin/compliance/dashboard` |
| GET | `/admin/compliance/evidence` |
| POST | `/admin/compliance/evidence` |
| GET | `/admin/compliance/evidence/:id/download` |
| GET | `/admin/compliance/findings` |
| POST | `/admin/compliance/findings` |
| POST | `/admin/compliance/findings/:id/close` |
| POST | `/admin/compliance/findings/:id/management-response` |
| GET | `/admin/compliance/policies` |
| POST | `/admin/compliance/policies` |
| PUT | `/admin/compliance/policies/:id/status` |
| GET | `/admin/compliance/profile` |
| PUT | `/admin/compliance/profile` |
| POST | `/admin/compliance/profile/confirm` |
| GET | `/admin/compliance/requirements` |
| POST | `/admin/compliance/requirements` |
| POST | `/admin/compliance/requirements/:id/activate` |
| PUT | `/admin/compliance/requirements/:id/applicability` |
| GET | `/admin/compliance/returns` |
| POST | `/admin/compliance/returns` |
| POST | `/admin/compliance/returns/:id/approve` |
| POST | `/admin/compliance/returns/:id/correct` |
| POST | `/admin/compliance/returns/:id/submit` |
| POST | `/admin/compliance/sweep` |
| GET | `/admin/customers/:id/documents` |
| GET | `/admin/dashboard` |
| GET | `/admin/employees` |
| POST | `/admin/employees` |
| DELETE | `/admin/employees/:id` |
| POST | `/admin/employees/:id/approve` |
| GET | `/admin/finance/bank-accounts/:id/balance` |
| GET | `/admin/finance/bank-accounts/balances` |
| GET | `/admin/finance/dashboard` |
| GET | `/admin/finance/expenses` |
| POST | `/admin/finance/expenses` |
| POST | `/admin/finance/expenses/:id/approve` |
| POST | `/admin/finance/expenses/:id/mark-paid` |
| POST | `/admin/finance/expenses/:id/reject` |
| POST | `/admin/finance/expenses/:id/reverse` |
| GET | `/admin/finance/vendors` |
| POST | `/admin/finance/vendors` |
| PUT | `/admin/finance/vendors/:id/status` |
| GET | `/admin/kyc/:customerId` |
| POST | `/admin/kyc/:customerId/approve` |
| POST | `/admin/kyc/:customerId/reject` |
| POST | `/admin/kyc/:customerId/require-review` |
| GET | `/admin/kyc/queue` |
| GET | `/admin/loans` |
| GET | `/admin/loans/:id` |
| POST | `/admin/loans/:id/approve-disbursement` |
| POST | `/admin/loans/:id/approve-sanction` |
| POST | `/admin/loans/:id/decision` |
| POST | `/admin/loans/:id/disburse` |
| GET | `/admin/loans/:id/documents` |
| GET | `/admin/loans/:id/emi-schedule` |
| POST | `/admin/loans/:id/prepare-disbursement` |
| POST | `/admin/loans/:id/reassign` |
| POST | `/admin/loans/:id/recommend` |
| POST | `/admin/loans/:id/restructure` |
| GET | `/admin/loans/pending-approvals` |
| POST | `/admin/logout` |
| GET | `/admin/mis/executive` |
| GET | `/admin/mis/financial-year` |
| GET | `/admin/mis/funnel` |
| GET | `/admin/mis/growth` |
| GET | `/admin/mis/unit-economics` |
| GET | `/admin/ops/integrity` |
| GET | `/admin/ops/metrics` |
| GET | `/admin/payments` |
| GET | `/admin/payments/:id` |
| GET | `/admin/reconciliation/bank-accounts` |
| POST | `/admin/reconciliation/bank-accounts` |
| POST | `/admin/reconciliation/bank-accounts/:id/import` |
| GET | `/admin/reconciliation/exceptions` |
| POST | `/admin/reconciliation/exceptions/:id/resolve` |
| GET | `/admin/reconciliation/transactions` |
| POST | `/admin/reconciliation/transactions/:id/match` |
| GET | `/admin/recovery-reports` |
| POST | `/admin/recovery-reports/:id/action` |
| GET | `/admin/reporting/audit-center/customers/:customerId` |
| GET | `/admin/reporting/audit-center/loans/:loanId` |
| GET | `/admin/reporting/compliance-health` |
| GET | `/admin/reporting/definitions` |
| POST | `/admin/reporting/definitions` |
| PUT | `/admin/reporting/definitions/:id/status` |
| POST | `/admin/reporting/definitions/:id/versions` |
| GET | `/admin/reporting/exports` |
| POST | `/admin/reporting/exports` |
| GET | `/admin/reporting/exports/:id` |
| POST | `/admin/reporting/exports/:reportType` |
| GET | `/admin/reporting/mis` |
| GET | `/admin/reporting/reconciliation` |
| POST | `/admin/reporting/reconciliation/:id/resolve` |
| POST | `/admin/reporting/reconciliation/run-checks` |
| GET | `/admin/reporting/reconciliation/summary` |
| GET | `/admin/reporting/retention` |
| POST | `/admin/reporting/retention` |
| POST | `/admin/reporting/retention/:category/validate` |
| GET | `/admin/reporting/runs` |
| POST | `/admin/reporting/runs` |
| GET | `/admin/reporting/runs/:id` |
| POST | `/admin/reporting/runs/:id/close` |
| POST | `/admin/reporting/runs/:id/revalidate` |
| POST | `/admin/reporting/runs/:id/review` |
| POST | `/admin/reporting/runs/:id/submit` |
| POST | `/admin/reporting/runs/:id/submit-for-review` |
| GET | `/admin/reporting/tasks` |
| POST | `/admin/reporting/tasks` |
| POST | `/admin/reporting/tasks/:id/assign` |
| POST | `/admin/reporting/tasks/:id/status` |
| POST | `/admin/reporting/tasks/run-periodic-checks` |
| GET | `/admin/risk/alerts` |
| POST | `/admin/risk/alerts/:id/acknowledge` |
| GET | `/admin/risk/buckets` |
| POST | `/admin/risk/buckets` |
| GET | `/admin/risk/classification-rules` |
| POST | `/admin/risk/classification-rules` |
| POST | `/admin/risk/classification-rules/:id/approve` |
| GET | `/admin/risk/dashboard` |
| GET | `/admin/risk/exceptions` |
| POST | `/admin/risk/exceptions/:id/resolve` |
| POST | `/admin/risk/exceptions/run-checks` |
| GET | `/admin/risk/jobs` |
| POST | `/admin/risk/jobs/run` |
| GET | `/admin/risk/loans/:loanId` |
| POST | `/admin/risk/loans/:loanId/override-classification` |
| GET | `/admin/risk/provisioning-rules` |
| POST | `/admin/risk/provisioning-rules` |
| POST | `/admin/risk/provisioning-rules/:id/approve` |
| GET | `/admin/roles` |
| GET | `/admin/screening` |
| POST | `/admin/screening/:customerId/run` |
| POST | `/admin/screening/requests/:requestId/decide` |
| GET | `/admin/sla-alerts` |
| GET | `/admin/support/threads` |
| POST | `/admin/support/threads/:id/close` |
| GET | `/admin/support/threads/:id/messages` |
| POST | `/admin/support/threads/:id/messages` |
| GET | `/admin/tax/calendar` |
| POST | `/admin/tax/calendar` |
| POST | `/admin/tax/calendar/:id/complete` |
| GET | `/admin/tax/challans` |
| POST | `/admin/tax/challans` |
| POST | `/admin/tax/challans/:id/mark-paid` |
| POST | `/admin/tax/challans/link` |
| GET | `/admin/tax/dashboard` |
| GET | `/admin/tax/deductees` |
| POST | `/admin/tax/deductees` |
| PUT | `/admin/tax/deductees/:id/pan-status` |
| GET | `/admin/tax/exceptions` |
| POST | `/admin/tax/exceptions/:id/resolve` |
| POST | `/admin/tax/exceptions/run-checks` |
| GET | `/admin/tax/rules` |
| POST | `/admin/tax/rules` |
| POST | `/admin/tax/rules/:id/approve` |
| POST | `/admin/tax/rules/:id/retire` |
| GET | `/admin/tax/transactions` |
| POST | `/admin/tax/transactions/:id/override` |
| POST | `/admin/tax/transactions/:id/post-deduction` |
| POST | `/admin/tax/transactions/:id/reverse` |
| POST | `/admin/tax/transactions/calculate` |
| GET | `/admin/users` |
| GET | `/admin/users/:id` |
| POST | `/admin/users/:id/kyc/approve` |
| POST | `/admin/users/:id/kyc/reject` |
| GET | `/admin/writeoffs` |
| POST | `/admin/writeoffs` |
| POST | `/admin/writeoffs/:id/approve` |
| POST | `/admin/writeoffs/:id/recovery` |
| POST | `/admin/writeoffs/:id/reject` |

