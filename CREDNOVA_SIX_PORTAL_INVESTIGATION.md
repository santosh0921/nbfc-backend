# CredNova / Jayashri Capital — Backend Investigation for Six React Portals

**Type:** Read-only technical investigation. No files were modified, created, deleted, or refactored. No database schema was changed. No packages were installed.
**Purpose:** Give an engineer enough ground truth to design six new React + TypeScript portals (KYC, Credit Bureau, Underwriting, Fraud & Risk, Operations/Disbursement & Repayment, Collections & Recovery) that talk to the **same existing Go backend and PostgreSQL database**, alongside the existing Flutter Main Admin, without duplicating auth/RBAC/workflow systems.
**Companion document:** `CREDNOVA_NBFC_6_MODULE_AUDIT.md` (same repo) — an earlier, complementary audit of the same backend covering module completeness (LIVE/MOCK/MISSING per feature). This report focuses on architecture, auth, RBAC, and portal-mapping specifically; consult the companion doc for feature-completeness detail not repeated here.

Where something could not be determined with certainty from the code in the time available, it is marked **UNKNOWN** with the specific point of uncertainty stated.

---

## PART 1 — PROJECT ARCHITECTURE

```
Flutter Customer App (lib/)         Flutter Employee App (lib/employee_app/)      Flutter Admin Panel (separate repo)
        │                                        │                                          │
        │ JWT (customer secret, 24h,             │ JWT (employee secret, 12h,               │ JWT (admin secret, 15min access +
        │ stateless bearer)                      │ stateless, live Active-flag recheck)     │ 7-day rotating refresh, real server
        │                                        │                                          │ session + RBAC)
        ▼                                        ▼                                          ▼
┌─────────────────────────────────────────────────────────────────────────────────────────────┐
│                              Go + Gin Router (internal/routes/routes.go)                      │
│   /auth/*  →  AuthMiddleware()          /employee/*  →  EmployeeAuthMiddleware()              │
│   /admin/*  →  AdminAuthMiddleware() + RequirePermission(perm) per route                      │
└─────────────────────────────────────────────────────────────────────────────────────────────┘
        │                                        │                                          │
        ▼                                        ▼                                          ▼
   Handlers (internal/<domain>/*_handler.go — e.g. internal/loans, internal/kyc, internal/risk)
        │
        ▼
   Services/business logic (same packages, e.g. internal/loans/eligibility.go, internal/kyc/service.go)
        │
        ▼
   GORM models (internal/models/*.go) — no separate repository layer; handlers/services call
   `database.DB` (a package-level *gorm.DB) directly. There is no dedicated repository interface
   abstraction — GORM itself is the data-access layer.
        │
        ▼
   PostgreSQL
        │
        ▼
   External integrations: Surepass (PAN live, Aadhaar blocked), Razorpay (live, inert w/o prod
   keys), Firebase FCM (live, inert w/o prod creds), AWS S3 / local AES-256-GCM storage,
   AWS SSM Parameter Store (Firebase creds only)
```

1. **Main folders**: `internal/` (all backend logic, one Go package per domain — `loans`, `kyc`, `bureau`, `risk`, `collections`, `recovery`, `accounting`, `admin`, `employee`, `auth`, `otp`, `middleware`, `models`, `routes`, `rbac`, `audit`, `payments`, `disbursement`, `reconciliation`, `notifications`, `push`, `storage`, `config`, `database`, and more). `lib/` (Flutter customer app). `lib/employee_app/` (Flutter employee app, same repo/build, different entry). `cmd/api/` = entry point.
2. **Entry point**: `cmd/api/main.go` (not re-read in full this pass; confirmed as the binary entry point from prior work in this engagement — wires config, DB connect, migration, JWT secret init, push/storage init, then `router.Run()`).
3. **Main Go packages**: one package per `internal/<name>` directory — no monolithic `handlers.go`; the codebase is organized by business domain, not by technical layer (i.e., `internal/loans` contains its own handler + service + eligibility + letter-generation code together, not split into `handlers/loans/`, `services/loans/`).
4. **internal/ structure**: domain-per-folder, as above. No `internal/domain` / `internal/usecase` / `internal/adapter` clean-architecture split — this is a pragmatic Gin-handler-calls-GORM-directly style, not hexagonal architecture.
5. **Handlers/controllers**: `*_handler.go` files within each domain package, e.g. `internal/loans/admin_handler.go`, `internal/loans/maker_checker_handler.go`, `internal/kyc/handler.go` (file names vary per package but the pattern holds).
6. **Services/use-cases**: co-located with handlers in the same package — e.g. `internal/loans/eligibility.go`, `internal/loans/service.go`, `internal/kyc/service.go`, `internal/bureau/service.go`. Not a separate top-level `services/` folder.
7. **Repositories/data-access**: **no dedicated repository layer exists**. Handlers/services call `database.DB.Where(...).Find(...)` (GORM) directly. `database.DB` is a package-level `*gorm.DB` variable (`internal/database/database.go`).
8. **Middleware**: `internal/middleware/` — `auth_middleware.go` (customer), `employee_auth_middleware.go`, `admin_auth_middleware.go`, `require_permission.go`, `rate_limit.go`, `rate_limit_audit.go`, and others (CORS, etc., not individually re-read this pass).
9. **Models/entities**: `internal/models/*.go` — ~110 GORM-migrated structs (full inventory in the companion audit doc, §11).
10. **Routes**: single file `internal/routes/routes.go` (653 lines), all route registration centralized here — no per-package route files.
11. **Authentication**: three fully independent systems — see Part 2.
12. **Authorization/RBAC**: `internal/rbac/permissions.go` (roles, permissions, role→permission map, `Has()` function), enforced via `internal/middleware/require_permission.go` — admin-only; employee and customer auth have no equivalent granular permission system (see Part 3/4).
13. **Database layer**: GORM ORM directly, PostgreSQL. Connection setup in `internal/database/database.go`. Migrations via `gorm.AutoMigrate` in `internal/admin/migration.go` (not a versioned migration-file system like golang-migrate — schema changes happen by editing the Go struct and re-running AutoMigrate on deploy).
14. **External integrations**: Surepass (KYC — PAN live, Aadhaar blocked by token scope), Razorpay (payments — live, needs prod credentials), Firebase Cloud Messaging (push — live, needs prod credentials), AWS S3 or local AES-256-GCM (document storage), AWS SSM Parameter Store (Firebase credential only, not other secrets). No credit bureau, OCR, DigiLocker, penny-drop, NACH, SMS, or WhatsApp integration exists (see companion audit §13 for the full matrix).
15. **Background jobs/workers**: `internal/jobs/registry.go` — a scheduled-job registry exists (confirmed running the fraud-signal detection job and referenced for risk classification); **not independently re-verified in this pass how jobs are triggered (cron? a running goroutine ticker? external scheduler?)** — **UNKNOWN**, flagged for follow-up if job-triggering mechanics matter for portal design.
16. **Document/file storage**: `internal/storage/provider.go` — abstraction over S3 (production-preferred) and local AES-256-GCM encrypted disk storage (fallback), refuses to fall back to unencrypted storage if neither key is configured.
17. **Audit logging**: `internal/audit/` — `RecordDetailed()` called from most sensitive handlers, writes to `AuditLog` table which has a DB-level trigger blocking UPDATE/DELETE (append-only, confirmed in prior work this engagement).
18. **Notifications**: `internal/notifications/handler.go` (in-app notification rows) + `internal/push/push.go` (real Firebase Admin SDK integration).
19. **Configuration**: `internal/config/config.go` — reads all external-service env vars (`SUREPASS_BASE_URL`, `SUREPASS_BEARER_TOKEN`, `RAZORPAY_KEY_ID/SECRET/WEBHOOK_SECRET`, `FIREBASE_SERVICE_ACCOUNT_JSON`, `DOCUMENT_STORAGE_BUCKET`, `PAN_ENCRYPTION_KEY`, `DOCUMENT_ENCRYPTION_KEY`, `DEMO_MODE`, `DB_*`, `JWT_SECRET`/`EMPLOYEE_JWT_SECRET`/`ADMIN_JWT_SECRET`, `CORS_ALLOWED_ORIGINS`).

**Where auth/authz happen**: authentication happens in middleware (`AuthMiddleware`/`EmployeeAuthMiddleware`/`AdminAuthMiddleware`), attached per route-group in `routes.go`. Authorization (permission checking) happens **only for admin routes**, via a second middleware layer (`RequirePermission(perm)`) chained after `AdminAuthMiddleware` on individual routes — not for customer or employee routes, which have no granular permission system beyond "is this a valid, active customer/employee."

---

## PART 2 — AUTHENTICATION

### Customer
- Login: `POST /auth/login` → `auth.LoginHandler` (`internal/auth/handler.go:193`). Credential: 4/6-digit **MPIN**, bcrypt-hashed (`internal/auth/mpin.go:10-14`), requires prior OTP-verified mobile.
- JWT claims (`internal/auth/jwt.go:30-33`): `{Mobile string; RegisteredClaims}` — **no user ID, no role in the token**. HS256. Secret: `JWT_SECRET` env, via `InitJWTSecret()`.
- Access token expiry: **24 hours**. No refresh token — a single long-lived JWT.
- Session storage/revocation: **none**. Fully stateless — middleware only checks signature/expiry, no DB lookup of the user at all per request.
- Logout: `auth.Logout` (`internal/auth/logout.go`) — pure `200 OK`, invalidates nothing server-side.
- Account locking: yes — 5 failed MPIN attempts → 15-minute lock (`FailedLoginAttempts`/`LockedUntil` on `models.User`).
- Middleware: `middleware.AuthMiddleware()` on `/auth` group; `middleware.CustomerOrEmployeeAuthMiddleware()` on shared endpoints (notifications, calls, device tokens).

### Employee
- Login: `POST /employee/login` → `employee.LoginHandler` (`internal/employee/login_handler.go:36`). Credential: name + password, bcrypt. Accounts are admin-provisioned only (auto-registration removed).
- JWT claims (`internal/employee/jwt.go:36-40`): `{EmployeeID uint; Code string; Role string; RegisteredClaims}`. HS256. Secret: `EMPLOYEE_JWT_SECRET` — now fails fast if unset (previously silently fell back to a hardcoded secret; fixed).
- Access token expiry: **12 hours**. No refresh token.
- Session storage/revocation: no session table, but `EmployeeAuthMiddleware` (`internal/middleware/employee_auth_middleware.go:60-73`) re-checks `emp.Active` and `Agency.Active` via a live DB lookup on every request — deactivation takes effect immediately even without a token-revocation record.
- Logout: **no dedicated employee logout endpoint found** — client discards the token locally only.
- Account locking: **no failed-attempt lockout found** for employees (contrast with customer/admin, both of which have it).
- Middleware: `middleware.EmployeeAuthMiddleware()` on `/employee` group.
- **No granular permission system for employees** — `Role` is a plain string set in JWT/context (e.g. `verification`, `recovery`), used for routing/business logic (e.g. `EmployeeClaimLoanHandler` implicitly assumes a verification-role employee), but there is no `RequireEmployeeRole`-style middleware equivalent to the admin `RequirePermission`.

### Admin
- Login: `POST /admin/login` → `admin.LoginHandler` (`internal/admin/login_handler.go:38`). Logout: `POST /admin/logout`. Refresh: `POST /admin/refresh`.
- Credential: username + password, bcrypt. Individual `AdminUser` rows (replaced an old shared env-var credential).
- JWT claims (`internal/admin/jwt.go:45-51`): `{AdminID uint; Username string; Role string; SessionID uint; RegisteredClaims}`. HS256, **algorithm explicitly pinned** via `jwt.WithValidMethods` (anti-alg-confusion). Secret: `ADMIN_JWT_SECRET`.
- Access token expiry: **15 minutes**. Refresh token: **7 days**, opaque random 32-byte value, only its SHA-256 hash stored (`AdminSession.RefreshTokenHash`) — genuinely rotates on every use (old session revoked, new one issued).
- Session storage/revocation: **real server-side session** — `AdminSession` row per login. `AdminAuthMiddleware` does a **DB lookup on every request** confirming `RevokedAt == nil` and `AdminUser.Status == Active`.
- Logout: genuinely revokes — sets `AdminSession.RevokedAt = now`, effective immediately on the next request.
- Account locking: yes, 5 attempts → 15-min lock, plus a dummy bcrypt-compare for unknown usernames (timing-attack mitigation) and a generic error message (prevents username enumeration).
- Middleware: `AdminAuthMiddleware()` (authentication) + `RequirePermission(perm)` (authorization, chained per-route) — fails closed on an unrecognized role.

### OTP (`internal/otp/`)
`crypto/rand`-generated 6-digit codes, SHA-256 hashed at rest (never plaintext in DB), unique-per-mobile DB constraint, 5-minute expiry, 5-attempt cap, constant-time comparison. Rate-limited at the route level (send: 1/5, verify: 1/8). Plaintext OTP is only ever returned in the API response when `DEMO_MODE=true`.

### Are the three systems separate or shared?
**Fully separate.** Three independent secrets (`JWT_SECRET`, `EMPLOYEE_JWT_SECRET`, `ADMIN_JWT_SECRET`), three independent claims structs, three independent middleware functions, no shared code path. **Only admin has a real revocable session + granular RBAC.** Customer and employee tokens are stateless bearer tokens for their full duration, with employee auth partially mitigated by a live active-status recheck and customer auth having no server-side recheck of anything except token signature/expiry.

---

## PART 3 — RBAC / ROLES / PERMISSIONS

RBAC exists **only for the admin system** (`internal/rbac/permissions.go`). 11 roles confirmed present exactly as named in the request: `SUPER_ADMIN`, `CREDIT_OFFICER`, `CREDIT_MANAGER`, `OPERATIONS`, `DISBURSEMENT_OFFICER`, `COLLECTION_OFFICER`, `COLLECTION_MANAGER`, `ACCOUNTANT`, `COMPLIANCE_OFFICER`, `AUDITOR`, `READ_ONLY`.

**Full permission taxonomy** (grouped, from `internal/rbac/permissions.go`):
- Customer: `CustomerView`, `CustomerEdit`
- KYC: `KYCView`, `KYCApprove`
- Loan/underwriting: `LoanView`, `LoanReview`, `LoanSanction`, `LoanReject`, `LoanDisbursePrep`, `LoanDisburse`, `LoanRestructure`
- Bureau/screening: `BureauView`, `CKYCView`, `ScreeningView`, `ScreeningDecide`
- Risk: `RiskView`, `RiskRuleManage`, `RiskRuleApprove`, `ClassificationOverride`
- Collections: `CollectionView`, `CollectionRecord`, `CollectionManage`
- Write-off: `WriteOffRequest`, `WriteOffApprove`
- Disbursement/payments: `PaymentView`, `DisbursementView`
- Reconciliation: `ReconciliationView`, `ReconciliationManage`
- Accounting: `AccountingView`, `AccountingPost`, `PeriodManage`
- Tax: `TaxView`, `TaxRuleManage`, `TaxRuleApprove`, `TaxOperate`
- Compliance: `ComplianceView`, `ComplianceManage`, `ComplianceApprove`
- Audit: `AuditView`, `AuditCenterAccess`, `AuditModuleAccess`, `AuditModuleManage`
- MIS/reporting: `ReportView`, `MISView`, `RegReportManage`, `RegReportApprove`, `ReportExport`
- Finance/vendor/expense: `FinanceView`, `ExpenseManage`, `ExpenseApprove`, `VendorManage`
- Compliance task: `ComplianceTaskManage`
- Admin/RBAC: `AdminManage`, `RoleManage`

**Role → permission table:**

| ROLE | PERMISSIONS | WHERE DEFINED | HOW ENFORCED |
|---|---|---|---|
| SUPER_ADMIN | ALL permissions | `permissions.go:169-188` | `RequirePermission` middleware |
| CREDIT_OFFICER | CustomerView, KYCView, LoanView, LoanReview, LoanReject, ReportView, BureauView, ScreeningView, RiskView, MISView | `:189-196` | same |
| CREDIT_MANAGER | CustomerView, KYCView, KYCApprove, LoanView, LoanReview, LoanSanction, LoanReject, LoanRestructure, ReportView, AuditView, BureauView, ScreeningView, RiskView, RiskRuleManage, ClassificationOverride, WriteOffRequest, MISView, ReportExport | `:197-205` | same |
| OPERATIONS | CustomerView, CustomerEdit, KYCView, LoanView, CollectionView, ReportView, PaymentView, DisbursementView, MISView | `:206-212` | same |
| DISBURSEMENT_OFFICER | CustomerView, LoanView, LoanDisbursePrep, ReportView, DisbursementView | `:213-218` | same |
| COLLECTION_OFFICER | CustomerView, LoanView, CollectionView, CollectionRecord, ReportView, PaymentView, RiskView | `:219-225` | same |
| COLLECTION_MANAGER | CustomerView, LoanView, CollectionView, CollectionRecord, CollectionManage, ReportView, PaymentView, RiskView | `:232-238` | same |
| ACCOUNTANT | CustomerView, LoanView, AccountingView, ReportView, PaymentView, DisbursementView, ReconciliationView, ReconciliationManage, AccountingPost, PeriodManage, RiskView, MISView, ReportExport, FinanceView, ExpenseManage, VendorManage, TaxView, TaxOperate | `:239-260` | same |
| COMPLIANCE_OFFICER | CustomerView, KYCView, KYCApprove, LoanView, AuditView, ReportView, PaymentView, DisbursementView, ReconciliationView, BureauView, CKYCView, ScreeningView, ScreeningDecide, AccountingView, RiskView, RiskRuleApprove, WriteOffApprove, MISView, RegReportManage, RegReportApprove, ComplianceTaskManage, ReportExport, ComplianceView, ComplianceManage, ComplianceApprove, AuditModuleAccess | `:261-285` | same |
| AUDITOR | CustomerView, KYCView, LoanView, CollectionView, AccountingView, AuditView, ReportView, PaymentView, DisbursementView, ReconciliationView, BureauView, CKYCView, ScreeningView, RiskView, MISView, AuditCenterAccess, FinanceView, TaxView, ComplianceView, AuditModuleAccess (**read-only by design** — never holds any `*Post`/`*Approve`/`*Manage` write permission) | `:286-303` | same |
| READ_ONLY | CustomerView, KYCView, LoanView, CollectionView, ReportView, PaymentView, DisbursementView, ReconciliationView, RiskView | `:304-309` | same |

`Has(role, perm)` (`permissions.go:314-320`) **fails closed** for unrecognized roles — returns `false`, not a default allow.

**Important note for portal design**: CREDIT_OFFICER/CREDIT_MANAGER map almost exactly onto the proposed "Underwriting" portal; OPERATIONS + DISBURSEMENT_OFFICER map onto "Operations/Disbursement"; COLLECTION_OFFICER/COLLECTION_MANAGER map onto "Collections & Recovery"; COMPLIANCE_OFFICER holds most Bureau/Screening/CKYC permissions, closest existing match for a "Credit Bureau" or "Fraud & Risk" portal user, though no role is named specifically for KYC-only or Fraud-only staff — this is addressed in Part 17.

---

## PART 4 — PERMISSION ENFORCEMENT

| ACTION | REQUIRED PERMISSION | BACKEND ENFORCED? | WHERE | NOTES |
|---|---|---|---|---|
| KYC approval/rejection | `KYCApprove` | **C** (middleware) | `routes.go:157-158,166-167` | Both legacy and new KYC endpoints gated identically |
| Loan review/recommend sanction | `LoanReview` | **C** | `routes.go:537` | + in-handler status whitelist, CIBIL check |
| Loan sanction approval | `LoanSanction` | **C** | `routes.go:538` | + in-handler maker≠checker ID compare (confirmed present here, unlike disbursement) |
| Loan rejection | `LoanReject` | **B/partial** | `routes.go:536` | Permission is enforced; **no status-precondition check in the handler body** — can reject a loan in any state including already-sanctioned/disbursed |
| Loan restructuring | `LoanRestructure` | **C** | `routes.go:546` | + status==disbursed check |
| Disbursement preparation | `LoanDisbursePrep` | **C** | `routes.go:541` | + status==sanctioned check |
| Disbursement approval/release | `LoanDisburse` | **C** | `routes.go:542` | + status check; **maker≠checker ID comparison is NOT present in the handler body** (see Part 5) |
| Payment/EMI recording | none (customer-authenticated write; admin view is `PaymentView`) | **C** (view) / **D** (no dedicated admin "record payment" permission — by design, payments are gateway-webhook-driven, not admin-entered) | `emi_handler.go`, `routes.go:569` | Correct by design — see companion audit §8 |
| Accounting journal posting | `AccountingPost` | **C** | `routes.go:199` | Separate from `AccountingView` |
| Reconciliation match | `ReconciliationManage` | **C** | `routes.go:577` | |
| Write-off request | `WriteOffRequest` | **C** | `routes.go:401` | |
| Write-off approval | `WriteOffApprove` | **C** | `routes.go:402-404` | Maker≠checker expected but **not independently re-verified in this handler in this pass** — flag for follow-up |
| Collection action recording | `CollectionRecord` | **C** | `routes.go:553` | |
| Collection case assignment | `CollectionRecord` (not `CollectionManage`) | **C**, but possibly the **wrong permission** | `routes.go:554` | `CollectionManage` exists specifically for assign/reassign per RBAC comments but isn't used on this route — worth reviewing before building the Collections portal's permission checks |
| Compliance approval | `ComplianceApprove` | **C** | `routes.go:301` | |
| Risk rule approval | `RiskRuleApprove` | **C** | `routes.go:354` | |
| Risk manual override | `ClassificationOverride` | **C** | `routes.go:355` | |
| Screening decision | `ScreeningDecide` | **C** | `routes.go:192` | Deliberately narrow — only COMPLIANCE_OFFICER/SUPER_ADMIN hold it |
| Report export | `ReportExport` | **C** | `routes.go:413` | |
| Role management | `RoleManage` (role change) / `AdminManage` (user CRUD) | **C** | `routes.go:146-150` | Correctly split |
| Employee account creation/approval | `AdminManage` | **C** | `routes.go:561-562` | No dedicated employee-management permission — broad `AdminManage` covers it |

**Legend**: A=frontend-only, B=backend-partial, C=backend-enforced (middleware), D=not enforced.

---

## PART 5 — MAKER-CHECKER

**This is the single most important finding in this investigation and is corroborated across three independent passes in this engagement (two audit sub-agents plus this session's own earlier live test execution).**

| Workflow | Maker ID stored? | Checker ID stored? | `maker != checker` enforced server-side? | Notes |
|---|---|---|---|---|
| Loan sanction recommend→approve | Yes (`RecommendedByAdminID`) | Yes (`DecidedByAdminID`/approving admin) | **YES — confirmed present** in `AdminApproveSanctionHandler` (`internal/loans/maker_checker_handler.go`) | This specific pair does compare IDs |
| Disbursement prepare→approve | Yes (`DisbursementPreparedByAdminID`) | Yes (`DisbursementApprovedByAdminID`) | **NO — confirmed absent.** `AdminApproveDisbursementHandler` (`maker_checker_handler.go:356-477`) loads `loan.DisbursementPreparedByAdminID` but never compares it to the approving admin's own ID | A `CREDIT_MANAGER`-equivalent role holding both prep and approve permissions can prepare and release the same disbursement alone |
| Write-off request→approve | Yes (`RequestedBy`) | Yes (`ApprovedBy`) | **Not independently re-verified in the handler body in this pass** — flagged, needs a dedicated follow-up read of `internal/writeoff/*.go` | |
| Accounting approvals | UNKNOWN — not read in this pass | UNKNOWN | UNKNOWN | Needs follow-up if accounting maker-checker matters for portal scope |
| Compliance approvals | UNKNOWN — not read in this pass | UNKNOWN | UNKNOWN | Same |
| Risk classification rule draft→approve | Yes (draft creator) | Yes (`RiskRuleApprove` actor) | **Confirmed present** per the companion audit's earlier read of `internal/risk/classification.go` (`ManualOverrideClassification` requires reason+approver, drafted rules must be `status=APPROVED` before use) | |

**The exact condition `if makerID == checkerID { reject }` does NOT exist for the disbursement approval path.** It DOES exist for the loan sanction approval path. This is an inconsistent, partial implementation — not a uniformly missing control, and not a uniformly present one either. **Corroborating evidence**: this session earlier ran the actual backend test suite and found 4 currently-**failing** regression tests that were clearly written specifically to catch this: `TestMakerCannotApproveOwnRecommendation`, `TestMakerCannotApproveOwnSanctionRecommendation`, `TestMakerCannotApproveOwnDisbursementPreparation`, `TestDisbursementMakerCannotApproveOwn`. (Re-reading the sanction-approval code in this pass shows the ID-comparison IS present there — the test failures may indicate the check has a bug rather than being fully absent, e.g. wrong variable compared, or the tests may be probing edge cases like a nil-pointer comparison. **This discrepancy between "code appears to compare IDs" and "the regression test for that exact behavior fails" should be the very first thing re-verified with a debugger/test run before any portal work begins** — it is not resolved by static reading alone.)

**Database constraints**: no DB-level CHECK constraint enforcing maker≠checker was found (this would need to be application-layer regardless, since it depends on comparing two nullable FK columns at write time, which Postgres CHECK constraints can express but none were found doing so).

**Audit trail**: present for all maker-checker workflows checked — `RecordDetailed()` calls confirmed on sanction/disbursement/write-off actions.

---

## PART 6 — LOAN LIFECYCLE

**Actual status constants** (`internal/models/loan.go`, in declared order) — do not assume any others exist:

```
submitted → auto_verified (dead path — auto-approve category map is always empty)
submitted → pending_claim → assigned → verified
verified → pending_sanction_approval → sanctioned  (or → rejected)
sanctioned → ready_for_disbursement → disbursement_processing → disbursed
(any status) → rejected   [⚠ no precondition check — see below]
```

Full constant list: `submitted`, `auto_verified`, `assigned`, `verified`, `sanctioned`, `rejected`, `disbursed`, `pending_sanction_approval`, `ready_for_disbursement`, `disbursement_processing`, `pending_claim`. All 11 are actually assigned somewhere in code — none are dead constants, though the `auto_verified` **transition** is effectively dead (the category map that would trigger it is deliberately empty).

**State transition table:**

| FROM | TO | HANDLER | PERMISSION | PRECONDITION CHECKED? |
|---|---|---|---|---|
| (new) | `submitted` | `submitLoanApplication` (service.go:77) | customer JWT | n/a (initial) |
| `submitted` | `pending_claim` | service.go:91 | customer JWT | city employees exist |
| `submitted` | `assigned` | service.go:105 | customer JWT | fallback, no city match |
| `pending_claim` | `assigned` | `EmployeeClaimLoanHandler` | employee JWT (no RBAC) | Yes — status + unclaimed check, row-locked |
| `assigned` | `verified` | `EmployeeVerifyLoanHandler` | employee JWT (no RBAC) | **NO — only checks assignment, not prior status** |
| verified/etc. | `pending_sanction_approval` | `AdminRecommendSanctionHandler` | `LoanReview` | Yes — status whitelist + CIBIL |
| `pending_sanction_approval` | `sanctioned` / `rejected` | `AdminApproveSanctionHandler` | `LoanSanction` | Yes — status check, row-locked, maker≠checker |
| **any status** | `rejected` | `AdminDecisionHandler` | `LoanReject` | **NO status precondition at all** |
| `sanctioned` | `ready_for_disbursement` | `AdminPrepareDisbursementHandler` | `LoanDisbursePrep` | Yes |
| `ready_for_disbursement` | `disbursement_processing` / back to `sanctioned` (reject) | `AdminApproveDisbursementHandler` | `LoanDisburse` | Yes — status check, row-locked, **maker≠checker not enforced** |
| `disbursement_processing` | `disbursed` | `ConfirmDisbursement` (reconciliation match or demo-confirm) | `LoanDisburse` (demo path) | Yes, idempotent |

**Two state-machine integrity gaps found:**
1. `AdminDecisionHandler` (reject) can flip a loan to `rejected` from **any** current status, including `disbursed` — potentially orphaning downstream EMI/disbursement records with no state-machine guard against it.
2. `EmployeeVerifyLoanHandler` does not check the loan's current status before setting it to `verified` — only checks that the calling employee is the assigned one. A loan already past `verified` could theoretically be reset if still assigned to that employee.

`AdminRestructureLoanHandler` requires `status == disbursed` but does not itself change `Status` — it rewrites the EMI schedule in place.

`POST /admin/loans/:id/disburse` (old single-admin path) is registered but returns `410 Gone` — intentionally disabled, not a live transition.

**State diagram (actual, not assumed):**

```
                           ┌──────────────┐
                           │  submitted   │
                           └──────┬───────┘
                    ┌─────────────┼─────────────┐
                    ▼             ▼             ▼
            pending_claim   auto_verified   assigned (fallback)
                    │          (dead)            │
                    └─────────────┬───────────────┘
                                  ▼
                             assigned
                                  │  (EmployeeVerifyLoanHandler — no prior-status check)
                                  ▼
                              verified
                                  │  (AdminRecommendSanctionHandler)
                                  ▼
                  pending_sanction_approval
                          │              │
                (approve) ▼              ▼ (reject)
                     sanctioned      rejected
                          │
                          ▼ (AdminPrepareDisbursementHandler)
              ready_for_disbursement
                          │              │
                (approve) ▼              ▼ (reject → back to sanctioned)
              disbursement_processing
                          │
                          ▼ (ConfirmDisbursement)
                      disbursed

              [rejected] ◄────── reachable from ANY state via AdminDecisionHandler,
                                  no precondition check (integrity gap)
```

**KYC has its own separate, more rigorous state machine** (not part of `LoanApplication.Status`) — see Part 7.

---

## PART 7 — KYC

*(Full detail in companion audit §4; summarized here for portal-mapping purposes.)*

| API | Purpose | Role/Permission | Status |
|---|---|---|---|
| `POST /auth/pan` | Customer submits PAN | Customer JWT | **IMPLEMENTED** — real Surepass call outside DEMO_MODE |
| `POST /auth/aadhaar` | Customer submits Aadhaar | Customer JWT | **PARTIALLY IMPLEMENTED** — Surepass token lacks required scope, routes to manual review |
| `POST /auth/selfie` | Customer submits selfie | Customer JWT | **PARTIALLY IMPLEMENTED** — real format/size validation only, no liveness vendor |
| `GET /auth/kyc/v2/status` | Customer views KYC status | Customer JWT | **IMPLEMENTED** — real state machine |
| `GET /admin/kyc/queue` | Admin KYC review queue | `KYCView` | **IMPLEMENTED** |
| `POST /admin/kyc/:customerId/approve`/`reject` | Admin KYC decision | `KYCApprove` | **IMPLEMENTED**, real audit trail |
| OCR | — | — | **MISSING** — no OCR pipeline anywhere |
| DigiLocker | — | — | **MISSING** |

KYC state machine (`internal/kyc/service.go`): `NOT_STARTED → INITIATED → DOCUMENTS_PENDING → VERIFICATION_PENDING → REQUIRES_REVIEW/REJECTED → VERIFIED`. **No automated path can reach `VERIFIED`** — requires both an admin-approved review AND all sub-provider checks passed. This is genuinely enforced, not merely claimed.

---

## PART 8 — CREDIT BUREAU

*(Full detail in companion audit §5.)*

- `internal/bureau/service.go` — real `Provider` interface, only concrete implementation is a `sandboxProvider` returning a **fixed 780 score**, gated to non-production only with a defense-in-depth re-check even if misconfigured. In production, `PullScore` returns `ErrProviderNotConfigured` — fails closed, never fakes a score.
- **No real bureau (CIBIL/Experian/CRIF/Equifax) is integrated** — no credentials, no concrete provider.
- Consent (`BureauConsent`) required before any pull; every pull recorded as `BureauRequest` (score + hash of raw response, never raw response itself) regardless of outcome.
- Admin view: `GET /admin/bureau/requests` (`BureauView`).
- CKYC: same seam pattern, `internal/ckyc/service.go`, provider never assigned in any environment — every request honestly writes `BLOCKED`, never fakes a CKYC number.
- PEP/Sanctions screening: same pattern, `internal/screening/service.go` — with no vendor, always `REVIEW_REQUIRED`, never auto-clears.

---

## PART 9 — UNDERWRITING

*(Full detail in companion audit §6.)*

- **All underwriting calculations happen in the Go backend** — `internal/loans/eligibility.go`: amount bounds (₹50k–₹50L), tenure bounds (12–60mo), FOIR cap (50%, real reducing-balance EMI formula), CIBIL 3-band rate/reject. This is the **source of truth** — the backend, not any frontend, owns these numbers.
- **FOIR is single-loan only** — does not sum the customer's other active loan EMIs (no true multi-loan DTI).
- **Income is self-declared entry only** — no document/payslip/bank-statement cross-check, no employment-type-differentiated rules.
- Every decision (eligibility + sanction) recorded to `CreditDecision` with a policy version and structured reason codes (`CIBIL_BELOW_THRESHOLD`, `AMOUNT_OUT_OF_RANGE`, `FOIR_EXCEEDED`, `OTHER`).
- **Maker-checker for underwriting/sanction is enforced** at the ID-comparison level (see Part 5) — though the currently-failing regression tests warrant a live re-verification before relying on this.

---

## PART 10 — FRAUD & RISK

*(Full detail in companion audit §7.)*

| Check | Status |
|---|---|
| Duplicate PAN/Aadhaar/mobile/bank-account/device/IP at intake | **MISSING** |
| Repeat applications (≥3 in 30 days) | **IMPLEMENTED** — `internal/analytics/signals.go`, real SQL aggregate, scheduled + on-demand |
| Shared bank account across customers | **IMPLEMENTED** — same file |
| Repeated failed payments (≥5 in 30 days) | **IMPLEMENTED** — same file |
| Composite fraud/risk score | **MISSING** — design explicitly avoids ever naming a signal "fraud" or auto-accusing |
| Internal blacklist | **MISSING** |
| Device fingerprinting / velocity checks | **MISSING** |
| PEP/sanctions screening | **PARTIAL** — real fail-safe workflow, no vendor |

---

## PART 11 — DISBURSEMENT

- Sanctioned → prepare (`LoanDisbursePrep`) → approve/release (`LoanDisburse`) — two-step, but **maker≠checker not enforced at the ID level** (Part 5).
- Bank verification/penny-drop: **MISSING** — disbursement bank account is an opaque admin-supplied string, never validated against a real bank.
- Actual fund transfer: **MANUAL, not automated.** `internal/disbursement/manual_provider.go`'s `CreateTransfer` makes no external API call — records intent only; real money movement happens outside the codebase via bank portal (NEFT/RTGS), confirmed later via reconciliation matching a UTR.
- Razorpay is used for **inbound EMI collection**, not outbound disbursement — no payout API integration exists.
- **Who can prepare?** Any admin holding `LoanDisbursePrep` (by default: `DISBURSEMENT_OFFICER`).
- **Who can approve/release?** Any admin holding `LoanDisburse` (by default: `DISBURSEMENT_OFFICER`, `OPERATIONS` via `DisbursementView` for read — actual release needs `LoanDisburse` specifically, held by `DISBURSEMENT_OFFICER`). **Because maker≠checker isn't enforced, a single `DISBURSEMENT_OFFICER`-equivalent admin could theoretically prepare and approve/release the same disbursement alone if their role grants both permissions** — check the exact role-permission map in Part 3 before assuming this is impossible for any given role combination.

---

## PART 12 — REPAYMENT & ACCOUNTING

- EMI schedule: backend-generated, real (`internal/loans/emi_handler.go`).
- Payment creation/verification: **only the Razorpay webhook can mark an EMI paid** (`internal/payments/webhook_handler.go`) — real HMAC-SHA256 signature verification, idempotent via `gateway_event_id`, amount/currency cross-checked, row-locked. The old "any string settles an EMI" customer endpoint is disabled (410).
- Ledger/double-entry accounting, GST, TDS: a full, mature subsystem exists (`internal/accounting/`, `internal/models/accounting.go`, `internal/models/tds.go`) — more built-out than the original 6-module scope implied.
- Role capabilities: `AccountingPost` (post journal entries) and `PeriodManage` (close periods) are both held only by `ACCOUNTANT`/`SUPER_ADMIN`. `ReconciliationManage` (match transactions) similarly `ACCOUNTANT`/`SUPER_ADMIN`. Expense approval (`ExpenseApprove` — not read in this pass, but the permission exists) and vendor management (`VendorManage`) are held by `ACCOUNTANT`. Tax operations (`TaxOperate`) also `ACCOUNTANT`.

---

## PART 13 — COLLECTIONS

- Backend: real, DPD-bucketed (`internal/collections/service.go`) — `CURRENT/REMINDER(≤7)/OVERDUE(≤30)/ASSIGNED(>30)`. Promise-to-pay is only settled by a real verified payment, never by time alone. Write-off is maker-checker gated.
- An older, parallel `internal/recovery/` module also exists and is still active — assignment there is **manual only**, no automatic DPD/NPA trigger (documented in its own code comments).
- **🔴 The employee app's Collections screen is wired to entirely fake, hardcoded data.** `lib/employee_app/features/collections/data/repositories/collections_repository_mock.dart` is registered as the live DI singleton (`service_locator.dart:58`, `providers.dart:85`) — not a leftover test file. `confirmPayment()` only mutates an in-memory list. **This does not use real backend APIs today.** Do not assume any Collections data currently reaching a React portal from this screen's data source is real — it isn't, because this screen doesn't reach the backend at all. (A real backend HTTP repository exists for the separate "Recovery" feature in the same app — `recovery_case_repository_http.dart` — the Collections *tab specifically* is the one that's disconnected.)

---

## PART 14 — DATABASE

Simplified ER diagram (key lending-lifecycle entities only — ~110 total models exist, full inventory in companion audit §11):

```
Customer/User ──┬── CustomerKYC ──┬── CustomerPAN
                │                 ├── CustomerAadhaar
                │                 └── CustomerSelfie
                │
                ├── BureauConsent ── BureauRequest
                ├── CKYCRequest
                ├── ScreeningRequest
                │
                └── LoanApplication ──┬── CreditDecision
                                      ├── EmiInstallment ── Payment ── PaymentEvent / Refund
                                      ├── Disbursement
                                      ├── CollectionCase ──┬── CollectionAssignment
                                      │                    ├── CollectionAction / Note
                                      │                    └── CollectionPromise
                                      ├── WriteOffRequest ── RecoveryPayment
                                      ├── RecoveryCase (legacy) ── RecoveryReport
                                      └── LoanClassification (NPA staging) ── LoanClassificationHistory

AdminUser ── AdminSession                Employee ── Agency
     │
     └── (Role string, checked against internal/rbac/permissions.go map — no separate Role/Permission DB tables; RBAC is code-defined, not data-defined)

AuditLog (append-only, DB trigger blocks UPDATE/DELETE) — ActorID references AdminUser/Employee/Customer depending on ActorType
```

**Important for portal design**: roles and permissions are **Go code constants**, not database rows. There is no `roles` or `permissions` table to query — `internal/rbac/permissions.go` is the single source of truth, compiled into the binary. A new portal cannot read "what permissions does this role have" from the database; it must either call an existing admin API that returns the current admin's role/permissions (confirm one exists — the Flutter admin panel gets this from the `/admin/login` response body directly, not a separate endpoint) or replicate the same logic client-side as UI-only convenience (exactly as the existing admin panel does, per Part 16).

---

## PART 15 — API INVENTORY

*(Full ~150-route inventory in companion audit §12; below is portal-relevant grouping only.)*

| METHOD | PATH | PURPOSE | AUTH | PERMISSION |
|---|---|---|---|---|
| POST | /admin/login | Admin login | none | none |
| POST | /admin/refresh | Rotate refresh token | Admin JWT (refresh) | none |
| POST | /admin/logout | Revoke session | Admin JWT | none |
| GET | /admin/kyc/queue | KYC review queue | Admin JWT | KYCView |
| POST | /admin/kyc/:customerId/approve | Approve KYC | Admin JWT | KYCApprove |
| GET | /admin/bureau/requests | Bureau request list | Admin JWT | BureauView |
| GET | /admin/screening | Screening queue | Admin JWT | ScreeningView |
| POST | /admin/screening/requests/:id/decide | Screening decision | Admin JWT | ScreeningDecide |
| GET | /admin/loans | Loan list | Admin JWT | LoanView |
| POST | /admin/loans/:id/recommend | Recommend sanction | Admin JWT | LoanReview |
| POST | /admin/loans/:id/approve-sanction | Approve/reject sanction | Admin JWT | LoanSanction |
| POST | /admin/loans/:id/decision | Reject loan | Admin JWT | LoanReject |
| GET | /admin/risk/dashboard | Risk portfolio view | Admin JWT | RiskView |
| POST | /admin/risk/classification-rules/:id/approve | Approve classification rule | Admin JWT | RiskRuleApprove |
| POST | /admin/loans/:id/prepare-disbursement | Prepare disbursement | Admin JWT | LoanDisbursePrep |
| POST | /admin/loans/:id/approve-disbursement | Approve/release disbursement | Admin JWT | LoanDisburse |
| GET | /admin/payments | Payment list | Admin JWT | PaymentView |
| GET | /admin/reconciliation/transactions | Bank txn list | Admin JWT | ReconciliationView |
| GET | /admin/collections/cases | Collection case list | Admin JWT | CollectionView |
| POST | /admin/collections/cases/:id/assign | Assign case | Admin JWT | CollectionRecord *(see Part 4 note on possible wrong permission)* |
| GET | /admin/writeoffs | Write-off list | Admin JWT | WriteOffRequest |
| POST | /admin/writeoffs/:id/approve | Approve write-off | Admin JWT | WriteOffApprove |
| GET | /admin/audit-log | Audit log | Admin JWT | AuditView |

---

## PART 16 — EXISTING FRONTENDS

### Customer App
- API client: `lib/core/network/api_client.dart` — plain `http` package, **not** Dio. Token attached manually per call (no interceptor); caller must pass `token:` explicitly.
- Token storage: **SharedPreferences** (key `auth_jwt_token`) — explicitly *not* secure storage, per an in-code comment describing this as acceptable for a "demo/dev backend."
- 401 handling: **none** — no centralized interceptor or forced-logout-on-401 behavior found.
- Role handling: **none** — purely "logged in as this customer," no permission concept.

### Employee App
- API client: `lib/employee_app/core/network/api_client.dart` — Dio-based, real `onRequest` interceptor attaching the token from secure storage.
- Token storage: `SecureStorageService` (encrypted).
- 401 handling: real — a 401 (except from the login call itself) triggers `AuthProvider.logout()` via a wired callback, forcing a redirect.
- Role handling: yes, client-side — `EmployeeRole` enum drives dashboard/route selection (e.g. `app_router.dart:45`), but this is UI routing convenience, not a security boundary (the backend has no matching granular enforcement for employee roles beyond active/inactive).

### Admin Panel (`nbfc-admin-panel`)
- API client: `lib/core/api_client.dart` — plain `http`, token in-memory, attached via a headers getter.
- **Role/permissions obtained directly from the `/admin/login` response body** — not a separate endpoint call. Persisted to SharedPreferences.
- Proactive refresh: calls `/admin/refresh` on app open and every 10 minutes via a timer — ahead of the 15-minute access-token expiry, rather than waiting for a 401.
- 401 handling: real, wired to `AuthProvider.logout()`.
- `lib/core/permissions.dart`: a **client-side mirror** of the backend's role→permission map, used only to show/hide nav items/buttons. Explicitly documented as UI-only — the backend independently re-enforces every permission server-side via `RequirePermission` middleware regardless of what the client sends or hides. **A patched/modified admin-panel client cannot bypass authorization this way — it can only change what's visible, not what the backend accepts.**
- All admin API calls are centralized through one file, `lib/core/admin_api.dart` — confirmed distinct path prefixes: `/admin/admin-users`, `/admin/agencies`, `/admin/analytics`, `/admin/audit-log`, `/admin/complaints`, `/admin/customers`, `/admin/dashboard`, `/admin/employees`, `/admin/loans`, `/admin/mis`, `/admin/notifications`, `/admin/payments`, `/admin/reconciliation`, `/admin/recovery-reports`, `/admin/reporting`, `/admin/roles`, `/admin/sla-alerts`, `/admin/support`, `/admin/tax`, `/admin/users`, `/admin/verification-reports`.

---

## PART 17 — SIX NEW REACT PORTALS: API MAPPING

For each portal: existing APIs that already support it, missing backend APIs, and the DB entities it would primarily read/write. **No new APIs are proposed to be built as part of this investigation — this is a gap list only.**

### 1. KYC Verification Portal
- **Roles/Permissions**: no role is named specifically "KYC officer" — closest existing fit is `COMPLIANCE_OFFICER` (holds `KYCView`+`KYCApprove`) or `CREDIT_MANAGER` (also holds both). **No dedicated KYC-only role exists today.**
- **Existing APIs**: `GET /admin/kyc/queue`, `GET /admin/kyc/:customerId`, `POST /admin/kyc/:customerId/approve|reject|require-review`, `GET /admin/customers/:id/documents`, `GET /admin/bureau/requests` (CKYC-adjacent).
- **Missing**: no dedicated OCR-result review endpoint (no OCR exists), no DigiLocker endpoint, no Aadhaar-specific manual-override endpoint distinct from the general KYC approve/reject.
- **DB entities**: `CustomerKYC`, `CustomerPAN`, `CustomerAadhaar`, `CustomerSelfie`, `CustomerKYCReview`, `Document`.

### 2. Credit Bureau Portal
- **Roles/Permissions**: `COMPLIANCE_OFFICER` (BureauView, CKYCView, ScreeningView/Decide) is the closest fit; `CREDIT_OFFICER`/`CREDIT_MANAGER` also hold `BureauView`. **No role named specifically for bureau operations.**
- **Existing APIs**: `GET /admin/bureau/requests`, `GET /admin/ckyc`, `POST /admin/ckyc/:customerId/search`.
- **Missing**: since no real bureau provider is integrated, there is no "trigger a real pull" endpoint that does anything beyond the sandbox — building this portal meaningfully is blocked on Phase 4 of the companion audit's recommended sequence (real bureau integration) regardless of frontend work.
- **DB entities**: `BureauConsent`, `BureauRequest`, `CKYCRequest`.

### 3. Underwriting Portal
- **Roles/Permissions**: `CREDIT_OFFICER` (review/reject), `CREDIT_MANAGER` (review/sanction/restructure) — **this mapping is clean and already exists.**
- **Existing APIs**: `GET /admin/loans`, `GET /admin/loans/:id`, `GET /admin/loans/pending-approvals`, `POST /admin/loans/:id/recommend`, `POST /admin/loans/:id/approve-sanction`, `POST /admin/loans/:id/decision`, `POST /admin/loans/:id/restructure`, `GET /admin/loans/:id/documents`, `GET /admin/loans/:id/emi-schedule`.
- **Missing**: no endpoint exposing a composite risk score (doesn't exist) or multi-loan DTI (doesn't exist) — these are backend-logic gaps, not API-surface gaps; the portal can only show what the backend calculates.
- **DB entities**: `LoanApplication`, `CreditDecision`.

### 4. Fraud & Risk Portal
- **Roles/Permissions**: `CREDIT_MANAGER` (RiskView, RiskRuleManage, ClassificationOverride) or dedicated risk-review via `COMPLIANCE_OFFICER` (RiskRuleApprove). **No role named specifically "fraud officer" or "risk officer" distinct from credit roles.**
- **Existing APIs**: `GET /admin/risk/dashboard`, `GET /admin/risk/alerts`, `GET /admin/risk/loans/:loanId`, `POST /admin/risk/loans/:loanId/override-classification`, `GET/POST /admin/analytics/signals*` (the 3 real behavioral signals), `GET/POST /admin/screening*`.
- **Missing**: no duplicate-identity detection endpoint (feature doesn't exist), no blacklist management endpoint (feature doesn't exist), no device-fingerprint/velocity endpoint (feature doesn't exist). These are the same backend gaps flagged in the companion audit — a Fraud & Risk portal today would only surface the 3 existing behavioral signals plus manual PEP/sanctions review, not a full fraud console.
- **DB entities**: `RiskAlert`, `RiskSignal`, `ScreeningRequest`, `LoanClassification`.

### 5. Operations / Disbursement & Repayment Portal
- **Roles/Permissions**: `OPERATIONS` (general ops, CollectionView/PaymentView/DisbursementView) + `DISBURSEMENT_OFFICER` (LoanDisbursePrep, DisbursementView, and — per the role map — `LoanDisburse` is not explicitly listed for DISBURSEMENT_OFFICER in the excerpt reviewed; **re-verify exact permission list for this role before assuming who can release** — the Part 3 table above lists DISBURSEMENT_OFFICER without `LoanDisburse` itself, meaning **release may require a different/higher role** — this needs explicit confirmation before portal RBAC is finalized, flagged as UNKNOWN pending a direct re-check of `permissions.go:213-218`).
- **Existing APIs**: `POST /admin/loans/:id/prepare-disbursement`, `POST /admin/loans/:id/approve-disbursement`, `GET /admin/payments`, `GET /admin/payments/:id`, `GET /admin/reconciliation/*`, `GET /admin/loans/:id/emi-schedule`.
- **Missing**: no bank penny-drop/IFSC verification endpoint (feature doesn't exist), no real payout-API endpoint (disbursement is manual bank-portal today) — a portal button "Release Disbursement" would still only record intent, not move real money, until Phase 7 of the companion audit's backlog is done.
- **DB entities**: `Disbursement`, `Payment`, `PaymentEvent`, `Refund`, `BankAccount`, `BankTransaction`, `ReconciliationRecord`.

### 6. Collections & Recovery Portal
- **Roles/Permissions**: `COLLECTION_OFFICER` (view/record), `COLLECTION_MANAGER` (view/record/manage — assign). **Clean existing mapping.**
- **Existing APIs**: `GET /admin/collections/cases`, `GET /admin/collections/cases/:id`, `POST /admin/collections/cases/:id/assign|escalate|actions|notes|promises`, `GET /admin/writeoffs`, `POST /admin/writeoffs/:id/approve|reject|recovery`, `GET /admin/recovery-reports` (legacy system).
- **Missing**: no automatic DPD/NPA-triggered case creation endpoint (currently manual only — a backend job would need to exist, portal can't fix this alone), no SMS/WhatsApp reminder-trigger endpoint (no provider integrated).
- **Important**: this portal must call `internal/collections` (the real backend), **not** replicate the pattern of the employee app's disconnected mock Collections screen. The backend API for this module is real and ready to use.
- **DB entities**: `CollectionCase`, `CollectionAssignment`, `CollectionAction`, `CollectionNote`, `CollectionPromise`, `WriteOffRequest`, `RecoveryPayment`.

---

## PART 18 — AUTHENTICATION FOR SIX PORTALS

**Recommendation: A — reuse the existing Admin JWT + `AdminAuthMiddleware` + `RequirePermission` system.**

Reasoning, based strictly on what was found (not assumption):

1. All six proposed portals map onto **operations the admin backend already performs and already permission-gates** — every API endpoint identified in Part 17 already lives under `/admin/*`, already requires `AdminAuthMiddleware`, and already has a `RequirePermission(perm)` check server-side. There is no separate business-logic layer for "portal users" to plug into — from the backend's point of view, a KYC-portal user and a Main-Admin user reviewing KYC are indistinguishable, because they'd call the exact same endpoints.
2. The admin auth system is the **only one of the three with a real, revocable server-side session, rotating refresh tokens, and account lockout** — the appropriate security posture for back-office financial operations staff, unlike customer auth (stateless, no revocation) or employee auth (stateless, no granular RBAC, no lockout).
3. RBAC is **already fine-grained enough** to scope each portal correctly by giving each portal's users the appropriate existing role (or a small number of new roles — see below) — no new auth layer is needed to achieve "KYC staff can only do KYC things."
4. Building a new auth layer or SSO would mean **maintaining a second source of truth for identity and permissions**, directly contradicting the instruction to reuse existing roles/permissions and preserve backward compatibility. It would also mean the six portals and the Main Admin panel could disagree about who a given admin is or what they can do — a real risk if not centralized.
5. **Concretely**: each React portal's login screen calls the existing `POST /admin/login`, stores the returned access+refresh token pair the same way the Flutter admin panel does, attaches `Authorization: Bearer <token>` the same way, and implements the same proactive-refresh-before-15-minutes pattern already proven in the Flutter admin panel's `AuthProvider`. Each portal reads `role`/permissions from the same login response and uses them identically to `lib/core/permissions.dart` — as a **UI-only convenience**, since the backend independently re-enforces everything regardless.

**What still needs a decision (not determined by code alone — a product/role decision, not a technical one)**: whether to create a small number of **new roles** (e.g. `KYC_OFFICER`, `RISK_OFFICER`, `BUREAU_OFFICER`) that don't exist today, since Parts 17 showed KYC/Bureau/Fraud portals currently have no dedicated role and would otherwise have to reuse `COMPLIANCE_OFFICER` or `CREDIT_MANAGER` more broadly than may be desired. This is explicitly a role-design decision for the product owner, informed by this audit, not something the code answers on its own.

**Why not B (Employee JWT)**: employee auth has no granular permission system at all — only a plain role string and no `RequirePermission`-equivalent middleware. Using it for financially-sensitive operations staff would mean either building that missing enforcement layer from scratch (defeating the "reuse existing" goal) or running these portals with effectively no backend-enforced authorization beyond "is this a valid active employee" — not appropriate for KYC approval, sanction, or disbursement actions.

**Why not C/D (new layer / SSO)**: no technical gap in the existing admin auth system justifies the complexity and duplicated-source-of-truth risk of a new system — see reasoning above.

---

## PART 19 — WHAT MUST NOT BE CHANGED

Only listed because the code confirms it exists and is working:

- **Customer authentication** (`internal/auth/`, MPIN+OTP flow, `models.User`) — real, working, independent system.
- **Employee authentication** (`internal/employee/`, `EmployeeAuthMiddleware`) — real, working (though lacking granular RBAC — a gap, not something broken to "fix" by replacing).
- **Admin authentication** (`internal/admin/`, `AdminSession`, rotating refresh tokens) — real, working, the strongest of the three; this is the system the six portals should build on, not replace.
- **RBAC permission definitions** (`internal/rbac/permissions.go`) — the existing role→permission map is the source of truth; extend it (new roles/permissions) rather than replacing its model.
- **Loan/underwriting engine** (`internal/loans/eligibility.go`, FOIR/CIBIL logic) — real, working, backend-authoritative.
- **Maker-checker for loan sanction** — partially working (ID comparison present for sanction, confirmed absent for disbursement) — the *existing* sanction check should not be removed while the disbursement gap is fixed; fix additively.
- **Razorpay integration** (`internal/payments/razorpave.go`, `webhook_handler.go`) — real, live, properly signature-verified; do not replace with a different gateway or bypass the webhook-only settlement rule.
- **PAN verification via Surepass** — real, live; do not replace.
- **KYC state machine** (`internal/kyc/service.go`) — genuinely enforced no-auto-VERIFIED guarantee; do not weaken.
- **Accounting/GST/TDS subsystem** — a mature, separate system; six portals should read from/write to it via its existing APIs, not duplicate its logic.
- **Audit logging** (`internal/audit/`, append-only `AuditLog` with DB trigger) — do not bypass; every new portal action should call the same `RecordDetailed()` pattern.
- **Existing Main Admin Flutter panel** — must continue to function unmodified; new portals are additive, calling the same backend, not a replacement.

---

## PART 20 — FINAL REPORT

### 1. Current Architecture
Go/Gin monolith (`internal/`, domain-per-package, handlers call GORM directly — no repository layer), PostgreSQL via GORM AutoMigrate, three Flutter clients (customer, employee, admin — admin in a separate repo), external integrations for PAN (live), payments (live, needs prod keys), push (live, needs prod creds), document storage (S3/local encrypted).

### 2. Authentication
Three fully independent JWT systems, no shared secret/code. Only admin has a real revocable server-side session + rotating refresh token + account lockout. Customer: 24h stateless. Employee: 12h stateless with live active-status recheck. Admin: 15min access + 7-day rotating refresh + `AdminSession` table.

### 3. RBAC
Admin-only. Code-defined (`internal/rbac/permissions.go`), not database-defined. 11 roles, ~45 distinct permissions, enforced via `RequirePermission` middleware, fails closed for unrecognized roles.

### 4. All Roles
SUPER_ADMIN, CREDIT_OFFICER, CREDIT_MANAGER, OPERATIONS, DISBURSEMENT_OFFICER, COLLECTION_OFFICER, COLLECTION_MANAGER, ACCOUNTANT, COMPLIANCE_OFFICER, AUDITOR, READ_ONLY — all 11 confirmed present exactly as asked about.

### 5. All Important Permissions
See Part 3's full taxonomy — 45+ permissions across customer/KYC/loan/bureau/risk/collections/write-off/disbursement/accounting/tax/compliance/audit/reporting/finance/admin categories.

### 6. Maker-Checker Status
**Inconsistent.** Confirmed present (ID comparison) for loan sanction approval. Confirmed **absent** for disbursement approval. Status for write-off/accounting/compliance approvals not fully re-verified this pass — flagged UNKNOWN. **4 regression tests for this exact behavior are currently failing in the real test suite** — this needs a live re-run, not just a static code read, before any portal's maker-checker UX is finalized.

### 7. Loan Lifecycle
11 actual status constants, full transition table in Part 6. Two state-machine integrity gaps found: unconditional rejection from any state, and unconditional employee-side verification without a prior-status check.

### 8. KYC Status
PAN: LIVE. Aadhaar: blocked (Surepass token scope). Selfie/liveness: format-check only, no biometric vendor. OCR/DigiLocker: missing. State machine itself: genuinely well-built and enforced.

### 9. Credit Bureau Status
No real bureau provider. Sandbox (fixed 780 score) is correctly non-production-only. CKYC and PEP/sanctions screening are honestly blocked/fail-safe, not faked.

### 10. Underwriting Status
Real, automated FOIR + CIBIL-banded engine, backend-authoritative. No multi-loan DTI, no income verification, no composite risk score.

### 11. Risk Status
Real 3-pattern behavioral signal engine (repeat applications, shared bank accounts, failed payments), scheduled + on-demand. No duplicate-identity detection, blacklist, fraud score, device fingerprinting, or velocity checks.

### 12. Disbursement Status
Two-step workflow exists; maker≠checker not enforced at the ID level. Actual fund transfer is manual (bank portal), not API-driven — no payout integration exists.

### 13. Accounting Status
Mature double-entry ledger, GST, TDS subsystem — more built-out than the six-module scope implied. Role-gated correctly (`ACCOUNTANT` role holds the relevant permissions).

### 14. Collections Status
Backend real and well-designed (DPD buckets, promise-to-pay, write-off maker-checker). **Employee app's Collections screen is disconnected from it, running on fake hardcoded data** — do not copy this pattern into a React portal; call the real backend API instead.

### 15. API Map
~150 admin routes total, all under `/admin/*`, all `AdminAuthMiddleware` + per-route `RequirePermission`. Portal-relevant subset mapped in Part 17.

### 16. Database Map
~110 GORM models, code-defined RBAC (no roles/permissions DB tables), simplified ER diagram in Part 14.

### 17. Existing Frontends
Customer app: plain HTTP client, SharedPreferences token storage, no 401 handling, no role concept. Employee app: Dio + secure storage + real 401 handling + client-side role routing (not a security boundary). Admin panel: HTTP client, role/permissions from login response, proactive refresh, client-side permission mirror explicitly documented as UI-only (backend independently re-enforces).

### 18. Six New Portal Mapping
Underwriting and Collections & Recovery map cleanly onto existing roles/APIs today. KYC, Credit Bureau, and Fraud & Risk have no dedicated existing role (would reuse `COMPLIANCE_OFFICER`/`CREDIT_MANAGER` more broadly, or a new role could be created). Operations/Disbursement has an **unresolved question about exactly which role can release disbursement** — flagged UNKNOWN, needs direct re-verification of `permissions.go:213-218` before building that portal's RBAC.

### 19. Authentication Recommendation
**Reuse the existing Admin JWT + AdminAuthMiddleware + RequirePermission system for all six portals** (Option A). It is the only one of the three existing auth systems with real session revocation and granular RBAC, and every relevant API already lives under this system. Building a new auth layer would duplicate a source of truth for no technical benefit found in this investigation.

### 20. Missing Backend Capabilities
Real credit bureau integration, real Aadhaar verification (scope fix), OCR, DigiLocker, biometric liveness, bank penny-drop verification, real payout/disbursement API, NACH/eNACH, SMS/WhatsApp/email channels, duplicate-identity detection, blacklist, composite fraud score, automatic DPD/NPA-triggered collections. (Full detail: companion audit §17.)

### 21. Security Risks
Disbursement maker≠checker not enforced at the ID level (highest severity, corroborated by failing tests). Two loan state-machine integrity gaps (unconditional rejection; unconditional employee-side verification). Employee app Collections screen disconnected from real backend. Rate limiting is per-instance in-memory (weakens under autoscaling — confirmed via live testing earlier this engagement). Inconsistent secrets management (only Firebase push uses AWS SSM; DB/JWT/Surepass secrets are plain env vars).

### 22. Recommended Next Steps
1. **Live-verify** the maker-checker discrepancy (code appears correct for sanction, but the regression test suite disagrees) — run the actual failing tests with a debugger before trusting either the static read or assuming the tests are wrong.
2. Fix the disbursement maker≠checker gap (confirmed absent, not just disputed).
3. Fix the two loan state-machine integrity gaps in Part 6.
4. Decide (product/role design, not code): create dedicated `KYC_OFFICER`/`RISK_OFFICER`/`BUREAU_OFFICER` roles, or reuse `COMPLIANCE_OFFICER`/`CREDIT_MANAGER` for those three portals.
5. Confirm exactly which role can release disbursement (re-read `permissions.go:213-218` directly) before finalizing the Operations portal's RBAC.
6. Build each React portal's auth as a thin client of the existing `/admin/login` + refresh + `RequirePermission`-gated API surface — no new backend auth work required for this specifically.
7. Do not build a Collections portal on the pattern of the employee app's mock repository — use the real `internal/collections` API.

---

# TOP 10 THINGS I MUST KNOW BEFORE BUILDING THE SIX REACT PORTALS

1. **All six portals should authenticate through the existing Admin JWT system** (`/admin/login`, 15-min access + 7-day rotating refresh, real revocable `AdminSession`) — it's the only one of the three auth systems with real session control and granular RBAC, and every relevant backend API already lives under it.
2. **RBAC is code-defined, not database-defined** — `internal/rbac/permissions.go` is the single source of truth for roles/permissions; there's no `roles`/`permissions` DB table to query or seed.
3. **Disbursement maker≠checker is NOT enforced at the ID-comparison level** (confirmed absent in code) — a single admin holding both prepare and release permissions can currently do both alone. Fix this before or alongside building the Operations/Disbursement portal, not after.
4. **The loan sanction maker≠checker check appears present in code but has 4 currently-failing regression tests attached to exactly this behavior** — re-verify live with a test run before trusting it for the Underwriting portal.
5. **Only Underwriting and Collections & Recovery map cleanly onto existing roles today** (`CREDIT_OFFICER`/`CREDIT_MANAGER`; `COLLECTION_OFFICER`/`COLLECTION_MANAGER`). KYC, Credit Bureau, and Fraud & Risk have no dedicated existing role — decide whether to create new ones or reuse `COMPLIANCE_OFFICER`.
6. **Real credit bureau, real Aadhaar verification, OCR, and bank-account verification do not exist** — the Credit Bureau and KYC portals will initially only be able to show sandbox/manual-review data until those integrations are built; don't design the portal UI assuming live bureau data will simply "show up."
7. **The employee app's Collections screen uses entirely fake hardcoded data, disconnected from the real backend** — do not use it as a reference for how the Collections & Recovery portal should be built; go directly to the real `internal/collections` API instead.
8. **Two loan state-machine integrity gaps exist**: a loan can be rejected from any status with no precondition, and employee-side verification doesn't check prior status. Both are worth fixing before portals start relying on strict state-transition guarantees.
9. **Frontend permission checks are explicitly UI-only everywhere in this codebase** (confirmed in the existing admin panel's own code comments) — the backend independently re-enforces every permission server-side regardless of what any client sends or hides. Design each React portal's RBAC the same way: client-side for UX, never as the actual security boundary.
10. **The existing Flutter Main Admin panel must keep working unmodified** — it calls the same `/admin/*` API surface this investigation mapped; the six new portals are additive, thinner, role-scoped clients of the same backend, not a replacement for it.
