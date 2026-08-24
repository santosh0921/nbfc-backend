# Production Infrastructure Readiness Report

Operator handoff and deployment gate, following Phase 16.1.
**Phase 17 was not started.**

---

# Executive Summary

The decisive finding of this phase is that **no production
infrastructure exists and no working cloud access is available**. That
was verified, not assumed:

- No Dockerfile, docker-compose, Terraform, CloudFormation, Kubernetes
  manifest, Procfile, CI pipeline, or any IaC of any kind in the
  repository.
- **No cloud SDK** in `go.mod` — the entire dependency set is gin, gorm,
  golang-jwt, uuid, websocket, godotenv, gofpdf, x/crypto, x/time.
- AWS CLI **is** installed and `~/.aws/credentials` **does** exist —
  but the credentials **do not authenticate**:
  `InvalidClientTokenId` from `sts get-caller-identity`.
- `kubectl` has **zero contexts**; no cluster reachable.
- No Docker daemon.

Consequently every gate requiring cloud resources is reported
**BLOCKED — EXTERNAL INFRASTRUCTURE ACCESS REQUIRED**, with the exact
operator action stated. **Nothing was fabricated.**

Within that constraint, real, evidenced work was completed:

| Delivered | Evidence |
|---|---|
| **All 4 required scheduler failure tests (A/B/C/D)** | 10/10 job tests pass; concurrency, crash-lock-release, DB failure, duplicate run key |
| **Backup monitoring** implemented and tested | Detects the real backup plus 6 distinct failure conditions |
| **Least-privilege DB role empirically proven** | A non-superuser role **could not** drop the audit trigger, tamper with history, or create tables |
| **Disaster Recovery Runbook** | 10 scenarios, each marked VERIFIED or UNTESTED |
| **Operator Handoff** | Daily/weekly/periodic checks with honest preconditions |
| Full regression | **18/18 packages green**, `go vet` clean |

**A new HIGH-severity security finding was discovered and proven**: the
application connects to PostgreSQL as a **superuser** and can therefore
drop the append-only audit trigger that every prior phase relied on.

---

# Current Infrastructure

Verified against the repository and host, not historical documentation.

| Component | Verified state |
|---|---|
| Deployment platform | **None.** Single Go binary, run directly |
| Cloud provider | **None working.** AWS CLI present, region `ap-south-1`, credentials **invalid** |
| Container / orchestrator | **None.** No Docker daemon; `kubectl` zero contexts |
| IaC | **None** |
| CI/CD | **None** |
| PostgreSQL | 16.14, native Windows service `postgresql-x64-16`, `localhost:5433`, `nbfc_db`, ~1069 MB |
| DB user | **`postgres` — full superuser** (see Security Review) |
| DB TLS | **Off** — `ssl_active = false` |
| Object storage | **None** |
| Secrets | `.env` on host; no secret manager |
| DNS / TLS termination | **None** |
| Monitoring | In-process only (`/admin/ops/metrics`), per-process, resets on restart |
| Logging | stdout, **not persisted, not aggregated** |
| Scheduler infrastructure | **None** — application support exists, nothing calls it |

**Environment character**: a development workstation. Three dev bootstrap
admins, dev-seeded branch fixtures, and 3.5 M rate-limit audit rows from
prior load testing.

---

# Production Database

**Status: BLOCKED — EXTERNAL INFRASTRUCTURE ACCESS REQUIRED.**

No production database was created, because creating one requires cloud
or host infrastructure that is not accessible. Fabricating an "RDS
configured" claim was explicitly forbidden and would have been false.

**What WAS verified about the existing database** (real findings):

| Property | Verified value | Assessment |
|---|---|---|
| Version | PostgreSQL 16.14 | Current |
| Application role | `postgres` | **FAIL — superuser** |
| `rolsuper` / `rolcreaterole` / `rolcreatedb` / `rolbypassrls` | all `true` | **FAIL — least privilege violated** |
| Can alter `audit_logs` triggers | `true` | **FAIL — app can disable its own audit control** |
| TLS in use | `false` | **FAIL for any networked deployment** |
| Connection pool | 25 max / 10 idle / 30 m lifetime | Reasonable for one instance |
| `statement_timeout` | 30 s | Present |
| Append-only audit trigger | Present and functioning | PASS |

**Operator action required**:
1. Provision PostgreSQL on a private network, not publicly reachable.
2. Enable encryption at rest and `DB_SSLMODE=require` or stronger.
3. Create the two roles from
   `scripts/create_least_privilege_role.sql` and set the application's
   `DB_USER` to `nbfc_app`.
4. Move migrations to a separate deploy step running as `nbfc_migrate`.

---

# Backup Architecture

`scripts/backup_postgres.sh` — `pg_dump --format=custom --compress=6`,
with SHA-256 sidecar, JSON manifest, and pattern-scoped retention.
Credentials come from the environment only; `PGPASSWORD` is unset via
`trap` on exit.

**Explicit scope limit stated in the script itself**: it does **not**
upload anywhere. A backup on the database's own disk is not a backup.
No object storage exists to replicate to.

**Off-host backup: BLOCKED — EXTERNAL INFRASTRUCTURE ACCESS REQUIRED.**

---

# Backup Evidence

Carried forward from Phase 16.1 — **actually executed**:

| Field | Value |
|---|---|
| Backup ID | `nbfc_db_20260820T045308Z` |
| Mechanism | `pg_dump --format=custom --compress=6` |
| Source size | 1069 MB |
| Backup size | **128,126,510 bytes** |
| Duration | **14 seconds** |
| SHA-256 | `7fd774bbcc8d841f04a5fb7fc4b59c9d16292b8f6864333f5e04110bde6601e9` |
| Status | SUCCESS |

**New this phase — backup monitoring, implemented and tested**
(`GET /admin/ops/backup-status`):

Verified against the real artifact:
```
health=OK count=1 id=nbfc_db_20260820T045308Z
size=128126510 dur=14s dumpPresent=true findings=[]
```

And proven to detect **all six** failure conditions:

| Condition | Detected as |
|---|---|
| No directory configured | `NOT_CONFIGURED` |
| Directory missing | `NO_BACKUP_FOUND` |
| Empty directory | `NO_BACKUP_FOUND` |
| Backup too old | `STALE` |
| Backup suspiciously small | `SUSPECT_SIZE` |
| Manifest present but dump file gone | finding raised, `dumpFilePresent: false` |

**Backup automation: BLOCKED.** The script exists; nothing schedules it.

---

# Restore Evidence

Carried forward from Phase 16.1 — **actually executed into an isolated
database**, with hard code-level refusals preventing a production
target:

| Step | Result |
|---|---|
| Checksum verification | **OK** |
| `pg_restore` exit code | **0** |
| Duration | **81 seconds** |
| Tables / indexes / constraints / triggers | 106 / 276 / 559 / 2 |
| Row counts (11 tables sampled) | **11/11 matched**, incl. 3,568,431 audit rows |
| Unbalanced journal entries | 0 |
| Duplicate journal numbers | 0 |
| Duplicate successful disbursements | 0 |
| Append-only audit trigger | **Survived restore** |
| App boot / health / readiness / login | **All PASS** |
| Integrity + reconciliation on restored DB | **PASS**, identical to source |
| Verdict | **`RESTORE_VERIFICATION=PASS`** |

**Not tested**: restore of a *production* backup (none exists), and
promotion of a restored database to live.

---

# RPO/RTO

| | Value |
|---|---|
| **Target RPO** | **NOT DEFINED — BUSINESS VALIDATION REQUIRED** |
| **Measured/verified RPO** | Bounded only by backup frequency, which is **not scheduled**. With backups taking 14 s, hourly is comfortably feasible (RPO ≈ 1 h). No WAL archiving/PITR |
| **Target RTO** | **NOT DEFINED — BUSINESS VALIDATION REQUIRED** |
| **Measured/verified RTO** | **96 seconds** dump → serving application (14 s backup + 81 s restore + ~15 s boot) |

The 96 s figure **excludes** detection time, decision time, host
provisioning, and traffic routing. With no deployment automation,
provisioning is manual and real-world RTO will be substantially higher.

---

# Scheduler Deployment

**Status: PARTIAL — application complete and tested; deployment BLOCKED.**

Implementation inspected and verified against every required property:

| Property | Verified |
|---|---|
| No in-process ticker | **Confirmed** — no `time.NewTicker`/`AfterFunc`/loop in `internal/jobs` |
| Advisory locking | Yes — `pg_try_advisory_lock`, FNV-1a key on job name |
| Dedicated DB connection | Yes — required, since advisory locks are session-scoped |
| Execution records | Yes — every attempt, including skips and failures |
| Run-key idempotency | Yes — enforced by unique index |
| Panic containment | Yes — recovered and recorded FAILED |
| Error handling | Yes — error text preserved on the record |
| Observability | Yes — `job.failures` metric, staleness endpoint |

**Deployment blocked**: no scheduler infrastructure exists. Wiring for
systemd timers, Kubernetes CronJobs, cloud schedulers, and Windows Task
Scheduler is documented in `scripts/README_OPERATIONS.md`.

---

# Scheduled Job Registry

Verified live via `GET /admin/ops/jobs` — 7 jobs registered.

| Job | Schedule | Financial | Idempotency | Lock | Retry | Failure action | Manual recovery |
|---|---|---|---|---|---|---|---|
| `risk.daily` | Daily 01:00 | **Yes** | Run key (once/day) | Advisory | Next fire | SEV-2 alert | `POST .../risk.daily/run?source=MANUAL` |
| `risk.consistency` | Daily, after above | **Yes** | Naturally idempotent | Advisory | Next fire | SEV-2 alert | Manual POST |
| `collections.broken_promises` | Daily | **Yes** | Naturally idempotent | Advisory | Next fire | SEV-2 alert | Manual POST |
| `compliance.sweep` | Daily | No | Dedupe keys | Advisory | Next fire | SEV-3 alert | Manual POST |
| `analytics.data_quality` | Daily | No | Dedupe keys | Advisory | Next fire | SEV-3 alert | Manual POST |
| `analytics.risk_signals` | Daily | No | Dedupe keys | Advisory | Next fire | SEV-3 alert | Manual POST |
| `integrity.financial` | Hourly + post-deploy | Read-only | Naturally idempotent | Advisory | Next fire | **SEV-1/2 alert** | Manual POST |

**Timeouts**: no per-job timeout is implemented — a hung job holds its
lock until its connection ends. Documented as a gap, not claimed.

**Deliberately NOT scheduled**: report/export generation (user-initiated);
write-off, sanction, and disbursement approval (**maker-checker — automating
an approval defeats the control**); retention/data deletion (retention
periods seeded UNVALIDATED at 0 years; automating deletion could destroy
legally-required records).

---

# Scheduler Test Evidence

All four required tests, executed:

**Test A — two instances simultaneously** ✅
```
8 concurrent invocations -> 1 ran, 7 skipped, max simultaneous=1
```

**Test B — worker crash during execution** ✅
Simulated by holding the advisory lock on a dedicated connection and
abandoning it without unlocking, exactly as a process kill does.
```
simulated worker crash released the advisory lock; job became runnable again
crashed run recorded as RUNNING with no completion time, and does not count as success
```
No permanent lock. The crashed run stays `RUNNING` with no completion
time — which is precisely how an operator distinguishes "crashed
mid-flight" from "succeeded" — and never registers as a success.

**Test C — database unavailable** ✅
```
database failure surfaced as FAILED with error preserved:
ERROR: relation "a_table_that_does_not_exist_16_2" does not exist (SQLSTATE 42P01)
```
No silent success; error preserved; does not count as last successful run.

**Test D — same run key twice** ✅
Second run rejected with `ErrAlreadyRanForPeriod`; handler executed
exactly once.

**Full suite: 10/10 job tests pass.**

---

# Secrets Management

**Status: PARTIAL — repository clean; production mechanism BLOCKED.**

| Check | Result |
|---|---|
| Credentials in Git | **PASS** — none. `.env` untracked; `.env.example` holds only `change_this_*` placeholders |
| Secrets in new scripts | **PASS** — environment variables only; `PGPASSWORD` unset via `trap` |
| Secrets in Docker images | **N/A** — no images |
| Secrets in logs | **PASS** — boot log contains no credential material |
| DB password in source | **PASS** — none |
| API keys / JWT secrets / payment credentials in source | **PASS** — none |
| Backup artifacts committable | **PASS** — `.gitignore` excludes `backups/`, `*.dump`, `*.sha256`, `*.manifest.json` |
| **Secret manager** | **BLOCKED** — none available; secrets live in `.env` on the host |
| **Rotation mechanism** | **NOT IMPLEMENTED** — and PAN/document encryption keys **cannot** be rotated by changing the variable; existing ciphertext would become undecryptable. No re-encryption tooling exists |

---

# Production Deployment

**Status: FAIL / BLOCKED — EXTERNAL INFRASTRUCTURE ACCESS REQUIRED.**

No deployment was performed: there is no production environment, no
container image, no pipeline, and no authorized target. Deploying would
have required inventing infrastructure.

---

# Rollback

**Status: NOT TESTED.**

- No previous release artifact is retained anywhere.
- Schema migration is `AutoMigrate` — **forward-only, additive, no
  down-migrations**. Rolling a binary back is safe **only if** the newer
  version introduced no incompatible schema change.
- **No destructive or irreversible migration was found** in this phase's
  changes: the only schema change (`job_executions`) is purely additive.
- Graceful shutdown / connection draining: **not implemented**.

---

# Monitoring

**Status: PARTIAL.**

| Signal | Available | Limitation |
|---|---|---|
| Application health | `/health`, `/readiness`, `/liveness` | — |
| Database health | `/readiness` | — |
| Request latency, error rate | `/admin/ops/metrics` | **Per-process; resets on restart** |
| DB connections / wait time | `/admin/ops/metrics` | Same |
| Scheduler executions & failures | `/admin/ops/jobs/executions` | — |
| Stale jobs | `/admin/ops/jobs/stale` | — |
| **Backup status** | `/admin/ops/backup-status` | **Local directory only** |
| Financial integrity | `/admin/ops/integrity` | — |
| Reconciliation | `/admin/ops/reconciliation` | — |
| CPU / memory / disk | **Not exposed** | Host-level; no agent |
| Log aggregation | **None** | Logs lost on restart |

**Sensitive-data check**: boot and request logs verified to contain no
passwords, OTPs, MPINs, tokens, PAN, Aadhaar, or payment secrets.

---

# Alerting

**Status: NOT IMPLEMENTED.** No alerting system exists. Every signal
below is currently a human action documented in the Operator Handoff.

The endpoints exist to drive these once a platform is available:

| Alert | Source | Threshold | Operator action |
|---|---|---|---|
| Application unavailable | `/liveness` | 2 consecutive failures | Runbook §2 |
| Database unavailable | `/readiness` | 2 consecutive failures | Runbook §1 |
| High 5xx rate | `/admin/ops/metrics` | >1% over 5 min | Investigate recent deploy |
| High latency | `/admin/ops/metrics` | p95 > 2 s | Check DB wait counters |
| Backup failed / stale / suspect | `/admin/ops/backup-status` | health ≠ `OK` | Runbook §3 |
| Scheduled job failed | `/admin/ops/jobs/executions?status=FAILED` | any financial job | Runbook §7 |
| Scheduler stale | `/admin/ops/jobs/stale` | `healthy: false` | Runbook §7 |
| Integrity check failed | `/admin/ops/integrity` | any critical | **SEV-1**, Runbook §9 |
| Reconciliation mismatch | `/admin/ops/reconciliation` | `unexplainedPaise > 0` | **SEV-1**, Runbook §9 |
| High DB connection utilization | `/admin/ops/metrics` | `db.wait_count` rising | Review pool sizing |

Every alert above has a defined operator action — none is noise.

---

# Disaster Recovery

**Delivered**: `DISASTER_RECOVERY_RUNBOOK.md`, covering all 10 required
scenarios with Detection / Immediate action / Containment / Recovery /
Verification / Escalation / Post-incident.

Each procedure is explicitly marked **[VERIFIED]** or **[UNTESTED]**.
Restore-from-backup is VERIFIED with measured timings; failover, rollback,
and promotion-to-live are UNTESTED and say so.

The runbook mandates **evidence preservation before any SEV-1
remediation**, and forbids direct SQL correction of financial data.

---

# Operator Handoff

**Delivered**: `PRODUCTION_OPERATOR_HANDOFF.md` — infrastructure
inventory, daily/weekly/periodic checks, routine commands, security debt,
prohibited actions, escalation matrix, and go-live prerequisites.

It opens by stating plainly that the system is **not running in
production and cannot be handed over as if it were**.

---

# Security Review

**New HIGH finding, verified by direct query and empirical test:**

**The application connects to PostgreSQL as a superuser and can drop its
own audit protection.**

```
app_db_user | is_superuser | can_create_roles | can_create_db | bypasses_rls
postgres    | t            | t                | t             | t
```

The application's credentials can `DROP TRIGGER audit_logs_append_only`
— the control every phase since Phase 1 has relied on to make the audit
trail authoritative. A SQL-injection defect or a leaked connection string
would carry full DDL power.

**A least-privilege role was built and empirically proven** (probe role
created, tested, then removed):

| Operation | Superuser (`postgres`) | Least-privilege role |
|---|---|---|
| Read application data | ✅ (needed) | ✅ |
| Insert audit rows | ✅ (needed) | ✅ |
| **Drop the audit trigger** | ✅ **(dangerous)** | ❌ `must be owner of relation audit_logs` |
| **Update audit history** | trigger-blocked | ❌ `audit_logs is append-only: UPDATE is not permitted` |
| **Create tables** | ✅ **(dangerous)** | ❌ `permission denied for schema public` |

Deliverable: `scripts/create_least_privilege_role.sql` (two roles —
`nbfc_app` DML-only, `nbfc_migrate` DDL). **Not yet adopted**, because
the app runs `AutoMigrate` at startup and needs a deployment change first.

*Note*: the probe inserted one audit row (`action='lp.test'`, id
3568685). It was **deliberately retained** — removing it would require
bypassing the append-only control, which is exactly what must never be
done.

**Second finding**: database connections are unencrypted
(`DB_SSLMODE=disable`, `ssl_active=false`). Acceptable for a same-host
dev database; **must not** persist into any networked deployment.

**Review of this phase's own changes**: no secrets committed, no
credentials logged, no unsafe SQL (advisory locks parameterized;
reconciliation query static), no audit bypass, no destructive migration,
no financial mutation, no in-process scheduler, production database never
targeted by restore testing.

---

# Infrastructure Gaps

1. No production environment.
2. No working cloud access (AWS credentials invalid).
3. No off-host backup storage.
4. No backup scheduling.
5. No external scheduler.
6. No alerting.
7. No secret manager.
8. No log aggregation.
9. No CI/CD or rollback capability.
10. No HA — single instance, single database, no replica, no failover.
11. Application runs as a database superuser.
12. No database TLS.
13. No per-job timeout.
14. No host-level resource monitoring.

---

# External Actions Required

| # | Action | Blocks |
|---|---|---|
| 1 | Restore valid cloud credentials, or designate a non-cloud production host | Everything below |
| 2 | Provision production PostgreSQL: private network, TLS, encryption at rest | Production database gate |
| 3 | Apply `create_least_privilege_role.sql`; set `DB_USER=nbfc_app`; move migrations to a `nbfc_migrate` deploy step | Database security |
| 4 | Provision off-host encrypted backup storage with access control + deletion protection | Off-host backup gate |
| 5 | Schedule `scripts/backup_postgres.sh`; set `BACKUP_DESTINATION`, `BACKUP_MAX_AGE_HOURS`, `BACKUP_MIN_BYTES` | Backup automation |
| 6 | Deploy an external scheduler for the 7 jobs; create a dedicated admin holding only `RISK_RULE_MANAGE`; treat 409 as success | Scheduler gate |
| 7 | Provision an alerting platform; wire the 10 alerts in §Alerting | Alerting gate |
| 8 | Provision a secret manager; migrate all secrets; rotate everything ever present in a dev `.env` | Secrets gate |
| 9 | Provision log aggregation with retention and redaction | Monitoring gate |
| 10 | Establish CI/CD with artifact retention and a **tested** rollback | Deployment + rollback gates |
| 11 | Obtain RPO/RTO targets from management | RPO gate |
| 12 | Perform a production restore drill once a production backup exists | Restore gate (production) |

---

# Final Production Gates

```
Production database
NOT TESTED — BLOCKED, EXTERNAL INFRASTRUCTURE ACCESS REQUIRED

Off-host backup
FAIL — backups exist only on the database's own host

Backup automation
NOT TESTED — script exists; no scheduler runs it

Backup monitoring
PASS
Evidence: GET /admin/ops/backup-status returned health=OK against the real
backup nbfc_db_20260820T045308Z (128,126,510 bytes, dumpPresent=true), and
detected all 6 failure conditions (NOT_CONFIGURED, NO_BACKUP_FOUND, empty
directory, STALE, SUSPECT_SIZE, missing dump artifact)

Backup alerting
NOT TESTED — no alerting platform exists

Restore drill
PASS (development environment only)
Evidence: RESTORE_VERIFICATION=PASS; 81s; 106 tables / 276 indexes / 559
constraints; 11/11 sampled row counts matched incl. 3,568,431 audit rows;
append-only trigger survived; app boot + health + readiness + login +
integrity + reconciliation all PASS

RPO
BUSINESS VALIDATION REQUIRED — no target defined; capability bounded by
backup frequency, which is not scheduled

RTO
PASS (measured)
Evidence: 96 seconds dump-to-serving (14s backup + 81s restore + ~15s boot);
excludes detection, decision, and provisioning time

Scheduler deployed
NOT TESTED — BLOCKED, no scheduler infrastructure exists

Scheduler locking
PASS
Evidence: 8 concurrent invocations -> 1 ran, 7 skipped, max simultaneous = 1

Scheduler idempotency
PASS
Evidence: duplicate run key rejected with ErrAlreadyRanForPeriod; handler
executed exactly once

Scheduler failure recovery
PASS
Evidence: simulated worker crash released the advisory lock and the job
became runnable again; crashed run recorded RUNNING with no completion time
and never counts as success; database failure surfaced as FAILED with the
error preserved

Secrets management
PARTIAL
Evidence: repository verified free of credentials; .gitignore excludes backup
artifacts; no secrets in logs. BLOCKED on a secret manager; no rotation
mechanism; encryption keys not rotatable without re-encryption tooling

Production deployment
FAIL — no production environment, no image, no pipeline, no authorized target

Rollback
NOT TESTED — no retained previous artifact; AutoMigrate is forward-only

Monitoring
PARTIAL
Evidence: health/readiness/liveness, metrics, job executions, backup status,
integrity and reconciliation endpoints all verified working. No log
aggregation, no host resource metrics, metrics are per-process only

Alerting
FAIL — not implemented

Disaster recovery runbook
PASS
Evidence: DISASTER_RECOVERY_RUNBOOK.md covering all 10 required scenarios,
each step marked [VERIFIED] or [UNTESTED]

Operator handoff
PASS
Evidence: PRODUCTION_OPERATOR_HANDOFF.md with inventory, daily/weekly/
periodic checks, security debt, prohibited actions, escalation matrix
```

---

# Final Verdict

## NOT READY FOR PHASE 17

**Why.** Phase 17 is a concurrency and security implementation phase. It
would add code to a system that has:

- **No production environment to deploy to** — and no working cloud
  credentials to create one.
- **No off-host backup.** The only backups sit on the same disk as the
  database and do not survive host loss. A gate marked FAIL.
- **No alerting.** Every failure signal built across Phases 13–16.1
  currently requires a human to remember to look.
- **A newly-discovered HIGH security finding**: the application runs as a
  database superuser and can drop its own append-only audit trigger. A
  proven fix exists but is not adopted.
- **No deployment or rollback capability.**

Adding concurrency and security code on top of an unresolved operational
foundation would increase risk without increasing safety. The remaining
work is **infrastructure and operator action, not application
development** — which is precisely why the correct answer is to stop
here rather than write more code.

**What genuinely improved this phase**: the scheduler's failure
behaviour is now proven rather than assumed (all four required tests),
backup monitoring exists and detects every failure mode it claims to,
a least-privilege database design is empirically validated, and the
operational documentation states honestly what is verified versus
untested.

---

**No claim is made that this system is RBI compliant, VAPT certified,
externally audited, or production ready.** Backups are proven restorable
*in a development environment*; that is a real result and is not the same
as a verified production backup regime.

**Phase 17 must not begin until the External Actions Required table is
addressed and this gate is re-reviewed.**
