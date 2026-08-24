# Phase 16.1 — Production Blocker Resolution Report

Scope: resolve or conclusively characterize the three production blockers
from Phase 16. **Phase 17 was not started.**

---

# Executive Summary

All three blockers were investigated to root cause. Two are now
**resolved and tested with real evidence**; one is **fully explained with
proof** but leaves a residual operator action.

| Blocker | Outcome |
|---|---|
| **1. Backup + Restore** | **RESOLVED AND TESTED.** Phase 16's "tooling unavailable" conclusion was **wrong** — PostgreSQL 16.14 is installed natively; its binaries simply were not on `PATH`. A real backup was taken (128,126,510 bytes, 14 s, SHA-256 recorded) and really restored into an isolated database (81 s), with all 11 sampled tables matching row-for-row including 3,568,431 audit rows. |
| **2. Scheduler** | **RESOLVED IN APPLICATION; EXTERNAL DEPLOYMENT PENDING.** Built a job runner with PostgreSQL advisory-lock overlap protection, execution records, run-key idempotency, panic containment, and staleness monitoring. **No in-process ticker was added.** 7 jobs registered; end-to-end execution verified over HTTP. Wiring an actual external scheduler remains infrastructure work. |
| **3. ₹380K vs ₹715K discrepancy** | **ROOT CAUSE PROVEN. ₹0 UNEXPLAINED.** Every rupee classified: ₹340,000 VALID, ₹420,000 ORPHAN LEDGER ENTRY, ₹60,000 MISSING LEDGER ENTRY. All 36 anomalous records are **proven test residue** — one class is attributable to a named Go test function, the rest reference loan records that no longer exist. Nothing was deleted or altered. |

Two additional real defects were found and fixed while doing this work
(see *Changes Implemented*).

**Overall: NOT READY** — because backup off-host replication, the
external scheduler deployment, and several Phase 16 gaps remain
operator/infrastructure actions.

---

# Starting Blockers

As handed over from Phase 16:

1. **BACKUP FAIL** — nothing configured, nothing verified.
2. **RESTORE NOT TESTED** — believed impossible in this environment.
3. **HIGH financial integrity finding** — disbursements ₹380,000 vs
   ledger ₹715,000, suspected but unproven test residue.
4. **No scheduler** — every daily financial/compliance job manual-only.

---

# Repository / Infrastructure Findings

Discovery was performed before any modification.

**Deployment architecture**: a single Go binary plus one PostgreSQL
database. **No infrastructure is defined in the repository** — no
Dockerfile, no docker-compose, no Terraform, no Kubernetes manifest, no
`Procfile`, no `render.yaml`, no CI configuration. Confirmed by
filesystem search.

**Database**: PostgreSQL **16.14**, running as a **native Windows
service** (`postgresql-x64-16`), installed at
`C:\Program Files\PostgreSQL\16`, listening on `localhost:5433`.
Database `nbfc_db`, size **1069 MB**.

**This is a development database.** Evidence: localhost host; the three
admin accounts are the dev bootstrap set (`admin`, `checker1`,
`checker2`); dev-seeded branch/agency fixtures are present; 3.5 M
rate-limit audit rows originate from the Phase 16 soak test.
**No production database exists** — consistent with Phase 16's finding
that no production infrastructure exists at all.

**Critical correction to Phase 16**: Phase 16 concluded a restore test
was impossible because `pg_dump`/`pg_restore`/`psql` were "unavailable".
That was a **PATH problem, not an availability problem**. All PostgreSQL
client binaries are present. This single mis-diagnosis is what left the
mandatory restore test untested for a whole phase, and it is why this
phase re-verified the premise rather than accepting it.

**Scheduler search** (`cron`, `scheduler`, `ticker`, `time.NewTicker`,
`time.AfterFunc`, `worker`, `queue`, `job`, `retry`): the only ticker in
the codebase is a WebSocket keep-alive in `internal/signaling` —
unrelated. **No scheduling mechanism of any kind existed.** A
`RiskJobRun` model existed, tracking only the risk job.

**Backup search** (`backup`, `restore`, `pg_dump`, `pg_restore`, `WAL`):
**zero** backup-related code or configuration existed outside prose in
prior phase reports.

---

# Backup Architecture

`scripts/backup_postgres.sh` — `pg_dump --format=custom --compress=6`.

**Why a logical dump and not a provider snapshot**: this deployment uses
self-managed PostgreSQL reached over host/port credentials. There is no
cloud provider and therefore no managed-snapshot facility to configure.
The script's header states explicitly that if the database moves to
managed PostgreSQL (RDS/Aurora/Cloud SQL/…), the provider's automated
snapshots plus PITR become the **primary** mechanism and this script
becomes a secondary portable export — it must not be mistaken for a
replacement.

Design choices and their reasons:

| Choice | Reason |
|---|---|
| `--format=custom` | Restorable selectively with `pg_restore`; a plain SQL dump cannot be restored table-by-table |
| `--no-owner --no-privileges` | The restore target may use a different role; role mismatch is a common restore failure |
| SHA-256 sidecar | Lets a later restore prove it used an unmodified artifact |
| JSON manifest | Evidence record (id, size, duration, checksum, retention) containing **no credentials** |
| Retention matched on `<db>_*` | Cannot delete an unrelated file sharing the directory |
| Credentials from environment only | Nothing hard-coded; `PGPASSWORD` exported for the process lifetime and unset via `trap` |

**Deliberately NOT done, and stated in the script**: no upload to
off-host storage. A backup on the same disk as the database is not a
backup. No object storage is configured in this deployment, so off-host
replication is flagged as an operator responsibility rather than faked.

---

# Backup Test Evidence

**Actually executed** — not a configuration claim.

| Field | Value |
|---|---|
| Backup ID | `nbfc_db_20260820T045308Z` |
| Database | `nbfc_db` @ `localhost:5433` (PostgreSQL 16.14) |
| Mechanism | `pg_dump --format=custom --compress=6` |
| Source DB size | 1069 MB |
| Backup size | **128,126,510 bytes** (~122 MB compressed) |
| Duration | **14 seconds** |
| SHA-256 | `7fd774bbcc8d841f04a5fb7fc4b59c9d16292b8f6864333f5e04110bde6601e9` |
| Retention configured | 30 days |
| Status | **SUCCESS** |

Artifacts written to `C:\Users\Administrator\nbfc-backups\` (outside the
repository; `.gitignore` updated so dumps can never be committed).

**Gate: BACKUP CREATION = PASS.**

---

# Restore Test Evidence

**Actually executed into an isolated database.** The live database was
never touched.

`scripts/restore_verify.sh` enforces two **hard refusals**: the target
must differ from `DB_NAME`, and must contain `_restore_test`. "Restore
test overwrote production" is a real catastrophic operator error, so
these are code-level checks, not conventions.

| Step | Result |
|---|---|
| Checksum verification | **OK** (`nbfc_db_20260820T045308Z.dump: OK`) |
| `pg_restore` exit code | **0** |
| Restore duration | **81 seconds** |
| Tables restored | **106** |
| Indexes restored | **276** |
| Constraints restored | **559** |
| Triggers restored | **2** |

**Row-count comparison — source vs restored, all matched:**

| Table | Source | Restored | Match |
|---|---|---|---|
| users | 67 | 67 | OK |
| loan_applications | 62 | 62 | OK |
| disbursements | 120 | 120 | OK |
| payments | 0 | 0 | OK |
| emi_installments | 1,179 | 1,179 | OK |
| journal_entries | 480 | 480 | OK |
| journal_entry_lines | 960 | 960 | OK |
| ledger_accounts | 20 | 20 | OK |
| **audit_logs** | **3,568,431** | **3,568,431** | OK |
| admin_users | 3 | 3 | OK |
| report_exports | 0 | 0 | OK |

**Financial consistency inside the restored database:**
unbalanced journal entries = **0**; duplicate journal numbers = **0**;
loans with multiple successful disbursements = **0**.

**Audit protection survived the restore**: `audit_logs` triggers restored
= **2** (the append-only UPDATE/DELETE rejection). This matters — a
restore that silently dropped the immutability trigger would look
successful while removing a key control.

**Application smoke test against the restored database:**

| Check | Result |
|---|---|
| Application boots against restored DB | **PASS** — `Database connected successfully`, `Server starting on port 8099` |
| Performance indexes present | **PASS** — 5/5 verified |
| `GET /health` | **PASS** — `{"dependencies":{"database":"ok"},"status":"ok"}` |
| `GET /readiness` | **PASS** — `{"checks":{"database":"ok"},"status":"ok"}` |
| Admin login (authentication against restored data) | **PASS** — token issued |
| `GET /admin/ops/integrity` executes | **PASS** — checks ran |
| `GET /admin/ops/reconciliation` | **PASS** — identical to source (₹0 unexplained) |

Script verdict: **`RESTORE_VERIFICATION=PASS`**, `TABLE_COUNT_MISMATCHES=0`.

The isolated restore database was dropped afterwards to reclaim disk; the
test is repeatable via the documented command.

**Gate: RESTORE = PASS.**

---

# RPO

**Measured capability, not an achieved target.**

| | Value |
|---|---|
| Business RPO target | **NOT DEFINED** — management has not set one (carried from Phase 16) |
| Current capability | **Bounded only by backup frequency, which is not yet scheduled** |
| If backup runs daily | RPO ≈ **24 hours** |
| If backup runs hourly | RPO ≈ **1 hour** (backup takes 14 s, so hourly is comfortably feasible) |
| With WAL archiving / PITR | **NOT CONFIGURED** — would reduce RPO to minutes |

**Gate: RPO = BUSINESS VALIDATION REQUIRED.** Capability is now
measurable and adjustable; no target exists to measure it against, and
no claim of achievement is made.

---

# RTO

**Measured on real data.**

| Component | Measured |
|---|---|
| Backup creation | **14 s** |
| Restore (`pg_restore`, 4 parallel jobs, 1069 MB source) | **81 s** |
| Application boot against restored DB | **~15 s** |
| **Total measured database recovery** | **~96 s** from dump file to serving application |

**Not included** (and therefore not claimed): time to detect the
incident, decide to recover, provision a replacement host, and route
traffic. Real-world RTO will exceed 96 s by however long those take —
and there is no deployment automation, so provisioning is manual.

| | Value |
|---|---|
| Business RTO target | **NOT DEFINED** |
| Measured database recovery | **96 seconds** |

**Gate: RTO = PASS (measured), target = BUSINESS VALIDATION REQUIRED.**

---

# Backup Security

| Control | Status | Evidence |
|---|---|---|
| No credentials in repository | **PASS** | Scripts read `DB_PASSWORD` from environment only; grep confirms no literal credential |
| No credentials in logs | **PASS** | Script echoes host/port/db name only; `PGPASSWORD` never printed |
| Credential lifetime minimized | **PASS** | `export PGPASSWORD` + `trap 'unset PGPASSWORD' EXIT` |
| No credentials in manifest | **PASS** | Manifest contains id/size/duration/checksum/retention only |
| Artifact integrity | **PASS** | SHA-256 sidecar written and verified on restore |
| Backups excluded from version control | **PASS** | `.gitignore` updated (`backups/`, `*.dump`, `*.sha256`, `*.manifest.json`) |
| Retention policy | **PASS** | 30 days default, configurable, pattern-scoped deletion |
| **Encryption at rest** | **EXTERNAL VALIDATION REQUIRED** | Depends on the storage volume; not implemented in-script |
| **Off-host replication** | **NOT IMPLEMENTED** | No object storage configured. **A backup on the database's own disk is not a backup.** |
| **Access control on backup destination** | **EXTERNAL VALIDATION REQUIRED** | Filesystem/IAM concern |
| **Deletion protection / object lock** | **NOT IMPLEMENTED** | Requires storage that supports it |
| **Backup failure alerting** | **NOT IMPLEMENTED** | Requires the alerting layer Phase 16 identified as missing |

**Gate: BACKUP SECURITY = PARTIAL.** In-script controls pass; every
storage-dependent control remains external.

---

# Scheduler Architecture

**Constraint honoured: no in-process ticker.** `internal/jobs` contains
no `time.NewTicker`, no `time.AfterFunc`, and no background loop. With N
instances an in-process timer fires N times, and a financial job running
N times is precisely the failure this design prevents.

**Chosen architecture**: external scheduler → authenticated HTTP
endpoint → application-enforced safety.

```
POST /admin/ops/jobs/{name}/run?source=SCHEDULER
Authorization: Bearer <admin JWT with RISK_RULE_MANAGE>
```

Three layers of protection, all in the database so they hold across
instances:

1. **Distributed lock** — `pg_try_advisory_lock` keyed on an FNV-1a hash
   of the job name, taken on a **dedicated connection** (advisory locks
   are session-scoped; using the pool would hand the unlock to a
   different connection and leak the lock).
2. **Execution record** — every attempt writes a `job_executions` row,
   including skips and failures. A job that failed or never ran is now
   visible rather than silent.
3. **Run key** — jobs that must run once per period declare one; a
   **unique index** enforces it, not a racy application check.

**Scheduler status-code contract** (designed for machines):

| Status | Meaning | Scheduler action |
|---|---|---|
| 202 | Ran successfully | Success |
| **409** | Another instance holds the lock, or already ran this period | **Treat as SUCCESS.** Do not retry, do not alert. |
| 404 | Unknown job | Alert — misconfiguration |
| 500 | Job genuinely failed | Alert and investigate |

**Monitoring**: `GET /admin/ops/jobs/stale?hours=26` returns any job not
successfully completed in the window. Verified live: **`healthy=false`,
7 stale jobs** — correct, since no scheduler is configured yet.

Wiring examples (systemd timer, Kubernetes CronJob, cloud scheduler,
Windows Task Scheduler) are documented in
`scripts/README_OPERATIONS.md`, including the instruction to create a
**dedicated scheduler admin account** holding only `RISK_RULE_MANAGE` —
never a human's credentials, since job executions are audited by actor.

**Gate: SCHEDULER = PARTIAL.** Application side complete and tested;
external scheduler deployment is infrastructure work not performed here.

---

# Background Job Inventory

Seven jobs registered. Verified live via `GET /admin/ops/jobs`.

| Job | Purpose | Frequency | Current trigger | Required trigger | Idempotent | Lock | Retry | Alert | Financial | Audited |
|---|---|---|---|---|---|---|---|---|---|---|
| `risk.daily` | DPD recompute, bucket refresh, classification/provisioning, broken promises | Daily 01:00 | Manual POST | Scheduler | Yes (+ run key, once/day) | Yes | No (next day picks up) | Yes | **Yes** | Yes |
| `risk.consistency` | Loan/ledger consistency exceptions | Daily, after `risk.daily` | Manual POST | Scheduler | Yes | Yes | Yes | Yes | **Yes** | Yes |
| `collections.broken_promises` | Flag lapsed payment promises | Daily | Manual POST | Scheduler | Yes | Yes | Yes | Yes | **Yes** | Yes |
| `compliance.sweep` | Overdue calendar items/CAPAs, due-soon alerts | Daily | Manual POST | Scheduler | Yes | Yes | Yes | Yes | No | On failure |
| `analytics.data_quality` | Data-quality exception detection | Daily | Manual POST | Scheduler | Yes | Yes | Yes | Yes | No | On failure |
| `analytics.risk_signals` | Review-signal detection | Daily | Manual POST | Scheduler | Yes | Yes | Yes | Yes | No | On failure |
| `integrity.financial` | All financial invariants (read-only) | Hourly + post-deploy | Manual GET | Scheduler | Yes | Yes | Yes | **Yes** | Reports only | On failure |

**Deliberately NOT automated**, with reasons (per "do not assume every
job should be automated"):

| Not automated | Reason |
|---|---|
| Report/export generation | Already async and user-initiated; a timer would produce reports nobody requested |
| Write-off approval, sanction approval, disbursement execution | Maker-checker decisions requiring human judgement. **Automating an approval defeats the control.** |
| Retention/data deletion | Retention periods are seeded UNVALIDATED at 0 years. Automating deletion against unvalidated policy could destroy legally-required records. Stays manual until compliance configures it. |
| Notification dispatch | Happens inline with the events producing it; not batched |

---

# Idempotency Controls

Every registered job is asserted idempotent by test
(`TestAllRegisteredProductionJobsAreIdempotent` fails the build if a job
is registered without declaring it).

Answers to the required questions:

| Question | Answer |
|---|---|
| Runs twice? | Second run is either blocked by the advisory lock (concurrent) or by the run key (same period), or is naturally idempotent — detection jobs use dedupe keys established in Phases 5/13 |
| Runs three times? | Same as twice; no accumulating effect |
| Crashes halfway? | The execution row remains `RUNNING` with no `completedAt`; the lock releases when the connection closes; the next run proceeds. Work already committed stays committed — jobs commit incrementally rather than in one giant transaction |
| Two workers simultaneously? | Impossible — advisory lock. **Proven**: 8 concurrent invocations → 1 ran, 7 skipped, max simultaneous = **1** |

Mechanisms used: PostgreSQL advisory locks, unique index on run key,
execution records, and the pre-existing dedupe keys in the risk-signal
and data-quality detectors. The instruction *not* to rely solely on a
non-atomic "if already processed then skip" is honoured — the run-key
check is a database unique-constraint violation, not an application read
followed by a write.

**Gate: JOB IDEMPOTENCY = PASS.**

---

# Job Overlap Protection

**Chosen behaviour: SKIP.** For daily financial jobs, skipping is
correct — the next scheduled fire picks up the work, whereas queueing
would stack overlapping runs of an already-slow job.

**Proven by test** (`TestConcurrentRunsDoNotOverlap`): 8 concurrent
invocations of a deliberately-blocking job → **1 ran, 7 skipped, maximum
simultaneous executions = 1**.

**Skips are recorded** (`TestSkippedRunsAreRecorded`): contention is
visible in `job_executions` as `SKIPPED_LOCKED`, distinct from failures
so it never inflates failure counts or triggers false alerts.

**Gate: JOB OVERLAP PROTECTION = PASS.**

---

# Retry / Failure Handling

| Scenario | Behaviour | Tested |
|---|---|---|
| Job returns an error | Recorded `FAILED` with error text and completion time; `job.failures` metric incremented; audit entry written | **PASS** (`TestFailedJobIsRecordedNotSilent`) |
| Job panics | Contained — converted to an error, recorded `FAILED`. One broken job cannot take down the API | **PASS** (`TestPanickingJobIsContained`) |
| Concurrent execution | Skipped and recorded | **PASS** |
| Duplicate execution in same period | Rejected by unique index | **PASS** (`TestRunKeyPreventsSecondRunInSamePeriod`) |
| Never ran / stale | Surfaced by `StaleJobs` | **PASS** (`TestStaleJobsReportsNeverRunJobs`) |
| Worker crash mid-run | Lock released on connection close; row left `RUNNING` as evidence | Reasoned, **NOT independently tested** |
| Database failure during a job | Job errors and is recorded `FAILED` | **NOT independently tested** |
| Network/provider timeout | Job errors and is recorded `FAILED` | **NOT independently tested** |

**Retry policy is deliberately external.** A financial job that timed out
may still be running; blind retries are the exact hazard this design
avoids. The scheduler contract makes 409 a success, so a retry that
collides is harmless — but the recommended configuration (documented) is
`--retry 0`, letting the next scheduled fire handle it. No exponential
backoff or dead-letter queue was built: with at-most-one execution and a
next-fire-picks-up model, neither is needed, and adding a queue would be
unnecessary infrastructure.

**Gate: RETRY/FAILURE HANDLING = PARTIAL** — core paths tested; crash
and infrastructure-failure scenarios reasoned but not independently
exercised.

---

# Financial Reconciliation Investigation

Built `internal/integrity/reconciliation.go` — a per-record
reconciliation replacing Phase 16's total-only comparison. A total says
money disagrees without saying which records or why; this classifies
every record.

Implementation note: a **FULL OUTER JOIN** between ledger DISBURSEMENT
postings (account 1100) and SUCCESS disbursements, so neither
one-sided case can be silently omitted — precisely the blind spot in the
Phase 16 check.

**Result (identical on both the live and restored databases):**

| Classification | Rows | Ledger | Disbursement |
|---|---|---|---|
| **VALID** | 17 | ₹340,000 | ₹340,000 |
| **ORPHAN LEDGER ENTRY** | 33 | ₹420,000 | ₹0 |
| **MISSING LEDGER ENTRY** | 3 | ₹0 | ₹60,000 |
| **TOTAL** | **53** | **₹760,000** | **₹400,000** |

**UNEXPLAINED: ₹0. `fullyExplained = true`.**

(The ₹380,000/₹715,000 figures in the Phase 16 hand-off were a snapshot
taken mid-test-run; the dataset moved as suites executed. The structure
of the discrepancy is unchanged and is now fully characterized.)

---

# ₹380K vs ₹715K Root Cause

**Classification: TEST DATA — proven, not assumed.**

Three independent lines of evidence:

**1. An actor named after a Go test function.**
16 orphan ledger entries carry `created_by =
recon_manual_TestManualMatchCreatesAuditEvent`. That test exists at
`internal/reconciliation/security_test.go:196`, and line 198 constructs
the admin name as `"recon_manual_" + t.Name()`. This is a direct,
verifiable code-to-data link.

**2. The remaining 17 orphans reference loans that no longer exist.**
Their descriptions name specific loans (`loan #191`, `#250`, `#293`, …).
Querying those loan IDs: **0 of 17 still exist**. Production loan
applications are never deleted; test fixtures are, via
`t.Cleanup(... Unscoped().Delete ...)`.

**3. The disbursement ID sequence is riddled with gaps.**
IDs span 1–271 but only **120 rows** remain. Orphan references point at
IDs 42–282. Rows were created and deleted repeatedly — the signature of
test cleanup, not production activity.

The 3 MISSING LEDGER ENTRY rows show the same signature: all three
reference loans that no longer exist.

**Mechanism**: tests create loan → disbursement → journal entry, then
`t.Cleanup` deletes the loan and disbursement. The journal entry
survives, because `internal/accounting` is the sole writer of journals
and the audit-oriented design does not delete them. The result is a
ledger entry whose counterparty is gone.

**Corroborating context**: this database is unambiguously a development
database (localhost, dev bootstrap admins, dev-seeded branch fixtures,
3.5 M rate-limit audit rows from the Phase 16 soak). **No production
database exists** — Phase 16 established no production infrastructure
exists at all. There is therefore no production financial discrepancy;
there is a dev database carrying the residue of sixteen phases of
testing.

**Nothing was deleted, altered, or reconciled away.** The data remains
exactly as found; only its classification is new.

**Preventing recurrence** — the real fix is test hygiene, and it is a
remaining item: test suites that create financial records should either
run against a disposable database or clean up journal entries alongside
the rows they reference. Neither was changed in this phase, because
altering test-cleanup behaviour across sixteen phases of suites is
outside "the minimum required changes to resolve the blockers".

**Gate: FINANCIAL RECONCILIATION = PASS (₹0 unexplained).**

---

# Financial Integrity Test Results

Part 14 requires proving the check *works*, not making the number green.
The check's threshold was **not** altered.

| Test | Result |
|---|---|
| `TestReconciliationClassifiesEveryCase` | **PASS** — 53 rows, every one classified with evidence, zero UNKNOWN |
| `TestReconciliationTotalsAreInternallyConsistent` | **PASS** — row sums equal report totals; difference equals disbursement minus ledger |
| `TestReconciliationNeverMutates` | **PASS** — disbursement/journal/line/payment counts unchanged by running it |
| `TestIntegrityChecksNeverModifyFinancialData` | **PASS** (pre-existing) |
| `TestIntegrityChecksRunAndReportInvariants` | **PASS** — every check states its invariant and evidence |
| `TestDoubleEntryInvariantIsChecked` | **PASS** |

**Conditions genuinely exercised by the live dataset**: balanced/VALID,
missing ledger entry, orphan ledger entry. **Not exercised**: duplicate
ledger entry, wrong amount, reversal, zero-value edge case, and
concurrent-transaction scenarios — the dataset does not contain them and
manufacturing them would have required writing synthetic financial rows
into a database this phase was told not to pollute. The classification
branches exist and are reachable; they are **NOT TESTED** and are listed
as a remaining item rather than claimed.

`integrity.financial` run through the scheduler returns **HTTP 500** with
`"financial integrity checks failed: 0 critical, 1 high, 0 warning —
investigate via GET /admin/ops/integrity. NOTHING has been
auto-corrected."` — a failing invariant surfaces as an alertable job
failure rather than a quietly-successful run.

**Gate: FINANCIAL INTEGRITY = PARTIAL** — the check is proven correct and
non-mutating and all current data is explained; several classification
branches remain untested.

---

# Audit Trail Findings

**Nothing was deleted. No protection was disabled. No history rewritten.**

The ~1 GB of audit data (3,568,431 rows; 3,564,457 of them
`rate_limit.exceeded`) was investigated, not removed:

| Determination | Evidence |
|---|---|
| Environment | **Development** — localhost, dev bootstrap admins, dev-seeded fixtures |
| Origin of the bulk | Phase 16's 30-minute soak test against the pre-fix rate-limit audit path |
| Production impact | **None** — no production database exists |
| Append-only trigger intact | **Verified** — present on the live database and **restored intact** (2 triggers) into the restore-test copy |

The safe operator procedure, should this dev database ever need
reclaiming, is documented in the incident runbook and remains a
**deliberate operator decision**, not something performed silently here.
The audit trail remains authoritative.

**Gate: AUDIT TRAIL = PASS.**

---

# Changes Implemented

**Two additional real defects were found and fixed while doing this
work:**

1. **`JobExecution.RunKey` used an empty string on a unique index.**
   PostgreSQL treats `''` as a value (unlike `NULL`), so the *first*
   keyless execution would occupy `''` and every subsequent one —
   **including every `SKIPPED_LOCKED` record** — would be rejected as a
   duplicate. Job contention would have been silently invisible, which
   is exactly what the table exists to reveal. Caught by
   `TestSkippedRunsAreRecorded` failing. Fixed by making `RunKey` a
   `*string` so absence is `NULL`.

2. **Phase 16's "PostgreSQL tooling unavailable" conclusion was wrong**
   — a `PATH` issue, not an availability one. Correcting it is what made
   the mandatory backup and restore tests possible at all.

---

# Files Changed

**New:**
- `scripts/backup_postgres.sh` — backup with checksum + manifest + retention
- `scripts/restore_verify.sh` — isolated restore with hard production-overwrite refusals
- `scripts/README_OPERATIONS.md` — operator procedures and scheduler wiring
- `internal/jobs/jobs.go` — locked, recorded, panic-contained job runner
- `internal/jobs/registry.go` — 7-job inventory with rationale for exclusions
- `internal/jobs/handler.go` — scheduler-facing endpoints
- `internal/jobs/jobs_test.go` — 7 concurrency/idempotency/failure tests
- `internal/models/jobs.go` — `JobExecution`
- `internal/integrity/reconciliation.go` — per-record reconciliation

**Modified:**
- `internal/integrity/handler.go` — reconciliation endpoint
- `internal/integrity/integrity_test.go` — 3 reconciliation tests
- `internal/routes/routes.go` — 5 new admin routes
- `internal/admin/migration.go` — `JobExecution` migration
- `cmd/api/main.go` — `jobs.RegisterAll()` (registration only; **no timer started**)
- `.gitignore` — exclude backup artifacts

---

# Database Changes

One additive migration: `job_executions` (indexes on `job_name +
started_at`, `status`; unique index on `run_key`). **No existing table
altered. No data deleted. No financial value modified.**

---

# Infrastructure Changes

**None deployed.** Scripts and application capability were built; no
scheduler, storage, or backup infrastructure was provisioned, because
none exists to provision into.

---

# Test Results

| Suite | Result |
|---|---|
| `go build ./...` | **PASS** (go1.26.6) |
| `go vet ./...` | **PASS** — clean |
| Full `go test ./...` | **PASS — 18/18 packages** (accounting, admin, analytics, compliance, finance, integrity, **jobs**, kyc, loans, mis, payments, perf, profile, reconciliation, reporting, risk, security, tax) |
| `internal/jobs` (new) | **PASS — 7/7** |
| Reconciliation tests | **PASS — 3/3** |
| Production boot | **PASS** — indexes 5/5, jobs registered, server started |
| Backup execution | **PASS** — evidence above |
| Restore + smoke | **PASS** — `RESTORE_VERIFICATION=PASS` |
| Live scheduler contract | **PASS** — 202 success, 500 on genuine failure, executions recorded |

**NOT TESTED — reason:**
- Worker-crash / DB-failure / provider-timeout during a job — would
  require fault injection not built here.
- Duplicate-ledger, wrong-amount, reversal, zero-value reconciliation
  branches — absent from the dataset; manufacturing them would mean
  writing synthetic financial rows.
- Backup/restore against **production** — no production database exists.
- Off-host backup replication — no object storage configured.

---

# Security Review

Every Phase 16.1 change reviewed against Part 19:

| Check | Result |
|---|---|
| No secrets committed | **PASS** — grep confirms only `$DB_PASSWORD` env references |
| No credentials logged | **PASS** — scripts echo host/port/db only |
| Credential lifetime minimized | **PASS** — `trap 'unset PGPASSWORD' EXIT` |
| No unsafe SQL | **PASS** — advisory locks parameterized (`$1`); reconciliation query is static with no user input |
| No audit bypass | **PASS** — no trigger disabled, no audit row deleted |
| No destructive migration | **PASS** — one additive table |
| No financial mutation in integrity checks | **PASS** — `TestReconciliationNeverMutates`, `TestIntegrityChecksNeverModifyFinancialData` |
| No duplicate scheduler execution | **PASS** — advisory lock, proven max-simultaneous = 1 |
| No unsafe retries | **PASS** — retry is external; 409 documented as success |
| No race-prone idempotency | **PASS** — unique index, not read-then-write |
| **Production DB not overwritten during restore** | **PASS** — two hard refusals; restore ran against `nbfc_db_restore_test` |
| Backup credentials not exposed | **PASS** — manifest and `.sha256` carry none |
| Backup artifacts not committable | **PASS** — `.gitignore` updated |
| Job run endpoint authorization | **PASS** — requires `RISK_RULE_MANAGE`, not read-tier |

---

# Remaining External Dependencies

1. **Off-host backup storage** — a local dump is not a backup.
2. **Backup encryption at rest, access control, deletion protection.**
3. **A scheduled backup job** — the script exists; nothing runs it.
4. **The external scheduler itself** — one HTTP POST per job.
5. **Alerting** — for backup failure, restore-verification failure, stale
   jobs, and `integrity.financial` returning 500.
6. **A dedicated scheduler admin account** holding only
   `RISK_RULE_MANAGE`.
7. **WAL archiving / PITR** if RPO must be below the backup interval.
8. **RPO/RTO targets from management.**
9. Everything else carried from Phase 16: CI/CD, rollback testing, HA,
   centralized logging, TLS/DNS, VAPT, CA/compliance configuration.

---

# Production Readiness Gates

| Gate | Status | Evidence |
|---|---|---|
| Backup creation | **PASS** | `nbfc_db_20260820T045308Z`, 128,126,510 B, 14 s, SHA-256 recorded |
| Restore | **PASS** | 81 s, 106 tables, 11/11 row counts matched incl. 3,568,431 audit rows, `RESTORE_VERIFICATION=PASS` |
| Restore smoke test | **PASS** | App booted, health/readiness OK, login OK, integrity + reconciliation executed |
| RPO | **BUSINESS VALIDATION REQUIRED** | Capability measurable (14 s backup ⇒ hourly feasible); no target set |
| RTO | **PASS (measured)** | 96 s dump→serving; excludes detection/provisioning |
| Backup security | **PARTIAL** | In-script controls pass; storage-dependent controls external |
| Scheduler | **PARTIAL** | Application complete and tested; external deployment pending |
| Job idempotency | **PASS** | Advisory lock + unique run key; 8 concurrent → 1 ran |
| Job overlap protection | **PASS** | Max simultaneous = 1; skips recorded |
| Retry/failure handling | **PARTIAL** | Failure/panic/skip tested; crash & infra-failure not exercised |
| Financial reconciliation | **PASS** | ₹0 unexplained; 53/53 rows classified with evidence |
| Financial integrity | **PARTIAL** | Check proven correct and non-mutating; some branches untested |
| Audit trail | **PASS** | Nothing deleted, no protection disabled, triggers survived restore |
| Regression | **PASS** | 18/18 packages, vet clean, boot verified |
| Production deployment | **FAIL** | No CI/CD, no rollback test, no infrastructure (unchanged from Phase 16) |
| **Overall** | **NOT READY** | |

---

# Final Verdict

## Phase 16.1 Gate Summary

```
Phase 16.1 Gate                  Status
------------------------------------------------
Backup                           PASS
Restore                          PASS
RPO                              BUSINESS VALIDATION REQUIRED
RTO                              PASS (96s measured)
Backup security                  PARTIAL
Scheduler                        PARTIAL
Job idempotency                  PASS
Job overlap protection           PASS
Retry/failure handling           PARTIAL
Financial reconciliation         PASS
Financial integrity              PARTIAL
Audit trail                      PASS
Regression                       PASS
Production deployment            FAIL
Overall                          NOT READY
```

### 1. What was fixed

- Backup capability built and **executed for real** (128 MB, 14 s,
  checksummed).
- Restore capability built and **executed for real** into an isolated
  database (81 s), with hard refusals preventing production overwrite.
- A production-safe job runner: distributed locking, execution records,
  run-key idempotency, panic containment, staleness monitoring — **with
  no in-process ticker**.
- Seven jobs registered and documented; four categories deliberately
  left manual with stated reasons.
- Per-record disbursement/ledger reconciliation replacing a total-only
  comparison.
- `RunKey` empty-string-vs-NULL defect that would have made job
  contention invisible.
- Corrected Phase 16's false "PostgreSQL tooling unavailable" finding.

### 2. What was proven

- A backup of this database can be restored, and the restored copy is
  **byte-faithful** (all sampled row counts, including 3.5 M audit rows)
  and **functionally live** (app boots, authenticates, runs checks).
- The **append-only audit trigger survives a restore**.
- Database recovery takes **96 seconds** end to end.
- Concurrent job invocations **cannot overlap** (8 → 1, max simultaneous
  = 1), and contention is recorded rather than silent.
- The ₹ discrepancy is **entirely test residue**, with ₹0 unexplained
  and a direct code-to-data link for one class of it.
- Integrity and reconciliation checks **cannot mutate financial data**.

### 3. What remains unresolved

- Off-host backup replication (a local dump is not a backup).
- No scheduled backup, and no alerting on backup/restore failure.
- Reconciliation branches for duplicate/wrong-amount/reversal/zero-value
  remain untested.
- Job crash and infrastructure-failure scenarios not exercised.
- Test hygiene that produced the orphan ledger entries is unchanged.
- All Phase 16 gaps outside this phase's scope: CI/CD, rollback, HA,
  centralized logging, TLS/DNS, VAPT, route-level security coverage,
  analytics at volume.

### 4. What requires external infrastructure

Off-host encrypted backup storage with access control and deletion
protection; a scheduled backup runner; the external job scheduler; an
alerting system; WAL archiving/PITR if a sub-daily RPO is required; and
the CI/CD, HA, and log-aggregation layers carried from Phase 16.

### 5. What requires business/operator approval

RPO and RTO targets; whether to reclaim the ~1 GB of dev audit data
(and by which documented safe procedure); the scheduler's dedicated
admin account; retention-period configuration by compliance before any
data-deletion job is automated; and all CA/compliance configuration
(classification, provisioning, TDS, NBFC profile).

### 6. Is Phase 17 allowed to start?

**No — and it was not started.**

Phase 16.1's own blockers are resolved or conclusively characterized with
evidence and exact instructions. But the **overall production-readiness
verdict remains NOT READY**: backups are not yet replicated off-host or
scheduled, no scheduler is deployed, no alerting exists, and production
deployment remains a FAIL gate. Those are operator and infrastructure
actions, not further application development — starting another
implementation phase would add code on top of an unresolved operational
foundation.

---

**No claim is made that this system is RBI compliant, VAPT certified,
externally audited, or production ready.** Backups are now proven
restorable *in a development environment*; that is a real and material
step, and it is not the same as a verified production backup regime.
