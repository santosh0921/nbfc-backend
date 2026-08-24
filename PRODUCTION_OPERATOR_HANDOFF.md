# Production Operator Handoff

Day-to-day operating instructions for the NBFC backend.

---

## ⚠️ Read this before anything else

**This system is not currently running in production, and cannot be
handed over as if it were.** As of this document:

| | Reality |
|---|---|
| Production environment | **Does not exist** |
| Production database | **Does not exist** — the only database is a development instance on a workstation |
| Cloud account access | **None working** — AWS CLI is installed and a credentials file exists, but the credentials are invalid (`InvalidClientTokenId`) |
| Container / orchestrator | **None** — no Docker daemon, `kubectl` has zero contexts |
| CI/CD | **None** — no pipeline of any kind in the repository |
| Off-host backups | **None** — backups are written to the same machine as the database |
| Scheduler | **Not deployed** — the application supports it; nothing calls it |
| Alerting | **None** — every check below is a human action |

This document therefore describes **how to operate the system once
infrastructure exists**, and states precisely which steps are already
proven versus merely planned.

- **[VERIFIED]** — executed at least once, produced the stated result.
- **[UNTESTED]** — written but never exercised.

---

## 1. Infrastructure inventory (current, verified)

| Component | Actual state |
|---|---|
| Application | Single Go 1.26.6 binary, `cmd/api`. No container image. |
| Database | PostgreSQL **16.14**, native Windows service `postgresql-x64-16`, `localhost:5433`, database `nbfc_db`, size ~1069 MB |
| DB credentials | **`postgres` — a full superuser.** See §Security debt |
| DB TLS | **Off** (`ssl_active = false`, `DB_SSLMODE=disable`) |
| Object storage | None |
| Secrets | `.env` file on the host. No secret manager |
| Scheduler | Application-side only (`internal/jobs`); no external caller |
| Backups | `scripts/backup_postgres.sh` → local directory |
| Monitoring | In-process metrics at `GET /admin/ops/metrics` (single-process only) |
| Logging | stdout; **not persisted, not aggregated** — lost on restart |
| Dependencies | gin, gorm/postgres, golang-jwt, uuid, websocket, godotenv, gofpdf, x/crypto, x/time. **No cloud SDK** |

---

## 2. Daily checks

Run these each morning. None is automated — there is no alerting.

### 2.1 Backup succeeded
```
GET /admin/ops/backup-status
```
Expect `health: "OK"` and `dumpFilePresent: true`.

| Health | Meaning | Action |
|---|---|---|
| `OK` | Recent backup present | None |
| `NOT_CONFIGURED` | `BACKUP_DESTINATION` unset | Configure it — backups are unmonitored |
| `NO_BACKUP_FOUND` | No manifest, or dump artifact missing | Run a backup now, investigate |
| `STALE` | Older than threshold (default 26 h) | Scheduled backup has stopped |
| `SUSPECT_SIZE` | Dump smaller than expected | Likely truncated — **do not trust it**, re-run |

**Caveat**: this monitors a **local** directory. A green status means a
dump was produced locally — **not** that a recoverable off-site backup
exists.

### 2.2 Scheduler healthy
```
GET /admin/ops/jobs/stale?hours=26
```
Expect `healthy: true`.

**Until an external scheduler is deployed this returns `healthy: false`
with all 7 jobs stale. That is expected, not an incident.**

### 2.3 Failed jobs
```
GET /admin/ops/jobs/executions?status=FAILED
```
Any row: read `errorMessage`. Financial jobs (`risk.daily`,
`risk.consistency`, `collections.broken_promises`) are SEV-2.

`SKIPPED_LOCKED` is **not** a failure — it means another instance held
the lock.

### 2.4 Financial integrity
```
GET /admin/ops/integrity
```
Expect `passed: true`, `criticalCount: 0`.

Any critical finding → SEV-1, follow the Disaster Recovery Runbook §9.
**Never repair financial data directly.**

### 2.5 Reconciliation
```
GET /admin/ops/reconciliation
```
Expect `unexplainedPaise: 0` and `fullyExplained: true`.

A non-zero unexplained amount means the financial gate FAILS — use
`?rows=true` for per-record classification.

### 2.6 Error rate and database health
```
GET /admin/ops/metrics
GET /readiness
```
Watch `http.errors.5xx` against `http.requests`, and `db.wait_count` /
`db.wait_duration_ms` (rising values mean the 25-connection pool is the
bottleneck).

**Metrics are per-process and reset on restart.** With more than one
instance they are not cluster-wide truth.

---

## 3. Weekly checks

| Check | How | Why |
|---|---|---|
| Backup verification | `GET /admin/ops/backup-status`; confirm size is consistent week-over-week | A silently shrinking dump is a failing dump |
| Storage headroom | Disk free on database and backup volumes | Disk exhaustion is a top cause of database outage; **there is no disk monitoring** |
| Failed job review | `GET /admin/ops/jobs/executions?status=FAILED` for the week | Catch intermittent failures |
| Audit growth | `SELECT count(*) FROM audit_logs;` | Was 3.5 M rows / ~1 GB, almost entirely rate-limit noise from load testing |
| Security events | `GET /admin/analytics/signals?status=OPEN` | Review-worthy patterns awaiting human triage |
| Data-quality exceptions | `GET /admin/analytics/data-quality?status=OPEN` | Detected problems awaiting resolution |

---

## 4. Periodic checks

| Cadence | Activity | Notes |
|---|---|---|
| **Monthly** | Restore drill | `scripts/restore_verify.sh` — see Runbook §Restore drill. **The recorded output IS the evidence.** |
| **Monthly** | Review open compliance/CAPA items | `GET /admin/compliance/*` |
| **Quarterly** | Disaster recovery exercise | Full Runbook walkthrough. **Never performed** |
| **Quarterly** | Access review | `GET /admin/admin-users` — confirm every account and role is still warranted |
| **Quarterly** | Credential rotation | See Runbook §10. **Note: PAN/document encryption keys cannot be rotated by changing the variable** — no re-encryption tooling exists |
| **Per release** | Dependency scan | `govulncheck ./...` |
| **After any migration** | Restore drill | Schema changes can break restore |

---

## 5. Routine operations

### Take a backup
```bash
export DB_HOST=... DB_PORT=... DB_USER=... DB_NAME=... DB_PASSWORD=...
export BACKUP_DESTINATION=/path/on/SEPARATE/storage
export PG_BIN="/c/Program Files/PostgreSQL/16/bin"   # if not on PATH
bash scripts/backup_postgres.sh
```
**[VERIFIED]** — 128,126,510 bytes in 14 s, SHA-256 recorded.

### Verify a restore
```bash
bash scripts/restore_verify.sh /path/to/nbfc_db_<stamp>.dump
```
**[VERIFIED]** — 81 s; refuses to target the live database.

### Run a job manually
```
POST /admin/ops/jobs/{name}/run?source=MANUAL
Authorization: Bearer <admin JWT with RISK_RULE_MANAGE>
```
**HTTP 409 is success**, not failure — the work is being done elsewhere.

### Job inventory
```
GET /admin/ops/jobs
```

---

## 6. Security debt an operator must know

These are **verified findings**, not theoretical:

1. **The application connects to PostgreSQL as a superuser (`postgres`).**
   Confirmed: `rolsuper`, `rolcreaterole`, `rolcreatedb`, `rolbypassrls`
   all true. **The application's own credentials can DROP the
   append-only audit trigger** — the control every prior phase relied on.

   A least-privilege design is provided
   (`scripts/create_least_privilege_role.sql`) and was **empirically
   proven**: a non-superuser role could read, could insert audit rows,
   and **could not** drop the trigger, tamper with audit history, or
   create tables.

   **It is not yet in use.** Adopting it requires a deployment change,
   because the application runs `AutoMigrate` at startup and therefore
   currently needs DDL. Migrations must become a separate step under
   `nbfc_migrate`, with the app running as `nbfc_app`.

2. **Database connections are not encrypted** (`DB_SSLMODE=disable`).
   Acceptable only for a same-host development database; **must be
   `require` or stronger** for any networked deployment.

3. **Secrets live in a `.env` file on the host.** No secret manager, no
   rotation mechanism, no audit of secret access.

4. **Logs are not persisted.** A crash loses the evidence of what caused
   it.

---

## 7. What an operator must NOT do

- **Do not** repair financial data with direct SQL `UPDATE`/`DELETE`.
  Use the application's reversal paths so the correction is auditable.
- **Do not** delete audit rows or drop the append-only trigger.
- **Do not** fill a gap in the journal-number sequence. Gaps are
  evidence; the sequence never reuses a number.
- **Do not** point `restore_verify.sh` at the live database (it refuses,
  but do not attempt to bypass it).
- **Do not** treat a `409` from the job endpoint as a failure.
- **Do not** treat local backups as disaster-recovery backups until
  off-host replication exists.

---

## 8. Escalation

| Condition | Severity | Escalate to |
|---|---|---|
| Any critical financial integrity check failing | SEV-1 | Engineering lead + Finance + Compliance, immediately |
| Unexplained reconciliation amount | SEV-1 | As above |
| Suspected credential compromise | SEV-1 | Engineering lead + Security |
| Database unavailable | SEV-1 | Engineering lead |
| Backup failing repeatedly | SEV-2 | Engineering lead, same day |
| Financial job failing | SEV-2 | Engineering lead, same day |
| Non-financial job failing | SEV-3 | Next business day |

Full procedures: `DISASTER_RECOVERY_RUNBOOK.md`.

---

## 9. Before go-live — operator prerequisites

None of these can be satisfied from the repository:

1. Provision a production PostgreSQL instance (private network, TLS,
   encryption at rest, non-superuser application role).
2. Provision off-host encrypted backup storage with access control and
   deletion protection.
3. Schedule the backup job; alert on failure.
4. Deploy an external scheduler for the 7 jobs; alert on stale.
5. Provision a secret manager; move all secrets out of `.env`; rotate
   every credential that has ever been in a development file.
6. Provision log aggregation.
7. Provision an alerting system with an on-call rotation.
8. Establish CI/CD with a tested rollback.
9. Obtain RPO/RTO targets from management.
10. Complete CA/compliance configuration (classification, provisioning,
    TDS rules, NBFC profile, retention periods) — all seeded empty by
    design.
