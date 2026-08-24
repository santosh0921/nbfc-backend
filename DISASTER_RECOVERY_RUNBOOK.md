# Disaster Recovery Runbook

Operational recovery procedures for the NBFC backend.

**Read this first — the honest preconditions.** This runbook is written
against what exists today:

- There is **no production environment**. The only database is a
  development instance on a workstation.
- Backups exist **on the same host as the database**. That is not a
  backup for disaster purposes — it does not survive host loss.
- There is **no alerting and no on-call rotation**. Every "Detection"
  step below describes what a human or an external monitor must actively
  check.
- There is **no replica and no automatic failover**.

Procedures marked **[VERIFIED]** have been executed at least once and
produced the stated result. Procedures marked **[UNTESTED]** are written
but never exercised — treat them as a plan, not a guarantee.

---

## Severity and escalation

| Severity | Definition | Escalate to |
|---|---|---|
| **SEV-1** | Financial integrity failure, unauthorized financial transaction, data loss, database corruption, confirmed security compromise | Engineering lead + Finance + Compliance, immediately |
| **SEV-2** | Major function unavailable without confirmed financial loss | Engineering lead, same business day |
| **SEV-3** | Degraded service, contained failure | Next business day |
| **SEV-4** | Minor / cosmetic | Backlog |

**Any SEV-1 touching money requires evidence preservation BEFORE
remediation.** See §Evidence preservation.

---

## Evidence preservation (mandatory before SEV-1 remediation)

1. Capture `GET /admin/ops/integrity` — save the full JSON with a timestamp.
2. Capture `GET /admin/ops/reconciliation?rows=true` — the per-record classification.
3. Capture `GET /admin/ops/jobs/executions` — recent job history.
4. Snapshot the relevant `audit_logs` rows (they cannot be altered, but fix the investigation set).
5. **Take a database backup before any corrective write**:
   `bash scripts/backup_postgres.sh`
6. Record affected identifiers: loan IDs, disbursement IDs, journal numbers, admin usernames, request IDs.

**Never correct financial data with a direct SQL `UPDATE`/`DELETE`.**
Corrections go through the application's own reversal paths
(`ReverseJournalEntry`, `ReverseExpense`) so the correction is itself
auditable.

---

## 1. Database failure

**Detection** — `GET /readiness` returns 503 with `database: unreachable`;
all API traffic failing.

**Immediate action** — Leave the application running so `/readiness`
reports honestly to any upstream proxy. Do not restart in a loop.

**Containment** — Confirm whether the database process is down, the disk
is full, or the network path is broken:
- Service state: `Get-Service postgresql-x64-16` (Windows) / `systemctl status postgresql`
- Disk space on the data volume
- `pg_isready -h <host> -p <port>`

**Recovery** — There is **no replica and no automatic failover**.
- If the instance is recoverable: restart it and confirm `/readiness` returns 200.
- If the data is lost or corrupt: restore from backup (§4).

**Verification** — `/readiness` 200; `GET /admin/ops/integrity` shows no
new critical findings; row counts sane against the last known figures.

**Escalation** — SEV-1 if data loss is suspected.

**Post-incident** — Record actual downtime against the (still undefined)
RTO target. If the cause was disk exhaustion, add disk monitoring — its
absence is a known gap.

---

## 2. Application failure

**Detection** — `/liveness` not responding; 5xx rate rising in
`GET /admin/ops/metrics`.

**Immediate action** — Restart the process. **Only one instance exists,
so this is an outage, not a failover.**

**Containment** — Capture the process log before restarting; it is not
persisted anywhere and will be lost.

**Recovery** — Start the binary; confirm `/readiness` returns 200
(proves the database reconnected).

**Verification**
- `/health`, `/readiness`, `/liveness` all 200
- Admin login succeeds
- `GET /admin/ops/integrity` — no new findings
- In-flight **exports are lost** (they run in-process); re-request them.

**Safe by design**: the payment webhook is idempotent by
`gateway_event_id`, so gateway retries after a restart cannot double-post.

**Escalation** — SEV-2, or SEV-1 if it recurs immediately after restart.

---

## 3. Backup failure

**Detection** — `GET /admin/ops/backup-status` returns health other than
`OK`:

| Health | Meaning | Action |
|---|---|---|
| `NOT_CONFIGURED` | `BACKUP_DESTINATION` unset — backups unmonitored | Configure it; treat as SEV-2 |
| `NO_BACKUP_FOUND` | No manifest, or the dump artifact is gone | Run a backup now; investigate why none exists |
| `STALE` | Newest backup older than threshold | Scheduled backup has stopped — investigate the scheduler |
| `SUSPECT_SIZE` | Dump unexpectedly small | Likely truncated/failed mid-write. **Do not trust it.** Re-run and compare sizes |

**Immediate action** — Run a backup manually:
`bash scripts/backup_postgres.sh`, then re-check status.

**Containment** — Until a good backup exists, **the RPO is unbounded**.
Avoid risky operations (migrations, bulk changes) during this window.

**Recovery** — Fix the underlying cause (credentials, disk space,
`pg_dump` availability, scheduler), then verify a successful run.

**Verification** — `backup-status` returns `OK`, `dumpFilePresent: true`,
checksum present; then run a restore drill (§Restore drill).

**Escalation** — SEV-2. SEV-1 if combined with any data-loss event.

---

## 4. Corrupt database / restore from backup **[VERIFIED]**

**Detection** — Integrity checks report critical findings; PostgreSQL
reports corruption; queries fail unexpectedly.

**Immediate action** — Stop writes. Preserve evidence (§above).

**Recovery — restore into an ISOLATED database first, never over the live one:**

```bash
export DB_HOST=... DB_PORT=... DB_USER=... DB_NAME=... DB_PASSWORD=...
export PG_BIN="/c/Program Files/PostgreSQL/16/bin"   # if not on PATH
bash scripts/restore_verify.sh /path/to/nbfc_db_<stamp>.dump
```

The script **refuses** to run if the target equals `DB_NAME` or does not
contain `_restore_test`. These are hard code-level checks.

**Verification** — the script asserts, and has been observed to produce:
checksum OK → `pg_restore` exit 0 → table/index/constraint/trigger counts
→ per-table row-count comparison → balanced journals, unique journal
numbers, no duplicate disbursements → audit triggers present. Verdict
line: `RESTORE_VERIFICATION=PASS`.

Then point the application at the restored database and confirm health,
readiness, login, `/admin/ops/integrity`, `/admin/ops/reconciliation`.

**[VERIFIED] measured**: restore 81 s; total dump-to-serving 96 s;
106 tables, 276 indexes, 559 constraints; 11/11 sampled row counts
matched including 3,568,431 audit rows; append-only audit trigger
survived.

**Promotion to live is [UNTESTED]** — swapping a restored database into
production has never been performed and requires an operator decision.

**Escalation** — SEV-1.

---

## 5. Accidental data deletion

**Detection** — Integrity/reconciliation failures (orphan ledger
entries, missing ledger entries), or a user report.

**Immediate action** — Preserve evidence. **Do not re-create deleted
financial rows by hand.**

**Critical constraint** — Journal numbers come from a PostgreSQL sequence
that never reuses a value. A deleted entry leaves a **permanent gap**.
That gap is evidence and must be preserved and explained, never filled.

**Recovery** — Restore to a **separate** database (§4), extract the
missing rows, and re-apply them through the application's own posting
paths so the re-application is itself audited.

**Verification** — `GET /admin/ops/reconciliation` shows the affected
records classified `VALID` and `unexplainedPaise: 0`.

**Escalation** — SEV-1.

---

## 6. Failed deployment

**Detection** — Health checks failing after deploy; 5xx spike.

**Immediate action** — Redeploy the previous binary.

**Critical constraint** — Schema migration is `AutoMigrate`:
**forward-only, additive, with no down-migrations**. Rolling the binary
back is safe **only if** the newer version introduced no incompatible
schema change. **[UNTESTED]** — rollback has never been exercised.

**Recovery** — Restore the previous binary; confirm `/readiness`.

**Verification** — health/readiness 200; login works; integrity checks
clean.

**Escalation** — SEV-2, SEV-1 if financial data was written by the bad
release.

---

## 7. Scheduler failure

**Detection** — `GET /admin/ops/jobs/stale?hours=26` returns
`healthy: false`, or `GET /admin/ops/jobs/executions?status=FAILED`
shows failures.

**Note**: until an external scheduler is deployed, **every job reports
stale**. That is the expected state, not an incident.

**Immediate action** — Determine which: (a) the scheduler is not calling,
(b) calls are failing auth, or (c) the job itself is failing.

**Recovery**
- Run the job manually:
  `POST /admin/ops/jobs/{name}/run?source=MANUAL`
- **HTTP 409 is not a failure.** It means another instance holds the lock
  or the job already ran this period.
- For a genuinely failing job (500), read `errorMessage` on the
  execution record.

**Crash recovery** — **[VERIFIED]** a crashed worker's advisory lock is
released when its database session ends; the job becomes runnable again
without manual intervention. A crashed run remains `RUNNING` with no
completion time and never counts as a success.

**Verification** — `GET /admin/ops/jobs` shows a recent `lastSuccessAt`.

**Escalation** — SEV-2 for a financial job (`risk.daily`,
`risk.consistency`, `collections.broken_promises`).

---

## 8. Payment reconciliation failure

**Detection** — `payment.failures` / `webhook.failures` counters rising;
payments stuck pending; reconciliation mismatches.

**Immediate action** — Determine whether the gateway is down, the
webhook secret is stale, or signature verification is failing.

**Containment** — Safe by design: an EMI is only ever marked paid by a
signature-verified webhook, so a gateway outage **delays** settlement
rather than corrupting it.

**Recovery** — Once the gateway recovers, replayed webhooks are
deduplicated by `gateway_event_id` (**[VERIFIED]** — 12 concurrent
duplicates produced exactly 1 stored row).

**Verification** — Reconciliation shows no double-posting; integrity
checks clean.

**Escalation** — SEV-2; SEV-1 if any double-posting is found.

---

## 9. Financial integrity mismatch

**Detection** — `GET /admin/ops/integrity` returns `passed: false`, or
the `integrity.financial` job returns HTTP 500.

| Failing check | Severity | First action |
|---|---|---|
| `journal_balance` | SEV-1 | Stop financial posting. The ledger does not balance; every derived report is unreliable |
| `journal_sequence` | SEV-1 | Duplicate journal numbers — the audit sequence is compromised |
| `duplicate_disbursement` | SEV-1 | Money may have been paid out twice. Confirm with the bank |
| `orphaned_journal_lines` | SEV-1 | Ledger totals include amounts with no parent entry |
| `closed_period` | SEV-2 | Postings after period close — accounting must decide treatment |
| `negative_amounts` | SEV-2 | Impossible values — find the write path |
| `disbursement_ledger_agreement` | SEV-2 | Run `GET /admin/ops/reconciliation?rows=true` for per-record classification |

**Immediate action** — Preserve evidence. Run the reconciliation report;
it classifies every record and reports `unexplainedPaise`.

**Critical constraint** — **Never silently repair.** The integrity and
reconciliation checks are read-only by construction (**[VERIFIED]** by
test), and nothing auto-corrects financial data. Any material
`UNKNOWN` amount means the financial gate FAILS.

**Escalation** — SEV-1 for any critical check; involve Finance/CA.

---

## 10. Credential compromise

**General procedure**: revoke → rotate → redeploy → validate → audit
historical usage. **Never** paste old or new secrets into a ticket or
chat.

| Credential | Rotation | Blast radius | Historical audit |
|---|---|---|---|
| `ADMIN_JWT_SECRET` | Change value, restart. Admin sessions are also re-checked server-side per request, so revoking rows in `admin_sessions` works without a restart | All admin access | `audit_logs` for the exposure window |
| `JWT_SECRET` (customer) | Change, restart — customers re-login via OTP | All customer sessions | Customer-action audit rows |
| `EMPLOYEE_JWT_SECRET` | Change, restart | Employee sessions | Employee-action audit rows |
| `PAN_ENCRYPTION_KEY` | **Cannot be rotated by changing the variable** — existing ciphertext becomes undecryptable. Requires a decrypt-old / re-encrypt-new migration. **No such tooling exists.** Planned operation only | All stored PANs | — |
| `DOCUMENT_ENCRYPTION_KEY` | Same constraint | All KYC/evidence documents | Document-access audit rows |
| Database credentials | Rotate at the database, update config, restart | Full data access | DB-side logs if available |
| `RAZORPAY_KEY_SECRET` / webhook secret | Rotate in the gateway dashboard **and** config together. A stale webhook secret shows as `webhook.failures` | Payment initiation and webhook trust | `payment_events` |

**After any rotation** — run `GET /admin/ops/integrity` and confirm no
financial invariant broke during the change.

**Escalation** — SEV-1 for any confirmed compromise.

---

## Restore drill (scheduled exercise)

Run **monthly** and after any schema migration:

1. `bash scripts/backup_postgres.sh`
2. `bash scripts/restore_verify.sh <newest dump>`
3. Point a non-production application instance at the restored database;
   confirm health, readiness, login, integrity, reconciliation.
4. Record backup duration, restore duration, total RTO.
5. Drop the restore-test database.

**The recorded output IS the evidence.** A drill without recorded output
did not happen.
