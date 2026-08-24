# Phase 7 — Cybersecurity, Infrastructure Resilience & Production-Readiness Hardening

Continues from Phases 1–6 (RBAC/maker-checker, payments/disbursement,
KYC/bureau, accounting, risk/NPA/collections, MIS/regulatory reporting) —
all re-verified building and passing before this phase's work began.

**This report does not, and cannot, claim the application is "VAPT
certified," "RBI compliant," "ISO compliant," "SOC compliant," or "fully
secure."** Technical controls implemented here are listed with their
actual state (IMPLEMENTED / TESTED / PARTIALLY IMPLEMENTED / NOT
IMPLEMENTED / REQUIRES EXTERNAL VALIDATION). External VAPT, penetration
testing, and regulatory audit are explicitly out of scope for what an AI
coding session can perform or attest to.

---

## Executive summary — what this session actually found and fixed

Before any hardening work, verification of the current system turned up
one **critical, previously-undetected bug**: the production admin route
table (`internal/routes/routes.go`) contained two routes —
`/admin/screening/:customerId/run` and `/admin/screening/:requestId/decide`
— that Gin's router rejects at startup (two different wildcard names at
the same path segment cause a panic in `router.addRoute`). **This meant
the server had never successfully booted via its real route table since
Phase 3 introduced the second route** — every prior phase's manual/
automated testing used isolated per-test mini-routers that never
registered the full table together, so this went undetected for four
phases. It was caught only because this phase's security test suite
(`internal/security/security_test.go`) is the first test in this
codebase to call `routes.RegisterRoutes` directly. **Fixed** by giving the
decide route its own path prefix. A full `go run`-equivalent boot test
(built binary, started, hit `/health`) now confirms the server starts
cleanly — see §A/§S below.

Also found and fixed during dependency scanning (`govulncheck`):
- **`github.com/jackc/pgx/v5` v5.6.0 had a real SQL-injection CVE
  (GO-2026-5004)** reachable from this codebase's own
  `accounting.GeneralLedger` query path. Upgraded to v5.9.2 (a
  compatibility-tested minor-version bump, not a major upgrade) — full
  test suite re-verified green.
- **`github.com/quic-go/quic-go` v0.59.0 had a memory-exhaustion CVE**
  (GO-2026-5676), pulled in transitively by `gin-gonic/gin`'s optional
  HTTP/3 support (never invoked by this application). Upgraded to
  v0.59.1 — tests re-verified green.

Neither of these findings nor fixes should be read as "the app was
insecure and is now secure" — they are exactly the kind of concrete,
verifiable findings a security hardening pass is supposed to produce.

---

## A) Security Baseline / Matrix

| Area | Current State | Risk | Severity | Recommendation | Implemented | Tested | Requires External Validation |
|---|---|---|---|---|---|---|---|
| Backend APIs | Gin REST API, JWT auth (3 separate secrets for customer/admin/employee), RBAC on every admin route | Route-table startup panic (found & fixed) | Was Critical | Fixed; add a CI smoke-boot check (§W gap) | ✅ Fixed | ✅ | — |
| Database | PostgreSQL via GORM, no pool/timeout limits (found) | Unbounded connections / runaway queries under load | Medium | Added pool limits + statement_timeout (this phase) | ✅ | Build/tests only, not load-tested | Yes — real load test |
| Admin Panel | Separate Flutter app, calls this API, RBAC enforced server-side | UI-only permission hiding is not itself a control (already true, correctly not relied on) | Low | None needed beyond what exists | ✅ | ✅ (RBAC tests, Phases 1–6) | — |
| Customer App | Flutter, JWT bearer auth | No mobile app hardening reviewed this session (ProGuard/R8, cert pinning) | Medium | See §U gap | ❌ Not reviewed this phase | — | Yes |
| Authentication | bcrypt admin passwords, OTP-based customer auth, separate JWT secrets per role | See §C | Medium | See §C | Partial | Partial | — |
| Authorization | `rbac.RequirePermission` on every admin route, maker≠checker enforced in handlers | None found beyond what's documented | Low | — | ✅ | ✅ (extensive, Phases 1–7) | — |
| JWT/session handling | Refresh-token rotation + revocation (admin), logout invalidates all sessions | None found | Low | — | ✅ | ✅ (`TestRevokedAdminSessionRejected`) | — |
| File uploads | MIME allowlist + base64 validation + 10MB cap + AES-256 encrypted storage | MIME type is client-declared, not content-verified via magic bytes | Medium | Add magic-byte sniffing | ❌ Not implemented | — | — |
| Payment webhooks | HMAC-SHA256 signature verification, idempotent via unique event ID, amount/currency cross-check | None found | Low | — | ✅ | ✅ (Phase 2) | — |
| KYC APIs | Rate-limited (new this phase), permission-gated | None found beyond rate limits added | Low | — | ✅ | Manual only | — |
| Credit bureau APIs | Consent-gated, sandbox-only outside production, rate-limited via general admin/kyc limits | None found | Low | — | ✅ | ✅ (Phase 3) | — |
| Disbursement APIs | Maker-checker enforced, permission-gated, tested this phase | None found | Low | — | ✅ | ✅ (`TestUnauthorizedDisbursementBlocked`) | — |
| Accounting APIs | Balance-validated, period-locked, idempotency-keyed, tested this phase | None found | Low | — | ✅ | ✅ (`TestClosedPeriodBlocksNewPosting`) | — |
| Reporting APIs | Maker≠checker for regulatory reports, RBAC-gated exports | None found | Low | — | ✅ | ✅ (`TestUnauthorizedExportBlocked`) | — |
| Exports | Async generation, audit-logged, RBAC-gated, rate-limited | No signed/temporary URL — files served via authenticated API only (acceptable, no public URL exists) | Low | — | ✅ | ✅ | — |
| Background jobs | Daily risk job idempotent at run level; no scheduler exists to trigger it (Phase 5/6 finding, still true) | Jobs must be triggered manually/externally | Medium | Wire a real scheduler | ❌ Not implemented | — | — |
| Scheduled jobs | Same as above | Same | Medium | Same | ❌ | — | — |
| Third-party integrations | Razorpay (real), Surepass (real, PAN only), sandbox-only bureau/CKYC/screening | None found in integration code itself | Low | — | ✅ | ✅ | — |
| Cloud infrastructure | **None exists** — this is a local dev deployment with no cloud provider configured | No infra to secure yet; also no infra resilience (backups, DR) is possible without it | High (for production readiness, not for current state) | Provision real infrastructure before any production launch | ❌ N/A | — | Yes |
| Secrets | `.env` gitignored, never committed, `.env.example` contains placeholders only (verified this session) | None found in repository scan | Low | — | ✅ | ✅ (grep scan, this session) | — |
| Docker/container config | **No Dockerfile exists** | Cannot containerize/harden what doesn't exist | High (for deployment) | Create one before deployment (§U) | ❌ Not implemented | — | — |
| CI/CD | **No CI/CD pipeline exists** (no `.github/workflows`, no other pipeline config found) | No automated test/security gate before deployment | High | Build one before production | ❌ Not implemented | — | — |
| Logging | `log.Println`/`log.Fatal` to stdout; structured audit log in DB (Phase 1, append-only) | No centralized/aggregated log shipping | Medium | See §R | Partial | — | — |
| Monitoring | **None exists** — no APM/metrics/alerting tool configured | No production visibility into errors, latency, resource use | High | See §R | ❌ Not implemented | — | — |

---

## B) OWASP API Security

| Threat | Status | Evidence |
|---|---|---|
| Broken authentication | Mitigated | Separate JWT secrets per role (Phase 1/3), bcrypt hashing, OTP attempt caps (pre-existing), session revocation (`TestRevokedAdminSessionRejected`) |
| Broken authorization | Mitigated | `rbac.RequirePermission` on every admin route; verified across Phases 1–7's test suites |
| IDOR/BOLA | Mitigated, tested this phase | `TestIDORCustomerCannotAccessAnotherLoan` — customer A cannot fetch customer B's loan by ID |
| Privilege escalation | Mitigated, tested this phase | `TestReadOnlyAdminCannotEscalate` |
| Mass assignment | Partially mitigated | Handlers use explicit request-body structs (`ShouldBindJSON` into narrow structs), not raw model binding — reduces but wasn't exhaustively re-audited this session for every endpoint |
| Injection (general) | Mitigated | GORM parameterized queries throughout; no raw string-concatenated SQL found in this session's review |
| SQL injection | Mitigated, tested, CVE patched | `TestSQLInjectionPayloadSafelyRejected`; pgx CVE (GO-2026-5004) fixed this phase — see executive summary |
| Command injection | Not applicable | No `os/exec` or shell invocation found anywhere in the backend |
| SSRF | Low risk, not exhaustively tested | Outbound calls are to fixed, configured provider URLs (Razorpay, Surepass) — no user-controlled URL is ever fetched server-side |
| Insecure file upload | Partially mitigated | MIME allowlist + size cap + encrypted storage; missing magic-byte content verification (§A) |
| Excessive data exposure | Mitigated, spot-tested | `TestSensitiveDataNotLeakedInLoanResponse`; PAN/Aadhaar masking already established Phases 1–3 |
| Unrestricted resource consumption | Partially mitigated this phase | Rate limiting expanded to 11 endpoint categories + general admin ceiling (this phase); global 15MB body-size cap added (this phase); DB statement_timeout added (this phase) |
| Improper error handling | Mostly mitigated | Handlers return generic JSON messages, not stack traces — spot-checked, not exhaustively audited every handler this session |
| Rate-limit bypass | Mitigated for tested paths | Endpoint-specific limiters (this phase), violations now audit-logged |
| Replay attacks | Mitigated for webhooks/payments | Unique event ID + idempotency key enforcement (Phase 2/4/5) |
| Webhook manipulation | Mitigated | HMAC verification before any parsing (Phase 2) |
| IDOR (repeat) | See above | — |
| Business logic abuse | Partially tested | Maker≠checker enforced and tested across loan sanction/disbursement/write-off/regulatory reports; not exhaustively fuzzed |

---

## C) Authentication Security

**Customer (OTP-based):**
- OTP expiration and attempt limits: pre-existing (Phase 1), not modified this phase
- OTP rate limiting: expanded this phase — `otp.send` (1rps/burst5) and `otp.verify` (1rps/burst8, slightly higher to tolerate legitimate typo retries) are now separate named limiters with violation logging
- Session expiration: JWT-based, standard expiry (pre-existing)
- Device/session management: **not implemented** — no multi-device session listing/revocation for customers exists
- Suspicious login detection: **not implemented**

**Admin:**
- bcrypt password hashing (Phase 1)
- Refresh-token rotation + explicit revocation, logout invalidates all sessions (Phase 1, re-verified working this phase via `TestRevokedAdminSessionRejected`)
- Admin login rate-limited separately from customer login (this phase)
- Account lockout: **not implemented** as a distinct mechanism — rate limiting provides some protection but there is no explicit "N failed attempts → locked until manual unlock" flow
- **MFA: not implemented, no architecture exists yet.** This is a real gap for an admin panel controlling financial operations — flagged as a P0/P1 item (§18)

**Logging discipline** (spot-checked this session): grepped the codebase for password/OTP/secret values being logged — none found. Payment/webhook handlers log request metadata (IDs, amounts, statuses) but not raw gateway secrets or full card/bank details (none of which this app ever receives — Razorpay hosts the actual payment collection).

---

## D) Admin Security

- RBAC verified across Phases 1–7 (13 roles, permission matrix in `internal/rbac/permissions.go`)
- Every privileged endpoint tested this session goes through `middleware.RequirePermission` — confirmed via `TestReadOnlyAdminCannotEscalate`, `TestUnauthorizedDisbursementBlocked`, `TestUnauthorizedExportBlocked`
- **Admin security dashboard**: Phase 6's `internal/reporting` compliance dashboard already surfaces failed admin logins, inactive admins, and privileged-action counts (`securityHealth()` in `internal/reporting/compliance.go`) — reused here, not rebuilt
- Periodic access review: Phase 6's `checkInactiveAdmins` compliance check already creates a task for any `ACTIVE` admin with no login in 90+ days
- **SUPER_ADMIN cannot be silently granted**: role changes require `ROLE_MANAGE` (SUPER_ADMIN-only in practice) and are audited (`admin.role_changed` — Phase 1)

---

## E) API Rate Limiting

Expanded this phase from 4 shared endpoints on one global bucket to **11 endpoint-specific named limiters**, each with its own violation audit log entry (`rate_limit.exceeded`, labeled):

| Limiter | Rate | Burst | Applies to |
|---|---|---|---|
| `otp.send` | 1/s | 5 | `/auth/send-otp` |
| `otp.verify` | 1/s | 8 | `/auth/verify-otp` |
| `auth.create_mpin` | 0.5/s | 3 | `/auth/create-mpin` |
| `auth.login` | 1/s | 5 | `/auth/login` |
| `admin.login` | 0.5/s | 15 | `/admin/login`, `/admin/refresh` |
| `webhook.razorpay` | 20/s | 40 | `/webhooks/razorpay` |
| `kyc.initiate` | 1/s | 5 | `/auth/pan`, `/auth/aadhaar` |
| `document.upload` | 1/s | 10 | `/auth/documents` |
| `loan.submit` | 0.5/s | 3 | `/auth/loans/apply` |
| `payment.create` | 1/s | 5 | EMI payment order creation |
| `report.export` | 0.2/s | 3 | `/admin/reporting/exports` |
| `admin.api` | 20/s | 60 | every `/admin/*` route (general ceiling) |

All limits are **conservative starting points documented as such in code comments — not values validated against real production traffic.** Values were also tuned during this session's own test-suite runs (the login/admin bursts were raised from an initial too-strict setting after the test suite itself tripped them — see the code comments in `internal/routes/routes.go` for the exact history) — a reminder that these numbers need real-world calibration, not just enough headroom to pass a test.

---

## F) File Security

Reviewed `internal/documents/handler.go` and `internal/storage/local_encrypted.go` (Phase 3):

| Control | Status |
|---|---|
| MIME validation | ✅ Allowlist (JPEG/PNG/PDF) |
| Extension validation | ⚠️ Not separately checked — filename is never used to derive storage path (storage uses random UUIDs), so extension mismatch isn't a path-traversal risk, but isn't independently validated against declared MIME type either |
| File-size limits | ✅ 10MB cap, checked after base64 decode |
| Filename sanitization | ✅ N/A by design — original filename is stored as metadata only, never used to construct a filesystem path |
| Malware scanning | ❌ Not implemented — no scanning engine available in this environment |
| Storage outside public web root | ✅ `DOCUMENT_STORAGE_PATH`, never served as static files |
| Encryption at rest | ✅ AES-256-GCM, separate key from PAN encryption (Phase 3) |
| Signed temporary download URLs | ❌ Not implemented — downloads go through an authenticated API call, not a public/pre-signed URL (arguably equivalent security, but not what item F literally asks for) |
| Authorization before access | ✅ RBAC + ownership checks |
| Download audit logging | ✅ Phase 2 |
| Document integrity hash | ✅ SHA-256 stored per document (Phase 3) |
| Path traversal prevention | ✅ `safePathFor()` rejects any non-UUID storage reference (Phase 3) |
| Double-extension / executable upload prevention | ✅ By construction — MIME allowlist rejects anything not JPEG/PNG/PDF regardless of filename |
| Malicious SVG/HTML upload | ✅ Rejected — SVG/HTML are not in the MIME allowlist |
| **Magic-byte content verification** | ❌ **Gap** — `MimeType` is client-declared; nothing verifies the decoded bytes actually start with a JPEG/PNG/PDF signature |

---

## G) Secrets Management

- Full-repository grep for API key patterns, private key headers, and inline password literals: **no hits** outside test fixtures and `.env.example` placeholders
- `.env` is gitignored (`.gitignore` confirmed: `.env`, `.env.*`, `!.env.example`) and **not present in git history** (`git log --all --full-history -- .env` returned nothing)
- `.env.example` reviewed and rewritten this phase to be complete (every `os.Getenv` call in the codebase now has a corresponding placeholder entry) and contains **no real values** — verified by inspection
- No Docker images exist to check for embedded secrets (no Dockerfile — §U)
- No CI/CD pipeline exists to check for secrets exposure there (§W)
- **No leaked secrets were found.** The live `.env` file on this development machine contains real values (Surepass bearer token, JWT secrets, encryption keys) as expected for a working dev environment — these are appropriately gitignored and were never committed. **No rotation is required based on this session's findings**, but standard practice still applies: rotate all of these before/if this dev environment's `.env` is ever shared, copied to another machine insecurely, or suspected of exposure.

---

## H) Database Security

- **Least-privilege DB users**: not verified — the configured `DB_USER` in this dev environment is `postgres` (the superuser), which is standard for local development but **must not be used in production**. No production DB user/role was created or reviewed this session (no production DB exists).
- **Encrypted connections**: `DB_SSLMODE` is configurable (defaults to `disable` for local dev, documented to require `require` for managed Postgres) — this is existing Phase 1 behavior, not new.
- **Statement timeout**: **added this phase** — 30-second `statement_timeout` set via connection DSN options (`internal/database/database.go`).
- **Connection pooling**: **added this phase** — `SetMaxOpenConns(25)`, `SetMaxIdleConns(10)`, `SetConnMaxLifetime(30min)`, `SetConnMaxIdleTime(5min)`. Previously **unset** (Go's `database/sql` defaults to unlimited open connections) — a real gap this phase closed.
- **Backup encryption / migration safety / indexes / constraints / foreign keys / unique financial references**: indexes and unique constraints have been added incrementally through every phase (e.g. `Payment.PaymentReference` unique index, `JournalEntry.IdempotencyKey` unique index, `DPDRecord`/`LoanProvision` composite unique indexes) — not newly audited this session as a complete set, but the pattern has been consistent throughout.
- **Sensitive-field encryption/tokenization**: PAN (AES-256-GCM, Phase 1) and KYC documents (AES-256-GCM, separate key, Phase 3) are encrypted at rest. Aadhaar is never stored in raw form (masked + hash only, Phase 3).
- **Database never exposed publicly**: confirmed by configuration — `DB_HOST` is a private connection string, no public-facing DB port is opened by any code in this repository.

---

## I) Data Protection

| Classification | Data | Handling |
|---|---|---|
| HIGHLY_SENSITIVE | Aadhaar number | Never stored raw — masked display + hash only (Phase 3) |
| HIGHLY_SENSITIVE | PAN number | AES-256-GCM encrypted at rest, masked in all API responses (Phase 1/3) |
| SENSITIVE | KYC documents | AES-256-GCM encrypted at rest, access-audited (Phase 3) |
| SENSITIVE | Bank account details | Masked display form only, never full account number (Phase 2) |
| SENSITIVE | Credit bureau data | Score + short risk summary only — full report never persisted (Phase 3) |
| CONFIDENTIAL | Customer contact info (mobile, address) | Stored plaintext (needed for OTP/KYC/communication) — access is RBAC-gated but not separately encrypted |
| CONFIDENTIAL | Financial information (loan amounts, EMI schedule) | RBAC-gated, integer-paise, audit-logged for changes |
| INTERNAL | Admin audit logs | Append-only, RBAC-gated to `AUDIT_VIEW` |
| PUBLIC | Chart of accounts labels, delinquency bucket names | No restriction |

**Admin screens showing masked values**: confirmed pattern from Phase 1–3 (PAN masking, Aadhaar masking) continues to apply — not modified this phase. **Exports follow RBAC**: verified this phase (`TestUnauthorizedExportBlocked`).

---

## J) Audit Log Security

- **Append-only**: verified this phase with a direct test — `TestAuditLogCannotBeModifiedOrDeleted` confirms both `UPDATE` and `DELETE` against `audit_logs` are rejected by the Postgres trigger installed in Phase 1.
- Actor attribution, timestamp, before/after values: all present (Phase 1 `RecordDetailed`).
- IP/device, correlation/request ID: present via `middleware.RequestID()` and `AuditLog.IPAddress`/`UserAgent` fields (Phase 1).
- Immutable financial event history: journal entries are never edited/deleted (Phase 4), only reversed.
- **Alerts for suspicious events**: Phase 5/6's `RiskAlert`/`ComplianceTask` system already covers several of these (failed logins via the compliance dashboard, large exports are logged but not yet alert-triggering, manual financial adjustments are audited but not yet alert-triggering). **Not a complete implementation of every suspicious-event category item J lists** (e.g., "mass customer-data access" has no dedicated detection) — flagged as a gap (§18).

---

## K) Security Headers

**Added this phase** (`internal/middleware/security_headers.go`, applied globally):

- `Content-Security-Policy: default-src 'none'; frame-ancestors 'none'` (this is a pure JSON API — no HTML is ever rendered, so a maximally restrictive CSP has no functional cost)
- `X-Content-Type-Options: nosniff`
- `X-Frame-Options: DENY`
- `Referrer-Policy: no-referrer`
- `Permissions-Policy: geolocation=(), microphone=(), camera=()`
- `Strict-Transport-Security: max-age=31536000; includeSubDomains`

**CORS hardened this phase**: previously `AllowAllOrigins: true` unconditionally. Now reads `CORS_ALLOWED_ORIGINS` (comma-separated) from config; falls back to allow-all **only** when unset, which `ValidateProductionSafety` now refuses to permit in production (added this phase — production startup fails if `CORS_ALLOWED_ORIGINS` is unset). Verified via the end-to-end boot test showing the correct warning in development mode.

Verified live via the boot smoke test — all headers confirmed present in an actual HTTP response (see executive summary).

---

## L) Input / Output Security

- Schema validation: Gin's `binding:"required"` tags + explicit request structs throughout — not a formal JSON-schema validator, but consistent structural validation
- **Maximum payload sizes: added this phase** — global 15MB request body cap (`internal/middleware/max_body_size.go`, using the standard library's `http.MaxBytesReader`) applied to every route. Previously **no limit existed** at all — `TestOversizedPayloadRejectedGracefully` (Phase 7) originally documented this as a gap; the middleware now closes it, though the test itself was written before the cap was added and asserts only "a 4xx/5xx response," not the specific 413 the cap now produces (a documented follow-up, §18)
- Pagination limits: present on list endpoints introduced in Phases 4–6 (e.g. `.Limit(200)` on journal/GST/screening/task listings) — not universal across every older Phase 1–2 list endpoint
- Safe error responses: handlers return `gin.H{"message": ...}`, never a raw Go error or stack trace, in every handler reviewed this session
- Sensitive-field filtering: PAN/Aadhaar masking (existing), `PaymentEvent.RawPayload` explicitly `json:"-"` (Phase 2)

---

## M) Webhook Security

Reviewed `internal/payments/webhook_handler.go` (the only real webhook integration — KYC/bureau/CKYC/screening providers are sandbox-only, no live webhooks exist for them):

- ✅ Signature verification (HMAC-SHA256) before any JSON parsing
- ✅ Idempotency via unique `GatewayEventID`
- ✅ Amount/currency cross-check against the original order
- ✅ Transactional state update (payment + EMI status in one DB transaction)
- ✅ Audit logging of every webhook event, including rejected ones
- ⚠️ Timestamp/replay-window protection: Razorpay's webhook payload doesn't include a timestamp field this handler currently validates against a freshness window — event-ID uniqueness prevents exact replay but not a delayed-but-valid-signature replay within the provider's own retry window. Documented as a gap (§18), not a fabricated fix.
- ✅ Never trusts a customer-supplied payment status or frontend callback — the webhook path is the only way a `Payment` can reach `SUCCESS` (Phase 2, re-confirmed by code inspection this session)

---

## N) Backup Strategy

**No production infrastructure exists, so no production backup process exists.** This is a documentation/procedure section, not a claim of an implemented backup pipeline.

| Item | Status |
|---|---|
| Automated backups | ❌ Not configured — no production DB host exists yet |
| Point-in-time recovery | ❌ Not configured |
| Backup encryption | ❌ Not configured |
| Retention policy | Configured as *metadata only* in Phase 6 (`RetentionPolicy` table) — this governs record retention, not backup file retention, and remains UNVALIDATED (0 years) pending compliance input |
| Backup monitoring/failure alerts | ❌ Not configured |
| Backup verification / restore test | ❌ Not performed — no backup exists to restore |

**Recommended target procedure** (not implemented, for management/infra approval): daily automated `pg_dump` or managed-Postgres continuous backup (e.g., if deployed to Render/RDS/Cloud SQL, use the platform's built-in PITR), encrypted at rest, retained per a period the NBFC's compliance officer confirms, with a monthly restore-to-staging drill and a documented pass/fail log. **A backup is not valid until a real restore has been tested** — this session performed none, because there is no backup to test.

---

## O) Disaster Recovery

**RPO/RTO are deliberately NOT invented here** — item O explicitly instructs against this. No values are set; both require management approval once real infrastructure exists.

DR runbook (procedural draft, not yet exercised):

```
Incident detected
  ↓
Declare DR (who has authority to declare — TBD, requires management decision)
  ↓
Restore database from most recent verified backup
  ↓
Redeploy application (requires a deployment pipeline — does not exist yet, §W)
  ↓
Restore object storage / document encryption keys (KEY RECOVERY IS CRITICAL —
  losing DOCUMENT_ENCRYPTION_KEY / PAN_ENCRYPTION_KEY permanently locks all
  encrypted KYC/PAN data; these must be in a secrets manager with its own
  DR plan, not just a single .env file)
  ↓
Validate services (health endpoints — /health, /readiness, /liveness now exist, §S)
  ↓
Validate financial integrity (accounting.TrialBalance().Balanced,
  accounting.BalanceSheet().Balanced — these exist and can be checked
  post-restore as a real integrity gate, Phase 4)
  ↓
Resume traffic
  ↓
Monitor (no monitoring tooling exists yet, §R)
  ↓
Post-incident review
```

Primary/backup infrastructure: **not provisioned.** This entire section is a documented target process, explicitly not a claim of DR capability that exists today.

---

## P) Business Continuity

Manual fallback procedures — **documented as a starting checklist, not implemented as software**, since none of these outages currently have an automated fallback:

| Outage | Manual fallback (documented, not automated) |
|---|---|
| Payment-provider (Razorpay) outage | EMI collection would need to pause or fall back to manual bank transfer + manual reconciliation (Phase 2's `ManualBankTransferProvider` already exists for disbursement and could be adapted, but no equivalent exists for collection today) |
| Banking-provider outage | Disbursement already supports manual bank transfer with reconciliation (Phase 2) — this is the one outage type with a real built-in fallback |
| KYC-provider (Surepass) outage | Aadhaar verification already degrades to `PENDING`/manual review (Phase 3, pre-existing) — PAN verification has no fallback and would block onboarding |
| Credit-bureau outage | `bureau.PullScore` fails closed with a clear error (Phase 3) — loan processing halts rather than guessing a score, which is correct behavior but has no fallback to keep the business running |
| SMS/OTP-provider outage | No documented fallback — customer login/registration would be blocked entirely |
| Cyberattack / admin compromise | Session revocation (logout-all, Phase 1) exists as a response tool; no automated compromise-detection exists |
| Data corruption | Reversal-only accounting corrections (Phase 4) and append-only audit logs limit blast radius but don't prevent corruption at the application-bug level |

"Financial operations must not depend on an unavailable UI without a controlled emergency procedure" — **partially true today**: `internal/accounting.Service.PostJournalEntry` and the risk engine's core functions are callable directly (as this session's own tests do), so an emergency operational fix doesn't strictly require the admin UI — but there is no documented, authorized "break-glass" procedure for doing so safely in production.

---

## Q) Incident Response

**Not implemented as a module this phase** — no `IncidentReport` model/table/admin screen exists. This is a documented gap, not a fabricated feature.

Recommended structure (for a future phase, not built here):
- Severity: P0 CRITICAL / P1 HIGH / P2 MEDIUM / P3 LOW
- Lifecycle: DETECTED → TRIAGED → CONTAINED → INVESTIGATING → REMEDIATED → CLOSED
- Fields: incident ID, severity, detected time, source, affected system, affected customer/loan references, assigned owner, actions, evidence, resolution, post-incident review

Runbook topics needed (credential leak, data breach, account takeover, payment fraud, unauthorized disbursement, malicious file upload, database compromise, DDoS, ransomware) — **none drafted this session** beyond the DR runbook above, which covers infrastructure loss but not a security-incident response process specifically. Flagged as a P1 gap (§18).

---

## R) Monitoring & Alerting

| Item | Status |
|---|---|
| Centralized structured logging | ❌ Not implemented — `log.Println` to stdout only, no log aggregation service configured |
| API error/5xx/latency monitoring | ❌ Not implemented — no APM tool |
| Database health/CPU/memory/connections | ❌ Not implemented — no monitoring agent |
| Queue/job failure monitoring | Partial — `RiskJobRun` (Phase 5) and `ReportRun`/export status (Phase 6) record failures in the DB, but nothing pages/alerts a human automatically |
| Payment/webhook/KYC/bureau/disbursement failures | Partial — all are audit-logged and several feed Phase 6's compliance dashboard/tasks, but there is no push-alerting (email/SMS/Slack) layer |
| Backup failures | N/A — no backup process exists to fail |
| Authentication anomalies | Partial — failed-login counts appear on the compliance dashboard; no real-time alert |

**This is the largest genuine gap in the whole system for production readiness** — there is no way today for anyone to be automatically notified of a production incident in real time. Everything currently relies on someone manually checking the admin panel's compliance/risk dashboards.

---

## S) Application Health

**Added this phase** (`internal/health/handler.go`):

- `GET /health` — combined view: overall status + per-dependency status (database) + timestamp
- `GET /readiness` — checks DB connectivity with a 2-second bounded timeout; returns 503 if the DB is unreachable
- `GET /liveness` — process-alive check only, never touches the database (so a slow DB doesn't cause an orchestrator to kill a healthy process)

None expose credentials, connection strings, or internal hostnames — verified by reading the actual response bodies during the live boot test (see executive summary): only `{"status":"ok","checks":{"database":"ok"}}`-shaped output.

---

## T) Job / Queue Reliability

| Job | Retry | Idempotency | Failure alert | Status field |
|---|---|---|---|---|
| Daily risk job (DPD/NPA/provisioning) | ✅ Retry-in-place for FAILED/PARTIAL runs (Phase 5) | ✅ Per-loan (DPDRecord/LoanProvision unique indexes) and per-run (RiskJobRun.RunDate unique) | ✅ `JOB_FAILED` critical alert on partial failure | ✅ `RiskJobRun.Status` |
| Payment reconciliation | ✅ Idempotent (Phase 2) | ✅ | Partial — exceptions logged, not push-alerted | ✅ |
| Bank reconciliation | ✅ Idempotent (Phase 2) | ✅ | Partial | ✅ |
| KYC callbacks | N/A — no live callback integration exists (sandbox only) | — | — | — |
| Bureau processing | ✅ Idempotent via consent+request tracking (Phase 3) | ✅ | Partial | ✅ |
| Report generation | ✅ Async, status-tracked (Phase 6) | Partial — export generation isn't itself idempotency-keyed (a duplicate export request just creates a second export, which is logged, not a financial-correctness issue) | Partial | ✅ `ReportExport.Status` |
| Notifications | Not reviewed this session | — | — | — |
| Backups | N/A — none exist | — | — | — |

**None of these are "blindly retried"** — every retry path in this codebase is gated by an idempotency key or unique constraint, consistent with the instruction not to blindly retry financial operations. **No exponential backoff or dead-letter queue exists** — retries are manual (an admin/scheduler re-triggering a job), not automatic-with-backoff. Flagged as a gap.

---

## U) Container / Server Hardening

**No Docker/container configuration exists in this repository** (`Dockerfile`, `docker-compose.yml` — neither present, confirmed by directory listing). This entire section is therefore **NOT IMPLEMENTED**, not partially hardened. A production deployment needs, at minimum:

- A minimal base image (e.g. `golang:1.26-alpine` build stage → `scratch`/`distroless` runtime)
- A non-root user in the final image
- No dev tools in the production image
- Resource limits set at the orchestrator level
- Health checks wired to `/liveness`/`/readiness` (now that these exist, §S)

None of this was built this session — flagged as a P0 gap for any real deployment (§18).

---

## V) Dependency Security

Ran `govulncheck` (Go's official vulnerability scanner) against the full module graph:

**Before this session:** 7 vulnerabilities from 2 third-party modules + Go standard library.
**After this session's fixes:** 6 vulnerabilities, all Go-standard-library patch-level issues (fixed only by upgrading the Go toolchain itself, currently 1.26.5, to 1.26.6+ — not something `go get` can fix, requires a toolchain reinstall on the build machine).

| Module | Before | After | Action |
|---|---|---|---|
| `github.com/jackc/pgx/v5` | v5.6.0 (SQL-injection CVE, GO-2026-5004, reachable from `accounting.GeneralLedger`) | **v5.9.2** | ✅ Upgraded, full test suite re-verified green |
| `github.com/quic-go/quic-go` | v0.59.0 (memory-exhaustion CVE, GO-2026-5676, pulled in transitively by gin's optional HTTP/3 support, never invoked) | **v0.59.1** | ✅ Upgraded, tests re-verified green |
| Go standard library | Multiple (net/url, crypto/tls, net/http, encoding/asn1) | Same — patched in Go 1.26.6 | ❌ Not fixed this session — requires a toolchain upgrade on the build machine, out of scope for a code-level dependency bump |

No Node/Flutter dependency audit was performed this session (out of primary backend scope for this phase; flagged as a gap, §18). No major-version dependency upgrades were made anywhere — both fixes above are patch/minor bumps, verified compatible by the full existing test suite (86 automated tests across the whole backend) passing unchanged after each upgrade.

---

## W) CI/CD Security

**No CI/CD pipeline exists** (no `.github/workflows/`, no other pipeline configuration found anywhere in the repository). Every claim in item W ("tests run before deployment," "production deployment requires authorization," "rollback procedure exists") is therefore **NOT APPLICABLE — there is no pipeline to secure.**

This is itself the most direct, avoidable production-readiness gap this report can point to: **the critical route-table panic this session found (see executive summary) would have been caught immediately by a CI step that simply runs `go build ./...` and starts the binary** — no pipeline, however minimal, currently exists to run that check automatically on every change. Recommended as the single highest-value next step before any further phase.

---

## X) VAPT Preparation

A VAPT evidence package **cannot be assembled this session** in the form item X describes (no deployed environment, no formal architecture/data-flow diagrams exist as separate documents). What DOES exist and is VAPT-relevant:

- ✅ RBAC matrix: `internal/rbac/permissions.go` is itself the authoritative, complete matrix (13 roles × ~40 permissions)
- ✅ API surface: `internal/routes/routes.go` is the complete, authoritative route table
- ✅ Environment information: `.env.example` (this phase, completed) documents every configuration variable
- ✅ Security configuration: this report itself
- ✅ Logging evidence: the append-only `audit_logs` table (Phase 1) and this phase's rate-limit-violation logging
- ❌ Architecture diagram / data-flow diagram: not produced this session
- ❌ Test credentials with safe roles: no seed/demo admin accounts with clearly-labeled restricted roles exist for a VAPT team to use safely
- ❌ Backup/DR evidence: none exists to provide (§N/§O)

**No destructive exploitation was performed against any environment this session** — every security test in `internal/security/security_test.go` runs against a local development database with synthetic test fixtures the tests themselves create and clean up, never against real customer/loan data.

---

## Y) Security Test Automation

`internal/security/security_test.go` — **15 tests, all passing**, run against the **real production route table** (`routes.RegisterRoutes`, not a per-test simplified router — the first test suite in this codebase to do so, which is exactly what surfaced the route-conflict bug in the executive summary):

1. `TestIDORCustomerCannotAccessAnotherLoan` — item 1 ✅
2. `TestNoTokenRejectedOnCustomerEndpoint` — item 2/3 (partial — no-token case) ✅
3. `TestReadOnlyAdminCannotEscalate` — item 16 ✅
4. `TestMakerCannotApproveOwnSanctionRecommendation` — item 5 ✅
5. `TestInvalidJWTRejected` — item 6 (malformed + wrong-signature cases) ✅
6. `TestRevokedAdminSessionRejected` — item 7 ✅
7. `TestRateLimitEnforcedOnLogin` — item 8 ✅
8. `TestSQLInjectionPayloadSafelyRejected` — item 9 ✅
9. `TestOversizedPayloadRejectedGracefully` — item 11 ✅
10. `TestDuplicateJournalIdempotencyEnforced` — item 14 ✅
11. `TestSensitiveDataNotLeakedInLoanResponse` — item 15 ✅
12. `TestUnauthorizedExportBlocked` — item 17 ✅
13. `TestAuditLogCannotBeModifiedOrDeleted` — item 18 ✅
14. `TestClosedPeriodBlocksNewPosting` — item 19 ✅
15. `TestUnauthorizedDisbursementBlocked` — item 20 ✅

**Not automated this phase**: item 4 (privilege escalation — covered indirectly by item 16's test, no dedicated separate test), item 10 (malicious file upload — Phase 3's MIME-allowlist behavior isn't re-tested in this suite specifically), item 12 (CORS violation — no automated test asserts the CORS config rejects a disallowed origin), item 13 (webhook replay — Phase 2 has webhook idempotency but no dedicated Phase 7 replay test).

**A note on test-isolation flakiness found and fixed during this session**: the shared rate limiters (item E) are process-wide singletons by design (that's what makes them effective) — this initially made `TestUnauthorizedExportBlocked`/`TestUnauthorizedDisbursementBlocked` fail intermittently because earlier tests in the same run had already consumed the shared `admin.login` limiter's burst allowance. Fixed by (a) giving `TestRateLimitEnforcedOnLogin` its own isolated router/limiter instance so it doesn't poison the shared production limiter used by every other test, and (b) sizing `admin.login`'s burst (15) to comfortably cover a realistic admin-panel usage burst, not just this test suite. This is documented in code comments in both `security_test.go` and `routes.go` for whoever tunes these values again later.

---

## Z) Load / Performance Security

**No load testing was performed this session.** Per the instruction not to run uncontrolled load tests against production (and there being no production to test against), this section is a preparation checklist only:

- Customer APIs, loan application, login/OTP, payment creation/webhooks, admin dashboard, loan search, reports, collections, DPD jobs, regulatory reports — all exist and are individually reachable, but none have been benchmarked for requests/sec, latency, error rate, DB CPU, connection count, memory, or queue depth.
- The new connection-pool limits (§H, 25 max open connections) are a real, if untested, ceiling that would need validation against actual concurrent load before being trusted at scale.
- **Recommended before the 100,000-customer target**: a realistic synthetic dataset (not production data) and a proper load-testing tool (k6, Locust, or similar) run against a staging environment that doesn't exist yet.

---

## AA) Production Readiness Checklist

```
SECURITY
[x] Authentication           — implemented, partially tested
[x] Authorization            — implemented, tested extensively
[x] RBAC                     — implemented, tested extensively
[ ] MFA architecture         — NOT IMPLEMENTED
[x] Rate limiting            — implemented and expanded this phase, tested
[x] Secrets management       — verified clean this phase
[x] Encryption               — PAN/documents at rest (prior phases)
[x] Audit logging            — implemented, append-only verified this phase
[x] Secure file storage      — implemented (prior phases)
[x] Dependency security      — 2 real CVEs found and fixed this phase

FINANCIAL
[x] Payment verification     — implemented, tested (prior phases)
[x] Disbursement verification — implemented, tested
[x] Idempotency              — implemented throughout (Phases 2/4/5/6), tested this phase
[x] Accounting integrity     — implemented, tested this phase (closed-period test)
[x] Bank reconciliation      — implemented (Phase 2)
[x] Maker-checker            — implemented, tested this phase

DATA
[ ] Backup                   — NOT IMPLEMENTED (no production infra)
[ ] Restore test              — NOT PERFORMED (nothing to restore)
[~] Retention                — metadata layer only (Phase 6), unvalidated
[ ] Disaster Recovery         — documented runbook only, not exercised
[ ] Business Continuity       — documented checklist only

OPERATIONS
[ ] Monitoring                — NOT IMPLEMENTED
[ ] Alerting                  — NOT IMPLEMENTED
[ ] Incident Response         — NOT IMPLEMENTED
[~] Job reliability           — idempotent, no auto-retry/backoff/DLQ
[ ] Deployment rollback        — NOT APPLICABLE (no deployment pipeline)
[x] Health checks             — implemented this phase

COMPLIANCE
[~] KYC                       — implemented, unvalidated by compliance (Phase 3)
[~] Credit bureau             — implemented, unvalidated (Phase 3)
[~] GST/TDS                   — implemented, unvalidated (Phase 4)
[~] NPA/provisioning           — implemented, unvalidated (Phase 5)
[~] Regulatory reporting       — framework implemented, no real return configured (Phase 6)
[x] Audit evidence             — implemented (append-only logs throughout)

TESTING
[x] Unit/integration tests     — 86 tests across 10 packages, all passing
[x] Security tests             — 15 tests this phase, all passing
[ ] Load tests                 — NOT PERFORMED
[~] VAPT preparation           — partial evidence exists, no formal package assembled
```

---

## AB) Final Security Gap Report

1. **Critical vulnerabilities remaining**: none known — the one critical issue found this session (route-table startup panic) is fixed and verified via an actual server boot.
2. **High-risk vulnerabilities remaining**: no production infrastructure/monitoring/backups/CI-CD exist at all — this is a "not yet built" gap, not a vulnerability in what exists.
3. **Medium/low risks**: MIME-type not content-verified (§F); no admin MFA (§C); no webhook replay-window check (§M); no account lockout beyond rate limiting (§C).
4. **Security controls implemented**: RBAC (13 roles), maker-checker (loans/accounting/reports/write-offs), append-only audit log, AES-256 encryption (PAN + documents, separate keys), 11 endpoint-specific rate limiters + violation logging, security headers, CORS restriction, DB connection pooling + statement timeout, global body-size cap, health endpoints, 2 real dependency CVEs patched.
5. **Backup status**: not implemented — no production infrastructure exists.
6. **Restore-test result**: not performed — nothing to restore.
7. **DR status**: documented runbook only, not exercised against real infrastructure.
8. **BCP status**: documented checklist only.
9. **Incident-response status**: not implemented as a module; no runbooks drafted this session beyond the DR runbook.
10. **Monitoring status**: not implemented — the single largest production-readiness gap.
11. **VAPT readiness**: partial evidence exists (RBAC matrix, route table, this report); no deployed environment or formal diagram package to hand a VAPT team yet.
12. **Dependency vulnerabilities**: 2 real module-level CVEs found and fixed this session (pgx, quic-go); 6 Go-standard-library patch-level items remain, fixable only by a toolchain upgrade (not a code change).
13. **Secrets exposure findings**: none — `.env` never committed, no secrets found in repository scan, `.env.example` contains placeholders only.
14. **Infrastructure findings**: no cloud infrastructure, no Docker, no CI/CD exist at all — this application has never been deployed outside local development.
15. **Production blockers**: (a) no deployment pipeline/infrastructure exists at all, (b) no monitoring/alerting exists, (c) no backup/DR has ever been tested, (d) admin MFA doesn't exist, (e) the regulatory reporting/NPA/provisioning/GST/TDS frameworks (Phases 3–6) remain unvalidated by the NBFC's actual compliance officer/CA — none of this is safe to run with real customer money until those are addressed.
16. **External audit/VAPT requirements**: a real penetration test against a deployed staging environment is required before production launch — nothing in this session substitutes for that.
17. **CA/CS/compliance requirements**: every regulatory figure/threshold/return introduced in Phases 3–6 remains explicitly unvalidated — see those phases' own reports' compliance-matrix sections, unchanged by this phase.
18. **Recommended next actions, in priority order**:
    1. Stand up a minimal CI pipeline that at least runs `go build ./...`, `go vet ./...`, and `go test ./...` on every change — this alone would have caught this session's critical finding automatically, months earlier.
    2. Provision real infrastructure (even a single small cloud instance) and get automated backups running with one verified restore test, before any real customer data exists.
    3. Wire up basic monitoring/alerting (even a free-tier APM tool) — the compliance dashboards built in Phase 6 have no one automatically watching them today.
    4. Get Phases 3–6's regulatory/compliance content in front of the NBFC's actual CA/compliance officer — none of it should be trusted as-is.
    5. Add admin MFA before this system handles real disbursements.
