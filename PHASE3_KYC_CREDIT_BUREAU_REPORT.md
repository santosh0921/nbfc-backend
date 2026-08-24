# Phase 3 — Customer Onboarding, KYC, Credit Bureau & KYC-Completeness Foundation

This report documents Phase 3 implementation, continuing from Phase 1 (admin
identity/RBAC/maker-checker/audit, see `PHASE1_SECURITY_IMPLEMENTATION_REPORT.md`)
and Phase 2 (payments/disbursement/reconciliation, see
`PHASE2_PAYMENT_DISBURSEMENT_REPORT.md`).

**This report describes technical implementation only. It does not, and
cannot, certify RBI compliance, legal sufficiency of any declaration
wording, or CKYC/CERSAI/bureau operational readiness — see §19.**

---

## 1. Files Changed

**New packages**
- `internal/kyc/` — `service.go`, `handler.go` — KYC state machine
- `internal/bureau/` — `service.go`, `sandbox_provider.go`, `handler.go` — credit bureau abstraction
- `internal/ckyc/` — `service.go`, `handler.go` — CKYC/CERSAI abstraction
- `internal/screening/` — `service.go`, `handler.go` — PEP/sanctions screening
- `internal/storage/` — `provider.go`, `local_encrypted.go` — encrypted document storage

**New models**
- `internal/models/kyc.go`, `bureau.go`, `ckyc.go`, `screening.go`, `declaration.go`

**New tests**
- `internal/kyc/kyc_security_test.go` — 7 Phase 3 tests

**Modified**
- `internal/config/config.go` — environment matrix, `ValidateProductionSafety()`
- `cmd/api/main.go` — environment/production-safety wiring, `storage`/`bureau`/`ckyc`/`screening` `Initialize()` calls
- `internal/admin/migration.go` — 8 new tables registered
- `internal/routes/routes.go` — new customer/admin KYC, bureau, CKYC, screening routes
- `internal/rbac/permissions.go` — 4 new permissions (`BUREAU_VIEW`, `CKYC_VIEW`, `SCREENING_VIEW`, `SCREENING_DECIDE`), role grants updated
- `internal/models/document.go` — storage-reference/hash/status fields added
- `internal/documents/handler.go` — encrypted-storage read/write path (API response shape unchanged)
- `internal/loans/customer_handler.go` — KYC gate on `SubmitLoanHandler`/`TopUpLoanHandler`, bureau-backed CIBIL
- `internal/loans/eligibility.go` — `PolicyVersion`, `recordEligibilityDecision`, `recordSanctionDecision`, `classifyRejectionReason`
- `internal/loans/reference.go` — `getOrCreateCibilScore` now routes through `internal/bureau`
- `internal/loans/maker_checker_handler.go` — sanction-time `CreditDecision` recording
- `.env` — added `DOCUMENT_ENCRYPTION_KEY`

**Explicitly NOT changed** (see §7): `internal/profile/pan_handler.go`, `internal/verification/aadhaar.go`, `internal/profile/kyc_status_handler.go` (legacy percentage-based status endpoint, left untouched for the shipped Flutter app's existing contract).

---

## 2. Database Migrations

All new tables are registered in `internal/admin/migration.go`'s `AutoMigrate` call (GORM auto-migration, run on every backend startup — no separate migration-runner step exists in this codebase, consistent with Phase 1/2). New tables:

`customer_kycs`, `bureau_consents`, `bureau_requests`, `credit_decisions`, `ckyc_requests`, `screening_requests`, `declarations`, `customer_declaration_responses`.

`documents` table gained columns (not a new table): `storage_reference`, `hash`, `size_bytes`, `status`, `retention_expiry`, `created_by`, `verified_at`. `data_base64` remains for legacy rows only (`json:"-"`, dev-fallback only).

---

## 3. New Tables / Models

| Table | Purpose |
|---|---|
| `customer_kycs` | One row per customer — the state-machine-backed KYC record (§5) |
| `bureau_consents` | Consent record required before any bureau pull |
| `bureau_requests` | One row per bureau pull attempt; score + hashed response, never the raw report |
| `credit_decisions` | Every eligibility/sanction decision — policy version, inputs, decision, reason codes, reviewer |
| `ckyc_requests` | CKYC/CERSAI interaction log (search/retrieve/create/update) |
| `screening_requests` | PEP/sanctions screening result + compliance decision |
| `declarations` | Configurable declaration catalog (e.g. FATCA/CRS) — **placeholder wording, not legal-approved** |
| `customer_declaration_responses` | Per-customer, per-version declaration responses |

---

## 4. New APIs

**Customer-facing**
- `GET /auth/kyc/v2/status` — state-machine status (additive; existing `GET /auth/kyc/status` unchanged)

**Admin**
- `GET /admin/kyc/queue`, `GET /admin/kyc/:customerId`, `POST /admin/kyc/:customerId/approve|reject|require-review`
- `GET /admin/bureau/requests`
- `GET /admin/ckyc`, `POST /admin/ckyc/:customerId/search`
- `GET /admin/screening`, `POST /admin/screening/:customerId/run`, `POST /admin/screening/:requestId/decide`

Every admin route above is gated by `middleware.RequirePermission(...)` — see §13.

---

## 5. KYC State Machine

`NOT_STARTED → INITIATED → DOCUMENTS_PENDING → VERIFICATION_PENDING → VERIFIED → REJECTED → EXPIRED/REQUIRES_REVIEW`

Implemented in `internal/kyc/service.go`. `Recompute(customerID, actorType, actorRef)` is the **only** writer of `CustomerKYC.Status` — it derives status fresh, on every call, from the underlying `CustomerProfile`/`CustomerAddress`/`CustomerPAN`/`CustomerAadhaar`/`CustomerSelfie`/`CustomerKYCReview` rows. `VERIFIED` is reachable **only** when `CustomerKYCReview.Status == "APPROVED"` (an admin action) **and** all provider-verified fields are set — the customer frontend has no path to set this status directly; there is no customer-writable field on `CustomerKYC` at all.

`IsSubmissionEligible(status)` gates loan submission — see §12.

---

## 6. Provider Integrations Completed

- **Document storage encryption** (`internal/storage`) — AES-256-GCM, separate key (`DOCUMENT_ENCRYPTION_KEY`) from PAN encryption, local-encrypted-filesystem provider active whenever the key is configured (now set in `.env`). Falls back to legacy plaintext-DB storage only when unconfigured, and that fallback is refused in production (`ValidateProductionSafety`).
- **Sandbox credit bureau provider** — `internal/bureau/sandbox_provider.go`, the same deterministic-per-customer generator previously inline in `internal/loans`, now reachable only through the consent-gated `bureau.PullScore`, and explicitly blocked from ever activating when `config.IsProduction()`.
- **DEMO_MODE production bypass (PAN/Aadhaar)** — solved at the config layer, not by touching the verification handlers. `ValidateProductionSafety()` fatally refuses to start if `Environment=production` and `DemoMode=true`. See §7 for why `pan_handler.go`/`aadhaar.go` needed no code changes.

---

## 7. Provider Integrations Still Awaiting Credentials

| Item | Status | What's needed |
|---|---|---|
| PAN verification (Surepass) | **Already real**, gated by DEMO_MODE (now provably impossible in production — see §6) | Nothing further required; already implemented pre-Phase-3 |
| Aadhaar verification | **Blocked** — the configured Surepass token 401s on the Aadhaar-validation scope (pre-existing, documented in `internal/verification/aadhaar.go`); non-demo calls return `PENDING`/`MANUAL_REVIEW` honestly, never a fake pass | A Surepass (or alternate UIDAI-authorized) credential with Aadhaar-validation scope |
| Real credit bureau (CIBIL/Experian/CRIF/Equifax) | **Architecture only** — `internal/bureau.Provider` interface exists; no concrete implementation | Bureau membership, API credentials, integration spec |
| CKYC/CERSAI | **Architecture only** — `internal/ckyc.Provider` interface exists; every request resolves `BLOCKED` | CERSAI registered-entity certification + onboarding (this cannot be self-provisioned in code) |
| PEP/sanctions screening | **Architecture only** — `internal/screening.Provider` interface exists; every screen defaults to `REVIEW_REQUIRED` (fail-safe) | A watchlist/screening vendor |
| Bureau reporting (loan performance → bureau) | **Not started** — see §9 | Provider file/API format spec |

---

## 8. Credit Bureau Implementation Status

- **Fake CIBIL removed from the real decision path.** `deterministicCibil`/`getOrCreateCibilScore(user) int` (no error return) is gone. The FNV-1a generator survives only as `bureau.sandboxProvider`, explicitly labeled `SANDBOX`, gated to `!config.IsProduction()`, and reachable only through `bureau.PullScore`'s consent-and-audit path.
- **Consent enforced.** `bureau.PullScore` refuses (`ErrConsentRequired`) without a matching `BureauConsent` row. Outside production, `getOrCreateCibilScore` auto-records an explicitly-labeled implicit consent (`v0-implicit-preconsent-flow`) because the shipped Flutter app has no consent-capture screen yet (§18 gap). **In production this auto-consent does not happen** — no consent flow exists yet, so bureau pulls in production fail closed with `ErrConsentRequired`.
- **Bureau data separated from underwriting rules from the credit decision** (item H). `evaluateEligibility` (unchanged thresholds) consumes the score; `recordEligibilityDecision`/`recordSanctionDecision` (new) write a `CreditDecision` row with `PolicyVersion="underwriting-v1"`, JSON inputs (score band, FOIR, tenure/amount — never raw report content), decision, reason codes, and reviewer (`"system"` for apply-time, the approving admin's username for sanction-time).
- **Response minimization.** `BureauRequest` stores `Score`, a short `RiskIndicators` string, and a SHA-256 `ResponseHash` — never the raw provider payload.
- Every caller (`SubmitLoanHandler`, `TopUpLoanHandler`, `CustomerCreditScoreHandler`) now handles `(score, error)` and returns `503` on failure rather than ever falling back to a fabricated score.

---

## 9. CKYC/CERSAI Status

Architecture-only, per instruction — **not production-ready**. `internal/ckyc.Request(customerID, requestType)` always writes a real `CKYCRequest` row; with no provider configured (true in every environment today), status resolves to `BLOCKED` with a clear `FailureReason`, never a fake reference number or fake `SUCCESS`. Admin can trigger `POST /admin/ckyc/:customerId/search` and see the `BLOCKED` result recorded and audited.

---

## 10. Screening Status

Architecture-only. `internal/screening.Screen` defaults to `REVIEW_REQUIRED` (never `CLEAR`) whenever no provider is configured — the fail-safe default required by item K. A `POTENTIAL_MATCH` from a real (future) provider is routed to the same `REVIEW_REQUIRED` compliance queue rather than ever auto-resolving. Resolution (`Decision: CLEARED|REJECTED`) is only possible via `POST /admin/screening/:requestId/decide`, gated to the new `rbac.ScreeningDecide` permission — held only by `SUPER_ADMIN` and `COMPLIANCE_OFFICER`. `CREDIT_OFFICER`/`CREDIT_MANAGER` have `ScreeningView` (read-only) but not `ScreeningDecide` — verified by `TestScreeningDefaultsToReviewRequiredAndOnlyComplianceCanDecide`.

---

## 11. Document-Storage Security Changes

- New `internal/storage.Provider` interface; `local_encrypted.go` implements AES-256-GCM at rest with a key **separate** from `PAN_ENCRYPTION_KEY` (`DOCUMENT_ENCRYPTION_KEY`, now set in `.env`), stored under `DOCUMENT_STORAGE_PATH` (default `./storage/documents`), filenames are random UUIDs (no customer/document ID leakage into the filesystem), and `safePathFor()` rejects any non-UUID reference (path-traversal guard).
- `Document` now carries `StorageReference`, `Hash` (SHA-256 of plaintext, integrity check), `SizeBytes`, `Status`. Every upload computes and stores the hash.
- The primary DB no longer holds raw document bytes for new uploads when a storage provider is active — `DataBase64` is legacy-only (`json:"-"`) and used only as a dev-mode fallback when `DOCUMENT_ENCRYPTION_KEY` is unset, which `ValidateProductionSafety` refuses to allow in production.
- **API response shape is unchanged** (`dataBase64` field still present, computed on read via `hydrateDocument`) — deliberate, to avoid any Flutter app changes.
- Document access continues to use the pre-existing Phase 2 `document.download` audit; `document.upload` audit added as new.
- **Not yet implemented** (flagged for §18): malware scanning, signed/temporary download URLs (current downloads are same-origin authenticated JSON, not pre-signed URLs), explicit file-type/size validation beyond what existed before.

---

## 12. Loan-Submission KYC Gate

`checkKYCEligibility` (`internal/loans/customer_handler.go`) calls `kyc.Recompute` then `kyc.IsSubmissionEligible`, called first thing in both `SubmitLoanHandler` and `TopUpLoanHandler` — before any other logic, including the existing financial eligibility check. Backend-authoritative: the Flutter app performs no equivalent check today, and even if it did, this server-side gate cannot be bypassed by the client.

**Design decision requiring compliance sign-off:** the gate allows submission at `VERIFICATION_PENDING`, not only at fully-provider-`VERIFIED` — because Aadhaar has no live verifier in this environment (§7) and would otherwise deadlock every application indefinitely. `REQUIRES_REVIEW` is blocked (a provider mismatch/failure state, treated conservatively). This is a real business/compliance judgment call the code cannot make unilaterally — see §19.

Verified by `TestLoanSubmissionBlockedWithoutKYC` (403 for a customer with zero KYC data on file).

---

## 13. RBAC Changes

4 new permissions added to `internal/rbac/permissions.go`: `BUREAU_VIEW`, `CKYC_VIEW`, `SCREENING_VIEW`, `SCREENING_DECIDE`.

| Role | Bureau | CKYC | Screening view | Screening decide |
|---|---|---|---|---|
| SUPER_ADMIN | ✅ | ✅ | ✅ | ✅ |
| CREDIT_OFFICER | ✅ | – | ✅ | – |
| CREDIT_MANAGER | ✅ | – | ✅ | – |
| COMPLIANCE_OFFICER | ✅ | ✅ | ✅ | ✅ |
| AUDITOR | ✅ (read-only role) | ✅ | ✅ | – |
| Others (OPERATIONS, DISBURSEMENT_OFFICER, COLLECTION_OFFICER, ACCOUNTANT, READ_ONLY) | – | – | – | – |

Existing `KYC_VIEW`/`KYC_APPROVE` permissions (Phase 1) are reused unchanged for the new `/admin/kyc/*` routes — no new KYC-specific permission was needed. `SCREENING_DECIDE` is deliberately its own permission, not folded into `KYC_APPROVE` or any existing one, so an ordinary loan/credit officer role can never be granted screening-override capability by accident.

---

## 14. Audit Events

New/extended audit actions recorded via `audit.RecordDetailed`:
`document.upload`, `kyc.require_review`, `kyc.decision.APPROVED`, `kyc.decision.REJECTED`, `bureau.consent`, `bureau.request`, `bureau.response`, `ckyc.request`, `ckyc.response`, `screening.request`, `screening.decision`.

None of these ever include raw Aadhaar numbers, OTPs, passwords, API keys, or full document/report bodies — `audit.Snapshot()` (Phase 1 redaction) is used for Before/After payloads, and bureau/CKYC/screening entries carry only IDs, statuses, and short reason strings by construction (the raw provider payloads are never even loaded into the structs that get audited).

---

## 15. Automated Test Results

`internal/kyc/kyc_security_test.go` — **7/7 passing** against the real Postgres dev DB (no mocks), run via `go test ./internal/kyc/...`:

1. `TestLoanSubmissionBlockedWithoutKYC` — item 8 ✅
2. `TestUnauthorizedAdminCannotViewOrApproveKYC` — items 4/5 ✅
3. `TestProductionSafetyRefusesDemoMode` — item 3 (see note below) ✅
4. `TestBureauPullRequiresConsent` — item 10 ✅
5. `TestBureauScoreOnlyViaConsentedPull` — item 11 ✅
6. `TestScreeningDefaultsToReviewRequiredAndOnlyComplianceCanDecide` — items 14/15 ✅
7. `TestLoanSubmissionAllowedAtVerificationPending` — item 9 ✅

**Note on item 3:** `ValidateProductionSafety()` calls `log.Fatal` (`os.Exit(1)`) by design — this cannot be safely invoked inside a `go test` process without killing the test binary. The test instead exercises the exact boolean precondition the real check enforces; the actual `os.Exit` path is covered by manual verification (§16).

Full suite (`go test ./...`) result: all packages pass, including Phase 1's `internal/admin` and Phase 2's `internal/loans`/`internal/payments`/`internal/reconciliation` — **no regressions**.

**Not yet automated** (§18 gap): items 1, 2, 6, 7, 12, 13, 16 from the spec's 16-scenario list — PAN verification success/failure (already covered pre-Phase-3 by the real Surepass sandbox, not re-tested here), invalid-document rejection, document-access authorization, bureau webhook/callback replay (no bureau webhook exists yet — nothing to replay), CKYC failure handling (covered implicitly — every CKYC request resolves BLOCKED and is handled, but no explicit test asserts this), and a dedicated "audit event exists" assertion (audit calls are exercised by every other test but not independently asserted against the `audit_logs` table).

---

## 16. Manual Test Procedure

```
go run ./cmd/api
```

1. **DEMO_MODE + production refusal**: set `APP_ENV=production` with `DEMO_MODE=true` still set — confirm the process exits immediately with the printed problem list, never reaching `router.Run`.
2. **KYC gate**: log in as a fresh customer with no profile/PAN/Aadhaar on file, `POST /auth/loans/apply` — expect `403` with `kycStatus: "NOT_STARTED"`.
3. **Admin KYC queue**: log in as an admin with `KYC_VIEW`, `GET /admin/kyc/queue` — confirm it lists only `VERIFICATION_PENDING`/`REQUIRES_REVIEW` customers by default.
4. **Document round-trip**: upload a document as a customer, confirm the DB row has a `storage_reference` (not `data_base64`) and a `hash`; fetch it back via the existing list endpoint and confirm `dataBase64` reconstitutes correctly.
5. **Bureau consent**: attempt a bureau pull with no consent row — expect `ErrConsentRequired`; record consent, retry — expect a completed `BureauRequest` with a score.
6. **Screening override**: as `CREDIT_OFFICER`, attempt `POST /admin/screening/:id/decide` — expect `403`; repeat as `COMPLIANCE_OFFICER` — expect `200`.

---

## 17. Required Environment Variables

| Variable | Required for | Status in this environment |
|---|---|---|
| `APP_ENV` | production-safety gating | unset → defaults to `development` |
| `DEMO_MODE` | PAN/Aadhaar sandbox bypass | `true` (dev only — refused in production) |
| `PAN_ENCRYPTION_KEY` | PAN encryption at rest | set |
| `DOCUMENT_ENCRYPTION_KEY` | KYC document encryption at rest | **newly added this phase** |
| `SUREPASS_BEARER_TOKEN` | PAN verification (and Aadhaar, once scoped) | set (PAN scope only) |
| *(none yet)* | Real bureau (CIBIL/Experian/CRIF/Equifax) credentials | not configured — architecture-only |
| *(none yet)* | CKYC/CERSAI registered-entity credentials | not configured — architecture-only |
| *(none yet)* | Screening vendor credentials | not configured — architecture-only |

---

## 18. Remaining P0/P1 Gaps

**P0**
- No customer-facing bureau-consent capture screen in the Flutter app — production bureau pulls will fail closed until this exists.
- No real Aadhaar, bureau, CKYC, or screening provider credentials — all four are architecture-only.
- Document downloads are authenticated JSON responses, not signed/temporary URLs; no malware scanning.

**P1**
- Bureau consent has no expiry enforcement (`latestConsent` takes the most recent row regardless of age).
- `Declaration`/`CustomerDeclarationResponse` models exist but no seed data, no customer-facing capture UI, and no admin management screen yet — wording is explicitly unapproved placeholder text that was never written (no rows seeded), so nothing customer-facing exists to show unapproved wording, but the feature itself is incomplete.
- Bureau reporting (loan performance → bureau, item I) has no code at all yet.
- Admin panel UI screens (item M: KYC queue/details, Document Verification, Credit Bureau Requests/Results, CKYC Status, Screening Results, KYC Exceptions) are not built — only the backend APIs exist.
- Rate limiting/anti-enumeration on the new `/admin/kyc/:customerId`-style routes relies only on RBAC + the existing global patterns, not bespoke additions.
- Idempotency keys/correlation IDs are not yet threaded through the new bureau/CKYC/screening request paths (item O).

---

## 19. Items Requiring CA/CS/Compliance/Legal/Provider Validation

- **The KYC-completeness gate's threshold** (submission allowed at `VERIFICATION_PENDING`, not full `VERIFIED`) is a business/compliance decision made here for practical necessity (no live Aadhaar verifier) — a compliance officer must confirm this is acceptable or specify a stricter gate.
- **Declaration wording** — `Declaration.Body`/`Title` are placeholders and must not be shown to a real customer without legal sign-off; no declaration rows have been seeded specifically because of this.
- **Bureau consent version/purpose text and retention window** — `ConsentVersion: "v0-implicit-preconsent-flow"` is a development-only stopgap, not compliance-approved consent language.
- **Bureau selection** — which of CIBIL/Experian/CRIF/Equifax to actually integrate is a business decision; the architecture supports any/all but none is chosen.
- **CKYC/CERSAI onboarding** — requires the entity's own CERSAI registration, which is outside this codebase's scope entirely.
- **Screening vendor selection and match-adjudication SOP** — who counts as a "compliance officer" for `SCREENING_DECIDE` purposes, and what the actual review procedure is, needs a written compliance policy this code only enforces the access-control shape of.
- **Document retention** — `RetentionExpiry` field exists on `Document` but no retention policy value/enforcement job has been implemented; needs a legally-defined retention period.

**This implementation does not make the NBFC "RBI compliant."** It builds the technical scaffolding (state machine, provider abstractions, consent/audit trails, access control) that a compliant process would run on top of — actual compliance requires the credentialed provider integrations, approved legal text, and documented SOPs listed above.
