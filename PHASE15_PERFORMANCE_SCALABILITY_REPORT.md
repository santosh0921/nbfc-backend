# Phase 15 — 100K+ Users, High-Scale Performance, Load Testing, Database Optimization & Production Scale Readiness

**This report does not claim the application supports 100,000+ users.**
It reports what was actually measured, at what scale, and states plainly
where scale was not proven. Every number below was produced by a real
test run in this environment; nothing is estimated unless labelled
ESTIMATE.

---

## 1. Executive Summary

The phase found and fixed **one catastrophic scale bottleneck** and **one
latent financial data-integrity bug**, and produced measured evidence for
concurrency correctness and load behaviour.

**Headline finding**: `GET /admin/users` loaded every customer with an
unbounded query and then issued **8 further queries per customer**
(1 + 8N total). Measured at only **2,000 seeded customers: p50 = 12.97
seconds, 0.8 req/sec**. The cost is linear in customer count, so at
100,000 customers it would have been roughly fifty times worse — a
guaranteed timeout. After pagination + batch loading, on the identical
dataset and concurrency: **p50 = 1.507 ms, 5,850 req/sec**.

| Metric | Before | After | Change |
|---|---|---|---|
| p50 latency | 12,971 ms | 1.507 ms | ~8,600× faster |
| Throughput | 0.8 req/sec | 5,850 req/sec | ~7,300× |
| Heap allocated | 36.9 MiB | 7.0 MiB | ~5× lower |
| Queries per request | 1 + 8N | 8 (constant) | independent of N |

**Second finding (financial correctness, not performance)**: journal
numbers were generated with `COUNT(*)+1`. Deleting any journal entry
moved the counter backwards, so the next posting collided with an
existing `journal_number` and the unique index rejected it — and would
keep rejecting **every subsequent posting**. A single row deletion was a
permanent outage of all financial posting. Found during regression, fixed
with a real Postgres sequence, regression-tested.

**Also closed this phase**: Phase 14's outstanding dependency CVE scan.
`govulncheck` was installed and run — results in §22.

**Not proven**: analytics/report performance at the target transaction
volume (100K loans, millions of payments). See §9 and §24 — this is the
principal honest gap.

## 2. Current Architecture

Single-process Go/Gin monolith, PostgreSQL via GORM, no cache layer, no
message queue, no separate worker tier. Exports run asynchronously in
goroutines within the same process. Storage is a local AES-256-GCM
encrypted filesystem provider.

**Single points of failure** (unchanged by this phase, documented for
Phase 16): one application process, one database, in-process export
workers (a restart loses in-flight exports), and an in-memory per-IP rate
limiter (per-instance, so limits multiply if the app is horizontally
scaled).

**Connection pool** (pre-existing from Phase 7, verified this phase):
`MaxOpenConns=25`, `MaxIdleConns=10`, `ConnMaxLifetime=30m`,
`ConnMaxIdleTime=5m`, plus a server-side `statement_timeout=30000` in
the DSN so one runaway query cannot hold a pool slot indefinitely.

## 3. Baseline Performance

Measured with the Phase 15 harness (`internal/perf`), driving the **real
production route table** (`routes.RegisterRoutes`) in-process. Dataset:
2,000 seeded customers. 5-second runs.

| Scenario | Concurrent | Requests | req/sec | p50 | p95 | p99 | Errors | Heap |
|---|---|---|---|---|---|---|---|---|
| health | 50 | 3,882,978 | 770,918 | <1ms | <1ms | <1ms | 0.00% | 64.3 MiB |
| admin user list (**before fix**) | 10 | 10 | 0.8 | 12.97 s | 13.08 s | 13.08 s | 0.00% | 36.9 MiB |
| admin user list (**after fix**) | 10 | 29,257 | 5,850 | 1.507 ms | 2.787 ms | 4.563 ms | 0.00% | 7.0 MiB |
| admin loan list | 25 | 15,846 | 2,838 | 3.225 ms | 11.515 ms | 27.75 ms | 0.00% | 10.7 MiB |
| admin dashboard | 25 | 16,793 | 3,062 | 3.52 ms | 12.021 ms | 29.812 ms | 0.00% | 10.0 MiB |
| analytics growth dashboard | 10 | 21,216 | 4,241 | 2.012 ms | 4.016 ms | 6.491 ms | 0.00% | 5.0 MiB |
| executive MIS | 10 | 20,506 | 4,089 | 2.02 ms | 3.91 ms | 6.696 ms | 0.00% | 7.8 MiB |

**Critical caveat on the analytics/MIS/dashboard rows**: those endpoints
aggregate over loans, payments, and journal entries — and the test
database holds very little of that data. Those numbers demonstrate the
query shape is not pathological; they are **NOT** evidence of analytics
performance at target transaction volume. See §9.

**Harness limitation** (applies to every number in this report): the
harness drives gin in-process via `httptest`, so it measures
handler + database time. It does **not** measure network stack, TLS
termination, reverse-proxy, or load-balancer behaviour. Real end-to-end
latency will be higher than these figures by whatever that infrastructure
adds. No out-of-process load tool (k6, wrk, ab, hey, vegeta, siege) was
available in this environment — all were checked and are absent, as is
`psql`.

## 4. Database Analysis

Existing single-column indexes (from GORM model tags) already cover the
obvious foreign keys and status columns: `loan_applications.customer_id`,
`payments.{loan_id,emi_id,customer_id,status,gateway_payment_id}`,
`emi_installments.loan_id`, `disbursements.{loan_id (unique),
customer_id, status, utr}`, `journal_entries.{transaction_type,
reference_type, reference_id, period_id, status}`, plus unique indexes on
`journal_entries.{journal_number, idempotency_key}` and
`payment_events.gateway_event_id`.

What was missing was **composite and ordering** coverage for the actual
hot paths — see §6.

## 5. Query Analysis

**N+1 confirmed and fixed** (the headline finding): `ListUsersHandler`
issued, per customer, one query each for profile, address, KYC review,
plus five more inside `computeKYCCompletion` (profile, address, PAN,
Aadhaar, selfie) — 8 per row. Confirmed by reading the query log during
the baseline run, which showed exactly that repeating 8-query block per
user UUID.

**Unbounded queries**: 113 `Find(&...)` call sites without a `Limit`
were enumerated. Most are correctly bounded by a narrow `WHERE`
(`loan_id = ?` returns one loan's ~12–60 installments; `customer_id = ?`
returns one customer's handful of loans) and are fine. Two were unbounded
over tables that grow with the business and are now paginated:
`/admin/users` and `/admin/loans`. `/admin/payments` was already
paginated (Phase 9).

## 6. Index Changes

Five indexes added (`internal/admin/perf_indexes.go`), each justified by
counting the real query occurrences first — item E's instruction not to
blindly index. All created `CONCURRENTLY IF NOT EXISTS` so the migration
does not take an ACCESS EXCLUSIVE lock on a live table (item AB).
Verified applied at boot: `✅ Performance indexes verified (5/5 present)`.

| Index | Query benefit | Write cost | Justification |
|---|---|---|---|
| `idx_users_created_at` | Admin customer list orders by `created_at DESC` every page load | 1 entry per registration | The endpoint measured at 12.97s p50 |
| `idx_loan_applications_status_created_at` | Status-filtered loan list + analytics `status IN (...) AND created_at BETWEEN` | 1 composite entry per insert/status change | 10 verified occurrences |
| `idx_emi_installments_due_date` | DPD bucketing, collection-efficiency denominators, overdue sweeps on the largest transactional table | 1 entry per installment, rarely updated | 13 verified occurrences |
| `idx_emi_installments_loan_id_status` | `EXISTS(... loan_id = ? AND status != 'paid')` closed/overdue derivation | 1 composite entry, updated once when paid | 12 verified occurrences |
| `idx_journal_entries_transaction_date` | Every P&L / revenue / expense / funding aggregation filters by date range on the fastest-growing table | 1 entry per posting, append-only | 10 verified occurrences |

## 7. API Performance

See §3. Classification against this application's own requirements (admin
back-office screens, not a consumer-facing hot path):

- **Excellent** (p95 < 50 ms at 25 concurrent): health, admin user list
  (post-fix), admin loan list, admin dashboard, analytics growth,
  executive MIS.
- **Critical bottleneck (fixed)**: admin user list pre-fix.

No arbitrary "industry standard" threshold was invented; the above bands
reflect what is comfortable for an internal admin UI.

## 8. Dashboard Performance

Admin dashboard measured at 3.52 ms p50 / 3,062 req/sec at 25 concurrent
on the current dataset. **No caching, summary tables, or materialized
views were introduced.** Item H explicitly warns against adding
materialized views merely because Phase 13 mentioned them, and the
measurements show no dashboard bottleneck at current data volume — so
adding an invalidation-bearing cache layer would have added correctness
risk (stale financial figures) to solve a problem that measurement does
not show exists. Revisit when §9's scale gap is closed with real data.

## 9. Analytics Performance — ⚪ NOT PROVEN AT TARGET SCALE

Item I asks for testing against 100K customers, 500K+ applications, 100K+
loans, and millions of payments/transactions.

**What was actually tested**: 2,000 seeded customers with profile and
address rows. **No loans, EMI schedules, payments, or journal entries
were seeded at volume.**

Analytics endpoints therefore returned in ~2 ms because they aggregated
over a nearly-empty transaction set. **These numbers must not be read as
analytics being fast at scale.** The aggregation queries are server-side
`SUM`/`COUNT`/`GROUP BY` (verified in Phase 13) and now have supporting
date-range indexes (§6), which is the right shape — but shape is not
proof.

Marked **NOT TESTED AT TARGET SCALE — ENVIRONMENT/TIME LIMITATION**.
Seeding a realistic multi-million-row transaction set with valid
double-entry journals is itself substantial work (every payment needs a
balanced posting to be realistic) and was not completed.

## 10. Export Performance

Exports already run asynchronously (`generateExportAsync` in a goroutine,
status tracked QUEUED → RUNNING → READY/FAILED), so a large export does
not block its HTTP request — item J's core requirement was already met by
Phase 6/8. Export requests are rate-limited (`report.export`, 0.2 rps /
burst 3).

**Item K gaps — NOT implemented this phase**: there is no maximum row
count, maximum date range, maximum concurrent exports per user, maximum
file size, or per-export execution timeout. An authorized admin can still
request an export spanning all time. The rate limiter bounds *frequency*,
not *size*. Documented as a remaining risk (§27).

## 11. Background Jobs

Not modified this phase. Existing jobs (daily risk job, compliance
sweep, broken-promise marking) were reviewed for idempotency in their own
phases. No new scheduler, queue, or worker tier was introduced — doing so
is Phase 16 infrastructure work, and the measurements here do not show
in-process execution failing at current volume.

## 12. Payment Concurrency — 🟢 VERIFIED

Item M is flagged CRITICAL by the brief. Four concurrency invariants were
tested with goroutines released simultaneously from a barrier channel to
maximize real contention. These run in the **normal** regression suite
(not behind the opt-in PERF flag) because correctness under concurrency
is a permanent invariant, not a benchmark.

| Test | Setup | Result |
|---|---|---|
| Exactly-once journal posting | 16 goroutines, same idempotency key | **1** journal entry (1 success, 15 deduplicated) ✅ |
| Duplicate webhook replay | 12 concurrent inserts, same `gateway_event_id` | **1** row stored, rest rejected by unique index ✅ |
| Double disbursement | 10 concurrent disbursements on one loan | **1** row (unique index on `loan_id`) ✅ |

## 13. Accounting Concurrency — 🟢 VERIFIED

| Test | Setup | Result |
|---|---|---|
| Balanced entries under load | 24 concurrent distinct postings | All 24 balanced, debits == credits ✅ |
| Journal numbering after deletion | post → delete → post | Advances without reuse ✅ (after fix, §26) |

## 14. Load Tests

`/admin/users?limit=25`, 5-second runs, 2,000-customer dataset:

| Concurrent users | Requests | req/sec | p50 | p95 | p99 | Error rate | Heap |
|---|---|---|---|---|---|---|---|
| 100 | 28,393 | 5,667 | 14.346 ms | 34.266 ms | 57.614 ms | 0.00% | 13.0 MiB |
| 500 | 30,667 | 6,056 | 71.267 ms | 174.155 ms | 243.714 ms | 0.00% | 15.1 MiB |
| 1,000 | 25,822 | 4,989 | 171.235 ms | 425.762 ms | 593.838 ms | 0.00% | 14.8 MiB |
| 2,500 | 22,191 | 4,100 | 429.185 ms | 1.529 s | 2.091 s | 0.00% | 22.4 MiB |

Throughput holds at roughly 4,000–6,000 req/sec across the range while
latency grows approximately linearly with concurrency — the expected
signature of a saturated but stable queue, not collapse. **Zero 5xx at
every level.**

## 15. Spike Tests

Sudden step changes, 4-second steps, measuring recovery (item U):

| Step | Concurrent | req/sec | p50 | p95 | p99 | Errors |
|---|---|---|---|---|---|---|
| 1 | 100 | 5,417 | 14.892 ms | 34.752 ms | 63.469 ms | 0.00% |
| 2 | 1,000 | 3,920 | 208.704 ms | 571.099 ms | 825.758 ms | 0.00% |
| 3 | **5,000** | 3,357 | 1.135 s | 3.065 s | 4.225 s | 0.00% |
| 4 (recovery) | 100 | 4,191 | 20.088 ms | 46.297 ms | 71.903 ms | 0.00% |

**Result**: survived a 50× spike to 5,000 concurrent with zero errors,
and recovered to ~20 ms p50 immediately afterwards. No connection
exhaustion, no error accumulation, no memory blow-up (peak 30 MiB).

## 16. Soak Tests — ⚪ NOT TESTED

**NOT TESTED — ENVIRONMENT LIMITATION.** Longest continuous run performed
was ~5 seconds per scenario. No long-duration soak was executed, so
memory growth, connection leaks, and error accumulation over hours are
**unmeasured**. Per item V, no 24-hour claim is made. The 5-second runs
showed stable heap and a constant 16 goroutines, which is a weak positive
signal, not soak evidence.

## 17. Database Load

Exercised indirectly by §14/§15 (every request hits Postgres). No
connection exhaustion or pool starvation occurred at 5,000 concurrent
against a 25-connection pool — requests queued for a connection rather
than failing. Direct DB-side instrumentation (DB CPU, lock waits,
deadlock counts, `pg_stat_statements`) was **NOT** captured: `psql` is not
available in this environment and no DB monitoring endpoint exists.
Marked NOT TESTED for the DB-internal metrics specifically.

## 18. Deadlocks

No deadlock was observed across all concurrency and load runs. The
posting path takes a transaction-scoped advisory lock (`pg_advisory_xact_lock`)
for journal-number allocation, held briefly and released at
commit/rollback, then inserts — a consistent lock ordering that does not
invite cycles. No deadlock-prone pattern was confirmed, so per item X
("fix only confirmed issues") nothing was changed here.

## 19. Memory

Heap allocated stayed between 5.0 and 64.3 MiB across every scenario,
including 5,000 concurrent. The user-list fix reduced that endpoint's
heap from 36.9 to 7.0 MiB by no longer materializing the entire customer
table per request. No unbounded buffer or leak was observed within the
tested durations (see §16 — long-run leak behaviour is unmeasured).

## 20. Connection Pool

Verified present and effective (§2). At 5,000 concurrent against 25 max
connections, requests queued rather than erroring — 0% error rate proves
connections were returned correctly and no leak occurred within the run.
**Sizing recommendation**: 25 is conservative and correct for a single
instance; a multi-instance deployment must divide the database's
`max_connections` across instances rather than raising this per-instance
value independently.

## 21. Frontend Performance — ⚪ NOT ASSESSED

No Flutter/admin frontend performance work was done, consistent with
every prior phase's backend-only scope. **One breaking change affects the
frontend**: `GET /admin/loans` now returns `{loans, pagination}` instead
of a bare JSON array, and `GET /admin/users` gained a `pagination` block
while `total` changed meaning from "rows in this response" to "rows in
the full result set". Any client consuming those endpoints must be
updated.

## 22. Dependency CVE Scan — Phase 14 residual item, NOW CLOSED

`govulncheck` was installed and run against the whole module. Result:
**6 vulnerabilities affecting called code, all in the Go standard
library**, none in this project's own code or its direct dependencies:

| ID | Component | Affected | Fixed in |
|---|---|---|---|
| GO-2026-6218 | `net/url` quadratic complexity in `resolvePath` | go1.26.5 | go1.26.6 |
| GO-2026-6090 | `crypto/tls` post-handshake message limit | go1.26.5 | go1.26.6 |
| GO-2026-6089 | `net/http` ReadHeaderTimeout on unencrypted HTTP/2 check | go1.26.5 | go1.26.6 |
| GO-2026-6088 | `encoding/xml` recursion depth guard | go1.26.5 | go1.26.6 |
| GO-2026-5972 | `encoding/asn1` max recursion depth | go1.26.5 | go1.26.6 |
| GO-2026-5026 | `x/net/idna` Punycode label rejection (via `net/http`) | go1.26.5 | go1.26.6 |

**Remediation**: all six are fixed by upgrading the Go toolchain from
1.26.5 to **1.26.6 or later** and rebuilding. No application code change
is required. This was **not** performed in this phase — upgrading the
toolchain in this environment was out of scope for a performance phase
and should be done as a deliberate, separately-regression-tested change.
It is the single highest-priority remaining security action.

Two further vulnerabilities exist in imported packages and one in a
required module, but govulncheck confirms this codebase does not call
them.

## 23. Capacity Model

**CURRENT TESTED CAPACITY** (measured, this environment, 8 CPU cores):
- 2,000 customers in the dataset.
- ~4,000–6,000 req/sec sustained on a paginated admin list endpoint.
- 5,000 concurrent connections handled with 0% errors.
- 25 database connections sufficient at that concurrency.
- Peak heap 64.3 MiB.

**ESTIMATED CAPACITY** (extrapolation — explicitly NOT proven):
- The user-list endpoint's cost is now independent of customer count
  (constant 8 queries/request), so 100,000 customers should behave like
  2,000 for that endpoint **provided** the new `idx_users_created_at`
  keeps the ordered page cheap. Deep offset paging (page 1,000+) will
  degrade — see §27.
- Analytics/report endpoints scale with transaction volume, which was
  **not** tested at volume. Their capacity is **unknown**, not estimated.
- Registered users ≠ concurrent users: 100,000 registered customers with
  a typical 1–2% concurrency implies 1,000–2,000 concurrent, which is
  within the tested range for the endpoints tested.

## 24. 100K Readiness

| Dimension | Verdict | Basis |
|---|---|---|
| 100K customers | **READY (for the endpoints fixed and tested)** | User-list cost is now constant per request, measured 5,850 req/sec; but only 2,000 customers were actually seeded, so this is measured-plus-reasoned, not measured-at-100K |
| 1M transactions | **NOT PROVEN** | No transaction volume was seeded (§9) |
| 10M transactions | **NOT PROVEN** | Same |
| High concurrency | **READY** | 5,000 concurrent, 0% errors, clean recovery (§15) |
| Large reports | **NOT READY** | Exports are async (good) but have no row/date-range/size/concurrency caps (§10, item K) |
| Financial transaction concurrency | **READY** | Exactly-once posting, webhook replay protection, double-disbursement prevention, balance-under-load all verified (§12, §13) |

## 25. Bottlenecks

1. **`GET /admin/users` N+1 + unbounded fetch** — FIXED (§1).
2. **`GET /admin/loans` unbounded fetch** — FIXED (paginated).
3. **Export resource limits absent** — OPEN (§10).
4. **Analytics at transaction volume** — UNKNOWN, untested (§9).
5. **Deep offset pagination** — known limitation, acceptable for admin
   screens, documented in `internal/pagination`.

## 26. Fixes

| Fix | Type | Evidence |
|---|---|---|
| Paginated + batch-loaded `ListUsersHandler` | Performance | 12.97 s → 1.507 ms p50, measured on identical data |
| Paginated `AdminListLoansHandler` | Performance | Unbounded → bounded page |
| Shared `internal/pagination` contract (default 25, hard max 200) | Performance | Applied to both endpoints |
| 5 justified indexes, created CONCURRENTLY | Performance | 5/5 verified at boot |
| Journal numbering: `COUNT(*)+1` → Postgres sequence | **Financial correctness** | `TestJournalNumberingSurvivesDeletion` |
| `govulncheck` installed and run | Security (Phase 14 residual) | §22 |

**On the journal-numbering fix** (item AE — optimization must never
sacrifice correctness): the first attempt used `MAX(suffix)+1`, which
fixed the outage but still reused a number when the *highest* entry was
deleted. The regression test caught that, and the fix became a real
Postgres sequence — non-transactional, never decreasing, concurrency-safe
on its own. Deletions now leave gaps, which is correct for an audit
sequence: a missing number is evidence of removal, a reused number
silently rewrites history.

## 27. Remaining Risks

- **Analytics/reporting at target transaction volume is unmeasured**
  (§9) — the largest gap in this phase.
- **No soak test** (§16) — leaks and drift over hours are unknown.
- **No export size/range/concurrency limits** (§10, item K).
- **Go toolchain CVEs open** (§22) — fix is a 1.26.6 upgrade.
- **No DB-internal metrics captured** (§17) — no `psql`/monitoring access.
- **No out-of-process load testing** — all figures exclude network/TLS/
  proxy overhead.
- **Deep offset pagination** degrades on large tables; keyset paging
  would be needed for whole-table walks.
- **In-memory rate limiter is per-instance** — limits multiply under
  horizontal scaling (Phase 16 concern).
- **Frontend breaking change** from the two paginated endpoints (§21).
- **Phase 14 residual still open**: security route coverage remains
  representative, not exhaustive across all ~250+ admin routes. This
  phase did not extend it.

## 28. Production Recommendations

1. Upgrade Go to 1.26.6+ and rebuild (closes all six CVEs).
2. Seed a realistic transaction-volume dataset and re-run the analytics
   scenarios before claiming any analytics scale readiness.
3. Add export caps (max rows, max date range, max concurrent per user,
   max file size, execution timeout).
4. Run a multi-hour soak before go-live.
5. Size `MaxOpenConns` against the database's `max_connections` divided
   by instance count.
6. Update frontend clients for the two paginated endpoints.
7. Move to a shared rate limiter (or accept per-instance semantics) if
   running more than one instance.
8. Capture DB-side metrics (`pg_stat_statements`, lock/deadlock counts)
   in the production environment, which this one could not.

---

# FINAL STATUS

## Performance classification

| Area | Status |
|---|---|
| Architecture audit | 🟢 COMPLETE |
| Baseline measurement | 🟢 COMPLETE |
| Database/index optimization | 🟢 COMPLETE |
| Query analysis / N+1 | 🟢 COMPLETE |
| Pagination | 🟡 PARTIAL (2 critical endpoints fixed; a full sweep of all list endpoints not done) |
| Dashboard performance | 🟢 COMPLETE (no bottleneck found at current volume) |
| Analytics at scale | ⚪ NOT TESTED at target volume |
| Export limits | 🔴 MISSING (item K) |
| Background jobs | 🟡 PARTIAL (unchanged; no new worker tier) |
| Payment concurrency | 🟢 COMPLETE |
| Accounting concurrency | 🟢 COMPLETE |
| Load tests | 🟢 COMPLETE (to 2,500) |
| Spike tests | 🟢 COMPLETE (to 5,000) |
| Soak tests | ⚪ NOT TESTED |
| Database load (DB-side metrics) | ⚪ NOT TESTED |
| Deadlocks | 🟢 COMPLETE (none confirmed) |
| Memory | 🟢 COMPLETE (within tested durations) |
| Connection pool | 🟢 COMPLETE |
| Frontend performance | ⚪ NOT ASSESSED |
| Observability | 🔴 MISSING (items AC/AD — no metrics interface or correlation IDs added this phase) |
| Capacity model | 🟢 COMPLETE (tested vs estimated clearly separated) |

**100K CUSTOMERS: READY** — for the endpoints fixed and tested; the
user-list bottleneck that made it impossible is removed and per-request
cost is now independent of customer count. Not measured at a literal
100,000-row dataset.

**HIGH CONCURRENCY: READY** — 5,000 concurrent, 0% errors, clean
recovery.

**FINANCIAL TRANSACTION LOAD: READY** — exactly-once posting, webhook
replay protection, double-disbursement prevention, and balance-under-load
all verified under real contention.

**LARGE REPORTS: NOT PROVEN** — exports are asynchronous, but have no
size/range/concurrency caps and were not tested at volume.

# PHASE 15 STATUS: **PARTIAL**

Two significant defects found and fixed with measured evidence, plus
verified concurrency correctness and load/spike behaviour. PARTIAL
because analytics at target transaction volume, soak testing, export
limits, and observability foundations were not completed — all listed
above rather than quietly omitted.

## REMAINING FROM PHASE 14
- **Dependency CVE scan: NOW COMPLETE** (§22) — but the six Go stdlib
  CVEs it found are **OPEN**, pending a Go 1.26.6+ toolchain upgrade.
- **Security route coverage remains representative, not exhaustive**
  across all ~250+ admin routes — still open, not addressed this phase.
- Missing `Referrer-Policy`/`Permissions-Policy` headers; no
  OpenAPI/Swagger inventory; business-logic/tax/compliance attack
  scenarios beyond those spot-checked; no live infrastructure/SSRF
  probing.

## REMAINING FROM PHASE 15
- Analytics/report performance at 100K loans / millions of payments
  (NOT TESTED).
- Soak/long-duration testing (NOT TESTED).
- Export row/date-range/size/concurrency limits (item K, MISSING).
- Observability metrics interface and request correlation IDs (items
  AC/AD, MISSING).
- Full pagination sweep across all remaining list endpoints.
- Out-of-process load testing including network/TLS/proxy overhead.
- DB-side metrics capture (locks, deadlocks, slow-query stats).
- Frontend performance assessment, and updating clients for the two
  paginated endpoints.

**NEXT PHASE: PHASE 16 — PRODUCTION INFRASTRUCTURE, DEPLOYMENT, MONITORING, BACKUP, DISASTER RECOVERY, HIGH AVAILABILITY & PRODUCTION GO-LIVE READINESS**

**AFTER PHASE 16: a FINAL WHOLE-SYSTEM AUDIT is required. Completing all
16 phases does NOT mean this NBFC is legally or regulatorily compliant —
that determination requires independent legal, CA, and regulatory review,
which no phase of this work substitutes for.**
