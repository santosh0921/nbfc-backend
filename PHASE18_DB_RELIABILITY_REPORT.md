# Phase 18 — Database Reliability & Backup Validation

## PHASE 18 RESULT

**PASS WITH WARNINGS**

Every mandatory verification that could be executed without
Administrator privileges was executed for real, produced concrete
evidence, and — where a genuine defect was found — was fixed and
re-tested. Two things keep this from an unqualified PASS: (1) the sync
mechanism is not yet scheduled, so RPO is currently unbounded between
manual runs, and (2) both PostgreSQL instances run as superuser, a
carried-forward finding not resolved this phase. Neither is a test that
failed — both are real, disclosed operational gaps.

---

## 1. Architecture

```
Application
     |
     v
Primary PostgreSQL 16.14 — localhost:5433 — db "nbfc_db"
(Windows service postgresql-x64-16, always-on)
     |
     | synchronization: scripts/sync_backup_database.sh
     | (pg_dump --format=custom -> DROP/CREATE -> pg_restore, on demand)
     v
Backup PostgreSQL 16.14 — localhost:5434 — db "nbfc_db"
(native instance, data dir C:\nbfc-backup-pg\data, manually started)
     |
     v
Recovery: point DB_HOST/DB_PORT at :5434, restart app
Measured: 3 seconds to first healthy request
```

## 2. Environment

| | Primary | Backup |
|---|---|---|
| PostgreSQL version | 16.14 | 16.14 |
| Port | 5433 | 5434 (localhost only) |
| Database size | 1,070 MB | 993 MB (at last measurement) |
| Tables | 107 | 107 |

No passwords or secrets are reproduced in this report.

## 3. Tests Executed

| # | Test | Result |
|---|---|---|
| 1 | Primary connectivity, auth, schema inventory | PASS |
| 2 | Backup connectivity, auth, schema inventory | PASS |
| 3 | Structural comparison (tables/indexes/constraints/FKs/sequences/extensions/triggers) | PASS — all identical |
| 4 | Data consistency (9 tables, count+min/max+timestamp, 2 tables paisa-exact checksummed) | PASS — zero divergence |
| 5 | Sync execution with timing/exit code capture | PASS |
| 6 | Repeatability (3 consecutive runs) | PASS — no drift, no duplication |
| 7 | Full incremental lifecycle (insert/update/delete → sync → verify each) | PASS — every step independently verified |
| 8 | Backup-target-unavailable failure test | PASS — real failure occurred naturally, non-zero exit, no false success, clean recovery on retry |
| 9 | Script security/reliability audit | PASS — 1 defect found and fixed (see §9) |
| 10 | Concurrent execution test | PASS — defect found and fixed (see §9), then verified: 1 ran, 1 cleanly refused |
| 11 | Backup freshness reporting | PASS — new endpoint built and verified against real manifests |
| 12 | RPO measurement | MEASURED — see §6 |
| 13 | RTO measurement | MEASURED — see §6 |
| 14 | Recovery/restore verification | PASS — full app smoke test against backup instance |
| 15 | Sequence & FK integrity | PASS (sequences); 1 pre-existing condition found, confirmed NOT a sync defect |
| 16 | Backup security audit | 1 CRITICAL-adjacent finding, found and fixed (see §7); 1 HIGH finding carried forward, unresolved |
| 17 | Performance baseline | RECORDED — see §8 |

## 4. Data Consistency

| Table | Primary | Backup | Difference | Status |
|---|---|---|---|---|
| users | 67 | 67 | 0 | OK |
| loan_applications | 62 | 62 | 0 | OK |
| disbursements | 126 | 126 | 0 | OK |
| payments | 0 | 0 | 0 | OK |
| emi_installments | 1,215 | 1,215 | 0 | OK |
| journal_entries | 510 | 510 | 0 | OK |
| journal_entry_lines | 1,020 | 1,020 | 0 | OK |
| audit_logs | 3,568,756 | 3,568,756 | 0 | OK |
| admin_users | 3 | 3 | 0 | OK |

**Beyond row counts** — the instruction was explicit that matching
counts alone don't prove consistency:

- `journal_entry_lines` debit/credit sums compared exactly:
  **114,494,632 paise = 114,494,632 paise** on both sides (both debit
  and credit totals independently).
- Min/max primary key and latest `created_at` compared per table — all
  identical (e.g. `disbursements`: min=1, max=293, latest
  `2026-08-20 10:45:21.246185+05:30` on both).
- Structural comparison (§1 above): 280 indexes, 567 constraints, 1 FK,
  92 sequences, 1 extension, 2 triggers — all identical counts on both
  instances.

No divergence detected anywhere.

## 5. Synchronization

- **Duration**: 9 executions observed, 56–176 seconds (varies with
  concurrent database load from simultaneous verification queries this
  phase was running).
- **Exit status**: 0 on success, non-zero (1 or 2) on every failure
  case — never silently non-zero-but-unreported.
- **Repeatability**: 3 consecutive runs with no primary changes produced
  identical row counts every time. The script's drop/recreate design
  makes duplication structurally impossible — each run *replaces*
  rather than *appends*.
- **Incremental test**: full insert → sync → verify-present → modify →
  sync → verify-modified → delete → sync → verify-absent cycle, using a
  dedicated `job_executions` test row (id 81), never touching real
  customer/loan/payment data. Every transition confirmed by direct
  query on both instances.
- **Failure handling**: the backup instance was found genuinely stopped
  mid-phase (it does not survive a session restart — see §10). The sync
  attempt against it failed exactly as it should: `Connection refused`,
  exit code 2, no manifest written, no false success. Restarting the
  instance and retrying produced a clean `SUCCESS`.
- **Concurrent execution**: see §9 — defect found, fixed, and verified.

## 6. Recovery

- **Restore result**: full application smoke test against the backup
  instance — admin login succeeded, `/health` and `/readiness` returned
  `ok`, and `GET /admin/ops/integrity` executed correctly against it.
- **RPO**: **not a fixed target, because nothing currently schedules the
  sync.** Measured exposure at time of writing was 18 minutes, purely
  because a manual run had happened recently — the honest ceiling is
  unbounded until a scheduler is configured (§10, §11).
- **RTO**: **measured at ~3 seconds** for cutover to the already-running
  backup instance (build + config change + app restart + first `200 OK`
  from `/health`). This is far faster than the 96-second cold
  restore-from-dump measured in Phase 16.1, precisely because this
  instance is a live standby, not a file that needs restoring.
- **Application compatibility**: confirmed — the same binary, unmodified,
  connected to port 5434 and served correctly, including exercising the
  financial-integrity endpoint.
- **Recovery limitations**: this validates *database* failover only.
  Cutting the application over in a real incident still requires a
  human to change `DB_HOST`/`DB_PORT` and restart the process — there is
  no automated failover, load balancer, or DNS-level redirection (none
  of that infrastructure exists per the Phase 16/17 findings).

## 7. Security Findings

| Finding | Severity | Status |
|---|---|---|
| `.pgpassword` and the entire backup data directory (2,350+ files, including raw database files) were readable by `BUILTIN\Users` and writable by `Authenticated Users` — any local account on the machine | **HIGH** | **FIXED this phase** — restricted to `Administrator`+`SYSTEM` only, verified file-by-file |
| `icacls`'s bulk `/T` recursive grant silently failed on ~2,350 files with a false "Successfully processed" message, leaving them with genuinely empty (deny-all) ACLs | **MEDIUM** (tooling defect, caused a self-inflicted incident during remediation of the finding above) | **FIXED** — worked around by per-file grant + per-file verification; documented so it is not repeated |
| Both PostgreSQL instances (primary and backup) run as `postgres`, a full superuser, rather than a least-privilege role | **HIGH** | **CARRIED FORWARD, NOT RESOLVED.** A working least-privilege design exists (`scripts/create_least_privilege_role.sql`, empirically proven in Phase 17) but has not been applied — doing so requires separating migration (DDL) from runtime (DML) credentials, a deployment change outside this phase's scope |
| Credentials never appear in command-line arguments (`PGPASSWORD=x cmd`, not `-p x`), so are not visible via `ps`/`tasklist` | — | **VERIFIED GOOD** — already correct in the existing script |
| Manifest files contain no credentials | — | **VERIFIED GOOD** |
| Port 5434 bound to `localhost` only, not exposed on the network | — | **VERIFIED GOOD** |

## 8. Performance Baseline

| Metric | Value |
|---|---|
| Primary database size | 1,070 MB |
| Backup database size | 993 MB (measured at a slightly earlier point; both track together after each sync) |
| Sync (dump + restore + verify) duration | 56–176 s across 9 runs |
| Restore-only portion (from Phase 16.1, same mechanism) | 81 s for a 1,069 MB source |
| Cutover RTO (already-synced standby) | ~3 s |
| `audit_logs` row count | 3,568,756 (dominant table by volume — prior-phase load-test residue, documented since Phase 16.1) |
| CPU/memory of either PostgreSQL process | Not captured — no perf counter tooling available in this environment; noted as a gap, not fabricated |

## 9. Defects Found

### Defect 1 — No concurrency lock

```
Finding:        Two overlapping sync invocations would race on
                 DROP DATABASE/pg_restore against the same target,
                 risking a mixed, silently-"passing" backup state.
Severity:        HIGH (data-integrity risk on the backup path)
Evidence:        Code review of sync_backup_database.sh — no lock,
                 pid file, or advisory-lock mechanism existed.
Root cause:      Script was written and validated for sequential,
                 single-operator use only; concurrent scheduling
                 was never exercised.
Fix:             Atomic mkdir-based lock (flock confirmed unavailable
                 in this environment) with PID-liveness stale-lock
                 detection, added to sync_backup_database.sh.
Re-test result:  Two syncs launched 2 seconds apart. First ran to
                 completion (SUCCESS, 84s). Second was refused
                 immediately: "REFUSING: another sync (PID 1748) is
                 already running", exit code 1. Lock directory
                 cleaned up correctly. Data integrity re-verified
                 identical afterward. PASS.
```

### Defect 2 — Open filesystem permissions on the backup instance

```
Finding:        C:\nbfc-backup-pg (password file + entire PGDATA,
                 including raw database files) inherited default
                 Windows ACLs readable by BUILTIN\Users and
                 modifiable by NT AUTHORITY\Authenticated Users —
                 any local account on the machine.
Severity:        HIGH
Evidence:        icacls "C:\nbfc-backup-pg\.pgpassword" showed
                 BUILTIN\Users:(RX), Authenticated Users:(M).
Root cause:      Directory was created without explicit ACL
                 hardening at setup time.
Fix:             icacls /inheritance:r + explicit grant to
                 Administrator+SYSTEM only.
Re-test result:  Verified via icacls on the password file: only
                 Administrator(F) and SYSTEM(F) remain. See Defect 3
                 for a complication encountered while applying this
                 fix, and its resolution.
```

### Defect 3 — Self-inflicted: icacls bulk recursion silently failed

```
Finding:        Applying Defect 2's fix with
                 "icacls ... /inheritance:r /grant:r ... /T" reported
                 "Successfully processed 2379 files; Failed
                 processing 0 files" and exit code 0 — but a
                 subsequent audit found 2,348 files with completely
                 EMPTY ACLs (deny-all, including the owner), which
                 crashed the backup PostgreSQL instance (it could no
                 longer read its own data/log/pid files).
Severity:        Was CRITICAL while unresolved (the backup instance
                 was down and its data temporarily inaccessible to
                 everyone, including the administrator account).
Evidence:        icacls and Get-Acl independently agreed the DACLs
                 were empty on ~2,350 files; a direct single-file
                 "icacls file /grant Administrator:F" succeeded
                 immediately on the same files, proving the tool's
                 bulk /T recursion — not a permissions or locking
                 problem — was the actual defect. Reproduced even
                 after stopping PostgreSQL first (ruling out file
                 locks as the cause) and even after an /reset pass.
Root cause:      A defect/limitation in icacls itself when performing
                 a combined /inheritance:r /grant:r /T operation
                 across a large (2,000+ file) tree — not attributable
                 to this codebase or to the earlier permission
                 change.
Fix:             Stopped the backup instance (releasing all file
                 handles), then applied the grant to every flagged
                 file INDIVIDUALLY via a PowerShell loop calling
                 icacls per file, verifying each one immediately
                 after.
Re-test result:  Final scan: 0 files with broken/empty ACLs (down
                 from 2,350). Backup instance restarted successfully
                 (PID confirmed). Full data-consistency re-check
                 afterward: all 7 sampled tables matched primary
                 exactly, including audit_logs at 3,568,756 rows on
                 both sides. PASS.
```

## 10. Files Changed

| File | Change |
|---|---|
| `scripts/sync_backup_database.sh` | Added the concurrency lock (Defect 1) |
| `internal/integrity/sync_status.go` | **New.** Backup-database sync freshness reporting (Step 11) — deliberately separate from `backup_status.go` because the sync mechanism's success is proven by verified row counts, not a persisted file's presence |
| `internal/integrity/handler.go` | Added `AdminSyncStatusHandler` |
| `internal/routes/routes.go` | Added `GET /admin/ops/sync-status` |
| `scripts/BACKUP_DATABASE_SETUP.md` | Rewritten to reflect this phase's validated architecture, evidence, and known limitations |
| Filesystem (not source-controlled) | `C:\nbfc-backup-pg` ACLs hardened (Defect 2/3) |

No changes were made to any table schema, to the primary database's
data, or to any file outside the scope above.

## 11. Blockers

**Administrator-required actions** (confirmed by direct attempt, not
assumed):
- Registering the backup instance as a Windows service
  (`pg_ctl register`) — fails with `Access is denied` in this
  non-elevated session.
- Registering a scheduled task to run the sync automatically —
  same failure mode.

**Environment blockers**:
- Docker/WSL2 — explicitly out of scope for Phase 18, not attempted.
- No CPU/memory perf-counter tooling available for §8.

**Code/configuration gaps that are NOT Administrator-blocked, but were
not completed this phase** (genuine remaining work, not "merely
inconvenient"):
- Applying the least-privilege database role to either PostgreSQL
  instance — this is achievable without elevation (it's a SQL/config
  change, not a Windows privilege) but requires separating the
  application's migration-time and runtime credentials, which is a
  deployment-flow change larger than this phase's scope.
- No per-job timeout on the sync script.

**Nothing in this phase was blocked by external infrastructure** (no
cloud, no third-party service was needed for any of the above).

## 12. Docker

Docker remains deferred because the current session lacks Administrator
privileges. Docker was not treated as a dependency for Phase 18, and no
Docker commands were run or retried during this phase.

## 13. Phase Gate

```
PHASE 18: PASS WITH WARNINGS
```

**Warnings, precisely stated** (not test failures — disclosed
operational gaps):

1. The synchronization mechanism is proven correct, repeatable,
   idempotent, concurrency-safe, and fast to recover from — but it is
   **not currently scheduled**, so RPO is unbounded between manual
   runs until an operator or scheduler configures a cadence.
2. Both PostgreSQL instances still run as superuser. A proven
   least-privilege alternative exists but is not applied.
3. The backup instance is not a registered service and does not
   survive a reboot without manual intervention — confirmed
   Administrator-blocked, not attempted further.

Phase 18 is complete. These three items are the explicit carry-forward
for whichever phase addresses production scheduling and credential
hardening next.
