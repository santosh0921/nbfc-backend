# Phase 14 — VAPT Scope Document

This document defines what an independent penetration test of this NBFC
backend should cover, what it should not, and what state the target
should be in when engaged. It is a scope definition, not a test report —
see `PHASE14_SECURITY_ASSESSMENT_REPORT.md` for this phase's own
findings.

---

## In Scope

**API** — the full Go/Gin backend at `c:\Users\Administrator\Desktop\NBFC`,
all routes registered in `internal/routes/routes.go`:
- Public/customer auth: `/auth/*` (registration, login, OTP, MPIN, JWT)
- Employee auth: `/employee/*`
- Admin auth and every `/admin/*` route (loans, disbursement, payments,
  accounting, tax, compliance, risk, collections, reporting, MIS,
  analytics — Phases 1–13)
- Payment gateway webhook: `POST /webhooks/razorpay` (or equivalent
  configured path)
- File upload/download endpoints (KYC documents, compliance evidence,
  loan documents)
- Export endpoints (Excel/CSV/PDF, all report types listed in
  `internal/reporting/registers.go`)

**Authentication mechanisms**: customer JWT (`internal/auth`), employee
JWT (`internal/employee`), admin JWT + server-side session revocation
(`internal/admin`), OTP flow, MPIN flow.

**Business logic**: loan application → KYC → credit assessment →
sanction → disbursement → EMI collection → write-off/recovery;
maker-checker workflows (expense approval, sanction recommendation,
regulatory return approval, compliance CAPA); accounting period
close/reopen; TDS deduction/deposit tracking; compliance evidence
lifecycle.

**Third-party integrations actually present in code**: Razorpay (payment
gateway), a configurable credit bureau provider (`internal/bureau`), CKYC
(`internal/ckyc`), a screening/PEP provider (`internal/screening`), an
OTP provider (`internal/otp`), a PAN-verification provider (currently a
demo/stub — see Known Limitations).

**Data classes in scope**: customer PII (name, mobile, PAN, Aadhaar
reference, address, bank account), loan and payment financial data,
admin/employee credentials and session tokens, KYC documents, compliance
evidence documents.

---

## Out of Scope

- **The Flutter mobile/web client app** (`lib/`) — this phase and every
  prior security phase are backend-API-only. A separate mobile
  application security assessment would be needed for the client.
- **Underlying cloud/hosting infrastructure** (Render, or wherever this
  is deployed) — network-level infrastructure, OS hardening, and cloud
  IAM are outside this codebase's control and this document's scope.
- **Physical security, social engineering, phishing** — not applicable
  to an API-only assessment.
- **Denial-of-service load testing at production scale** — rate limiting
  is in scope to verify it exists and functions; sustained high-volume
  DoS/stress testing against a shared environment is not, to avoid
  disrupting other tenants or the database. See Phase 15's scope for
  scale/load testing.
- **Destructive testing against the production database** — all testing
  must run against an isolated test/staging database seeded per the
  procedure below, never a database containing real customer data.
- **Physical bypass of Phase 3's document encryption or Phase 7's PAN
  encryption at rest** — attempting to brute-force AES-256 keys is out
  of scope; testing that the encryption is actually applied and that keys
  are not hard-coded or logged IS in scope.
- **Third-party providers' own infrastructure** (Razorpay's servers, the
  bureau's servers, CKYC's servers) — only this codebase's integration
  with them (signature verification, credential handling, error
  handling) is in scope.

---

## Known Limitations (state these to the assessor up front)

- **DEMO_MODE**: when `DEMO_MODE=true`, PAN/Aadhaar verification is
  bypassed for local/sandbox development (see `.env.example` and
  `internal/config`). A VAPT engagement MUST run against an environment
  with `DEMO_MODE=false` and `APP_ENV=production` (or a production-like
  staging config) — `internal/config.ValidateProductionSafety` already
  refuses to start otherwise, which the assessor should confirm rather
  than assume.
- **No payment gateway credentials configured by default** — Razorpay
  order/webhook flows require `RAZORPAY_KEY_ID`/`RAZORPAY_KEY_SECRET`/
  `RAZORPAY_WEBHOOK_SECRET` to be set for the assessor's test environment;
  without them EMI online payment is disabled entirely (confirmed by this
  phase's own boot log).
- **No production PAN-validation provider is wired in** — PAN validation
  is currently a stub for non-DEMO_MODE too (carried forward from Phase
  1's report); this is a functional gap, not a security one, but the
  assessor should not assume real third-party validation is occurring.
- **No branches/geographic infrastructure exist** — this system has no
  branch entity; any test scenario assuming branch-level isolation does
  not apply (see Phase 12/13 reports).
- **Management thresholds/budgets/compliance requirements are seeded
  empty by design** (Phases 11–13) — an assessor should not interpret an
  empty alert/threshold list as "monitoring is broken"; it means nothing
  has been configured yet, which is the intended out-of-the-box state.

---

## Test Accounts / Roles Procedure

**Do not use real production credentials for testing.** This system's
seeded/test-admin creation pattern (used throughout every phase's own
test suite, e.g. `internal/security/security_test.go`'s `createTestAdmin`)
is the model to follow:

1. Provision the assessment environment against an isolated database
   (never production).
2. Create one admin account per role actually defined in
   `internal/models/admin_user.go` (`SUPER_ADMIN`, `CREDIT_OFFICER`,
   `CREDIT_MANAGER`, `OPERATIONS`, `DISBURSEMENT_OFFICER`,
   `COLLECTION_OFFICER`, `COLLECTION_MANAGER`, `ACCOUNTANT`,
   `COMPLIANCE_OFFICER`, `AUDITOR`, `READ_ONLY`) via
   `POST /admin/login`-compatible bootstrap or direct DB seed —
   `internal/rbac/*.go` defines exactly what each role can and cannot do;
   use that file as the ground truth for expected-vs-actual authorization
   testing (item F/AC of the assessment report).
3. Create at least two distinct customer accounts (for horizontal IDOR
   testing) and at least two distinct employee accounts, each with their
   own loans/applications/payments, so cross-account access attempts have
   a real target to fail against.
4. Rotate/destroy all test credentials at the end of the engagement.

## Environment Requirements

- PostgreSQL instance, isolated from production, with the same schema
  (`admin.Migrate()` on startup creates it).
- All required `.env` values from `.env.example`, with real (not
  placeholder) values for `JWT_SECRET`, `ADMIN_JWT_SECRET`,
  `EMPLOYEE_JWT_SECRET`, `PAN_ENCRYPTION_KEY`, `DOCUMENT_ENCRYPTION_KEY` —
  each distinct, per the startup validation the app already enforces.
- `DEMO_MODE=false` for a realistic assessment (see Known Limitations).
- Optional: Razorpay test-mode credentials, for payment/webhook-specific
  test cases.

## Test Data Requirements

- At least 2 customers, each with at least one loan application at each
  major status (submitted, verified, sanctioned, disbursed, rejected).
- At least one disbursed loan with an active EMI schedule, including at
  least one overdue installment (for DPD/collections testing).
- At least one expense pending approval and one already approved (for
  maker-checker testing).
- At least one accounting period, open and (separately) closed (for
  closed-period-modification testing).

## Evidence Collection Procedure

- Every finding must record: request/response pair (redacting any real
  secret value), affected endpoint, role/account used, timestamp.
- Prefer the format in `PHASE14_SECURITY_ASSESSMENT_REPORT.md` §28
  (VAPT finding format) for consistency with this phase's own findings.
- Do not store captured evidence containing real customer PII outside
  the assessment environment; use test data only, per Test Data
  Requirements above.

## Security Controls Already in Place (context for the assessor)

Summarized in full in `PHASE14_SECURITY_ASSESSMENT_REPORT.md`; briefly:
admin session revocation on logout/disable (checked on every request,
not just token expiry), maker-checker on expense approval/sanction
recommendation/regulatory-return approval, append-only audit log
(database trigger, not just application logic), AES-256-GCM encryption
for PAN and documents (Phase 3/7, separate keys), per-endpoint rate
limiting (`internal/middleware/rate_limit.go`), RBAC via
`internal/rbac`, security headers (CSP/X-Frame-Options/HSTS/
X-Content-Type-Options — `internal/middleware/security_headers.go`),
Razorpay webhook HMAC signature verification.

## Open Findings At Time of Scoping

See `PHASE14_SECURITY_ASSESSMENT_REPORT.md` §28/§30 for the full list.
Both findings identified and fixed this phase (JWT algorithm-confusion
hardening, CSV/Excel formula-injection sanitization) are closed with
regression tests before this scope document was written.
