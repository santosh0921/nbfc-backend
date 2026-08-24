# NBFC Compliance Gap Audit

**Scope audited:** Go backend (`c:\Users\Administrator\Desktop\NBFC\internal`), customer + employee Flutter app (`lib/`), admin panel (`C:\Users\Administrator\Desktop\nbfc-admin-panel`).
**Method:** Read-only source inspection (no code changed). Every claim below is grounded in an actual file/line; nothing is inferred from UI screenshots alone — backend persistence, authorization, and audit trail were checked for each item per the brief.
**Legend:** ✅ Fully integrated · 🟡 Partially integrated · 🔴 Not integrated · ⚠️ Needs compliance/professional validation (a legal/regulatory judgment call, not a code question)

---

## 1. IT / Security Policies & Access Controls

| Requirement | Status | Evidence | Gap / Risk |
|---|---|---|---|
| Customer authentication | 🟡 | `internal/auth/jwt.go`, `internal/auth/mpin.go` — bcrypt-hashed MPIN, server-side check (`handler.go:271`), 5-attempt/15-min lockout persisted to DB (`handler.go:273-294`) | No refresh token; logout is a no-op (see below) |
| Admin authentication | 🔴 | `internal/admin/login_handler.go:31-46` | **Plaintext string comparison** against `ADMIN_USERNAME`/`ADMIN_PASSWORD` env vars. No hashing, no per-account lockout, no `admin_users` table — single shared credential for every admin. The repo's own `.env` still has `ADMIN_PASSWORD=change_this_admin_password` — the placeholder was never rotated. |
| Employee authentication | 🟡 | `internal/employee/login_handler.go` — bcrypt password, per-request re-check of `Employee.Active`/`Agency.Active` (`middleware/employee_auth_middleware.go:60-72`) | No per-account lockout (only generic 1 req/s IP rate limit) |
| Biometric login | 🔴 | `internal/auth/profile.go:71-112` (`ToggleBiometric`) | Purely a client-side preference boolean; server never receives or verifies biometric data — it's OS-level unlock of a locally cached session, not server-side auth |
| Session invalidation / logout | 🔴 | `internal/auth/logout.go:9-13` | Pure no-op handler. No token blocklist/`jti` tracking anywhere. A stolen JWT stays valid for its full 12–24h lifetime after "logout." |
| Formal RBAC / permission model | 🔴 | No roles table, no permission matrix anywhere in `internal/` | Authorization is ad hoc per-handler ("is this a valid admin/employee JWT" + occasional ownership check), not policy-driven. `employee_role` is set into request context but **never read/enforced** by any handler found — the verification/recovery/supervisor distinction is cosmetic |
| Resource-level (IDOR) checks | 🟡 | Good pattern found: `internal/loans/employee_handler.go:34-37`, `internal/recovery/handler.go:36-39` explicitly check `AssignedEmployeeID == empID` | Admin endpoints have no scoping by design (admin is global) — acceptable if intentional, but means one compromised admin credential = unrestricted access to every customer's loan/PII, and there was no time in this pass to exhaustively check every customer-facing handler for missing ownership filters |
| Admin role hierarchy | 🔴 | `internal/admin/jwt.go:34-37` — JWT claims carry only `Username`, no role/permission field | Every admin session is equally, fully privileged — no super-admin vs. read-only admin |
| Secrets management | 🟡 | `internal/config/config.go`, `.env` gitignored (confirmed not committed), JWT secrets fail-fast if unset | `ADMIN_PASSWORD` is the unrotated placeholder in the actual `.env` — a live walk-up-login risk in this environment right now |
| TLS enforcement | 🟡 | No `ListenAndServeTLS`/explicit TLS in Go source — delegated entirely to hosting platform | Acceptable if platform (Render) terminates TLS, but nothing in-app enforces HTTPS-only cookies/headers (HSTS not found) |
| CORS policy | 🔴 | No `cors`/`Access-Control` reference anywhere in `internal/` | No explicit, documented origin allowlist — undefined default behavior |
| Rate limiting | 🟡 | `internal/middleware/rate_limit.go` — in-memory per-IP token bucket, applied only to auth endpoints (`routes.go:28,39-42,64,121-122`) | Explicitly commented as single-instance-only (breaks under horizontal scaling); no protection on any non-auth endpoint (loans, documents, KYC) |

**⚠️ Needs validation:** whether a single shared admin credential and absent RBAC hierarchy is acceptable under the NBFC's own IT Security Policy / RBI IT Governance framework — this is a policy decision for compliance/CISO sign-off, not just a code fix.

---

## 2. Backup / DR / BCP

| Requirement | Status | Evidence |
|---|---|---|
| Automated DB backups | 🔴 | NOT FOUND — no backup script, cron job, or documented backup/restore procedure anywhere in the repo |
| Multi-region / failover | 🔴 | NOT FOUND — rate limiter's own comment confirms single-instance design (`rate_limit.go:19-21`) |
| Disaster recovery runbook | 🔴 | NOT FOUND — no `DR.md`, no documented RTO/RPO anywhere |
| BCP documentation | ⚠️ | Not a code artifact — needs the NBFC's own BCP document, which is outside this codebase's scope entirely |

This is a single Postgres instance with no evidenced backup strategy at all. **This is a P0 gap** — a disk failure or accidental `DROP TABLE` would be unrecoverable as things stand, for a system holding loan/KYC/financial data.

---

## 3. Incident Response

| Requirement | Status | Evidence | Gap |
|---|---|---|---|
| Brute-force protection | 🟡 | Customer: real lockout (`auth/handler.go:273-278`). Admin/Employee: **none** — only generic 1 req/s/IP limit | Admin/employee accounts crackable via distributed low-and-slow attempts with no lockout and no alerting |
| IP/forensic logging | 🔴 | Rate limiter tracks IP only in-memory, evicted after 15 min idle (`rate_limit.go:59`) — never persisted. `AuditLog` model has **no IP address field** at all (`internal/models/auditlog.go:5-14`) | No forensic trail for investigating an incident after the fact |
| Anomaly / fraud detection | 🔴 | NOT FOUND — no velocity checks, impossible-travel detection, device fingerprinting anywhere | |
| Documented IR plan/playbook | ⚠️ | Outside codebase — organizational policy needed |
| Monitoring / alerting (APM) | 🔴 | NOT FOUND — no Sentry/Datadog/Crashlytics, no structured logging (plain `log.Println` throughout, 20+ call sites) | No error visibility, no alert-on-anomaly capability in production today |

---

## 4. Vendor Security

| Vendor | Purpose | Evidence | Status |
|---|---|---|---|
| Surepass | PAN verification (sandbox endpoint) | `internal/surepass/*` | 🟡 integrated but bypassed under `DEMO_MODE=true` (see §6) |
| Google STUN | WebRTC video-call NAT traversal | `lib/core/services/call_service.dart:91-93` | 🟡 no TURN relay configured (comment confirms P2P-only) |
| — | SMS/OTP provider | Self-managed `internal/otp` package — no Twilio/MSG91 found | ⚠️ verify OTP delivery mechanism is production-grade, not simulated |
| — | Payment gateway | NOT FOUND anywhere | 🔴 see §14 |

No vendor security assessment, SLA, or risk documentation exists for Surepass or any other integration — expected to be absent from a codebase, but flagged since the brief asked. **⚠️ Needs validation:** vendor due-diligence documentation is a compliance/procurement responsibility, not a code deliverable.

---

## 5. Audit Logs / Complete Audit Trail

| Requirement | Status | Evidence |
|---|---|---|
| Audit log exists | 🟡 | `internal/audit/log.go`, model `internal/models/auditlog.go:5-14`: `ActorType, ActorRef, Action, TargetType, TargetID, Details, CreatedAt` |
| Before/after value capture | 🔴 | `Details` is a free-text string built ad hoc per call site — **no structured diff/old-vs-new field capture anywhere** |
| Coverage | 🟡 | Logged: agency create/delete, employee create/approve/delete, loan reassign/sanction/reject/disburse/restructure, EMI payment, recovery admin action. **Not logged:** customer login (success or failure), admin login (success or failure — no record of who accessed the admin panel), employee login, any read access to sensitive data, KYC profile edits (no `kyc.updated` action found anywhere), CSV/document exports (see §7 admin panel findings — bulk PII export leaves zero audit trail) |
| Tamper-evidence | 🔴 | Plain mutable GORM table, no checksum/hash-chain/append-only constraint — anyone with DB access can edit or delete audit rows undetected |
| IP address captured | 🔴 | No IP field on the audit log model at all |
| Admin panel viewer | 🟡 | `audit_log_screen.dart` exists, real backend-backed (`GET /admin/audit-log`) | Filters (action/targetType) applied **client-side** on a capped 200-row fetch; **no actor filter, no date-range filter**, and the backend itself has no date-range query param support |

**This is a P0 gap for an NBFC** — RBI expects a complete, tamper-evident audit trail, and today's implementation misses login events, has no before/after values, no IP, is mutable, and can't be searched by actor or date range.

---

## 6. Customer Onboarding / KYC

| Requirement | Status | Evidence |
|---|---|---|
| PAN verification | 🟡 → 🔴 in current config | Real Surepass integration exists (`internal/surepass/pan.go`) **but `DEMO_MODE=true` in the checked-in `.env`** auto-approves every PAN with no API call (`internal/profile/pan_handler.go:131-138`) |
| Aadhaar verification | 🔴 | `internal/verification/aadhaar.go:34-51` — no working real-time provider in any mode; code comment states the current Surepass token returns 401 for this scope. Falls to manual admin queue outside demo mode |
| PAN encryption at rest | ✅ | AES-256-GCM, actually invoked: `internal/security/crypto.go` + `pan_handler.go:197-203` |
| Aadhaar storage | 🟡 | Raw number never stored (only masked + SHA-256 hash) — a defensible design, but effectively means Aadhaar can never be re-verified against source later |
| KYC document storage | 🔴 | `internal/models/document.go` — base64 **plaintext** TEXT column in primary DB, no object storage, no encryption, no retention policy, 10MB cap only |
| CKYC / CERSAI registry | 🔴 | NOT FOUND anywhere — mandatory NBFC obligation, entirely absent |
| Credit bureau (CIBIL) | 🔴 | `internal/loans/reference.go:63-80` — **deterministic hash of the customer's UUID**, not a real bureau pull. No CIBIL/Experian/CRIF/Equifax API integration exists |
| Bureau reporting (loan performance → bureau) | 🔴 | NOT FOUND |
| Video KYC | 🔴 | `internal/signaling/handler.go` is a generic WebRTC relay that explicitly never touches media server-side — no geo-tagging, liveness detection, or consent recording. Not RBI-compliant VKYC |
| Mandatory KYC gate before loan submission | 🔴 | `internal/loans/customer_handler.go` `SubmitLoanHandler` only checks financial eligibility (amount/tenure/FOIR) — does **not** require completed PAN/Aadhaar/selfie records to exist first |
| PEP / sanctions-list / OFAC screening | 🔴 | NOT FOUND |
| FATCA/CRS declaration | 🔴 | NOT FOUND |
| Reference verification (2 references) | 🔴 | Model exists but is **dead code** — confirmed by the code's own comment: "References are no longer collected on application/top-up." No handler ever creates a `LoanReference` row |

**⚠️ Needs validation:** whether the fake CIBIL score and absent CKYC integration are acceptable for a pilot/demo vs. blocking for any real lending — this is a regulatory question for your compliance officer, not a technical one. But as shipped, **no real credit decision in this app is backed by a real bureau score.**

---

## 7. LOS (Loan Origination)

| Requirement | Status | Evidence |
|---|---|---|
| Application state machine | 🟡 | `internal/models/loan.go:11-19` — a status string, transitions enforced ad hoc per-handler, not a modeled state machine |
| Underwriting policy | 🟡 | Hardcoded constants (`CibilRejectThreshold=650`, etc., `loan.go:87-92`) + FOIR≤50% check (`eligibility.go`) — deterministic but not a configurable, versioned policy engine |
| Multi-level approval / exposure limits | 🔴 | **Any single admin can sanction any amount with no exposure limit, no committee threshold** |
| Maker-checker on sanction/disbursement | 🔴 | Same admin can sanction then immediately disburse in two API calls — no second approver anywhere (see §12) |

## 8. LMS (Loan Management / Servicing)

| Requirement | Status | Evidence |
|---|---|---|
| EMI schedule generation | ✅ | `internal/loans/emi_service.go:41-87` — correct reducing-balance amortization, generated at disbursement |
| Restructure / re-amortization | ✅ | `AdminRestructureLoanHandler` (`admin_handler.go:295-379`) regenerates remaining schedule |
| Payment recording | 🔴 | `CustomerPayEmiHandler` (`emi_handler.go:139-223`) — `VerifyGatewaySignature` is an **explicit stub** (code comment: "no payment gateway wired into this project yet"); only checks the transaction ID string is non-empty. **A customer can self-report any string as proof of payment and have an EMI marked paid with zero real verification.** Row-locking and reference-uniqueness prevent double-spend of the *same* reference, but not fabrication of a new one |
| Interest accrual (ongoing) | 🔴 | One-time amortization table at disbursement only — no daily/periodic accrual ledger, no penal-interest recalculation |

**This payment-verification gap is a P0 finding** — it's the integrity of the NBFC's core cash-collection record.

## 9. eSign / Documents

| Requirement | Status | Evidence |
|---|---|---|
| Digital signature | 🔴 | `internal/loans/esign_handler.go` — a hand-drawn signature **image** rasterized into the PDF. No Aadhaar eSign (NSDL/eMudra), no PKI/DSC, no OTP-bound signing event |
| Document integrity / non-repudiation | 🔴 | No hash/checksum computed at signing time anywhere in `letter.go` — no cryptographic proof the PDF wasn't altered afterward |

**⚠️ Needs validation:** whether this level of eSign is legally sufficient for your loan agreements under the IT Act — recommend CS/legal sign-off; as implemented it is closer to a scanned wet signature than a compliant eSign.

## 10. Disbursement / Collections

| Requirement | Status | Evidence |
|---|---|---|
| Disbursement ↔ actual bank rail | 🔴 | `AdminDisburseHandler` — purely a DB status/amount update, **no NEFT/RTGS/IMPS API, no UTR captured anywhere** |
| Bank reconciliation | 🔴 | NOT FOUND — no bank statement import or UTR-matching logic anywhere |
| Collections / recovery pipeline | 🔴 | `RecoveryCase` is only ever created by `internal/seed/seed.go` (demo data) or manually — **no automatic pipeline from overdue EMIs to a recovery case exists.** The recovery employee app UI reflects only manually seeded cases, not the live delinquency state of the loan book |

## 11. NPA / DPD Classification & Provisioning

🔴 **Entirely absent.** Searched exhaustively for `NPA`, `DPD`, `IRAC`, `provision`, bucket terms — nothing found. The only "overdue" concept is a derived display string computed at read time (`emi_service.go:90-95`), never persisted, never bucketed, never rolled up into an asset-classification or provisioning calculation. **This is a P0 regulatory gap** for any NBFC required to follow RBI IRAC norms.

## 12. Segregation of Duties / Maker-Checker

🔴 **Entirely absent across the whole loan lifecycle.** Confirmed in `internal/loans/admin_handler.go` (full file reviewed): KYC approval, sanction, disbursement, and restructuring are each single-admin, single-call, immediate-effect actions with no second-approver field, no pending-approval intermediate state, anywhere. Combined with §1's single shared admin credential, **one compromised login can originate and fund a fraudulent loan end-to-end with zero independent checkpoint.** This is the single highest-severity finding in this audit.

## 13. Accounting: Chart of Accounts, Ledger, Loan/Interest/Fee/NPA Accounting, Financial Close, Bank Reconciliation

🔴 **No formal accounting system exists at all.** No chart of accounts, no journal entries, no double-entry ledger, anywhere in the codebase. `CustomerTransaction` (`internal/loans/transactions_handler.go`) is explicitly a **read-only synthesized view** over `LoanApplication`/`EmiInstallment` fields for the customer's app screen — not a ledger of record. No trial balance, P&L, or balance-sheet generation exists (backend or admin panel). No period-end close process. This is expected for a fintech MVP but is a **hard blocker for real regulatory financial reporting** — an NBFC must be able to produce IRAC-compliant books.

## 14. GST / TDS / Processing-Fee Accounting

| Requirement | Status | Evidence |
|---|---|---|
| GST calculation on processing fee | ✅ | `internal/loans/gst.go` — 2% fee, 18% GST split CGST9+SGST9 (intra-state) / IGST18 (inter-state), computed once at sanction against `CustomerAddress.CurrentState` vs. `COMPANY_STATE` env var (defaults `"Maharashtra"`), persisted on `LoanApplication`, shown as line items in both letter PDFs and admin/customer loan-detail screens — **this was verified as genuinely working**, not display-only math |
| Company GSTIN | 🔴 | NOT FOUND anywhere — no field, no hardcoded value. `companyState()` is an unvalidated env var, not backed by an actual registered GSTIN |
| GST-compliant tax invoice | 🔴 | GST figures only ever appear as line items inside the Sanction/Disbursement Letter PDF, identified by loan ID — **not a sequential, GST-Rule-46-compliant tax invoice number** |
| GSTR-1 style export/filing report | 🔴 | NOT FOUND |
| Processing fee actually deducted from disbursement | 🔴 | `AdminDisburseHandler` sets `DisbursedAmount` from the request/`AmountRequested` with **no reference to the computed fee/GST fields at all** — the fee is purely informational and never reflected in actual money movement |
| TDS | 🔴 | NOT FOUND anywhere — no `194`/TDS deduction logic |

**⚠️ Needs validation:** GST invoicing compliance (sequential tax-invoice numbering, GSTIN display) should be confirmed with your CA before this is relied on for actual GST returns — as implemented, it computes the right numbers but doesn't produce a compliant tax invoice document.

## 15. MIS / Management Reporting / Regulatory Reporting

| Requirement | Status | Evidence |
|---|---|---|
| Dashboard KPIs | ✅ | `internal/admin/dashboard_handler.go` — real server-side `Count()`/`Where()` aggregation |
| Loan portfolio KPIs | 🟡 | `dashboard_screen.dart:206-278` — real data but **aggregated client-side** by fetching the entire loan list into the browser; won't scale and isn't a real backend report |
| SLA alerts | ✅ | `internal/alerts/handler.go` |
| NPA/DPD/PAR reports | 🔴 | NOT FOUND (follows from §11) |
| GST summary report | 🔴 | Per-loan GST is computed (§14) but no aggregated GST-collected-by-period report exists |
| RBI regulatory returns (e.g., DNBS) | 🔴 | NOT FOUND |
| Bulk PII export controls | 🔴 | CSV export (customers/loans/recovery) exists and works, is real-data-backed, but **generates zero audit-log entry** for the export itself — an admin can bulk-download the full loan book with no record of having done so. No DLP, no watermarking |

## 16. Internal Audit / Credit-KYC-Collection Audits / Regulatory Compliance Testing

🔴/⚠️ No internal-audit workflow, no periodic credit/KYC/collection audit tooling, and no regulatory-compliance test suite exist in this codebase — and this category is inherently mostly organizational process, not code. **⚠️ Needs validation from your internal audit function / external auditor** on what evidence/reporting they'll require the system to produce; right now it can't produce most of it (see §11, §13, §15).

## 17. Testing

🔴 Only one real backend test exists (`internal/profile/name_match_test.go`, KYC name-matching logic). No integration/e2e tests, no SAST/DAST, no security test suite. Flutter side has only the default unmodified scaffold test.

---

# Roadmap

## P0 — Critical before any real-money production use
1. **Maker-checker on sanction & disbursement** — require a second, distinct admin identity to approve before funds move. (§12, the single highest-severity finding)
2. **Real admin identity + hashed credentials** — replace the single shared plaintext `.env` credential with an `admin_users` table, bcrypt passwords, per-account lockout, and individual audit attribution. (§1)
3. **Rotate `ADMIN_PASSWORD`** off the checked-in placeholder immediately, independent of any code change.
4. **Real payment verification** — integrate an actual payment gateway (Razorpay/Cashfree/etc.) and replace the `VerifyGatewaySignature` stub with real webhook/signature verification before trusting any EMI as "paid." (§8)
5. **Database backup strategy** — automated, tested, documented backup/restore with a defined RTO/RPO. (§2)
6. **Complete the audit trail**: log all login attempts (success/failure, all 3 roles), capture IP address, add before/after value diffing, log PII exports/document downloads, make the audit table append-only (DB-level trigger or a separate write-only store). (§5)
7. **NPA/DPD classification engine** — persisted DPD bucket + IRAC asset classification per loan, recomputed on a schedule. (§11)
8. **Turn off `DEMO_MODE` for any real KYC** and get Aadhaar verification actually working (current Surepass token is confirmed non-functional for this scope) before onboarding real customers. (§6)
9. **Real credit bureau integration** replacing the deterministic hash "CIBIL score" before any real underwriting decision relies on it. (§6, §7)
10. **Disbursement ↔ bank rail integration** with UTR capture, so "disbursed" reflects real money movement. (§10)

## P1 — High priority
1. Formal RBAC/permission model with an admin role hierarchy (super-admin / operational admin / read-only).
2. Session invalidation on logout (token blocklist or short-lived + refresh-token model).
3. CKYC/CERSAI registry integration.
4. Chart of accounts + double-entry ledger for loan disbursement, EMI collection, fee, and GST — the foundation every other accounting item (§13, §15) depends on.
5. GST-compliant sequential tax invoice generation + GSTIN capture.
6. Automated recovery-case creation from overdue EMIs (currently manual-seed-only).
7. Document encryption at rest (or move to encrypted object storage) — currently plaintext base64 in the primary DB.
8. Server-side KYC-completeness gate before loan submission is accepted.
9. Bank reconciliation (statement import + UTR matching).
10. TDS logic if/where applicable to this NBFC's fee/interest structure (confirm scope with your CA — ⚠️).

## P2 — Important
1. Server-side loan-portfolio KPI aggregation (replace client-side computation).
2. NPA/DPD/PAR and GST summary reports in admin MIS.
3. Real Aadhaar eSign / DSC-based signing with document hash + non-repudiation trail, replacing the embedded-image signature.
4. PEP/sanctions-list screening at onboarding.
5. Rate limiting extended beyond auth endpoints; move rate-limit state to shared storage (Redis) for horizontal scaling.
6. Structured logging + APM/error monitoring (Sentry or equivalent).
7. Data retention/purge policy implementation.
8. Either implement real reference verification or remove the dead `LoanReference` scaffolding to avoid a false sense of coverage.

## P3 — Future improvement
1. Multi-region/DR failover.
2. Anomaly/fraud detection (velocity checks, impossible travel, device fingerprinting).
3. FATCA/CRS declaration capture.
4. Security/integration/e2e test suite; periodic penetration testing.
5. Vendor security assessment documentation for Surepass and any future payment/SMS vendors.
6. Multi-currency, multi-branch exposure-limit policy engine (if the NBFC scales beyond a single-branch model).

---

# Closing Summary

**1. What is already production-ready:**
EMI amortization math (reducing balance, correctly implemented and restructure-aware); PAN encryption at rest (real AES-256-GCM, actually invoked); the newly added GST calculation logic itself (correct CGST/SGST/IGST math, though not yet wrapped in a compliant tax invoice); customer MPIN authentication with real server-side lockout; per-resource ownership checks on employee-facing loan/case endpoints; the admin dashboard's core KPI aggregation; rate limiting on authentication endpoints.

**2. What is incomplete:**
Almost everything downstream of "a loan exists" — real payment verification, NPA/DPD classification, any general-ledger/accounting system, bank reconciliation, real credit bureau integration, CKYC, RBI-compliant Video KYC and eSign, maker-checker on any financial action, and a genuinely tamper-evident, complete audit trail.

**3. Critical risks (in order of severity):**
- A single admin credential can unilaterally sanction and disburse loans with no second approver, and that credential is currently a never-rotated placeholder in `.env`.
- EMI payments can be marked "paid" on unverified customer self-assertion — the core cash-collection integrity control is a stub.
- No backup/DR strategy for a database holding financial and KYC data.
- No NPA/DPD/provisioning capability — a hard regulatory gap.
- KYC verification is effectively bypassed in the current `DEMO_MODE=true` configuration, and the underlying "CIBIL score" isn't from a real bureau.
- No real accounting ledger — the app cannot currently produce books of account.

**4. Exact work remaining:** captured in the P0–P3 roadmap above — 10 P0 items are the minimum bar before this system should touch real customer money.

**5. Recommended implementation order:** P0 items 1→10 as listed (maker-checker and admin-credential fixes first, since they gate everything else; payment verification and backups in parallel; NPA/KYC/bureau work before any real onboarding), then P1 (accounting foundation + CKYC + RBAC), then P2/P3.

**Items requiring your CA/CS/compliance officer/legal counsel before any code decision is made, not just an engineering call:**
GST tax-invoice compliance and GSTIN handling; whether the current eSign approach is legally sufficient; TDS applicability to this NBFC's specific fee/interest structure; data-retention period requirements; whether a single-admin, no-maker-checker model was ever acceptable under this NBFC's internal control policy; CKYC/CERSAI integration timeline and vendor selection; sanctions/PEP screening vendor selection; and the overall IT Security Policy this system should be measured against (this audit inferred requirements from your prompt, not from a supplied policy document).
