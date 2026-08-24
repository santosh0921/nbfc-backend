# Operations — Backup, Restore, and Scheduling

Operator procedures for the NBFC backend. Everything here has been
executed at least once in a development environment; where something has
**not** been executed against production, it says so.

---

## 1. Backup

### Mechanism

`scripts/backup_postgres.sh` — `pg_dump --format=custom --compress=6`.

**Why a logical dump**: this deployment uses a self-managed PostgreSQL
instance reached over host/port credentials. There is no cloud provider
and therefore no managed snapshot facility. If the database is later
moved to managed PostgreSQL (RDS, Aurora, Cloud SQL, …), the provider's
automated snapshots + point-in-time recovery become the **primary**
mechanism and this script becomes a secondary portable export — do not
treat it as a replacement for provider backups in that case.

### Running it

```bash
export DB_HOST=... DB_PORT=... DB_USER=... DB_NAME=... DB_PASSWORD=...
export BACKUP_DESTINATION=/path/on/SEPARATE/storage
export BACKUP_RETENTION_DAYS=30
# Only needed where pg_dump is not on PATH:
export PG_BIN="/c/Program Files/PostgreSQL/16/bin"

bash scripts/backup_postgres.sh
```

Produces three files per run:

| File | Purpose |
|---|---|
| `<db>_<UTC timestamp>.dump` | the backup itself |
| `<db>_<UTC timestamp>.dump.sha256` | integrity checksum |
| `<db>_<UTC timestamp>.manifest.json` | evidence record (size, duration, checksum, retention) — contains no credentials |

### ⚠️ Critical operator requirement

`BACKUP_DESTINATION` **must be on storage separate from the database
volume**. A backup sitting on the same disk as the database is not a
backup — it dies with the disk. This script writes locally and
deliberately does **not** upload anywhere, because no object storage is
configured in this deployment. Off-host replication is an operator
responsibility and is currently **NOT IMPLEMENTED**.

### Retention

Files older than `BACKUP_RETENTION_DAYS` are removed, matched strictly
by the `<db>_*` naming pattern so an unrelated file sharing the directory
can never be deleted.

---

## 2. Restore verification

**A backup that has never been restored is not a backup.**

```bash
export DB_HOST=... DB_PORT=... DB_USER=... DB_NAME=... DB_PASSWORD=...
export PG_BIN="/c/Program Files/PostgreSQL/16/bin"   # if needed

bash scripts/restore_verify.sh /path/to/nbfc_db_<stamp>.dump
```

### Safety guarantees (hard checks, not conventions)

The script **refuses to run** if:
- the restore target equals `DB_NAME` (the live database), or
- the target name does not contain `_restore_test`.

It will not drop any database that fails those checks. "The restore test
overwrote production" is a real and catastrophic operator error, so these
are enforced in code.

### What it verifies

Checksum → restore → table/index/constraint/trigger counts → per-table
row-count comparison against the source → financial consistency in the
restored copy (balanced journals, unique journal numbers, no duplicate
disbursements) → audit-trigger presence.

Exits non-zero and prints `RESTORE_VERIFICATION=FAIL` if anything fails.

### Cadence

Run a restore verification **at least monthly** and after any schema
migration. Record the output — that output *is* the evidence.

---

## 3. Scheduling background jobs

### Architecture: external scheduler, never in-process

The application contains **no timer, no ticker, and no background
scheduling loop**, deliberately. With N application instances an
in-process timer fires N times, and a financial job running N times is
exactly the failure this design prevents.

Instead, an external scheduler calls an authenticated endpoint, and the
application makes each invocation safe:

- **Distributed lock** — a PostgreSQL advisory lock keyed on the job
  name, shared across all instances. A second concurrent invocation
  records `SKIPPED_LOCKED` and returns HTTP 409.
- **Execution record** — every attempt (success, failure, or skip)
  writes a `job_executions` row.
- **Run key** — jobs that must run at most once per period declare one;
  a unique index enforces it in the database.

### Scheduler contract

```
POST /admin/ops/jobs/{name}/run?source=SCHEDULER
Authorization: Bearer <admin JWT with RISK_RULE_MANAGE>
```

| Status | Meaning | Scheduler should |
|---|---|---|
| `202` | Ran successfully | Treat as success |
| `409` | Another instance holds the lock, or it already ran this period | **Treat as SUCCESS.** The work is happening — just not by this call. Do not retry, do not alert. |
| `404` | Unknown job name | Alert — misconfiguration |
| `500` | The job genuinely failed | Alert and investigate |
| `401/403` | Auth problem | Alert — credential issue |

### Scheduler identity

Create a **dedicated admin account** for the scheduler holding only
`RISK_RULE_MANAGE`. Do not reuse a human's credentials — job executions
are audited by actor, and attributing automated runs to a person
corrupts that record.

### Job inventory and suggested cadence

Read the live inventory at `GET /admin/ops/jobs`. As registered:

| Job | Suggested schedule | Financial impact | Once per period |
|---|---|---|---|
| `risk.daily` | Daily, after end-of-day (e.g. 01:00) | Yes | Yes (per calendar day) |
| `risk.consistency` | Daily, after `risk.daily` | Yes | No |
| `collections.broken_promises` | Daily | Yes | No |
| `compliance.sweep` | Daily, early morning | No | No |
| `analytics.data_quality` | Daily | No | No |
| `analytics.risk_signals` | Daily | No | No |
| `integrity.financial` | Hourly, and after every deployment | Reports only | No |

### Example wiring

**Linux systemd timer** (one unit per job):

```ini
# /etc/systemd/system/nbfc-risk-daily.service
[Service]
Type=oneshot
Environment="TOKEN_FILE=/etc/nbfc/scheduler.token"
ExecStart=/usr/bin/curl -fsS --retry 0 \
  -X POST "https://api.example.com/admin/ops/jobs/risk.daily/run?source=SCHEDULER" \
  -H "Authorization: Bearer @/etc/nbfc/scheduler.token" \
  -o /dev/null -w "%%{http_code}\n"
```

Note `--retry 0`: retries are unnecessary because 409 already means
"someone else is doing it", and blind retries on a financial job are the
behaviour this architecture exists to avoid.

**Kubernetes CronJob**, **cloud scheduler** (EventBridge Scheduler, Cloud
Scheduler), or **Windows Task Scheduler** follow the same shape: one
scheduled HTTP POST per job, treating 409 as success.

### Monitoring that scheduling is actually happening

```
GET /admin/ops/jobs/stale?hours=26
```

Returns any job that has not completed successfully within the window.
`healthy: false` means scheduled work is not running — alert on it.

**Until an external scheduler is configured, every job will appear
stale.** That is the correct and expected state, not a bug.

---

## 4. What is still an operator responsibility

These are **NOT IMPLEMENTED** in this repository and cannot be, because
they depend on infrastructure that does not exist here:

- Off-host / off-site replication of backup files.
- Backup encryption at rest beyond filesystem-level protection.
- Access control on the backup destination.
- Deletion protection / object-lock on backup storage.
- Alerting when a backup or restore verification fails.
- The external scheduler itself.
- Centralized log aggregation.

Each requires a decision about where this system is actually deployed.
