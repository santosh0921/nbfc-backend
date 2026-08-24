# Phase 14 — Complete VAPT Readiness, OWASP Security Validation, Penetration-Test Readiness & Security Remediation

**This report does not claim "VAPT PASSED," "100% SECURE," "RBI CYBER
COMPLIANT," "ISO CERTIFIED," or "SOC COMPLIANT."** No independent
assessment or certification has occurred. This is an internal
self-assessment that identifies vulnerabilities, fixes what was found,
and documents what remains — see §31 for what "VAPT READY" actually
means here.

---

## 1. Executive Summary

This phase performed an active security review of the accumulated
Phase 1–13 codebase (not a re-read of prior phases' own security claims)
and found **2 confirmed vulnerabilities**, both fixed with regression
tests in this phase:

1. **JWT signing-algorithm not pinned** (all 4 auth middlewares) — MEDIUM
2. **CSV/Excel formula injection unmitigated in exports** — MEDIUM

Both are closed. No CRITICAL or HIGH-severity issue was found in the
areas actually reviewed (see §3 Methodology for what was and was not
covered — this is a scoped review, not an exhaustive one, and §31 states
that plainly).

Beyond these two fixes, this phase verified — rather than assumed —
that a substantial set of controls Phase 7 and later phases claimed to
have built actually behave as claimed: session revocation, maker-checker
enforcement, rate limiting, append-only audit logging, webhook signature
verification, security headers, and encryption-at-rest key separation
were all independently re-checked against the current code, not taken on
faith from earlier reports.

## 2. Scope

See `PHASE14_VAPT_SCOPE.md` for the full in-scope/out-of-scope
definition. This report's own testing was limited to the backend Go
codebase; no live external penetration test, no infrastructure scan, and
no dependency CVE scan (`govulncheck` was not available in this
environment — see §22) were performed.

## 3. Methodology

1. **Inventory first** (§4): mapped every route, middleware, and external
   integration actually present in `internal/routes/routes.go` and the
   package tree, rather than assuming the OWASP checklist's categories
   all apply.
2. **Targeted code inspection** against each OWASP category actually
   applicable to this codebase (§5–6), verified by reading the real
   implementation, not by re-reading prior phases' reports.
3. **Where a control was claimed by an earlier phase**, re-verified it
   against current code (e.g., re-read `AdminAuthMiddleware`,
   `RazorpayWebhookHandler`, the audit-log trigger, `ApproveExpense`)
   rather than trusting the earlier report's own claim.
4. **For confirmed findings**: reproduced via a safe automated test
   (never a live exploit against running infrastructure), fixed, added
   a permanent regression test, and re-ran the full suite.
5. **Explicitly did not** attempt dependency CVE scanning (no
   `govulncheck` available), live SSRF probing against real
   infrastructure, or load-based rate-limit exhaustion testing beyond
   what Phase 7's existing test already does.

## 4. Attack Surface Inventory

**Authentication surfaces** (4 independent JWT schemes, confirmed by
reading `internal/middleware/*_auth_middleware.go`):
customer (`internal/auth`), employee (`internal/employee`), admin
(`internal/admin`, with server-side `AdminSession` revocation checked
per-request), and a combined customer-or-employee middleware for shared
notification endpoints. Each uses a distinct HMAC secret
(`JWT_SECRET`/`ADMIN_JWT_SECRET`/`EMPLOYEE_JWT_SECRET`), confirmed
non-interchangeable by design (Phase 7's own report claim, re-verified:
the app refuses to start if any two of these three secrets are equal or
unset).

**Route surface**: every route in `internal/routes/routes.go` — public
customer auth/OTP/MPIN, employee auth, ~15 admin permission groups
(`perm(rbac.X)` closures) covering loans, disbursement, payments,
accounting, tax, compliance, risk, collections, reporting, MIS,
analytics. All admin routes sit behind `AdminAuthMiddleware` +
`adminAPILimit` (blanket rate limit) at the group level
(`routes.go:123-124`), then an individual `rbac.Permission` per route.

**External integrations actually present**: Razorpay (webhook HMAC
verified — `internal/payments/webhook_handler.go`), a credit bureau
provider abstraction (`internal/bureau`), CKYC (`internal/ckyc`), a
screening/PEP provider (`internal/screening`), an OTP provider
(`internal/otp`), a storage provider abstraction for encrypted documents
(`internal/storage`, AES-256-GCM, Phase 3).

**File handling**: document upload/download (`internal/documents`),
compliance evidence upload/download (`internal/compliance`), all
authenticated and RBAC-gated; no publicly-reachable file path exists —
confirmed no route serves `internal/storage`'s encrypted blobs by a raw
filesystem path.

**Export surface**: 20+ named Excel/CSV/PDF report types
(`internal/reporting/registers.go`), dispatched only to
server-computed content (never client-supplied rows for the controlled
types), gated by `rbac.MISView`/`rbac.RiskView`/`rbac.ReportExport`, all
funneled through one function (`RequestExportWithMetadata`) — which is
exactly why this phase's formula-injection fix (§10/§28 finding 2) only
needed to touch one place.

**Rate-limited endpoints** (`internal/middleware/rate_limit.go`, all
confirmed wired in `routes.go`): OTP send/verify, MPIN creation, customer
login, admin login, the Razorpay webhook itself, KYC initiation, document
upload, loan submission, payment order creation, report export, and a
blanket admin-API limit covering every other admin route including this
phase's own new analytics endpoints.

**Secrets/config**: `.env`/`.env.example` inspected — only placeholder
values present (`change_this_...`), no real secret committed. Startup
validation (`internal/config.ValidateProductionSafety`, Phase 7)
refuses to boot in production with `DEMO_MODE=true` or missing
encryption keys — re-confirmed present in current code.

## 5. OWASP API Security Top 10 Assessment

| Risk | Assessment |
|---|---|
| API1 Broken Object Level Authorization | 🟢 Spot-checked customer-facing loan/EMI/payment handlers (`CustomerEmiScheduleHandler`, payment handlers) — every one loads the resource then explicitly compares `resource.CustomerID != currentUser.ID` before returning data. Phase 7's own `TestIDORCustomerCannotAccessAnotherLoan`/`TestIDORCustomerCannotAccessAnotherCustomersPayment` re-run and pass. |
| API2 Broken Authentication | 🟡 → 🟢 after fix. Found: JWT signing algorithm was not pinned at any of the 4 verification sites (§28 finding 1). Fixed. Session revocation re-verified live (`TestRevokedAdminSessionRejected` passes). |
| API3 Broken Object Property Level Authorization | 🟢 Spot-checked `AdminCreateThresholdHandler` (this phase's own new endpoint) and `ApproveExpense` — server-assigned fields (`ID`, `CreatedBy`, `ApprovedByAdminID`, `Status`) are explicitly overwritten after bind, never trusted from the request body. See §8 for the broader mass-assignment pass. |
| API4 Unrestricted Resource Consumption | 🟢 Per-endpoint rate limits exist for every expensive/sensitive endpoint (§4); export/analytics queries are server-side SQL aggregations, not unbounded fetches (Phase 13's own design constraint, re-verified). |
| API5 Broken Function Level Authorization | 🟢 Every admin route requires a named `rbac.Permission`; `TestReadOnlyAdminCannotEscalate`/`TestUnauthorizedDisbursementBlocked` re-run and pass. This phase's own new analytics write-endpoints (`/admin/analytics/thresholds`, `/signals/:id/review`) require `riskManage`, not the weaker `riskView` read permission — verified in `internal/routes/routes.go`. |
| API6 Unrestricted Access to Sensitive Business Flows | 🟢 Disbursement, expense approval, and regulatory-return approval all enforce maker≠checker (`ErrSameActor` pattern, re-verified in `internal/finance/expense.go`). |
| API7 Server-Side Request Forgery | 🟡 See §19 — no endpoint in this codebase accepts an arbitrary user-supplied URL to fetch (webhook receiver is inbound only, not outbound), so SSRF as classically defined has no entry point identified. Not exhaustively probed against live infrastructure (out of scope per §2). |
| API8 Security Misconfiguration | 🟢 Security headers present (§20), CORS restricted to configured origins with a documented refusal to allow wildcard in production, `.env.example` contains only placeholders. |
| API9 Improper Inventory Management | 🟡 No formal OpenAPI/Swagger spec exists — the attack-surface inventory in §4 was built by reading `routes.go` directly rather than from generated documentation, which is itself a gap: undocumented API surface is harder for legitimate maintainers and assessors alike to fully enumerate. |
| API10 Unsafe Consumption of APIs | 🟢 The one inbound third-party payload this codebase trusts by default (the Razorpay webhook) is signature-verified before any data from it is used (§13/§15). |

## 6. OWASP Web Application Security Assessment

**Injection**: SQL — every raw query found in `internal/analytics`
(Phase 13, written this session) and pre-existing packages uses
parameterized placeholders (`?`), confirmed by grep across the codebase
for string-concatenated SQL; none found. `TestSQLInjectionPayloadSafelyRejected`
(Phase 7) re-run and passes. Command/template injection: no
`os/exec`, `text/template`/`html/template` execution of user input, or
dynamic code evaluation found anywhere in `internal/`.

**Broken Access Control**: covered under API1/API5 above.

**Cryptographic Failures**: covered in §23.

**Identification and Authentication Failures**: covered in §7/§8.

**Security Misconfiguration**: covered in §20/§21.

**Vulnerable Components**: not scanned this phase — no CVE database
tool was available in this environment (§22). Documented as an open
item, not silently skipped.

**Logging/Monitoring Failures**: covered in §24.

**SSRF**: covered in §19.

**Software/Data Integrity Failures**: the audit log is append-only at
the database level via a Postgres trigger rejecting `UPDATE`/`DELETE`
(re-confirmed present in this session's own boot log:
`✅ audit_logs is now append-only at the database level`), not merely an
application-layer convention. `TestAuditLogCannotBeModifiedOrDeleted`
(Phase 7) re-run and passes.

**Business Logic vulnerabilities**: covered in §14 (Phase 14's own
central concern per the task brief).

CSRF/host-header/open-redirect/prototype-pollution: not independently
applicable — this is a stateless-JWT bearer-token API (no cookie-based
session, so classic CSRF does not apply the way it does to a
cookie-authenticated app); no redirect-issuing endpoint exists; Go has
no prototype-pollution-equivalent construct.

## 7. Authentication

Customer login (`internal/auth`), admin login (`internal/admin`), and
employee login (`internal/employee`) each: hash passwords/MPIN with
bcrypt (confirmed, not re-implemented — see §23), rate-limit the login
endpoint itself (§4), and issue an HMAC-signed JWT with a distinct
per-role secret.

**Re-verified, not assumed**:
- `TestRevokedAdminSessionRejected` — logout revokes the session
  server-side and the same token is rejected on the very next request,
  not merely once it expires.
- `TestRateLimitEnforcedOnLogin` — login attempts are actually throttled,
  not just configured-but-unwired.
- OTP: `sendOTPLimit`/`verifyOTPLimit` rate limits confirmed wired
  (`routes.go:57-58`).

**Not independently re-tested this phase** (carried forward as an open
item rather than re-verified from scratch): OTP replay-after-use,
concurrent-session limits, and password-reset-abuse flows specific to
each of the three auth systems — Phase 7's own report already
documents these as tested for the customer flow; this phase did not
re-derive new test cases for the employee/admin equivalents.

## 8. Authorization / RBAC

Built the permission matrix from the application's own actual role
model (`internal/rbac/*.go`), not an assumed generic one — confirmed the
11 real `AdminRole*` constants in `internal/models/admin_user.go`
(`SUPER_ADMIN`, `CREDIT_OFFICER`, `CREDIT_MANAGER`, `OPERATIONS`,
`DISBURSEMENT_OFFICER`, `COLLECTION_OFFICER`, `COLLECTION_MANAGER`,
`ACCOUNTANT`, `COMPLIANCE_OFFICER`, `AUDITOR`, `READ_ONLY`), each mapped
to its actual permission set in `internal/rbac`'s role table (not
reproduced in full here — that table itself is the ground truth an
assessor should read directly rather than a second copy risking drift).

**Verified this phase**: `COLLECTION_OFFICER` (has `RiskView`, not
`MISView`) is blocked from the Phase 13 financial growth dashboard and
from configuring a management threshold
(`TestUnauthorizedCannotAccessAnalytics`, new this phase). `READ_ONLY`
cannot prepare a disbursement (`TestUnauthorizedDisbursementBlocked`,
re-run).

Horizontal privilege escalation (cross-customer) and vertical
(role-to-role) both covered by the tests above and §9.

## 9. IDOR / BOLA

Aggressively re-checked the resource types item G names, against
current code:

| Resource | Ownership check found? |
|---|---|
| Customer loan/EMI (customer-facing) | 🟢 `loan.CustomerID != user.ID` check present |
| Customer payment | 🟢 covered by `TestIDORCustomerCannotAccessAnotherCustomersPayment` |
| Admin-side loan/application/disbursement/collection | 🟢 gated by RBAC permission, not per-record ownership (correct model: an admin's authority is role-based, not row-based, in this system's design) |
| Expense/vendor | 🟢 RBAC-gated; approval additionally enforces maker≠checker |
| Compliance evidence | 🟢 `AdminDownloadEvidenceHandler` requires `ComplianceView`; download itself is audit-logged |
| Report export | 🟢 gated by `ReportExport`/`MISView`; controlled report types never accept client-supplied content |
| Analytics resources (Phase 13, this session) | 🟢 signal review/threshold creation require `RiskRuleManage`, not just `RiskView` |

No cross-customer or cross-role resource access was found in the
handlers reviewed. This was a targeted review of representative handlers
per resource type, not an exhaustive per-endpoint enumeration of all
~250+ admin routes — see §29/§31 for what that residual risk means.

## 10. Injection

Grepped the entire `internal/` tree for raw SQL construction (`Raw(`,
`Exec(`, string concatenation into a query). Every raw query found —
including the ~15 new ones written in Phase 13's `internal/analytics`
this session — uses `?` placeholders with arguments passed separately to
GORM, never string-interpolated values. `TestSQLInjectionPayloadSafelyRejected`
re-run and passes.

## 11. NoSQL/Other Injection

Not applicable — this codebase uses PostgreSQL exclusively via GORM; no
Redis, MongoDB, Elasticsearch, or dynamic-expression engine is present
anywhere in `go.mod` or the code.

## 12. File Security

Upload (`internal/documents`, `internal/compliance` evidence): stored via
`internal/storage`'s AES-256-GCM encrypted provider (Phase 3), never
written to a public web root, never served by raw filesystem path.
Download requires authentication + the relevant RBAC permission and is
audit-logged. Not independently re-tested this phase: oversized-file and
malicious-filename handling specific to the upload handlers (carried
forward as an open item — Phase 3's own report documents basic
validation; this phase did not add new path-traversal/zip-bomb test
cases beyond confirming no raw filesystem path is ever exposed).

## 13. Excel / PDF Export Security

**Finding and fix**: see §28 finding 2 (CSV/Excel formula injection).
Unauthorized export: gated by RBAC (§9). Cross-user export: controlled
report types are always computed server-side from the requester's
authorized scope, never keyed by a client-supplied "whose data" — there
is no cross-user export surface for controlled report types to leak
through the export mechanism itself.

## 14. Payment Security

Re-verified rather than assumed: `internal/loans/emi_handler.go`'s
`CustomerPayEmiHandler` is **deliberately disabled** (`HTTP 410 Gone`) —
its doc comment states it used to let a customer settle any EMI by
supplying an arbitrary transaction-reference string, and this was closed
in an earlier phase by removing the endpoint entirely in favor of
gateway-webhook-verified settlement only. This phase confirms that fix
is still in place, not regressed.

Webhook signature verification (`VerifyWebhookSignature`) is checked
before any webhook payload data is trusted (§15). Duplicate/replay
webhook handling: not independently re-tested this phase for a
signature-valid-but-replayed webhook — carried forward as an open item.

## 15. Webhook Security

`internal/payments/webhook_handler.go` verifies the `X-Razorpay-Signature`
header via HMAC before processing (re-confirmed in code, §4). Idempotency
against duplicate delivery: `TestDuplicateJournalIdempotencyEnforced`
(Phase 7) re-run and passes, confirming double-posting the same webhook
event does not double-post to the ledger.

## 16. Business Logic Security

This is the phase brief's explicitly named central concern. Re-verified
by reading the actual enforcement code, not by re-reading prior claims:

- **Approve own expense**: blocked (`ErrSameActor` in `ApproveExpense`).
- **Bypass maker-checker on sanction recommendation**:
  `TestMakerCannotApproveOwnSanctionRecommendation` (Phase 7) re-run,
  passes.
- **Modify a closed accounting period**:
  `TestClosedPeriodBlocksNewPosting` (Phase 7) re-run, passes.
- **Disburse an unapproved/unsanctioned loan**: disbursement handlers
  require `LoanStatusSanctioned`/`ReadyForDisbursement` before
  proceeding (Phase 2/8's own state-machine enforcement, spot-checked in
  `internal/loans`/`internal/disbursement`, not regressed).
- **Direct disbursement without permission**:
  `TestUnauthorizedDisbursementBlocked` re-run, passes.

Not independently re-derived this phase (carried forward from earlier
phases' own claims rather than freshly re-tested): "disburse twice",
"refund twice", "modify approved transaction after approval",
"manipulate due date/interest/fees post-sanction" — each of these would
need a dedicated new test per workflow; only the four above were
re-verified live this session.

## 17. Accounting Security

`TestClosedPeriodBlocksNewPosting` re-confirms a closed accounting
period rejects new postings. Every financial transaction posts as a
balanced double-entry journal (Phase 4's own invariant); this phase did
not add a new automated "every journal entry balances" scan across
historical data, but the posting code path itself enforces balance at
write time (not re-derived independently this phase — carried forward).

## 18. Tax / TDS Security

Not independently re-tested this phase beyond what Phase 10's own report
already covers (TDS amount/status fields are admin-entered and
RBAC-gated, never auto-computed without configured rules). No new
tax-specific attack was attempted this session — open item.

## 19. Compliance Security

Not independently re-tested this phase beyond re-confirming (via §9)
that evidence download requires `ComplianceView` and is audit-logged.
Phase 11's own report documents evidence versioning (never overwritten,
old versions superseded not deleted) and maker≠checker on regulatory
return approval — not re-derived fresh this session.

## 20. Rate Limiting / Resource Abuse

Full inventory in §4. Every login/OTP/MPIN/webhook/upload/export/
analytics-adjacent endpoint has a `NamedRateLimit`. `TestRateLimitEnforcedOnLogin`
re-run and passes. Query-level protection (statement timeout, connection
pool limits): not independently re-verified this phase at the database
configuration level — open item (§26).

## 21. SSRF

Searched the entire codebase for any handler accepting a URL parameter
that the backend itself then fetches. **No such endpoint was found** —
every external call this codebase makes (Razorpay, bureau, CKYC,
screening, OTP, PAN-validation) targets a hard-coded/config-driven
provider base URL, never a user-supplied one. The webhook receiver is
inbound-only. SSRF as classically defined therefore has no identified
entry point in this codebase. This is a code-review finding, not a
live-probe result — no actual request to `127.0.0.1`/`169.254.169.254`/
etc. was attempted, per the out-of-scope note in `PHASE14_VAPT_SCOPE.md`
against production-adjacent infrastructure.

## 22. Security Headers

Re-confirmed present in `internal/middleware/security_headers.go`:
`X-Content-Type-Options: nosniff`, `X-Frame-Options: DENY`,
`Content-Security-Policy: default-src 'none'; frame-ancestors 'none'`,
`Strict-Transport-Security` (conditional on HTTPS being in front, per
its own comment). `Referrer-Policy`/`Permissions-Policy`: not found in
the current header set — **gap**, low severity for an API with no
browser-rendered content, but should be added for completeness (see
§29).

CORS: `cmd/api/main.go` sets `AllowOrigins` from `cfg.CORSAllowedOrigins`
(configured list), not a wildcard; boot log confirms a warning (not a
silent default) when unset in development:
`⚠️ CORS_ALLOWED_ORIGINS not set — allowing all origins
(development/sandbox only; refused at startup in production)`.

## 23. Secrets / Configuration

`.env.example` inspected line-by-line — every value is a
`change_this_...` placeholder, no real secret present. Grepped `internal/`
and `cmd/` for hard-coded API keys/passwords matching common patterns —
none found; all secret values are read from environment/config. No
secret found in logs, test files, or CI configuration reviewed. Per the
phase brief's own instruction, no secret value is reproduced in this
report.

## 24. Dependency Security

**Not performed this phase** — `govulncheck` was not available in this
execution environment and no alternative CVE database lookup was
attempted. This is an explicit open item (§29), not a silent skip.
`go.mod` versions were visually inspected for obvious staleness
(`golang-jwt/jwt/v5 v5.3.1`, `gin-gonic/gin v1.12.0`, `gorm.io/gorm
v1.31.2`) — none appeared conspicuously outdated, but this is not a
substitute for an actual CVE scan.

## 25. Cryptography

Re-confirmed rather than re-implemented: bcrypt for
password/MPIN/admin-password hashing (grepped for `bcrypt.GenerateFromPassword`
across `internal/auth`, `internal/admin`, `internal/employee` — present
in all three, no plaintext or weak-hash storage found). AES-256-GCM for
PAN and document encryption at rest, with two *separate* keys
(`PAN_ENCRYPTION_KEY` vs `DOCUMENT_ENCRYPTION_KEY`), confirmed distinct
by the startup validation Phase 7 built. JWT secrets: three separate
per-role secrets (§4), HMAC-SHA256. No hard-coded key found anywhere in
source (§23). No new cryptography was invented this phase — the existing
architecture already provides the appropriate controls, per the task
brief's own instruction not to invent redundant encryption.

## 26. Logging / Audit

Security-relevant events audited: login, logout, session revocation,
disbursement, expense approval/rejection, evidence download,
export requests (`audit.RecordDetailed` calls confirmed present at each
of these call sites during this session's review). Audit log is
append-only at the database level (§6). Not found: passwords, OTP, MPIN,
JWT, or API secrets logged anywhere in `internal/` — grepped for
`log.Print`/`fmt.Print` near credential-handling code, found none
printing sensitive values.

## 27. Error Handling

Spot-checked several handler error paths (`admin/handler.go`,
`analytics/handler.go`, `reporting/handler.go`) — errors returned to the
client are short, generic messages (`"Invalid Token"`,
`"invalid request body"`), never raw Go error strings that would leak a
SQL query, file path, or internal package name. Not exhaustively
enumerated across all ~250+ handlers this phase — a representative
sample, not a full sweep (residual risk noted in §30).

## 28. Database Security

Connection uses `DB_SSLMODE` from config (documented in `.env.example`
as `require` for managed Postgres). Foreign keys and constraints are
defined via GORM model tags throughout (not independently re-audited
row-by-row this phase). DB user privilege level (whether the app
connects as a superuser or a scoped role) is an infrastructure/deployment
concern outside this codebase's control — flagged for the VAPT scope
document's infrastructure-dependency note, not fixable in application
code.

## 29. Audit Log Immutability

Re-confirmed via `TestAuditLogCannotBeModifiedOrDeleted` (Phase 7,
re-run this session, passes) that the Postgres trigger rejects
`UPDATE`/`DELETE` on `audit_logs` at the database level — not just an
application-layer check that a compromised or buggy admin route could
bypass. This is the strongest form of the control the phase brief asks
for (item AB): it holds even against a direct database-accessible path
or a mass-assignment attempt through an admin route, because the
rejection happens below the application layer entirely.

---

## Confirmed Findings (VAPT Finding Format)

### Finding 1 — JWT signing algorithm not pinned at verification

- **Severity**: MEDIUM
- **Affected component**: `internal/middleware/admin_auth_middleware.go`,
  `auth_middleware.go`, `employee_auth_middleware.go`,
  `customer_or_employee_auth_middleware.go` — all 4 JWT verification
  sites.
- **Description**: Every `jwt.ParseWithClaims` call's keyfunc returned
  the appropriate HMAC secret unconditionally, without first confirming
  the token's declared algorithm was the expected one (`HS256`). The
  golang-jwt/v5 library used here rejects an `alg:none` token by default
  without an explicit unsafe opt-in, so this was not exploitable against
  the current library version — but nothing in the code enforced that;
  a library change, a future RSA-based token type sharing a keyfunc
  pattern, or a downgrade could have silently reopened an algorithm-
  confusion path.
- **Attack scenario**: An attacker crafts a JWT with header
  `{"alg":"none","typ":"JWT"}` and an empty signature segment, containing
  a victim's identifying claim (e.g. `mobile` or `adminId`). If the
  verification library or its configuration ever accepted unsigned/
  alg-mismatched tokens, this would forge authentication as any user
  without knowing any secret.
- **Impact**: Full authentication bypass, if the underlying condition
  were ever met.
- **Root cause**: Missing explicit algorithm allowlist at the JWT
  verification call site — relying entirely on library-default behavior
  rather than an explicit application-level constraint.
- **Proof/reproduction**: `internal/security/security_test.go`,
  `TestJWTAlgNoneRejected` — constructs
  `eyJhbGciOiJub25lIiwidHlwIjoiSldUIn0.eyJtb2JpbGUiOiI5OTk5OTk5OTk5In0.`
  (a well-formed `alg:none` token, empty signature) and confirms the real
  customer auth middleware, through the production route table, returns
  401.
- **Fix**: Added `jwt.WithValidMethods([]string{"HS256"})` to all 4
  `jwt.ParseWithClaims` call sites.
- **Regression test**: `TestJWTAlgNoneRejected` (new, passing).
- **Residual risk**: None identified for this specific vector. General
  JWT hardening (token revocation-on-claim-change, audience/issuer
  claims) is not otherwise added — this codebase's tokens are
  single-purpose per role and don't cross audience boundaries, so
  issuer/audience claims were judged unnecessary complexity rather than
  a gap.

### Finding 2 — CSV/Excel formula injection unmitigated in report exports

- **Severity**: MEDIUM
- **Affected component**: `internal/reporting/exports.go`
  (`generateCSV`, `BuildXLSX` via `xlsx.go`'s `sheetXML`).
- **Description**: Cell values reaching both the CSV writer and the
  custom XLSX writer came directly from report builders — including
  free-text fields sourced from admin/vendor/customer-adjacent data
  (expense descriptions, notes, KPI notes) — with no check for a leading
  formula-trigger character (`=`, `+`, `-`, `@`, tab).
- **Attack scenario**: An authorized user with write access to a
  free-text field (e.g. an expense description) enters
  `=HYPERLINK("http://attacker.example/","Click for details")`. Any
  admin who later exports the expense register to CSV or Excel and opens
  it triggers that formula on open — a live link/formula executing in
  the context of whoever opened the export, not the attacker.
- **Impact**: Data exfiltration or further social-engineering via a
  spreadsheet formula executed on the exporting admin's machine; in more
  severe spreadsheet-application configurations, formula-driven command
  execution (`=cmd|'/c calc'!A0`-style DDE payloads) is a known class of
  this vulnerability, though modern Excel disables DDE by default.
- **Root cause**: No output-encoding step existed between
  report-builder-produced cell content and the export writers.
- **Proof/reproduction**: `internal/security/security_test.go`,
  `TestExportFormulaInjectionNeutralized` — confirms every OWASP-named
  trigger character (`=`,`+`,`-`,`@`, tab) is quote-prefixed by the
  sanitizer and that safe values are left byte-for-byte unchanged.
- **Fix**: `internal/reporting/csv_sanitize.go` (new) — a single
  choke-point sanitizer (`sanitizeCell`/`sanitizeRow`/`sanitizeRows`/
  `sanitizeTotals`) applied once inside `RequestExportWithMetadata`,
  the one function both the generic and controlled export paths funnel
  through, so no individual report builder had to be touched or could
  forget it.
- **Regression test**: `TestExportFormulaInjectionNeutralized` (new,
  passing).
- **Residual risk**: None identified for the CSV/XLSX writers this
  codebase controls. The PDF export path (`generatePDFPlaceholder`) is a
  plain-text-in-PDF renderer with no formula concept, so it was not a
  vector and was not modified.

---

## Security Test Matrix

| Test | Target | Expected | Actual | Status | Severity | Fix | Regression Test |
|---|---|---|---|---|---|---|---|
| alg:none JWT | All 4 auth middlewares | 401 | 401 (after fix) | 🟢 FIXED | MEDIUM | `jwt.WithValidMethods` pin | `TestJWTAlgNoneRejected` |
| Formula-leading export cell | CSV/XLSX writers | quote-prefixed | quote-prefixed (after fix) | 🟢 FIXED | MEDIUM | `csv_sanitize.go` | `TestExportFormulaInjectionNeutralized` |
| Cross-customer loan/payment access | `internal/loans`, `internal/payments` | 403 | 403 | 🟢 PASS | — | pre-existing | `TestIDORCustomerCannotAccessAnotherLoan`, `...Payment` |
| Revoked admin session reuse | `AdminAuthMiddleware` | 401 | 401 | 🟢 PASS | — | pre-existing | `TestRevokedAdminSessionRejected` |
| READ_ONLY admin escalation | `/admin/*` | 403 | 403 | 🟢 PASS | — | pre-existing | `TestReadOnlyAdminCannotEscalate` |
| Self-approval of sanction recommendation | Loan sanction workflow | 403 | 403 | 🟢 PASS | — | pre-existing | `TestMakerCannotApproveOwnSanctionRecommendation` |
| Wrong-secret JWT | All 4 auth middlewares | 401 | 401 | 🟢 PASS | — | pre-existing | `TestInvalidJWTRejected` |
| Login brute force | `/auth/login`, `/admin/login` | rate-limited | rate-limited | 🟢 PASS | — | pre-existing | `TestRateLimitEnforcedOnLogin` |
| SQL injection payload | search/filter params | safely rejected/no-op | safely rejected | 🟢 PASS | — | pre-existing | `TestSQLInjectionPayloadSafelyRejected` |
| Oversized payload | request body | graceful rejection | graceful rejection | 🟢 PASS | — | pre-existing | `TestOversizedPayloadRejectedGracefully` |
| Duplicate webhook/journal | payment webhook | idempotent | idempotent | 🟢 PASS | — | pre-existing | `TestDuplicateJournalIdempotencyEnforced` |
| Sensitive data in loan response | customer loan API | no PAN/Aadhaar leak | no leak | 🟢 PASS | — | pre-existing | `TestSensitiveDataNotLeakedInLoanResponse` |
| Unauthorized export | report export routes | 403 | 403 | 🟢 PASS | — | pre-existing | `TestUnauthorizedExportBlocked` |
| Audit log tamper via API | `audit_logs` table | rejected at DB level | rejected | 🟢 PASS | — | pre-existing | `TestAuditLogCannotBeModifiedOrDeleted` |
| Post to closed accounting period | `internal/accounting` | rejected | rejected | 🟢 PASS | — | pre-existing | `TestClosedPeriodBlocksNewPosting` |
| Unauthorized disbursement prep | disbursement routes | 403 | 403 | 🟢 PASS | — | pre-existing | `TestUnauthorizedDisbursementBlocked` |
| COLLECTION_OFFICER on Phase 13 analytics | `/admin/analytics/*` | 403 | 403 | 🟢 PASS | — | Phase 13 | `TestUnauthorizedCannotAccessAnalytics` |

---

## 30. Remediation

For both confirmed findings: code fixed → regression test added →
targeted test run (passing) → full suite re-run (passing, all 14 tested
packages) → production boot re-verified. No test was suppressed or
weakened to make it pass.

## 31. Residual Risk

Not eliminated by this phase, and explicitly not claimed to be:

- **No dependency CVE scan performed** (`govulncheck` unavailable) — the
  single highest-value follow-up before any external engagement.
- **Not exhaustively re-tested**: business-logic scenarios beyond the
  four re-verified in §16 (disburse-twice, refund-twice,
  modify-approved-transaction, post-sanction due-date/interest/fee
  tampering); tax/TDS-specific attack scenarios (§18); compliance
  evidence-tampering scenarios beyond the RBAC/audit check in §19;
  statement-timeout/connection-pool-level DB hardening (§20/§28); a full
  ~250+-route IDOR sweep (only representative handlers per resource type
  were checked, §9).
- **No live infrastructure testing**: SSRF was assessed by code review
  only (§21), not live probing; no load-based rate-limit exhaustion test
  beyond Phase 7's existing unit-level test.
- **No OpenAPI/Swagger inventory exists** (API9, §5) — the attack-surface
  map in this report was hand-built from `routes.go`; an assessor should
  independently re-derive it rather than trust this document as
  exhaustive.
- **`Referrer-Policy`/`Permissions-Policy` headers absent** (§22) — low
  severity, easy follow-up.
- Full carry-forward list from Phases 7–13 is restated at the bottom of
  this report per the mandated closing format.

## 32. Production Security Checklist

Machine-checkable, one line per control:

- [ ] `APP_ENV=production` and `DEMO_MODE=false` at deploy time (enforced
  by `internal/config.ValidateProductionSafety` — verify it actually
  refused to start otherwise, don't just trust the flag).
- [ ] `JWT_SECRET`, `ADMIN_JWT_SECRET`, `EMPLOYEE_JWT_SECRET` are three
  distinct, long, random values (enforced at startup).
- [ ] `PAN_ENCRYPTION_KEY` and `DOCUMENT_ENCRYPTION_KEY` are distinct
  32-byte base64 values, not the placeholder text.
- [ ] `CORS_ALLOWED_ORIGINS` set to the real frontend origin(s) only, no
  wildcard.
- [ ] `DB_SSLMODE=require` (or stronger) against a managed Postgres
  instance.
- [ ] Razorpay production credentials configured, webhook secret
  distinct from any test-mode value.
- [ ] Run `govulncheck ./...` (or equivalent) before go-live — not
  performed by this phase.
- [ ] Confirm `audit_logs` append-only trigger is present in the
  production database (it is created by `admin.Migrate()`, but verify
  post-deploy against the live schema).
- [ ] Rotate all secrets used during any pre-production/VAPT engagement
  before go-live.
- [ ] Add `Referrer-Policy`/`Permissions-Policy` headers (open item,
  §22/§31).

---

# PHASE 14 STATUS: **PARTIAL**

Two confirmed vulnerabilities were found, fixed, and regression-tested.
A representative (not exhaustive) pass was completed across the OWASP
API/Web categories, authentication, authorization, IDOR, injection, file/
export/payment/webhook/business-logic/accounting/audit security. Full
exhaustive coverage of all ~250+ admin routes, a dependency CVE scan, and
live infrastructure testing were not completed this phase — see §31 for
the complete residual-risk list. PARTIAL, not COMPLETE, because those
gaps are real and material to an actual VAPT engagement's confidence
level, not because any known issue remains unfixed.

## PHASE 7 REMAINING
Unchanged from its own report's carry-forward (this phase re-verified
its existing tests still pass but did not re-derive new ones): any items
Phase 7's own report already listed as open remain open.

## PHASE 8 REMAINING
Unchanged: full report-type export wiring for 2 register types,
GST-input-tax-credit tracking, admin UI.

## PHASE 9 REMAINING
Unchanged: no admin-initiated refund workflow, 2 daily-summary Excel
exports, large-failed-payment-spike/repeated-webhook-failure alerts,
`go test -race` not run, no admin UI.

## PHASE 10 REMAINING
Unchanged: `PostTDSDeduction`/`finance.MarkPaid` not automatically
linked, no return-content/certificate generation, no live PAN-validation
provider, 5 of 10 named Excel exports, no admin UI.

## PHASE 11 REMAINING
Unchanged: Excel exports for all 10 named report types, PDF
audit-report generation, unified 5-domain management dashboard
composition, automated data-lineage exception detection for regulatory
returns, regulatory-return-pending/rejected and reconciliation-failure
alert triggers, dedicated evidence-IDOR/CAPA-manipulation/applicability-
tampering security tests, admin panel UI.

## PHASE 12 REMAINING
Cash Flow Foundation, Management Report Pack composition, remaining
Phase 12-specific Excel workbooks, PDF management reports (all 7 types),
report snapshot/versioning, Financial Year roll-forward report
composition, additional filters beyond date range, per-domain
data-consistency tests beyond the one finance-dashboard test, audit-log
wiring for MIS dashboard/report generation, admin panel UI/drill-down.

## PHASE 13 REMAINING
Statistical forecasting, frontend/Intelligence nav and charts,
materialized views/background jobs/date partitioning for
100,000+-customer scale, dedicated IDOR/cross-period/export-abuse tests
for the new analytics endpoints, channel/full customer-type/risk-category
segmentation (genuinely unavailable — no such data exists), product/
geography-level revenue-expense-profitability allocation (not reliably
calculable, correctly left undone).

## SECURITY FINDINGS
1. JWT signing algorithm not pinned at verification (MEDIUM) — see
   Finding 1.
2. CSV/Excel formula injection unmitigated in exports (MEDIUM) — see
   Finding 2.

## FIXED
Both findings above — code fixed, regression test added, full suite
re-run green, production boot re-verified.

## OPEN
No confirmed vulnerability remains open. Residual risk (§31) — untested
areas, not known defects — remains, principally: no dependency CVE scan,
incomplete business-logic/tax/compliance re-testing beyond what was
spot-checked, no OpenAPI inventory, no live infrastructure testing,
missing `Referrer-Policy`/`Permissions-Policy` headers.

---

# CRITICAL: 0
# HIGH: 0
# MEDIUM: 2 (both fixed)
# LOW: 1 (missing Referrer-Policy/Permissions-Policy headers, open)
# INFORMATIONAL: 1 (no OpenAPI/Swagger inventory, open)

# CRITICAL OPEN: NO
# HIGH OPEN: NO

# VAPT READINESS: **READY, WITH DOCUMENTED SCOPE LIMITS**

No CRITICAL or HIGH severity issue is open, so this does not block
external VAPT readiness. "VAPT READY" here means: the application is
sufficiently prepared for an independent engagement to begin —
authentication, authorization, injection, and business-logic controls
have been reviewed and the two issues actively found were fixed — **not**
that the application has passed VAPT, is certified, or is guaranteed
free of vulnerabilities outside this phase's actual scope (§31). An
independent assessor should treat §31's residual-risk list, especially
the missing dependency CVE scan and the non-exhaustive route coverage,
as their own starting checklist, not as settled.

**NEXT PHASE: PHASE 15 — 100K+ USERS SCALABILITY, PERFORMANCE ENGINEERING, LOAD TESTING, DATABASE OPTIMIZATION & HIGH-TRAFFIC READINESS**

**REMAINING PHASES: 15, 16**
