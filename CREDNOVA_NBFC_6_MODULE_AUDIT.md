# CredNova / Jayashri Capital Pvt Ltd — NBFC 6-Module Technical Audit

**Audit type:** Read-only. No code was modified, deleted, or refactored during this audit.
**Scope:** Go backend (`internal/`), Flutter customer app (`lib/`), Flutter employee app (`lib/employee_app/`), Flutter admin panel (separate repo `nbfc-admin-panel/lib/`).
**Method:** Full-file reads of every handler/service/model referenced below (not filename-only inference), cross-referenced against actual route registrations and DI wiring.

---

## 1. Executive Summary

This is a substantially more complete system than a typical MVP or prototype. The lending core — application intake, rule-based underwriting (FOIR + CIBIL banding), a genuinely separated maker-checker approval hierarchy, EMI scheduling, a real payment-gateway integration with signature-verified webhooks, and a full double-entry accounting/GST/TDS/regulatory-reporting subsystem — is real, working code, not scaffolding.

The codebase is also unusually disciplined about **not faking third-party results**: every unconfigured external integration (Aadhaar verification, credit bureau, CKYC, PEP/sanctions screening) fails safe to `PENDING` / `MANUAL_REVIEW` / `BLOCKED` with a full audit trail, rather than silently returning a fake success. This is a meaningfully better failure mode than most systems at this stage, and it means nothing in production is *lying* about its state — it's honestly incomplete in specific, identifiable places.

Two findings are severity-critical and should be treated as immediate priorities, not backlog items:

1. **Maker-checker is not actually enforced in code**, despite doc comments claiming it is. The same admin who recommends a loan sanction can also approve it, and the same admin who prepares a disbursement can also release it, as long as their role holds both permissions (which `CREDIT_MANAGER` does by default). This defeats the entire purpose of the two-person control on the two most money-sensitive actions in the system. **Independently confirmed by two separate audit passes in this session.**
2. **The employee app's Collections screen is wired to fully fake, hardcoded data** via dependency injection — not a leftover test file, but the actual live repository the screen calls. Any "payment confirmed" there is lost, never reaching the real backend.

Beyond those two, the largest completeness gaps are: no real Aadhaar/OCR/DigiLocker/liveness verification, no real credit bureau (CIBIL/Experian/CRIF/Equifax) integration, no bank-account penny-drop verification, no actual fund-transfer API (disbursement is manual/bank-portal), and no duplicate-application/fraud/blacklist detection at intake.

---

## 2. Project Architecture

```
Flutter Customer App (lib/)  ─┐
Flutter Employee App          ├──► REST API (JSON, JWT auth) ──► Go / Gin backend (internal/) ──► PostgreSQL (GORM)
Flutter Admin Panel (separate repo, web) ─┘                                  │
                                                                              ├──► Surepass (PAN — LIVE; Aadhaar — blocked, scope issue)
                                                                              ├──► Razorpay (orders + webhooks — LIVE, inert w/o prod keys)
                                                                              ├──► Firebase Cloud Messaging (push — LIVE, inert w/o prod creds)
                                                                              ├──► AWS S3 / local AES-256-GCM (document storage)
                                                                              ├──► AWS SSM Parameter Store (Firebase creds only)
                                                                              └──► Credit bureau / CKYC / DigiLocker / OCR / penny-drop / NACH / WhatsApp / SMS-OTP-gateway / Email — NOT INTEGRATED
```

Three auth domains exist side by side: customer JWT (`CustomerAuthMiddleware`), employee JWT (`EmployeeAuthMiddleware`), and admin JWT with server-side session revocation (`AdminAuthMiddleware`) — each independently gated, not a shared identity system.

---

## 3. Six-Module Status

| Module | Implemented | Partial | Mock | Missing | Overall Status |
|---|---:|---:|---:|---:|---|
| 1. KYC Verification | KYC state machine, PAN verification, audit trail, encrypted storage | Face/selfie (format check only, no liveness) | Aadhaar/PAN/Selfie in `DEMO_MODE` | Aadhaar (prod), OCR, DigiLocker, address/DOB independent verification, agency field-verification | **Partially implemented — one real integration (PAN), rest fail-safe to manual** |
| 2. Credit Bureau | Consent + request audit trail, admin view | — | Sandbox bureau (fixed score, non-prod only) | Real bureau integration (CIBIL/Experian/CRIF/Equifax), CKYC, PEP/sanctions vendor | **Mostly missing — real plumbing around an absent provider** |
| 3. Underwriting | FOIR calc, CIBIL banding, eligibility engine, maker-checker sanction/disbursement, rejection reason codes, audit trail | Multi-loan DTI (single-loan only), income verification (self-declared only) | — | Composite risk score, employment-type-specific rules, conditional-approval state | **Substantially implemented, real automated engine** |
| 4. Fraud & Risk | Anomaly-signal engine (repeat applications, shared bank accounts, failed-payment patterns), screening fail-safe workflow | Screening (workflow real, no vendor) | — | Duplicate PAN/Aadhaar/mobile/device detection, blacklist, composite fraud score, device fingerprinting, velocity checks | **Partial — real signals for 3 specific patterns, no intake-time fraud checks** |
| 5. Disbursement & Repayment | Sanction/disbursement maker-checker, EMI engine, Razorpay orders+webhooks (real HMAC verification), PDF letters/e-sign, reconciliation | Fund transfer (workflow real, execution manual) | Demo payment/disbursement paths (correctly gated) | Bank penny-drop/IFSC verification, NACH/eNACH/UPI-autodebit, foreclosure/penalty/bounce-charge handlers | **Workflow layer real, money-movement layer manual/absent** |
| 6. Collections & Recovery | DPD-bucket case workflow, promise-to-pay (payment-verified only), write-off maker-checker, legacy recovery-case assignment | Auto DPD-triggered recovery assignment (manual-only today) | **Employee app Collections screen — fully fake data, not connected to backend** | Automatic DPD/NPA-triggered case creation, SMS/WhatsApp reminders, confirmed write-off completeness | **Backend real; one major frontend screen disconnected from it** |

*(Implemented/Partial/Mock/Missing columns are qualitative summaries per the audit findings below, not counts — a strict numeric tally was not requested and would misrepresent the qualitative nature of these findings.)*

---

## 4. KYC Audit

| Feature | Status | Evidence |
|---|---|---|
| Mobile/OTP | LIVE | `internal/otp/service.go` — crypto/rand generation, SHA-256 hashed at rest, 5-min expiry, 5-attempt cap, constant-time compare |
| PAN verification | **LIVE** | `internal/surepass/client.go:43-99`, `internal/surepass/pan.go:11-30` — real signed HTTP POST to Surepass `/api/v1/pan/pan-comprehensive`, distinguishes provider outage from business rejection. Handler: `internal/profile/pan_handler.go:148-195` (name-match check, `NAME_MISMATCH` status). Storage: `models.CustomerPAN` — AES-encrypted PAN, SHA-256 hash for dedup. `DEMO_MODE` auto-verifies (clearly separated MOCK branch). |
| Aadhaar verification | **PLACEHOLDER/MISSING (prod)** | `internal/verification/aadhaar.go:8-53` — doc comment: configured Surepass token returns 401 "not valid for this API/Scope" against `/api/v1/aadhaar-validation/aadhaar-validation`. Every non-demo submission → `VerificationPending`, provider `"MANUAL_REVIEW"`. `DEMO_MODE` auto-verifies (MOCK). Handler: `internal/profile/aadhaar_handler.go` (consent-gated, SHA-256 hashed, masked storage). |
| Aadhaar/PAN OCR | **MISSING** | No OCR package found anywhere. PAN/Aadhaar numbers are manually typed by the customer and only cross-checked against Surepass's returned *name* field — not against an OCR'd document image. |
| Face verification/selfie/liveness | **PLACEHOLDER** | `internal/verification/selfie.go:20-74` — real format/size validation (decodes base64, checks JPEG/PNG ≥5KB), but **no biometric liveness or face-match vendor configured**. Non-demo → `VerificationPending`/`MANUAL_REVIEW`. Storage note: `models.CustomerSelfie.ImagePath` stores raw base64 in the DB column directly, inconsistent with the S3/encrypted-storage pattern used for other documents. |
| DigiLocker | **MISSING** | No integration found anywhere in `internal/` or `lib/`. |
| Address/DOB verification | **MISSING** | Only a presence check (`internal/kyc/service.go:56-57` — "has profile + has address"), no independent verification. |
| Document authenticity checks | **MISSING** | No forgery/tamper-detection logic found. |
| KYC status/state machine | **LIVE — well-built** | `internal/kyc/service.go` — `Recompute()` (47-135) derives NOT_STARTED → INITIATED → DOCUMENTS_PENDING → VERIFICATION_PENDING → REQUIRES_REVIEW/REJECTED → VERIFIED. **No automated path can set VERIFIED** — requires an admin-approved review AND all providers verified (line 93), genuinely enforced. |
| KYC rejection reason / resubmission | LIVE (rejection) / **gap flagged** (resubmission) | `CustomerKYCReview.RejectionReason` propagates to `CustomerKYC.FailureReason`. Resubmission: handlers reject duplicate submission with 409 "already submitted" (`pan_handler.go:92-97`, `aadhaar_handler.go:98-103`) with **no visible delete/reset path found** — needs follow-up verification against `internal/admin/kyc_review_handler.go`. |
| KYC audit trail | **LIVE** | `audit.RecordDetailed` called on every recompute/approve/reject (`internal/kyc/service.go:128-132`). |
| Admin KYC review | **LIVE** | `internal/admin/kyc_review_handler.go`, plus newer state-machine endpoints (`internal/kyc` package: queue/detail/approve/reject/require-review). |
| Agency/field verification for KYC specifically | **MISSING** | No evidence of a field-agent KYC verification workflow (distinct from the employee-app "verification" role which handles loan-visit verification, not identity KYC). |

**Integration summary — KYC:**
| Provider | Endpoint | Backend file | Env vars | DB storage | Frontend | Status |
|---|---|---|---|---|---|---|
| Surepass (PAN) | `/api/v1/pan/pan-comprehensive` | `internal/surepass/pan.go`, `internal/profile/pan_handler.go` | `SUREPASS_BASE_URL`, `SUREPASS_BEARER_TOKEN` | `CustomerPAN` | `lib/features/` PAN screens | **LIVE** |
| Surepass (Aadhaar) | `/api/v1/aadhaar-validation/aadhaar-validation` | `internal/verification/aadhaar.go` | same as above | `CustomerAadhaar` | `lib/features/` Aadhaar screens | **BLOCKED — token lacks scope** |
| Face verification | — | `internal/verification/selfie.go` | — | `CustomerSelfie` | selfie capture screen | **PLACEHOLDER — no vendor** |
| DigiLocker | — | none | — | — | — | **MISSING** |
| OCR | — | none | — | — | — | **MISSING** |

---

## 5. Credit Bureau Audit

| Feature | Status | Evidence |
|---|---|---|
| Credit score retrieval | **MOCK (non-prod) / disabled (prod)** | `internal/bureau/service.go` — `sandboxProvider.PullScore` (lines 30-37) returns a **fixed score of 780** for every customer, explicitly labeled `"sandbox-generated, not a real bureau assessment"`. Only assigned when `!config.IsProduction()` (56-58). In production, `activeProvider` stays nil and `PullScore` returns `ErrProviderNotConfigured` (121-136) — fails closed, does not fake a score. |
| Real bureau (CIBIL/Experian/CRIF/Equifax) | **MISSING** | No API keys/env vars for any real bureau found anywhere in `internal/config/config.go`. The `Provider` interface exists and is designed for a real plug-in, but zero concrete implementations beyond the sandbox exist. |
| Consent tracking | **LIVE** | `models.BureauConsent` — required before any pull (`internal/bureau/service.go:67-97`); no expiry window enforced (documented gap, business/compliance decision). |
| Bureau report storage/audit | **LIVE** | `models.BureauRequest` — every pull recorded (pending/completed/failed/blocked) regardless of outcome; raw response never stored, only a SHA-256 hash. Admin view: `GET /admin/bureau/requests` (`bureau.AdminListHandler`, perm `BureauView`). |
| Credit history/active-loan/DPD/enquiry data from bureau | **MISSING** | `models.BureauRequest` only has `Score` + free-text `RiskIndicators` — no structured fields for external active loans, enquiries, write-offs, or utilization. Root cause: no live bureau provider to source this from. |
| CKYC/CERSAI | **MISSING (blocked by design, not faked)** | `internal/ckyc/service.go` — same seam pattern; `activeProvider` never assigned in any environment. Every `Request()` writes a real `CKYCRequest` row with status `BLOCKED` — never fakes a CKYC number. Admin: `GET /admin/ckyc`, `POST /admin/ckyc/:customerId/search`. |
| PEP/sanctions screening | **MISSING vendor, fail-safe workflow real** | `internal/screening/service.go` — with no provider configured, `Screen()` always writes `Result: REVIEW_REQUIRED` (never CLEAR); even a hypothetical real provider's `POTENTIAL_MATCH` requires human `Decide()` (gated by `ScreeningDecide` perm) before resolving. Admin: `GET/POST /admin/screening*`. |

**Integration summary — Bureau/Screening:**
| Integration | Found? | Provider | Env vars | Actually called | Status |
|---|---|---|---|---|---|
| Credit bureau | Seam only | None real (sandbox in non-prod) | None | Sandbox only outside prod | **MISSING (prod) / MOCK (non-prod)** |
| CKYC | Seam only | None | None | Always blocked | **MISSING** |
| PEP/Sanctions screening | Seam only | None | None | Always REVIEW_REQUIRED | **MISSING** |

---

## 6. Underwriting Audit

**Real, automated flow confirmed:** Application → `internal/loans/customer_handler.go` (`SubmitLoanHandler`) → `internal/loans/eligibility.go` (`evaluateEligibility()`) → human verification assignment (`internal/loans/service.go`) → maker-checker sanction (`internal/loans/maker_checker_handler.go`).

| Component | Status | Evidence |
|---|---|---|
| FOIR calculation | **LIVE** | `eligibility.go:43` (`maxFOIRPercent` = 50%) computed via real reducing-balance EMI formula (`calculateEmi()`, line 62) against `monthlyIncome`. |
| Multi-loan DTI | **PARTIAL/MISSING** | FOIR is single-loan EMI/income only — does not sum this customer's *other* active loan EMIs into the ratio. |
| Amount/tenure bounds | **LIVE** | ₹50k–₹50L, 12–60 months (`eligibility.go:34-37`). |
| CIBIL-based rate/reject | **LIVE** | 3-band system: below `CibilRejectThreshold` = hard reject; above `CibilPrimeThreshold` = min rate; middle = mid rate (`rateForCibil`, lines 84-94). Re-enforced again at maker step (`maker_checker_handler.go:81`) and at final checker approval (`maker_checker_handler.go:200-211`) — not just trusted once. |
| Income verification | **MISSING (entry only)** | `monthlyIncome` is a self-declared number (`customer_handler.go:64`) with no cross-check against payslips/bank statements/ITR. No employment-type-differentiated rules (salaried/business/daily-earner all use the same flat number). |
| Composite risk score | **MISSING** | CIBIL band is the entire risk signal — no separate numeric risk score engine. |
| Counter-offer on rejection | **LIVE** | `maxAffordablePrincipal()` (`eligibility.go:73`) computes a real, concrete affordable amount, not just a rejection message. |
| Rejection reason codes | **LIVE** | `classifyRejectionReason()` (`eligibility.go:207`) → structured codes: `CIBIL_BELOW_THRESHOLD`, `AMOUNT_OUT_OF_RANGE`, `FOIR_EXCEEDED`, `OTHER`. |
| Underwriting decision audit trail | **LIVE** | Every decision (eligibility + sanction) written to `models.CreditDecision` with `PolicyVersion="underwriting-v1"` and reason codes (`recordEligibilityDecision`/`recordSanctionDecision`). |
| Manual verification gate | **LIVE (fully manual, not automated)** | `internal/loans/service.go:76` (`submitLoanApplication`) — routes every application to a human "verification" employee (city-broadcast, first-to-claim). `autoApproveCategories` map is deliberately **empty** — an old instant-approval path was intentionally removed. |
| Maker-checker sanction hierarchy | **LIVE at the routing level, gap at the identity level (see §7 Security)** | `AdminRecommendSanctionHandler` (maker, `LOAN_REVIEW` perm) → `AdminApproveSanctionHandler` (checker, `LOAN_SANCTION` perm), row-locked via `SELECT ... FOR UPDATE`. Old single-admin sanction path explicitly disabled (410 Gone). **However: no code compares the recommending admin's ID against the approving admin's ID** — see Security Gap Report, §16, item 1. |
| Conditional approval | **MISSING** | Only ELIGIBLE/REJECTED at application stage and SANCTIONED/REJECTED at decision stage — no distinct "approved with conditions" state. |

---

## 7. Fraud & Risk Audit

| Feature | Status | Evidence |
|---|---|---|
| Duplicate mobile/PAN/Aadhaar/bank-account/device/IP at intake | **MISSING** | No dedup check found anywhere in loan submission, KYC, or registration paths. All "duplicate" grep hits are unrelated (idempotency keys for journals/webhooks/payments). |
| Repeat-application detection | **LIVE** | `internal/analytics/signals.go:127` `detectMultipleLoansShortPeriod` — SQL aggregate, ≥3 applications from one customer within 30 days → `RiskSignal`, deduped. Explicitly informational only, not an auto-reject. |
| Shared bank account across customers | **LIVE** | `signals.go:64` `detectSharedBankAccounts` — flags a disbursement account referenced by >1 customer. |
| Repeated failed payments | **LIVE** | `signals.go:98` `detectRepeatedFailedPayments` — ≥5 failed payments on one loan in 30 days. |
| Signal engine wiring | **LIVE** | `RunSignalDetection()` registered in the scheduled-jobs registry (`internal/jobs/registry.go:113`) AND exposed as an on-demand admin endpoint (`POST /admin/analytics/signals/run-detection`) — genuinely invoked, not a dead model. |
| Composite fraud/risk score | **MISSING** | No numeric fraud score exists. Design comment (`signals.go:1-12`) explicitly states no signal is ever named "fraud" and none automatically accuses a customer. |
| Internal blacklist | **MISSING** | No blacklist table/model/check found anywhere. |
| Device fingerprinting / IP velocity checks | **MISSING** | `IPAddress` fields exist on several models but are audit/consent metadata only — nothing reads them for duplicate-device or velocity detection. |
| Document/face/name/DOB/address mismatch detection | **Not confirmed present** | No mismatch-detection logic surfaced under fraud-related search terms; flagged as needing a deeper dedicated read of `internal/documents`/`internal/profile` to fully rule out. |
| PEP/sanctions screening | **PARTIAL** | Real fail-safe workflow (see §5), no actual watchlist vendor — every customer lands in manual review. |
| RiskAlert mechanism | **PARTIAL** | `internal/risk/alerts.go` `CreateAlert()` is a real writer, but is a *sink* — no code path in `internal/risk` calls it from a customer/application-level fraud condition in the files reviewed; it's invoked from accounting/NPA-side checks (e.g. duplicate-provision detection), not from fraud detection. |

---

## 8. Disbursement & Repayment Audit

| Component | Status | Evidence |
|---|---|---|
| Sanction letter / agreement generation | **LIVE** | `internal/loans/letter.go` — real PDF generation (gofpdf), branded letterhead, separate sanction/disbursement letter builders. |
| Customer e-sign | **LIVE** | `internal/loans/esign_handler.go:30-107` — customer-drawn signature PNG embedded directly into the regenerated PDF, stored as a real Document. |
| Bank account verification (penny-drop, IFSC, name-match) | **MISSING** | No penny-drop, account-holder-name verification, or IFSC validation against the customer's own bank account anywhere. Disbursement only stores an opaque, admin-supplied `BankAccountReference` string (`models/disbursement.go:40-46`) — never independently validated against a real bank. |
| Disbursement maker-checker | **LIVE (workflow)** | `AdminPrepareDisbursementHandler` (maker) → `AdminApproveDisbursementHandler` (checker, different permission). Old single-admin path disabled (410 Gone). Same identity-comparison gap as sanction maker-checker — see §16. |
| Actual fund transfer | **MOCK/MANUAL by design** | `internal/disbursement/manual_provider.go` — the only active provider is `ManualBankTransferProvider`. `CreateTransfer` makes **no external call**, just records intent (status `INITIATED`). Real transfer happens outside the codebase (NBFC ops via bank portal/NEFT/RTGS); confirmation comes via bank-statement reconciliation matching a UTR, or a `DEMO_MODE`-gated fake-UTR handler (404 in production). |
| EMI schedule | **LIVE** | `internal/loans/emi_handler.go` — full build/read, due-date tracking, overdue computed at read time. |
| EMI settlement | **LIVE, gateway-only** | `CustomerPayEmiHandler` (old, any-string-settles) is **disabled** (410 Gone). Real settlement now only via the verified Razorpay webhook. `CustomerDemoPayEmiHandler` runs the same real state machine (accounting post, receipt, NOC) but is `DEMO_MODE`-gated, 404 in production. |
| Loan restructuring | **LIVE** | `AdminRestructureLoanHandler` — real re-amortization, deletes pending installments, regenerates schedule. |
| Foreclosure / penalty / late fee / bounce charges | **MISSING (not found)** | No distinct handler code found for these in the files reviewed — flagged for follow-up. |
| Unified transaction ledger (customer-facing) | **LIVE** | `internal/loans/transactions_handler.go` — synthesized from real `LoanApplication`/`EmiInstallment` rows, explicitly replaces old client-side mock data. |
| Payment gateway (Razorpay) | **LIVE, inert without prod keys** | `internal/payments/razorpay.go` — real Orders API POST, real HMAC-SHA256 webhook signature verification with constant-time compare. Only activates if `RAZORPAY_KEY_ID`/`_KEY_SECRET`/`_WEBHOOK_SECRET` are all set; otherwise gateway endpoints return 503. |
| Webhook handling | **LIVE** | `internal/payments/webhook_handler.go` — the *only* code path that can mark an EMI paid: signature verified before parsing, idempotent via `gateway_event_id`, amount/currency cross-checked, row-locked settlement. Handles captured/authorized/failed/refunded, including cumulative over-refund detection (raises a risk alert). |
| NACH/eNACH, UPI auto-debit, Cashfree | **MISSING** | No trace of any of these in `internal/payments/`. Razorpay is the only gateway. |

---

## 9. Collections Audit

| Component | Status | Evidence |
|---|---|---|
| DPD bucketing / case lifecycle | **LIVE (backend)** | `internal/collections/service.go:19-70` — `TouchCase` maps DPD → CURRENT/REMINDER(≤7)/OVERDUE(≤30)/ASSIGNED(>30), separate from regulatory NPA classification owned by `internal/risk`. |
| Case assignment / escalation | **LIVE** | `AssignCase` (manager-only, RBAC-gated), `EscalateCase`. Officer-scoped visibility enforced *server-side* in `AdminListCasesHandler`, not just client filtering. |
| Collection actions / notes | **LIVE, deliberately no financial field** | `RecordAction` has no amount field by design — collection staff cannot touch accounting balances directly. |
| Promise-to-pay | **LIVE, correctly conservative** | Only settled by a real verified payment reaching `processPaymentSuccess` → `CheckPromiseAfterPayment` — never by time passing. `MarkBrokenPromises` (daily job) flags overdue open promises, never silently auto-resolves. |
| Automatic DPD/NPA-triggered case creation | **MISSING (manual only)** | The older, still-active `internal/recovery/handler.go` module explicitly documents (in its own code comments) that it has **no automatic DPD/NPA-threshold trigger** — only manual admin-initiated assignment exists. |
| Write-off workflow | **LIVE** | `models.WriteOffRequest` — maker-checker gated (`WriteOffRequest`/`WriteOffApprove` perms), loan/payment history never deleted. `RecoveryPayment` tracks post-write-off recovered money. |
| SMS/WhatsApp/Email reminders | **MISSING** | Only in-app notification + FCM push confirmed (`internal/push/push.go` — real Firebase Admin SDK integration, SSM-sourced credential in production, silently inert if unconfigured but not a mock). No SMS/WhatsApp/Email provider found. |
| **Architectural note** | — | Two collections systems coexist: `internal/collections` (newer, DPD-bucketed, promise-to-pay) and `internal/recovery` (older, manual case assignment). Both are live and active — this is overlap worth resolving, not a bug per se. |
| **Employee app Collections screen** | **🔴 DISCONNECTED FROM BACKEND** | `lib/employee_app/features/collections/data/repositories/collections_repository_mock.dart` — hardcoded list of 4 fake cases ("Deepak Kumar", "Anjali Desai", etc.). **This mock is the live, wired implementation**: `lib/employee_app/core/di/service_locator.dart:58` registers `CollectionsRepositoryMock` as the singleton injected into `CollectionsProvider` (`providers.dart:85`). No real HTTP-backed repository is wired for this screen. `confirmPayment()` only mutates an in-memory list — any "collection" recorded here is never persisted and never reaches the real backend. This directly contradicts the otherwise-real backend collections API and is a genuine, high-priority frontend/backend integration gap. |
| Admin panel collections dashboard | **Present** | `nbfc-admin-panel/lib/screens/recovery_screen.dart` exists, backing the `internal/recovery` module by naming (not inspected line-by-line in this pass). |

---

## 10. End-to-End Loan Journey

| Stage | Status | Notes |
|---|---|---|
| Customer Registration | ✅ IMPLEMENTED | Mobile + MPIN, real OTP flow |
| Mobile OTP | ✅ IMPLEMENTED | Hashed, rate-limited, attempt-capped |
| Profile | ✅ IMPLEMENTED | `internal/profile` |
| KYC — PAN | ✅ IMPLEMENTED | Real Surepass integration |
| KYC — Aadhaar | 🟡 MOCK (demo) / ❌ NOT IMPLEMENTED (prod) | Token lacks API scope; routes to manual review |
| KYC — Selfie/Liveness | 🟡 MOCK (demo) / ⚠️ PARTIAL (prod) | Format check only, no biometric vendor |
| Documents | ✅ IMPLEMENTED | Real encrypted storage (S3 or local AES-256-GCM) |
| Credit Bureau | 🟡 MOCK (non-prod, fixed 780) / ❌ NOT IMPLEMENTED (prod) | No real bureau credentials anywhere |
| Income Verification | ⚠️ PARTIAL | Self-declared entry only, no document cross-check |
| Fraud/Risk | ⚠️ PARTIAL | 3 real behavioral signals; no intake-time dedup/blacklist |
| Underwriting | ✅ IMPLEMENTED | Real FOIR + CIBIL-banded rule engine, automated |
| Loan Approval (maker-checker) | ⚠️ PARTIAL — **security gap** | Workflow real; maker≠checker identity check missing in code |
| Agreement/Sanction Letter | ✅ IMPLEMENTED | Real PDF + e-sign |
| Bank Verification | ❌ NOT IMPLEMENTED | No penny-drop/IFSC check |
| Disbursement (authorization) | ✅ IMPLEMENTED | Real maker-checker workflow |
| Disbursement (fund movement) | 🟡 MOCK/MANUAL | No payout API; manual bank-portal transfer |
| EMI Schedule | ✅ IMPLEMENTED | Real, accurate |
| Repayment (gateway) | ✅ IMPLEMENTED | Real Razorpay + verified webhook, inert w/o prod keys |
| Repayment (customer app UI) | ⚠️ PARTIAL | Screen-stuck bug found and fixed earlier this engagement |
| Collections — backend | ✅ IMPLEMENTED | Real DPD/PTP/write-off workflow |
| Collections — employee app UI | 🔴 BROKEN | Wired to fake hardcoded data, not the real backend |
| Closure | ⚠️ PARTIAL | NOC generation exists (`AdminSendNOCHandler`); full closure lifecycle not independently audited in this pass |

---

## 11. Database Audit

~110 GORM-migrated models in `internal/admin/migration.go`. **No models were found to be defined-but-unused** — every model traced to at least one handler/service outside its own file.

Key lending-lifecycle models:

| Model | Purpose | Key fields | Relations |
|---|---|---|---|
| `LoanApplication` | Central loan record, full lifecycle | Status (9 states), CibilScore, Recommended/Decided/DisbursementPrepared/Approved admin IDs, GST-split processing fee | CustomerID; AssignedEmployeeID; self-FK for top-ups |
| `CustomerKYC` | Authoritative KYC state machine | Status, VerificationLevel | CustomerID |
| `BureauRequest` / `BureauConsent` | Bureau pull attempt / consent | Score, ResponseHash, RiskIndicators | CustomerID |
| `CreditDecision` | Underwriting decision audit, versioned | Decision, ReasonCodes, PolicyVersion | LoanID/CustomerID |
| `Disbursement` | Money-out record, maker-checker | Status (7 states), UTR, Prepared/ApprovedBy | LoanID (unique) |
| `Payment` / `PaymentEvent` / `Refund` | Gateway payment attempt / immutable webhook log / refund | GatewayOrderID (unique), GatewayEventID (unique) | LoanID, EmiID |
| `EmiInstallment` | Scheduled installment | PaymentReference (unique — replay-proof) | LoanID |
| `CollectionCase` / `CollectionPromise` / `WriteOffRequest` | Modern collections workflow | DPD, Status, PromisedAmountPaise | LoanID |
| `RecoveryCase` / `RecoveryReport` | Legacy recovery workflow (still active) | Status | — |
| `RiskAlert` / `RiskSignal` | Ops alert / decision-support signal | AlertType, Severity, DedupeKey | LoanID |
| `AuditLog` | Append-only compliance trail (DB trigger blocks UPDATE/DELETE) | ActorType, Action, redacted before/after | ActorID |
| `AdminUser` / `AdminSession` | Individually-attributable admin login + revocable sessions | Role (11 roles), LockedUntil | — |

Full domain coverage confirmed present: users/auth/RBAC, customers/KYC, bureau/screening, loans/underwriting, risk/fraud/NPA, disbursement/payments/reconciliation, EMI/repayment, collections/recovery, notifications, audit/compliance, accounting/GST/TDS/regulatory-reporting. The accounting/tax/compliance subsystem in particular is far more built-out than the audit's original 6-module scope implied (double-entry ledger, TDS engine, regulatory-return framework, internal-audit/CAPA lifecycle) — this appears to be a mature part of the system, not a gap.

---

## 12. API Audit

Full route table read from `internal/routes/routes.go` (653 lines). Highlights by category (see agent transcripts for the complete ~150-route inventory):

- **Public/Auth**: `/auth/send-otp`, `/auth/verify-otp`, `/auth/login`, `/admin/login`, `/employee/login`, `/webhooks/razorpay` — rate-limited, no auth (as expected for entry points).
- **Customer**: KYC submission, loan application, EMI schedule/payment-order, e-sign, transactions, complaints, support — all under `CustomerAuthMiddleware`.
- **Admin — KYC/Bureau/CKYC/Screening**: full queue/detail/approve/reject/decide endpoints, all RBAC-gated.
- **Admin — Loan/Underwriting**: recommend/approve-sanction, prepare/approve-disbursement, restructure, NOC. **Two routes intentionally disabled and returning 410 Gone**: `/admin/loans/:id/disburse` (old single-admin) and `/loans/:id/emi/:installmentId/pay` (old any-string-settles) — good evidence of deliberate security hardening, not dead code left by accident.
- **Admin — Risk**: buckets, classification rules (maker-checker), provisioning rules (maker-checker), portfolio dashboard, daily job trigger, alerts, consistency-exception checks.
- **Admin — Collections/Recovery**: case list/assign/escalate/action/note/promise, write-off maker-checker, legacy recovery-report/assign endpoints.
- **Admin — Accounting/Tax/Compliance**: large, mature route groups covering ledger/journal/periods/GST/invoices, vendor/expense maker-checker, TDS full lifecycle (rule→deductee→challan→exception→filing), compliance requirements/calendar/evidence/policy/internal-audit/CAPA, regulatory-report definition→run→review→submission chain.
- **Employee**: tasks-today, verify/claim loan, document/verification-report upload, gold-appraisal/property-verification/attendance (added this engagement), change-password, recovery cases/reports.

Every admin route observed carries a `perm(rbac.X)` gate matching its action — no route was found missing a permission check in this pass (spot-checked, not exhaustively re-verified line-by-line for all ~150 routes).

---

## 13. Third-Party Integration Audit

| Integration | Found? | Provider | Code Exists? | Env Var Referenced? | Actually Called Live? | Status |
|---|---|---|---|---|---|---|
| OTP/SMS gateway | Partial | Unnamed/generic | Yes (`internal/otp/`) | **No SMS gateway env var found** | Handler wired, actual SMS-send mechanism unclear | **NEEDS VERIFICATION — likely missing a real SMS provider** |
| KYC provider | Yes | Surepass | Yes | `SUREPASS_BASE_URL`, `SUREPASS_BEARER_TOKEN` | Yes | **LIVE** |
| PAN verification | Yes | Surepass (shared) | Yes | same | Yes | **LIVE** |
| Aadhaar verification | Yes | Surepass (shared) | Yes | same | Blocked (token scope) | **BLOCKED** |
| DigiLocker | No | — | No | No | No | **MISSING** |
| OCR | No | — | No | No | No | **MISSING** |
| Face verification/liveness | Partial | None named | Yes (format check only) | — | Format check only | **PLACEHOLDER** |
| Credit bureau (CIBIL/Experian/CRIF/Equifax) | Seam only | None real | Seam yes, no real provider | None | Sandbox only, non-prod | **MISSING (prod) / MOCK (non-prod)** |
| Bank account verification/penny-drop | No | — | No | No | No | **MISSING** |
| Payment gateway | Yes | Razorpay | Yes | `RAZORPAY_KEY_ID/SECRET/WEBHOOK_SECRET` | Yes | **LIVE** |
| NACH/eNACH | No | — | No | No | No | **MISSING** |
| WhatsApp Business API | No | — | No | No | No | **MISSING** |
| Email (SMTP/SendGrid/SES) | No | — | No | No | No | **MISSING** |
| Push notification | Yes | Firebase FCM | Yes | `FIREBASE_SERVICE_ACCOUNT_JSON` (SSM in prod) | Yes | **LIVE** |
| Cloud storage | Partial | Custom S3-compatible | Yes | `DOCUMENT_STORAGE_BUCKET` | Yes if configured; falls back to local-disk AES-256-GCM (documented, not silent) | **LIVE / documented fallback** |
| PDF/document generation | Yes | In-house (gofpdf) | Yes | N/A (no external API) | Yes | **LIVE (no third party)** |

No secret values were read or reproduced in this report — only whether the corresponding env var *name* is referenced in code.

---

## 14. Admin Panel Audit

Screens confirmed present (`nbfc-admin-panel/lib/screens/`): admin_shell, admin_users, agencies_employees, audit_log, broadcast, complaints, customer_detail, customers, dashboard, loan_detail, loans, login, mis, payments, pending_approvals, reconciliation, recovery, report_preview_dialog, roles_permissions, support, tds, verification_reports.

Real UI + backend both confirmed for: customer management, KYC review, loan applications, maker-checker sanction/disbursement queue, EMI management, payment reconciliation, collections/recovery (backend real, see §9 for the employee-app-side caveat), agent/agency management, report export (CSV via `shared_widgets.dart:211`, plus PDF/Excel backend generation), audit log viewing, roles/permissions viewer.

**RBAC**: 11 fixed roles (SUPER_ADMIN, CREDIT_OFFICER, CREDIT_MANAGER, OPERATIONS, DISBURSEMENT_OFFICER, COLLECTION_OFFICER, COLLECTION_MANAGER, ACCOUNTANT, COMPLIANCE_OFFICER, AUDITOR, READ_ONLY). Fail-closed for unknown roles. 101+ explicit permission-gated routes.

**Maker-checker — see Security Gap Report below; this is the single most important admin-panel finding.**

**Audit trail**: real and structured, invoked consistently across sensitive handlers reviewed.

No dedicated standalone screen was found for risk/fraud/screening review specifically (permissions exist — `RiskView`, `ScreeningView/Decide`, `BureauView`, `CKYCView` — but no confirmed matching `.dart` screen file); likely folded into loan_detail or mis screens, not independently verified.

---

## 15. Mock/Placeholder Audit

This codebase has clearly already been through a "remove the fake stuff" pass. Most `mock`/`fake`/`hardcoded`/`TODO` grep hits are now **defensive comments explaining why something is no longer mocked**, not actual gaps. The real, functionally significant mocks are:

| Location | What's fake | Why it matters |
|---|---|---|
| `lib/employee_app/features/collections/data/repositories/collections_repository_mock.dart`, wired via `service_locator.dart:58` and `providers.dart:85` | Employee app's entire Collections tab: 4 hardcoded fake customer names/cases, `confirmPayment()` only mutates an in-memory list | **Live, reachable in production build** — field officers see fabricated data and any collection they record is silently lost |
| `internal/bureau/service.go` `sandboxProvider` | Fixed 780 credit score for every customer | Correctly gated to non-production only, with defense-in-depth re-check even if misconfigured |
| `lib/mock/mock_data.dart` (`MockData` class) | Loan product catalog (names/rates/APR/fees) shown on Explore Loans, Loan Detail, Compliance screens; branch list on Support screen | **Live, reachable in production** — if the NBFC changes actual rates, the app shows stale/wrong figures to customers until a rebuild |
| `CustomerDemoPayEmiHandler`, `AdminDemoConfirmDisbursementHandler` | Fake UTR / demo settlement | Correctly gated behind `config.DemoMode`, verified 404 in production — not a real gap |
| `NotificationsRepositoryMock` (employee app) | Fake notification list | **Dead code** — not wired into DI, unreachable |
| `AuthMockDataSource` (employee app) | Fake auth | Only reachable if `AppConfig.useMockData == true`; confirmed `false` in all environments — unreachable, harmless |

---

## 16. Security Gap Report

1. **🔴 CRITICAL — Maker≠checker is not enforced in code, contradicting doc comments claiming it is.**
   `internal/loans/maker_checker_handler.go:132-261` (`AdminApproveSanctionHandler`) and `:356-477` (`AdminApproveDisbursementHandler`) — both load the recommending/preparing admin's ID (`loan.RecommendedByAdminID` / `loan.DisbursementPreparedByAdminID`) but **never compare it against the approving admin's own ID**. A `CREDIT_MANAGER` holds both `LOAN_REVIEW` and `LOAN_SANCTION` permissions by default, so a single admin can recommend and approve the same sanction, or prepare and release the same disbursement, entirely alone. This was independently found by two separate audit passes in this session and is corroborated by this session's own earlier live testing (4 failing regression tests — `TestMakerCannotApproveOwnRecommendation`, `TestMakerCannotApproveOwnSanctionRecommendation`, `TestMakerCannotApproveOwnDisbursementPreparation`, `TestDisbursementMakerCannotApproveOwn` — all currently fail against the real test suite). **This is not a theoretical finding — it is a demonstrated, currently-failing control on the two most money-sensitive actions in the system.**

2. **Rate limiting weakens under horizontal scaling.** `internal/middleware/rate_limit.go:19-26` is an explicitly-documented in-memory, per-instance limiter ("good enough for this single-instance demo deployment"). If the app autoscales to N instances, the effective limit on login/OTP/upload endpoints multiplies by N with no shared state. Confirmed via live testing earlier this session (10 rapid-fire login attempts against a 3-instance deployment produced zero 429s).

3. **Inconsistent secrets management.** Only Firebase push credentials use AWS SSM Parameter Store (`internal/push/push.go:54-91`). DB password, JWT signing secrets, and Surepass API tokens (`internal/config/config.go:83-115`) are plain environment variables with no managed-secret-store fallback, rotation, or KMS involvement.

4. **Duplicate PAN/Aadhaar application prevention not confirmed present.** No dedup check found in `internal/loans/customer_handler.go`'s application-creation path in the time available — flagged as **likely missing but not exhaustively confirmed**; needs a targeted follow-up read plus a check for a DB-level unique constraint.

5. **Document retrieval access-control scope not fully verified.** Storage encryption itself is real (AES-256-GCM local / S3 in production, refuses unencrypted fallback if keys are missing — `internal/storage/provider.go:7-38`), but which admin roles can retrieve *decrypted* Aadhaar/PAN documents was not independently confirmed in this pass. **Flagged for follow-up, not confirmed as a flaw.**

6. **No dedicated device fingerprinting, velocity checks, or intake-time duplicate-identity detection** (see §7) — this is a fraud-prevention gap more than a pure security bug, but is listed here because it affects the same risk surface as account-takeover/synthetic-identity fraud.

**Not gaps (verified as solid):**
- JWT: HS256-pinned, fail-fast on missing secret, 15-min access token + revocable session, 7-day refresh TTL.
- OTP: SHA-256 hashed, crypto/rand, 5-min expiry, 5-attempt cap, constant-time compare.
- Webhook signature verification: real HMAC-SHA256, constant-time compare, verified before parsing, idempotent, amount/currency cross-checked.
- Admin privilege escalation: role changes require `AdminManage`/`RoleManage`, held only by `SUPER_ADMIN`; a guard prevents demoting the last active `SUPER_ADMIN`.

**TECHNICAL GAP — REQUIRES LEGAL/COMPLIANCE REVIEW**: bureau-consent expiry window is not enforced (a business/compliance decision, not a code defect); RBI/NBFC data-retention periods for OTP/audit/KYC documents, CKYC/CERSAI integration completeness, and PEP/sanctions screening data-source adequacy are policy questions the code cannot answer on its own — the *mechanics* (permissions, fail-safe review routing) exist, but whether they meet actual regulatory thresholds requires a compliance officer's review, not a code read.

---

## 17. Missing Components

- Real credit bureau integration (CIBIL/Experian/CRIF/Equifax)
- Real Aadhaar verification (current token lacks required Surepass API scope)
- OCR pipeline for Aadhaar/PAN document images
- DigiLocker integration
- Biometric face-match/liveness vendor
- Bank account penny-drop / IFSC / account-holder-name verification
- Real fund-transfer/payout API (current disbursement is manual bank-portal only)
- NACH/eNACH/UPI auto-debit
- SMS gateway (OTP delivery mechanism unconfirmed) / WhatsApp Business API / Email provider
- Duplicate PAN/Aadhaar/mobile/device/IP detection at application intake
- Internal blacklist
- Composite fraud/risk score, device fingerprinting, velocity checks
- Automatic DPD/NPA-threshold-triggered collections case creation (currently manual-only)
- Foreclosure/penalty/late-fee/bounce-charge EMI handlers
- Employee app Collections screen → real backend wiring (currently fake data)
- Maker≠checker identity enforcement in code (currently permission-only, not identity-compared)

---

## 18. Recommended Implementation Sequence

**Phase 1 — Foundation (do first, blocks nothing else but blocks trust in everything else)**
Fix maker≠checker identity enforcement (§16.1). This is a code-only fix (add an ID comparison + 403), no external dependency, and it's the highest-severity finding in the whole audit.

**Phase 2 — Frontend/Backend Reconciliation**
Wire the employee app's Collections screen to the real backend collections API (already exists, just not connected). No new backend work — this is a DI/repository swap plus verifying the real HTTP repository's contract matches what the screen expects.

**Phase 3 — KYC Completion** *(depends on: none, but blocks real underwriting accuracy)*
Resolve the Surepass Aadhaar API scope issue (likely a support ticket with Surepass, not code). Select and integrate an OCR provider. Select and integrate a liveness/face-match vendor. Decide on DigiLocker (regulatory nice-to-have vs requirement).

**Phase 4 — Credit Bureau** *(depends on: Phase 3 KYC completion, since bureau consent/identity should be solid first)*
Select and integrate a real bureau (CIBIL is the most common for NBFC personal/MSME lending in India). Wire bureau-sourced active-loan/DPD/enquiry data into the `BureauRequest` model, which currently only stores a bare score.

**Phase 5 — Underwriting Enhancement** *(depends on: Phase 4, needs real bureau data to be meaningful)*
Add multi-loan DTI (sum this customer's other active EMIs). Add income-verification document cross-check. Add employment-type-differentiated eligibility rules. Add a composite risk score beyond CIBIL banding.

**Phase 6 — Fraud & Risk** *(depends on: Phase 3 KYC data being reliable — dedup needs real Aadhaar/PAN)*
Add duplicate-identity detection at intake (PAN/Aadhaar/mobile/bank-account/device). Add an internal blacklist. Add device fingerprinting and velocity checks. Consider a PEP/sanctions vendor.

**Phase 7 — Disbursement & Repayment Completion** *(depends on: none, can run parallel to Phases 3-6)*
Integrate a bank penny-drop/IFSC verification provider. Integrate a real payout/fund-transfer API to replace manual bank-portal disbursement. Add foreclosure/penalty/late-fee/bounce-charge EMI logic. Consider NACH/eNACH for auto-debit EMI collection.

**Phase 8 — Collections Automation** *(depends on: Phase 2 frontend fix, and benefits from Phase 7's payment automation)*
Add automatic DPD/NPA-threshold-triggered case creation (currently manual-only). Integrate SMS/WhatsApp/Email reminder channels. Reconcile the two parallel collections systems (`internal/collections` vs `internal/recovery`) into one.

**Phase 9 — Hardening & Reporting**
Move remaining secrets (DB, JWT, Surepass) to a managed secret store with rotation. Address the rate-limiter's per-instance weakness (shared store, e.g. Redis, if the app will autoscale). Confirm document-retrieval RBAC scope. Confirm duplicate-application DB constraint.

Dependency logic: Phases 1-2 are pure code fixes with zero external dependency and should not wait on anything. Phase 3 (KYC) gates Phase 4 (bureau needs reliable identity), which gates Phase 5 (underwriting needs real bureau data to improve meaningfully). Phase 6 (fraud) benefits from Phase 3's identity data being real. Phase 7 (disbursement/repayment) and Phase 8 (collections) are largely independent of the KYC/bureau chain and can run in parallel. Phase 9 is cross-cutting hardening, best done continuously but especially before any production scale-up.

---

## 19. Current Completion Matrix

| Area | Status |
|---|---|
| Customer registration/auth | ✅ |
| OTP | ✅ |
| PAN verification | ✅ |
| Aadhaar verification | ❌ (prod) / 🟡 (demo) |
| Face/liveness verification | ⚠️ (format only) |
| OCR | ❌ |
| DigiLocker | ❌ |
| KYC state machine | ✅ |
| Credit bureau (real) | ❌ |
| Credit bureau (sandbox) | 🟡 |
| CKYC | ❌ |
| PEP/sanctions screening | ⚠️ (workflow only) |
| Underwriting engine (FOIR/CIBIL) | ✅ |
| Multi-loan DTI | ❌ |
| Income verification | ❌ (entry only) |
| Composite risk score | ❌ |
| Maker-checker workflow | ✅ |
| Maker-checker identity enforcement | 🔴 (broken) |
| Fraud behavioral signals | ✅ (3 patterns) |
| Duplicate-identity detection | ❌ |
| Blacklist | ❌ |
| Sanction letter/e-sign | ✅ |
| Bank account verification | ❌ |
| Disbursement authorization | ✅ |
| Disbursement fund movement | 🟡 (manual) |
| EMI schedule | ✅ |
| EMI settlement (gateway) | ✅ |
| Foreclosure/penalty logic | ❌ |
| Razorpay integration | ✅ (inert w/o keys) |
| NACH/eNACH | ❌ |
| Collections backend | ✅ |
| Collections employee-app UI | 🔴 (fake data) |
| Auto DPD-triggered recovery | ❌ |
| Write-off workflow | ✅ |
| SMS/WhatsApp reminders | ❌ |
| Push notifications | ✅ (inert w/o creds) |
| Admin panel core screens | ✅ |
| RBAC | ✅ |
| Audit trail | ✅ |
| Accounting/GST/TDS subsystem | ✅ (more mature than scoped) |

---

## 20. Exact Files Reference

Grouped by module for a downstream developer's convenience:

**KYC**: `internal/profile/pan_handler.go`, `internal/profile/aadhaar_handler.go`, `internal/verification/aadhaar.go`, `internal/verification/selfie.go`, `internal/surepass/client.go`, `internal/surepass/pan.go`, `internal/kyc/service.go`, `internal/admin/kyc_review_handler.go`, `internal/models/kyc.go`, `internal/models/customer_kyc_review.go`

**Bureau/Screening**: `internal/bureau/service.go`, `internal/bureau/sandbox_provider.go`, `internal/ckyc/service.go`, `internal/screening/service.go`, `internal/models/bureau.go`, `internal/models/ckyc.go`, `internal/models/screening.go`

**Underwriting**: `internal/loans/eligibility.go`, `internal/loans/customer_handler.go`, `internal/loans/service.go`, `internal/loans/maker_checker_handler.go`, `internal/loans/admin_handler.go`, `internal/models/loan.go`

**Fraud/Risk**: `internal/analytics/signals.go`, `internal/risk/classification.go`, `internal/risk/alerts.go`, `internal/jobs/registry.go`, `internal/models/risk.go`, `internal/models/analytics.go`

**Disbursement/Repayment**: `internal/loans/letter.go`, `internal/loans/esign_handler.go`, `internal/disbursement/manual_provider.go`, `internal/disbursement/provider.go`, `internal/loans/emi_handler.go`, `internal/loans/transactions_handler.go`, `internal/payments/razorpay.go`, `internal/payments/webhook_handler.go`, `internal/payments/provider.go`, `internal/reconciliation/`, `internal/models/disbursement.go`, `internal/models/payment.go`, `internal/models/emi.go`

**Collections**: `internal/collections/service.go`, `internal/collections/handler.go`, `internal/recovery/handler.go`, `internal/models/collections.go`, `internal/models/recovery.go`, `lib/employee_app/features/collections/data/repositories/collections_repository_mock.dart`, `lib/employee_app/core/di/service_locator.dart`, `lib/employee_app/core/di/providers.dart`

**Notifications**: `internal/push/push.go`, `internal/notifications/handler.go`

**Admin/RBAC/Audit**: `internal/rbac/permissions.go`, `internal/admin/admin_handler.go`, `internal/admin/adminuser_handler.go`, `internal/middleware/admin_auth_middleware.go`, `internal/audit/detailed.go`, `internal/routes/routes.go`

**Mocks to address**: `lib/employee_app/features/collections/data/repositories/collections_repository_mock.dart`, `lib/mock/mock_data.dart`

---

# WHAT CLAUDE HAS ALREADY DONE

*(This section summarizes what this audit found to already be implemented in the project — it does not describe work done by any Claude session during this specific audit, which was strictly read-only.)*

### Already Implemented
- Customer/employee/admin JWT authentication with distinct middleware per domain
- OTP generation/verification with proper hashing, expiry, and attempt limits
- PAN verification via a real, live Surepass integration
- KYC state machine with a genuinely enforced no-automated-VERIFIED guarantee
- Rule-based underwriting engine: FOIR calculation, CIBIL banding, amount/tenure bounds, structured rejection reasons
- Maker-checker workflow (routing/permission level) for loan sanction and disbursement
- EMI scheduling, gateway-only settlement, restructuring
- Real Razorpay integration with signature-verified webhooks and idempotent settlement
- PDF sanction/disbursement letter generation with customer e-sign embedding
- Collections backend: DPD-bucketed cases, promise-to-pay (payment-verified only), write-off maker-checker
- A real behavioral fraud-signal engine (3 specific patterns), scheduled + on-demand
- Encrypted document/PII storage (AES-256-GCM local, S3 in production)
- Real Firebase push notification integration
- A mature double-entry accounting, GST, TDS, and regulatory-reporting subsystem (beyond original scope)
- Comprehensive, append-only audit logging with a DB-level tamper trigger
- RBAC with 11 roles and fail-closed permission checks across ~150+ routes

### Partially Implemented
- Face/selfie verification (real format validation, no biometric vendor)
- PEP/sanctions screening (real fail-safe workflow, no watchlist vendor)
- Collections auto-assignment (manual-only, no DPD/NPA auto-trigger)
- Underwriting income assessment (self-declared entry, no verification/DTI)
- KYC resubmission path (unclear/possibly missing)

### Mock / Placeholder
- Credit bureau: sandbox returns a fixed 780 score, correctly non-production-only
- Employee app Collections screen: fully fake hardcoded data, live and reachable
- Customer app loan-product catalog (`lib/mock/mock_data.dart`): static, live and reachable
- Demo payment/disbursement paths: correctly `DEMO_MODE`-gated, 404 in production

### Broken
- Maker≠checker identity enforcement: the ID comparison is simply absent from the code, contradicting doc comments and confirmed by 4 currently-failing regression tests
- Employee app Collections screen: not a partial gap, actively disconnected from the real backend it appears to represent

### Completely Missing
- Real Aadhaar verification (blocked by a Surepass token scope issue)
- OCR for Aadhaar/PAN documents
- DigiLocker
- Real credit bureau (CIBIL/Experian/CRIF/Equifax)
- CKYC/CERSAI (intentionally blocked, no provider)
- Bank account penny-drop/IFSC verification
- Real fund-transfer/payout API
- NACH/eNACH/UPI auto-debit
- SMS/WhatsApp/Email reminder channels
- Duplicate-identity detection, internal blacklist, device fingerprinting, velocity checks
- Composite fraud/risk score
- Foreclosure/penalty/late-fee/bounce-charge EMI handlers

### External Integration Required
- A credit bureau (CIBIL is the standard choice for Indian NBFC lending)
- An OCR provider (for Aadhaar/PAN document extraction)
- A biometric liveness/face-match vendor
- A bank-account verification/penny-drop provider
- A real payout/fund-transfer API
- An SMS gateway, WhatsApp Business API, and/or email provider
- Optionally: DigiLocker, a PEP/sanctions watchlist vendor

### Requires Business/Compliance Decision
- Whether DigiLocker is a regulatory requirement or a nice-to-have for this NBFC's KYC process
- Bureau-consent expiry window policy
- Data retention periods for OTP/audit/KYC documents against RBI NBFC norms
- Whether the current PEP/sanctions fail-safe-to-manual-review approach is acceptable long-term or requires a real watchlist vendor
- Reconciling the two parallel collections systems (`internal/collections` vs `internal/recovery`) into one canonical workflow

---

# WHAT NEEDS TO BE DONE NEXT

| # | Task | Why Required | Dependencies | Affected Files/Modules | Backend Work | Frontend Work | DB Work | External API | Testing Required |
|---|---|---|---|---|---|---|---|---|---|
| 1 | Enforce maker≠checker by ID comparison | Currently a single admin can self-approve sanctions/disbursements — highest-severity finding | None | `internal/loans/maker_checker_handler.go` | Add ID comparison + 403 in both approve handlers | None | None | None | Re-run the 4 existing failing regression tests until they pass |
| 2 | Wire employee app Collections screen to real backend | Field officers currently see fake data; real collections are silently lost | None | `collections_repository_mock.dart`, `service_locator.dart`, `providers.dart` | Confirm/extend real HTTP repository if contract mismatch | Swap DI registration to real repository | None (backend API exists) | None | Manual QA on-device: real case list, real payment confirmation persists |
| 3 | Resolve Surepass Aadhaar API scope | Aadhaar verification currently always fails/routes to manual review | None | `internal/verification/aadhaar.go` | None (support/contract issue with Surepass) | None | None | Surepass (scope upgrade) | Re-test a real Aadhaar submission once resolved |
| 4 | Integrate OCR provider | No automated cross-check between entered PAN/Aadhaar numbers and actual document images | Ideally after #3 | New `internal/ocr/` package, `pan_handler.go`, `aadhaar_handler.go` | New provider integration | Document capture flow may need adjustment | New fields for OCR-extracted values | An OCR vendor | Test against real and forged sample documents |
| 5 | Integrate real credit bureau | Underwriting currently has zero real external credit signal | Ideally after #3, #4 | `internal/bureau/`, `internal/loans/eligibility.go` | New provider implementing `bureau.Provider` | Minimal (bureau score already flows into eligibility UI) | `models.BureauRequest` may need structured fields for active-loan/DPD/enquiry data | CIBIL (or equivalent) | Test consent flow, real pull, and downstream eligibility impact |
| 6 | Add multi-loan DTI to underwriting | Current FOIR is single-loan only | #5 for maximum value, but can be done with internal-loan data alone first | `internal/loans/eligibility.go` | Sum customer's other active loan EMIs into FOIR | None | Query across `LoanApplication` for same customer | None | Unit tests for FOIR calc with multiple concurrent loans |
| 7 | Add duplicate-identity detection at application intake | No dedup on PAN/Aadhaar/mobile/bank-account/device today | #3/#4 for reliable identity data | `internal/loans/customer_handler.go`, new fraud package | New dedup check before application creation | Show a clear rejection/flag message | Possibly a unique index or lookup table | None | Test with duplicate submissions across accounts |
| 8 | Integrate bank penny-drop/IFSC verification | Disbursement bank details are never validated against a real bank today | None | `internal/loans/maker_checker_handler.go`, new verification package | New provider integration | Show verification status before disbursement prep | New fields on `Disbursement`/`BankAccount` model | A penny-drop/bank-verification vendor | Test with valid/invalid account numbers |
| 9 | Integrate real fund-transfer/payout API | Disbursement is currently manual (bank-portal), not automated | #8 ideally first | `internal/disbursement/` (new provider alongside `manual_provider.go`) | New `disbursement.Provider` implementation | Minimal | Possibly new transfer-status fields | A payout API vendor | Test end-to-end disbursement with sandbox payout credentials |
| 10 | Add automatic DPD/NPA-triggered collections case creation | Currently fully manual assignment only | None | `internal/collections/service.go` or new scheduled job | New scheduled job checking DPD thresholds | None | None | None | Verify cases auto-create at correct DPD thresholds without duplicates |
| 11 | Add SMS/WhatsApp/Email reminder channels | Only in-app + push notifications exist today | None | `internal/notifications/`, new provider packages | New provider integrations | None (backend-driven) | Possibly a channel-preference field | SMS gateway / WhatsApp Business API / email provider | Test delivery across all three channels |
| 12 | Move remaining secrets to a managed secret store | DB/JWT/Surepass secrets are plain env vars; only Firebase uses SSM today | None | `internal/config/config.go` | Extend the SSM pattern already used for Firebase to other secrets | None | None | AWS SSM Parameter Store (already in use) | Verify no plaintext secrets remain in EB environment config after migration |
| 13 | Address per-instance rate limiting under autoscaling | Effective rate limits multiply by instance count today | None | `internal/middleware/rate_limit.go` | Move to a shared store (e.g. Redis) if the app will run multi-instance | None | None | Redis (or similar) | Load-test with multiple instances to confirm shared limiting works |

*(No code has been implemented as part of this audit. All items above are the recommended backlog derived strictly from the audit findings.)*

---

# FINAL STATUS

- **Current loan lifecycle**: Real end-to-end path exists from application through underwriting, sanction, disbursement authorization, EMI, and gateway-verified repayment. Two breaks: bank-account verification is entirely missing, and actual fund transfer is manual, not API-driven.
- **KYC**: PAN is live; Aadhaar, OCR, face-liveness, DigiLocker, and address/DOB verification are not. The state-machine and audit-trail plumbing around KYC is solid and genuinely enforced.
- **Credit Bureau**: No real bureau integrated. Sandbox exists and is correctly disabled in production. CKYC and PEP/sanctions screening are honestly blocked (fail-safe), not faked.
- **Underwriting**: A real, automated FOIR + CIBIL-banded rule engine exists and is genuinely wired into the application flow, with a full decision audit trail. Gaps: no multi-loan DTI, no income verification, no composite risk score.
- **Fraud/Risk**: A real 3-pattern behavioral signal engine exists and runs on a schedule. No intake-time duplicate-identity detection, blacklist, or fraud score exists.
- **Disbursement**: Maker-checker authorization workflow is real. Actual money movement is manual (bank portal), not automated via any payout API.
- **Repayment**: Real, live Razorpay integration with properly verified webhooks — genuinely production-capable once real keys are configured. EMI scheduling and settlement logic is solid.
- **Collections**: Backend is real and well-designed (DPD buckets, promise-to-pay, write-off maker-checker). The employee app's Collections screen is disconnected from it entirely, running on fake data.
- **Admin**: Comprehensive screen coverage, real RBAC, real audit logging. The maker-checker approval control it displays is not actually enforced at the identity level in the backend it calls.
- **Database**: ~110 models, all traced to active use, spanning a notably mature accounting/compliance subsystem beyond the original 6-module scope.
- **External integrations**: 2 genuinely live (Surepass PAN, Razorpay), 1 live-but-inert-without-credentials in this environment pattern (Firebase push), and roughly 10 entirely missing (bureau, Aadhaar, OCR, DigiLocker, penny-drop, payout API, NACH, SMS, WhatsApp, email).
- **Major blockers**: (1) maker-checker identity enforcement is broken in code, undermining the two most money-sensitive controls in the system; (2) the employee-app Collections screen is silently disconnected from the real backend.
- **Immediate next technical step**: Fix the maker≠checker ID comparison in `internal/loans/maker_checker_handler.go` (§16.1, Backlog item #1) — it is the smallest, fastest, highest-severity fix available, requires no external dependency, and has 4 existing regression tests ready to confirm the fix.
