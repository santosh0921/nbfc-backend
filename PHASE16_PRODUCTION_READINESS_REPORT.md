# Phase 16 — Production Infrastructure, Deployment, Monitoring, Backup, Disaster Recovery, High Availability & Production Go-Live Readiness

**This report does not claim "production ready", "disaster recovery
tested", "backup verified", "high availability", "zero downtime", or
"RBI compliant".** Each of those is addressed below with what was
actually verified and what was not. Several are explicitly NOT TESTED.

---

## 1. Executive Summary

**A premise correction first.** The Phase 15 hand-off stated the Go
toolchain had been upgraded 1.26.5 → 1.26.6. It had not — my own Phase 15
report said explicitly that I did not perform it. Verified at the start
of this phase: the toolchain was still 1.26.5 and all six stdlib CVEs
were open. **This phase actually performed the upgrade**: `toolchain
go1.26.6` is now pinned in `go.mod`, and `govulncheck ./...` reports
**0 vulnerabilities**.

This phase found and fixed **one new security defect**, built the
monitoring and financial-integrity foundations Phase 15 left missing,
closed the Phase 15 export-limits gap, and produced a complete route
inventory. It also established, with evidence, that several
production-critical capabilities **do not exist at all**.

**New security finding — DoS amplification in rate-limit auditing
(MEDIUM, fixed).** Discovered during the item-AM soak test: every
rate-limited request wrote an audit row synchronously on the request
path, and under sustained load those blocking inserts were measured
taking **3.7 seconds each**. The harder an attacker floods an endpoint,
the more database writes the defence itself generates — competing for the
same connection pool serving legitimate traffic and inflating an
append-only table that by design cannot be pruned. Fixed by aggregating
to at most one audit row per (IP, endpoint) per minute, carrying a count
of suppressed violations. Regression-tested: **200 requests → 198
denials → 1 audit row**.

**The three findings that most affect go-live are absences, not bugs:**

1. **No automated backups exist, and no restore has ever been tested.**
   Item K calls a restore test mandatory; it could not be performed here
   (`pg_dump`/`pg_restore`/`psql` are unavailable and the Docker daemon
   is not running). **NOT TESTED.**
2. **No scheduler exists.** Every "daily" job — the risk job, compliance
   sweep, reconciliation checks, data-quality checks — is reachable
   *only* via a manual admin POST. Nothing runs automatically. Financial
   and compliance tasks the system's design assumes run daily currently
   do not run at all.
3. **No alerting, no centralized logging, no on-call, no deployment
   pipeline, no CI.** The repository contains no Dockerfile, no IaC, and
   no `.github/` directory.

## 2. Current Infrastructure

Audited from the repository, not assumed. **No infrastructure is defined
in this codebase**: no Dockerfile, no docker-compose, no Terraform, no
Procfile, no render.yaml, no Kubernetes manifest, no CI configuration.

What exists is a single Go binary (`cmd/api`) that:
- listens on `PORT` (default 8080), plain HTTP — TLS termination is
  assumed to be an upstream proxy's job,
- connects to one PostgreSQL database (pool: 25 max / 10 idle / 30m
  lifetime, `statement_timeout=30000`),
- writes AES-256-GCM encrypted documents to a local filesystem path,
- runs export rendering in-process goroutines,
- holds rate-limiter state in memory, per process.

**Single points of failure**: one application process, one database (no
replica, no failover), local document storage, in-process export
workers, per-instance rate limiting.

**External integrations present in code**: Razorpay, a credit-bureau
abstraction, CKYC, screening/PEP, OTP provider, PAN verification (still a
stub — carried from Phase 1).

## 3. Environment Separation

The mechanism is correct; the environments are not provisioned.

`internal/config.ValidateProductionSafety` (Phase 7) refuses to start in
production when `DEMO_MODE=true`, when encryption keys are missing, when
`CORS_ALLOWED_ORIGINS` is unset, or when any two of the three JWT secrets
are equal — verified present in current code. Separation of
database/secrets/storage/webhook config between dev, staging, and
production is therefore **enforced by configuration**, but **no staging
or production environment exists to separate** (§2). 👤 EXTERNAL.

## 4. Secrets

`.env.example` was read line by line: every value is a `change_this_…`
placeholder. No real secret is committed. No secret is logged (verified
in Phase 14 and re-checked here for the new logging middleware, which
emits only method/route/status/duration/request-id). No secret is
returned through any API.

Rotation procedure documented in
`PHASE16_INCIDENT_RESPONSE_RUNBOOK.md`, including the important
constraint that **`PAN_ENCRYPTION_KEY` and `DOCUMENT_ENCRYPTION_KEY`
cannot be rotated by changing the variable alone** — existing ciphertext
becomes undecryptable. No decrypt/re-encrypt migration tooling exists;
that is a documented gap, not a solved problem.

No secret values appear anywhere in this report, per the phase brief.

## 5. TLS

Application-side controls exist: HSTS, CSP, `X-Frame-Options: DENY`,
`X-Content-Type-Options: nosniff` (`internal/middleware/security_headers.go`).
`Referrer-Policy` and `Permissions-Policy` remain **missing** (Phase 14
finding, still open, LOW).

Certificate provisioning, renewal, HTTP→HTTPS redirect, and TLS cipher
configuration are all upstream-proxy concerns and are **NOT configured
in this repository**. Expired/wrong-certificate behaviour was **NOT
TESTED** — there is no TLS terminator to test against. 👤 EXTERNAL.

## 6. DNS

**NOT DEFINED.** No production domain, API domain, admin domain, or
webhook domain is recorded anywhere in the repository. `CORS_ALLOWED_ORIGINS`
and `TRUSTED_PROXIES` are the two configuration points that must be set
once domains exist. 👤 EXTERNAL.

## 7. Application Deployment

**No deployment pipeline exists.** No build/test/package/deploy
automation, no smoke-test stage, no migration gate. Deployment today
means building a binary and running it manually.

Schema migration is GORM `AutoMigrate` at startup — additive and
forward-only, with **no migration versioning and no down-migrations**.
The Phase 15 performance indexes are created `CONCURRENTLY IF NOT
EXISTS` specifically so they do not take an ACCESS EXCLUSIVE lock on a
live table.

**Zero-downtime deployment is NOT supported** — a single instance means
deployment is an outage. Rolling/blue-green/canary all require the
multi-instance capability that does not exist. Per item S, no
zero-downtime claim is made.

## 8. Database

| Setting | Value | Status |
|---|---|---|
| Max open connections | 25 | ✅ set |
| Max idle | 10 | ✅ set |
| Conn max lifetime / idle time | 30m / 5m | ✅ set |
| `statement_timeout` | 30,000 ms | ✅ set in DSN |
| TLS (`DB_SSLMODE`) | configurable, defaults `disable` | ⚠️ must be `require` in production |
| Replica / failover | none | ❌ |
| Least-privilege DB user | unverifiable from application code | ⚠️ INFRA must confirm the app user is not a superuser |
| Disk monitoring | none | ❌ |

Pool statistics are now exposed at `GET /admin/ops/metrics`
(`db.connections.open/in_use/idle/max_open`, `db.wait_count`,
`db.wait_duration_ms`) — a rising wait count is the signal that the pool,
rather than the database, is the bottleneck.

## 9. Backup

**❌ NOT CONFIGURED.** No automated backup exists, no retention policy,
no backup encryption, no backup access control, and no backup tooling in
the repository. Per item J, no claim is made that backups work because a
hosting provider might offer them — nothing was verified.

This is a **hard go-live blocker**.

## 10. Restore

**❌ NOT TESTED — item K calls this mandatory.**

A restore test could not be performed in this environment: `pg_dump`,
`pg_restore`, and `psql` are all absent, and the Docker daemon is not
running, so no isolated restore target could be created.

The validation procedure a real restore test must follow is defined so
it can be executed the moment tooling exists:

1. Take a backup; record its timestamp.
2. Restore into an isolated database; record restore duration.
3. Start the application against it; confirm `AutoMigrate` completes.
4. Confirm `/readiness` returns 200.
5. Compare counts against the source: customers, loans, payments,
   journal entries, audit logs.
6. Run `GET /admin/ops/integrity` and require `passed:true` with zero
   critical findings.
7. Spot-check ledger balances against the pre-backup trial balance.

**Backup timestamp: n/a. Restore duration: n/a. Result: NOT TESTED.**

## 11. RPO / RTO

**❌ NOT DEFINED BY THE BUSINESS.** Per item L, no arbitrary values were
invented or presented as regulatory requirements.

| | Target | Current capability | Gap |
|---|---|---|---|
| **RPO** | Not defined — MGMT must set | **Unbounded** — no automated backup exists, so worst case is total loss since the last manual backup (if any) | Entire capability missing |
| **RTO** | Not defined — MGMT must set | **Unbounded** — no tested restore procedure, no replica, no automated deployment | Entire capability missing |

## 12. Disaster Recovery

Nine scenarios documented with detection/response/recovery/validation
steps in `PHASE16_INCIDENT_RESPONSE_RUNBOOK.md`: application failure,
database failure, storage failure, region failure (⚪ not applicable —
single deployment), credential compromise, accidental data deletion,
corrupt deployment, payment-integration outage, third-party API outage.

**No DR drill has ever been executed. Documentation is not a test.**

Two design properties genuinely help recovery and were verified rather
than assumed: the payment webhook is idempotent by `gateway_event_id`
(so a gateway retry after a restart cannot double-post), and journal
numbers come from a Postgres sequence that never reuses a value (so a
deleted entry leaves an auditable gap rather than a silently reused
number).

## 13. High Availability

**❌ NOT AVAILABLE. No HA claim is made.**

The application is *mostly* stateless — JWT-based auth, shared database,
no server-side session affinity — so multiple instances are
architecturally plausible. Two pieces of state are **per-instance** and
would break or degrade under horizontal scaling:

1. **Rate limiter** — in-memory per process, so N instances multiply
   every effective limit by N.
2. **Export rendering and its concurrency semaphore** — in-process, so
   in-flight exports are lost on restart and the concurrency cap is
   per-instance rather than global.

Instance failure, replacement, and traffic redistribution were **NOT
TESTED** — there is no load balancer and only one instance.

## 14. Monitoring

**Built this phase** (Phase 15 recorded metrics as MISSING):

`internal/observability` — a vendor-neutral in-process registry exposed
at `GET /admin/ops/metrics` (admin JWT + `MIS_VIEW`; deliberately not
public, since traffic shape and route names aid an attacker).

Tracked: total requests, 5xx, 4xx, auth failures, rate-limit violations,
payment failures, webhook failures, export failures and rejections, job
failures, integrity-check failures; per-route p50/p95/p99 latency; DB
pool gauges. Latency uses a bounded 2,048-sample reservoir per route —
tested (`TestLatencyReservoirIsBounded`) so the observability layer
cannot itself leak the memory it exists to detect.

**Limitation stated in the payload itself**: metrics are in-memory,
single-process, reset on restart, and are **not aggregated across
instances**. This is a foundation for monitoring, not a metrics backend.

**Correction to Phase 15**: that report listed request correlation IDs as
MISSING. That was wrong — `middleware.RequestID()` already existed and
was already wired in `cmd/api/main.go`, and `models.AuditLog.RequestID`
was already populated from it. Only the metrics layer was genuinely
missing.

## 15. Logging

Added one structured line per request:
`level=info ts=… request_id=… method=… route=… status=… duration_ms=…`

It logs the **path template** (`/admin/loans/:id`), never the raw URL, so
identifiers in paths are not written to logs, and metrics group by route
rather than exploding into one series per id. No password, OTP, MPIN,
JWT, secret, or request body is logged.

**❌ Logs are written to stdout only and are not persisted or
aggregated anywhere.** They do not survive a restart. Centralized
logging, retention, access control, search, and redaction are all
undefined. 👤 EXTERNAL / BLOCKER.

## 16. Alerting

**❌ NOT IMPLEMENTED.** Metrics and integrity results are exposed, but
**nothing watches them and nothing notifies anyone.** No alerting
system, no thresholds wired to a notifier, no on-call rotation.

Note the distinction from Phase 13: `ManagementThreshold` /
`ManagementAlert` exist for *business* KPI alerting (deliberately seeded
empty). Those are not operational infrastructure alerts, and neither
surface is connected to a pager.

The alert conditions item W names are all *derivable* from
`/admin/ops/metrics` and `/admin/ops/integrity` — but derivable is not
implemented.

## 17. Financial Integrity Monitoring

**Built this phase** — `internal/integrity`, exposed at
`GET /admin/ops/integrity`. Seven checks, each carrying its own severity
and a plain-language statement of the invariant it enforces:

| Check | Severity on failure | Invariant |
|---|---|---|
| `journal_balance` | CRITICAL | Every journal entry's debits equal its credits |
| `journal_sequence` | CRITICAL | A journal number identifies exactly one entry |
| `duplicate_disbursement` | CRITICAL | At most one successful disbursement per loan |
| `orphaned_journal_lines` | CRITICAL | Every line belongs to an existing entry |
| `closed_period` | HIGH | No posting after an accounting period closes |
| `negative_amounts` | HIGH | No negative disbursement/payment amounts |
| `disbursement_ledger_agreement` | HIGH | Disbursement table agrees with the ledger |

**Detection only — never repair.** Tested explicitly
(`TestIntegrityChecksNeverModifyFinancialData`): running every check
changes no row count in journals, journal lines, disbursements, or
payments.

**⚠️ One HIGH check currently FAILS** in the development database:
`disbursement_ledger_agreement` reports ₹380,000 in the disbursement
table against ₹715,000 posted to the ledger (difference ₹335,000). The
most likely cause is sixteen phases of accumulated test data — tests
that created journal entries without matching disbursement rows, and
cleanups that removed disbursements but not journals. **It has not been
proven to be only that.** This must be re-run against production data
before go-live and resolved with accounting; the check is doing exactly
what it was built to do.

**❌ Nothing schedules these checks.** The endpoint exists; no
automation calls it.

## 18. Payment Monitoring

Metric counters exist (`payment.failures`, `webhook.failures`) and
Phase 9's reconciliation surfaces remain in place. Webhook signature
verification and `gateway_event_id` idempotency were re-verified
(idempotency additionally concurrency-tested in Phase 15: 12 concurrent
duplicate events → 1 stored row). Nothing alerts on any of it (§16).

## 19. Background Jobs

**❌ CRITICAL OPERATIONAL GAP — no scheduler exists.**

Verified: `cmd/api/main.go` contains no ticker, no goroutine scheduler,
no cron integration. Every job is reachable **only** through a manual
admin POST:

| Job | Trigger endpoint | Runs automatically? |
|---|---|---|
| Daily risk job (DPD, classification, provisioning) | `POST /admin/risk/jobs/run` | **No** |
| Risk consistency checks | `POST /admin/risk/exceptions/run-checks` | **No** |
| Compliance sweep (overdue calendar items, CAPAs) | `POST /admin/compliance/sweep` | **No** |
| Tax reconciliation | `POST /admin/tax/exceptions/run-checks` | **No** |
| Reconciliation checks | `POST /admin/reporting/reconciliation/run-checks` | **No** |
| Analytics data-quality checks | `POST /admin/analytics/data-quality/run-checks` | **No** |
| Risk-signal detection | `POST /admin/analytics/signals/run-detection` | **No** |
| Financial integrity checks (new) | `GET /admin/ops/integrity` | **No** |

Item AA asks that no important financial/compliance task run silently
without monitoring. The reality is worse: **they do not run at all**
unless a human remembers to click.

**Deliberate decision not to build an in-process scheduler.** Adding a
ticker to the application would run every job N times under horizontal
scaling and would silently double-post if a job were not perfectly
idempotent. The correct architecture is an external scheduler invoking
the existing authenticated endpoints, with its own success/failure
monitoring. That is infrastructure work outside this codebase, and it is
recorded as a blocker rather than papered over with a fragile in-process
timer.

## 20. Security Monitoring

Counters for auth failures and rate-limit violations feed
`/admin/ops/metrics`; `audit_logs` remains append-only at the database
level (trigger-enforced) and now carries aggregated rate-limit violation
rows including suppressed counts. Phase 13's `risk_signals` and
`analytics_data_quality_exceptions` remain available.

Detection surfaces exist; **automated response and notification do not**
(§16).

## 21. Incident Response

`PHASE16_INCIDENT_RESPONSE_RUNBOOK.md` — SEV-1..SEV-4 definitions, the
detection surfaces that actually exist, a standard response process,
mandatory evidence preservation for financial incidents (including the
rule never to correct financial data by direct SQL), nine DR playbooks, a
credential-compromise matrix per secret type, a financial-integrity alarm
response table, and post-incident review requirements.

The runbook states up front that it presumes an alerting and on-call
layer **which does not yet exist**.

## 22. Dependency Scan

**✅ CLEAN — and this closes the Phase 14 residual.**

- Before: Go 1.26.5, **6 stdlib CVEs** that govulncheck confirmed this
  code calls (GO-2026-6218 `net/url`, -6090 `crypto/tls`, -6089
  `net/http`, -6088 `encoding/xml`, -5972 `encoding/asn1`, -5026
  `x/net/idna`).
- Action: pinned `toolchain go1.26.6` in `go.mod` so every build — local,
  CI, production — uses the fixed toolchain rather than whatever Go
  happens to be on the build host.
- After: **`govulncheck ./...` → "No vulnerabilities found." 0 affecting
  called code.** One vulnerability remains in a required module that this
  code does not call.

Full regression re-run on the new toolchain (§ Final Regression).

## 23. Route Security Inventory

**✅ 100% INVENTORY — and inventory is explicitly not testing.**

`PHASE16_ROUTE_INVENTORY.md` is generated from the real route table by
`TestGenerateRouteInventory`, so it cannot drift from the code.

| Tier | Routes |
|---|---|
| Public (unauthenticated) | 10 |
| Customer JWT | 26 |
| Employee JWT | 7 |
| Admin JWT + RBAC permission | 248 |
| **TOTAL** | **291** |

Per item AH, the three states are kept separate:

- **INVENTORIED: 291 / 291 (100%)**
- **SECURITY-TESTED: a representative subset** — Phase 14's suite plus
  this phase's additions cover IDOR, privilege escalation, maker-checker
  bypass, JWT forgery/alg-none, session revocation, rate limiting,
  SQL injection, formula injection, audit immutability, closed-period
  posting, and rate-limit audit amplification. That is roughly 20
  scenario-level tests, not 291 per-route tests.
- **NOT-TESTED: the majority of individual admin routes** have not been
  individually probed for IDOR or parameter manipulation.

This remains the Phase 14 residual that Phase 15 did not address and
Phase 16 has only partially improved — inventory is now complete,
coverage is not.

## 24. Frontend Validation

**⚪ NOT APPLICABLE in-repo / 👤 EXTERNAL.**

Verified by search: **no Dart file in this repository references
`/admin/loans` or `/admin/users`.** The Flutter code here is a customer
app plus an employee app (222 Dart files); there is no admin panel in
this repo. The Phase 15 breaking changes therefore have **no in-repo
consumer**.

They remain real for any **external** admin panel:
- `GET /admin/loans` now returns `{loans, pagination}` instead of a bare
  JSON array.
- `GET /admin/users` gained a `pagination` block, and `total` changed
  meaning from "rows in this response" to "rows in the full result set".

Flutter build and frontend security review were **NOT RUN** this phase.

## 25. Export Limits

**✅ CLOSES THE PHASE 15 GAP.** All five caps item AK requires, each
configurable by environment variable and documented in `.env.example`:

| Limit | Default | Enforcement point |
|---|---|---|
| Max rows | 200,000 | Refused at request time, before any export row is created |
| Max date range | 400 days | Before any database work |
| Max file size | 100 MiB | Before the artifact reaches storage |
| Max concurrent | 3 | Counting semaphore; excess refused with a retryable error |
| Max execution time | 300 s | Deadline on the render |

Tested: limits enforced at the boundary, configurable via env,
over-limit exports refused up front rather than queued and silently
failed later.

**Honest limitation**: the execution-time cap bounds how long an export
stays pending and frees the concurrency slot, but does **not** kill the
rendering goroutine — Go provides no safe way to do that. An abandoned
render completes and is discarded. Documented in code rather than
implied to be a hard kill.

## 26. Analytics Scale

**❌ STILL NOT PROVEN AT TARGET SCALE — unchanged from Phase 15.**

Item AL permits keeping this as NOT PROVEN if realistic seeding is not
feasible, and requires documenting exactly what was tested. What was
tested: 2,000 seeded customers with profile and address rows. **No loans,
EMI schedules, payments, or journal entries were seeded at volume.**

Seeding a realistic multi-million-row transaction set is not simply bulk
insertion — every payment needs a balanced double-entry posting through
`PostJournalEntry` (which serializes journal-number allocation behind an
advisory lock) for the data to be representative. Generating it faster by
inserting rows directly would produce data that fails the very integrity
checks built in §17, making any analytics measured against it
meaningless.

Analytics endpoints measuring ~2 ms therefore reflect aggregation over a
nearly-empty transaction set and **must not be read as analytics
performance at scale**.

## 27. Soak Testing

**✅ PERFORMED — 30 minutes**, item AM's preferred minimum, at 50
concurrent users against `/admin/users?limit=25`, in six 5-minute
segments.

**Critical caveat: this soak ran the PRE-FIX binary.** It was launched
before the rate-limit audit amplification fix compiled, so it captures
the defective behaviour — which is precisely how the defect was found.

| Segment | req/sec | p50 | p95 | Errors | Heap |
|---|---|---|---|---|---|
| 1/6 | 3,953.8 | 10.183 ms | 24.522 ms | 0.00% | 39.8 MiB |
| 2/6 | 3,623.5 | 11.081 ms | 26.021 ms | 0.00% | 30.8 MiB |
| 3/6 | 3,784.9 | 11.022 ms | 26.958 ms | 0.00% | 32.0 MiB |
| 4/6 | 138,755.1 | <1 ms | <1 ms | 0.00% | 671.1 MiB |
| 5/6 | 143,713.8 | <1 ms | <1 ms | 0.00% | 1,012.8 MiB |
| 6/6 | 129,776.7 | <1 ms | 507 µs | 0.00% | 904.1 MiB |

Goroutines: 16 at start, 15 at end. Heap: 39.8 MiB → 904.1 MiB.

**Reading segment 4 honestly**: 138,755 req/sec is **not a performance
achievement**. It is the `admin.api` rate limiter (20 rps, burst 60)
exhausting its bucket and rejecting nearly every request with a cheap
429. Throughput rose because the application stopped doing work. The
heap growth to 671 MiB over the same period is the amplification defect
in action.

**Measured consequence of the amplification defect.** After the soak,
the `audit_logs` table was inspected:

| Measure | Value |
|---|---|
| Total audit rows | **3,568,175** |
| Of which `rate_limit.exceeded` | **3,564,457 (99.9%)** |
| On-disk size of `audit_logs` | **1,029 MB** |
| Time for a simple `COUNT(*)` | 1.2–2.0 seconds |

**One soak test produced a gigabyte of audit log, 99.9% of it noise.**
Three consequences make this materially worse than ordinary table growth:

1. **It cannot be undone.** `audit_logs` is append-only at the database
   level — the trigger rejects `DELETE`. That immutability is a
   deliberate and correct security control, which means this bloat is
   *permanent* and cannot be pruned through the application.
2. **Real security events get buried.** Genuine audit entries are now
   0.1% of the table, and queries over it have become slow.
3. **It is attacker-controllable.** Any party able to send traffic can
   grow the one table that cannot be cleaned up.

This is the evidence that turned a latency observation into a security
finding. Post-fix, the same flood produces 1 audit row per source per
minute instead of one per request (regression-tested: 198 denials → 1
row).

**Note on the development database**: this 1 GB of test-generated audit
data remains in the dev database. It was deliberately **not** removed —
doing so would require disabling the append-only trigger, and
circumventing that control casually is precisely the wrong instinct for
a financial audit trail. Cleaning it is a deliberate operator decision,
not something to do quietly mid-phase.

Goroutine count stayed flat at 16 throughout, so there is **no goroutine
leak**. Latency in the genuine-work segments (1–3) was stable: p50
10.2 → 11.0 ms, p95 24.5 → 27.0 ms, zero 5xx. That is the meaningful
soak result.

### Post-fix soak (10 minutes, same shape, fix compiled in)

Re-run after the amplification fix, at the same 50 concurrent users
against the same endpoint:

| Segment | req/sec | p50 | p95 | Errors | Heap |
|---|---|---|---|---|---|
| 4/6 | 7,378.6 | 3.998 ms | 13.234 ms | 0.00% | 11.1 MiB |
| 5/6 | 6,201.5 | 4.325 ms | 17.126 ms | 0.00% | 16.1 MiB |
| 6/6 | 6,252.5 | 3.978 ms | 13.992 ms | 0.00% | 11.8 MiB |

**Summary: heap 16.9 MiB → 11.8 MiB, goroutines 16 → 16, p95 14 ms →
14 ms.**

Side by side:

| | Pre-fix (30 min) | Post-fix (10 min) |
|---|---|---|
| Heap at end | **904.1 MiB** | **11.8 MiB** |
| Heap trend | 39.8 → 904.1 MiB (climbing) | 16.9 → 11.8 MiB (flat/declining) |
| Sustained behaviour | Degenerated into 429-rejection with unbounded audit writes | Kept doing real work throughout |
| p95 | Meaningless once rejecting | Stable 13–17 ms across all segments |
| Goroutines | 16 → 15 | 16 → 16 |

The post-fix run sustained ~6,200–7,400 req/sec of genuine work with
**flat memory**, where the pre-fix run climbed to nearly a gigabyte of
heap. That is the amplification defect removed, measured rather than
asserted.

**What the soak still did not establish**: anything beyond 30 minutes,
and post-fix behaviour beyond 10 minutes. Both remain open.

## 28. Smoke Testing

A production smoke-test checklist is defined in
`PHASE16_PRODUCTION_GO_LIVE_CHECKLIST.md`. It has **not been executed
against a production environment**, because none exists.

## 29. Rollback

**❌ NOT TESTED.** No rollback has ever been performed.

The constraint that matters: `AutoMigrate` is forward-only and additive
with **no down-migrations**. Rolling an application binary back is
therefore safe *only* if the newer version introduced no incompatible
schema change. Per item Q, financial schema changes must never be blindly
auto-rolled-back, since doing so could destroy data. There is no
migration versioning to reason about this systematically.

## 30. Go-Live Checklist

`PHASE16_PRODUCTION_GO_LIVE_CHECKLIST.md` — 16 sections, ~80 items, each
with owner, status, evidence, and blocker flag.

## 31. Production Gates

| Gate | Status | Basis |
|---|---|---|
| 1. Build | ✅ PASS | `go build ./...` clean on go1.26.6 |
| 2. Tests | ✅ PASS | Full suite green (§Final Regression) |
| 3. Security | ⚠️ PARTIAL | CVEs 0; 100% route inventory; but coverage is representative and no independent VAPT |
| 4. Performance | ⚠️ PARTIAL | Load/spike/soak done; analytics at volume NOT PROVEN |
| 5. Database | ⚠️ PARTIAL | Pool/timeout/indexes set; TLS, least-privilege, disk monitoring, replica all unverified or absent |
| 6. Backup | ❌ **FAIL** | None configured |
| 7. Restore | ❌ **NOT TESTED** | Tooling unavailable |
| 8. Monitoring | ⚠️ PARTIAL | Metrics + integrity + health exist; **no alerting, no log persistence, no on-call** |
| 9. Incident response | ⚠️ PARTIAL | Runbook complete; never exercised; presumes absent alerting layer |
| 10. Financial integrity | ⚠️ PARTIAL | 7 automated checks built and verified non-mutating; **1 HIGH open**; nothing schedules them |
| 11. Frontend | ⚪ N/A in-repo | No admin panel here; external panel must handle Phase 15 breaking changes |
| 12. Deployment rollback | ❌ **NOT TESTED** | No pipeline, no rollback test, forward-only migrations |

## 32. Open Risks

**Blocking:**
1. No automated backups; restore never tested.
2. No scheduler — daily financial/compliance jobs never run.
3. No alerting, no centralized logging, no on-call.
4. No deployment pipeline, no CI, no rollback test.
5. RPO/RTO undefined by the business.
6. One HIGH integrity finding open (must be re-run on production data).
7. No independent VAPT; per-route security coverage incomplete.
8. Production credentials (payment/KYC/bureau/OTP) not configured; PAN
   verification still a stub.
9. Zero compliance/CA configuration — classification rules, provisioning
   rules, TDS rules, retention periods, NBFC profile are all
   deliberately unseeded and must be set by qualified people.

**Non-blocking but material:**
10. Analytics unproven at transaction volume.
11. Metrics single-process, reset on restart.
12. Rate limiter and export workers per-instance — break under
    horizontal scaling.
13. In-flight exports lost on restart.
14. `Referrer-Policy` / `Permissions-Policy` headers missing.
15. No OpenAPI/Swagger inventory.
16. Encryption-key rotation has no migration tooling.
17. Deep offset pagination degrades on large tables.

## 33. Residual Technical Debt

- Full pagination sweep across remaining list endpoints (Phase 15).
- Export execution cap cannot kill a running render.
- No migration versioning or down-migrations.
- Soak beyond 30 minutes; post-fix soak not yet run.
- Frontend build/security review not exercised.

## 34. Final Recommendation

The **application layer** is in reasonable shape: dependencies are clean,
financial concurrency invariants are verified under real contention,
integrity monitoring exists and is provably non-mutating, exports are
bounded, the audit trail is immutable at the database level, and the
worst performance and security defects found across Phases 14–16 have
been fixed with regression tests.

The **operational layer barely exists**. There is no backup, no tested
restore, no scheduler, no alerting, no log persistence, no deployment
pipeline, and no on-call. These are not refinements — an NBFC running
without backups or automated daily risk/compliance jobs is taking on
operational and regulatory risk that no amount of application-level
correctness offsets.

**Recommendation: NOT READY for production go-live.** The path to
readiness is primarily infrastructure and business configuration, not
application development.

---

# FINAL STATUS

## PHASE 14 REMAINING
- ~~Dependency CVE scan~~ → **CLOSED**: run, 6 CVEs found, toolchain
  pinned to go1.26.6, now 0 vulnerabilities.
- **Route security coverage still representative, not exhaustive** —
  inventory is now 100% (291 routes) but per-route security testing is
  not. Partially improved, not closed.
- `Referrer-Policy` / `Permissions-Policy` headers still missing.
- No OpenAPI/Swagger inventory.
- No independent VAPT engagement.
- Business-logic/tax/compliance attack scenarios beyond those
  spot-checked; no live infrastructure/SSRF probing.

## PHASE 15 REMAINING
- **Analytics at target transaction volume — still NOT PROVEN** (§26).
- ~~Soak test~~ → **CLOSED**: 30-minute soak performed (§27), though
  pre-fix and not repeated post-fix.
- ~~Export size/range/concurrency limits~~ → **CLOSED** (§25).
- ~~Observability metrics~~ → **CLOSED** (§14). Correlation IDs were
  never actually missing — Phase 15 reported that in error.
- Full pagination sweep across all remaining list endpoints — open.
- Out-of-process load testing including network/TLS/proxy — open.
- DB-side metrics (locks, deadlocks, slow-query stats) — open.
- Frontend performance assessment — open; the breaking-change update is
  ⚪ N/A in-repo but real for any external admin panel.

## PHASE 16 REMAINING
- Automated backups + **mandatory restore test** (NOT TESTED).
- External scheduler for all daily financial/compliance jobs.
- Alerting, centralized logging, on-call rotation.
- Deployment pipeline, CI, rollback test.
- RPO/RTO definition by management.
- DR drill execution.
- Resolve the open HIGH `disbursement_ledger_agreement` finding against
  production data.
- Multi-instance HA (requires shared rate limiting and durable export
  workers).
- TLS/DNS provisioning and production infrastructure definition.
- Encryption-key rotation tooling.

## Gate Summary

**SECURITY: PARTIAL** — 0 CVEs, all found defects fixed, 100% route
inventory; but coverage is representative and no independent assessment
has occurred.

**PERFORMANCE: PARTIAL** — load, spike, and 30-minute soak all passed
with zero 5xx; analytics at transaction volume unproven.

**BACKUP: FAIL** — none configured.

**RESTORE: NOT TESTED** — tooling unavailable.

**DISASTER RECOVERY: PARTIAL** — documented in full, never exercised.

**MONITORING: PARTIAL** — metrics, health, integrity and structured
logging exist; alerting, log persistence, and on-call do not.

**FINANCIAL INTEGRITY: PARTIAL** — 7 automated checks, verified
non-mutating, concurrency invariants proven; 1 HIGH finding open and
nothing schedules the checks.

**PRODUCTION DEPLOYMENT: FAIL** — no pipeline, no rollback test, no
infrastructure definition.

# OVERALL PRODUCTION READINESS: **NOT READY**

Blocked principally by the absence of backups, an untested restore, no
job scheduler, and no alerting — not by application defects.

**This report does not declare the system legally or regulatorily
compliant. It does not declare RBI compliance — technical controls
existing is not the same as regulatory compliance. It does not declare
VAPT passed. It does not declare any external audit completed.** Each of
those requires independent validation by qualified parties, and nothing
in sixteen phases of this work substitutes for it.

---

**NEXT: STOP. Per the phase brief, no further implementation phase
begins automatically. A FINAL WHOLE-SYSTEM GAP REPORT covering Phases
1–16 follows separately.**
