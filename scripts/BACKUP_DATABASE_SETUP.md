# Backup Database — Setup and Operation

A second, standing PostgreSQL instance that mirrors the primary
database. Built 2026-08-21. **Validated end-to-end in Phase 18
(2026-08-24)** — see `PHASE18_DB_RELIABILITY_REPORT.md` for the full
evidence; this document is the operational reference.

---

## Why this exists, and its honest scope limit

The original plan was a Docker container running PostgreSQL. **Docker
Desktop's engine cannot start in this environment** — its backend
processes launch but never come up, because WSL2 is not functional on
this machine. This is a host configuration issue requiring
Administrator action, not something fixable by retrying commands.

Instead, a **second native PostgreSQL 16 instance** was created — its
own process, its own data directory, its own port.

**Protects against:**
- Primary PostgreSQL process crashing or hanging
- Primary data directory corruption
- An accidental `DROP`/`DELETE` against the primary (the sync only ever
  writes on demand, so a mistake on the primary is not instantly mirrored)

**Does NOT protect against:**
- **Disk failure** — both instances share the same physical disk (`C:`)
- **Host loss** — both instances are the same machine
- **Being an actual off-site backup**

This is a **local standby**, not off-host replication.

---

## Architecture

```
Application
     |
     v
Primary PostgreSQL 16.14 — localhost:5433 — db "nbfc_db"
(Windows service postgresql-x64-16, always-on)
     |
     | scripts/sync_backup_database.sh
     | (pg_dump --format=custom -> DROP/CREATE -> pg_restore,
     |  on demand — no in-process timer, ever)
     v
Backup PostgreSQL 16.14 — localhost:5434 — db "nbfc_db"
(native instance, data dir C:\nbfc-backup-pg\data,
 manually started — NOT a registered service)
     |
     v
Recovery: point DB_HOST/DB_PORT at :5434 and restart the app —
measured 3 seconds to first healthy request (Phase 18)
```

---

## What was validated in Phase 18 — with evidence

### Structural parity (primary vs. backup, identical)

| | Primary | Backup |
|---|---|---|
| PostgreSQL version | 16.14 | 16.14 |
| Tables | 107 | 107 |
| Indexes | 280 | 280 |
| Constraints | 567 | 567 |
| Foreign keys | 1 | 1 |
| Sequences | 92 | 92 |
| Extensions | 1 | 1 |
| Triggers (incl. audit_logs append-only) | 2 | 2 |

### Data consistency (checksummed, not just counted)

9 key tables compared by count + min/max id + latest timestamp; two
financial tables additionally cross-checked by **summed
debit/credit paise** (exact-to-the-paisa match): all OK, zero
divergence.

### Full lifecycle test (insert → sync → update → sync → delete → sync)

A test record was created on the primary, confirmed **absent** on the
backup pre-sync, then present after sync; modified on the primary,
confirmed the modification propagated; deleted on the primary,
confirmed absent on the backup after a final sync. Every step verified
with a direct query, not assumed.

### Idempotency (3 consecutive runs, no drift)

Ran the sync 3 times back-to-back with no primary changes: row counts
identical every time (drop/recreate design makes this replace, not
append — cannot duplicate).

### Failure handling (real failure, not simulated)

The backup instance was actually down when a sync was attempted:
`psql: connection to server ... failed: Connection refused` — the
script exited non-zero (2), wrote **no** success manifest, and made
**no** false claim. After restarting the instance, the retry succeeded
cleanly.

### Concurrency lock (Phase 18 fix)

**Defect found**: the script had no protection against two overlapping
runs, which would race on `DROP DATABASE`/`pg_restore` and could leave
the backup in a mixed, silently-"passing" state. **Fixed**: an atomic
`mkdir`-based lock (`flock` is unavailable in this environment) with
PID-liveness stale-lock detection. **Tested**: two syncs launched
2 seconds apart — the first ran to `SUCCESS`, the second was cleanly
refused (`REFUSING: another sync (PID ...) is already running`, exit 1).
Lock directory cleaned up correctly afterward; data integrity
unaffected.

### Sequence and foreign-key integrity

Every sequence on the backup instance sits **at or ahead of** its
table's `MAX(id)` (e.g. `journal_entries_id_seq` = 706 vs. max id 700) —
custom-format `pg_restore` preserves actual sequence state rather than
deriving it, so no ID-collision risk after a restore.

One pre-existing condition, not a sync defect: 804 `emi_installments`
rows reference a `loan_applications` row that no longer exists — this
count is **identical on primary and backup**, confirming it is
inherited test-data residue on the primary (same root cause documented
in the Phase 16.1 reconciliation report — test fixtures delete a loan
without cascading its installments), faithfully replicated rather than
silently "fixed" by the sync.

### RPO / RTO (measured, not estimated)

| | Value |
|---|---|
| Sync duration | 56–176s per run across 9 executions (varies with concurrent load) |
| **RPO** | Bounded only by how often the sync runs. **Currently unscheduled** — no external trigger calls it, so exposure grows unbounded until an operator runs it or a scheduler is configured. Measured exposure at time of writing: 18 minutes since last run, purely because a manual run happened recently |
| **RTO (cutover to the live standby)** | **~3 seconds** — build time + `DB_HOST`/`DB_PORT` pointed at :5434 + app restart + first `200 OK` from `/health`. This is dramatically faster than a cold restore-from-file (measured at 96s in Phase 16.1), because the backup instance is already running and synced |

Full application compatibility on the backup instance was confirmed
beyond just data: admin login succeeded, and
`GET /admin/ops/integrity` executed correctly against it.

### Security hardening (Phase 18)

`.pgpassword` and the entire `C:\nbfc-backup-pg` tree (including raw
database files) previously inherited open Windows ACLs —
`BUILTIN\Users` could **read**, `Authenticated Users` could **modify**,
any local account on the machine. Restricted to `Administrator` +
`SYSTEM` only, verified file-by-file (2,350 files individually
confirmed — `icacls`'s bulk `/T` recursion was found to silently fail
on a subset of files with no error, a real defect in the tool itself,
worked around by per-file verification). Data integrity re-confirmed
identical after the hardening.

**Still open**: the backup instance runs as PostgreSQL superuser
`postgres`, same finding as the primary in the Phase 16/17 reports — a
least-privilege role (`scripts/create_least_privilege_role.sql`) exists
but is not yet applied to either instance.

### Freshness reporting (new this phase)

`GET /admin/ops/sync-status` reads the sync manifests and reports
health (`OK`/`STALE`/`LAST_SYNC_FAILED`/`NO_SYNC_FOUND`), last sync
timestamp, age, and row-mismatch count. Verified against the real
manifest directory: `health=OK, count=9, lastId=sync_20260824T032147Z,
mismatches=0, status=SUCCESS`. This is intentionally a separate
function from `/admin/ops/backup-status` (the file-based mechanism) —
the sync target is a live database, not a persisted dump file, so the
health model differs (no "artifact present" check makes sense here).

---

## Running the sync

```bash
export DB_HOST=localhost DB_PORT=5433 DB_USER=postgres DB_NAME=nbfc_db DB_PASSWORD=<primary password>
export BACKUP_DB_HOST=localhost BACKUP_DB_PORT=5434 BACKUP_DB_USER=postgres BACKUP_DB_NAME=nbfc_db
export BACKUP_DB_PASSWORD="$(cat /c/nbfc-backup-pg/.pgpassword)"
export PG_BIN="/c/Program Files/PostgreSQL/16/bin"   # if not on PATH
export SYNC_MANIFEST_DIR="/c/nbfc-backup-pg"          # where manifests are written

bash scripts/sync_backup_database.sh
```

Refuses to run if: the target resolves to the primary's host:port, or
another sync already holds the lock (`SYNC_LOCK_DIR`, default
`${TMPDIR}/nbfc_backup_sync.lock`).

Dumps the primary, drops/recreates `nbfc_db` on the backup instance,
restores, verifies row counts across 9 key tables, writes a manifest
(no credentials in it).

## Checking backup-database health

```
GET /admin/ops/sync-status
```

## Recovery procedure (cutover to the standby)

1. Confirm the backup instance is healthy: `GET /admin/ops/sync-status`.
2. Point the application at it: `DB_HOST=localhost DB_PORT=5434
   DB_PASSWORD=<backup instance password>`.
3. Restart the application.
4. Confirm `GET /health` and `GET /readiness` return `ok`.
5. Confirm `GET /admin/ops/integrity` runs cleanly.

Measured total time for steps 2–4: **~3 seconds**. This procedure was
executed for real in Phase 18, not merely described.

---

## Starting / stopping the backup instance manually

**Not yet a registered Windows service** — `pg_ctl register` requires
Administrator rights this session does not have (`Access is denied`,
confirmed). Must be started manually after any reboot or crash:

```powershell
& "C:\Program Files\PostgreSQL\16\bin\pg_ctl.exe" -D "C:\nbfc-backup-pg\data" -l "C:\nbfc-backup-pg\server.log" start
```

To stop it:
```powershell
& "C:\Program Files\PostgreSQL\16\bin\pg_ctl.exe" -D "C:\nbfc-backup-pg\data" stop
```

### Making it start automatically (Administrator-required action)

```powershell
& "C:\Program Files\PostgreSQL\16\bin\pg_ctl.exe" register -N "nbfc-backup-postgres" `
  -D "C:\nbfc-backup-pg\data" -l "C:\nbfc-backup-pg\service.log" -S auto
```

Confirmed to fail with `Access is denied` in a non-elevated session —
this is a genuine Administrator-required blocker, not a convenience gap.

## Scheduling the sync (Administrator/operator-required action)

**Currently unscheduled** — this is the single largest remaining gap.
Nothing calls `sync_backup_database.sh` automatically, so RPO is
unbounded until either a manual run happens or a scheduler is
configured. Do **not** wire this to an in-process timer — schedule it
externally (systemd timer, Windows Task Scheduler, cloud scheduler)
exactly as documented in `scripts/README_OPERATIONS.md`, at a cadence
matching your RPO tolerance (given ~1–3 minute sync durations, hourly
is comfortably achievable).

---

## Known limitations (honest, current)

1. **Not off-host.** Same physical disk as the primary.
2. **Not scheduled.** RPO is currently "whenever someone runs it
   manually" — unbounded.
3. **Not a registered service.** Does not survive a reboot without
   manual restart (Administrator-required to fix).
4. **Runs as superuser**, same as the primary — least-privilege role
   exists but is not applied.
5. **No per-job timeout** on the sync script — a hung `pg_dump`/
   `pg_restore` would hold the lock until it eventually fails or is
   killed.
6. **Concurrency lock is local-filesystem-based** (`mkdir`), which is
   correct for a single-host setup but would need a different mechanism
   (e.g. a database advisory lock, as `internal/jobs` already uses for
   in-application scheduled jobs) if this script were ever run from
   multiple hosts.

---

## If Docker is fixed later

Should WSL2/Docker Desktop become functional on this host, the same
`sync_backup_database.sh` script works unmodified against a
containerized PostgreSQL target — just point `BACKUP_DB_HOST`/
`BACKUP_DB_PORT` at the container's mapped port instead of
`localhost:5434`. The script has no dependency on how the target
instance is hosted. **Docker remains deferred and was not attempted in
Phase 18.**
