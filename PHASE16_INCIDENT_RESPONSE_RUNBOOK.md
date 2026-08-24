# Phase 16 — Incident Response Runbook

Operational runbook for this NBFC backend. Written against what the
system **actually has** — the endpoints, tables, and controls that exist
in this codebase — not against a hypothetical cloud architecture.

**Read this first**: there is currently **no on-call rotation, no paging
system, no centralized log aggregation, and no automated alerting
configured**. Every "detect" step below therefore describes what a human
or a scheduled check must actively look at. Establishing the alerting
and on-call layer is a prerequisite for this runbook to function as
written, and is listed as a production blocker in the Phase 16 report.

---

## Severity Definitions (item AE)

| Severity | Definition | Examples | Response |
|---|---|---|---|
| **SEV-1** | Financial integrity failure, unauthorized financial transaction, database corruption, production-wide outage, or confirmed security compromise | Ledger does not balance; a loan disbursed twice; audit log tampered; database unreachable; admin credentials confirmed compromised | Immediate. Preserve evidence **before** remediating. Notify management and compliance. |
| **SEV-2** | Major functional failure without confirmed financial loss | Payment webhooks failing; disbursements stuck; reconciliation mismatches accumulating; one API tier down | Same business day. |
| **SEV-3** | Degraded service or a contained failure | Elevated latency; export failures; a single scheduled job failing | Next business day. |
| **SEV-4** | Minor or cosmetic | Non-blocking UI defect; log noise | Backlog. |

**Financial incidents (any SEV-1 touching money) require evidence
preservation before any corrective action.** See §Evidence Preservation.

---

## Detection Surfaces That Exist Today

| Surface | What it tells you | How to read it |
|---|---|---|
| `GET /admin/ops/integrity` | Debit=credit, journal-number uniqueness, closed-period violations, duplicate disbursements, negative amounts, orphaned journal lines, disbursement-vs-ledger agreement | Admin JWT + `MIS_VIEW`. `passed:false` with `criticalCount>0` is a SEV-1 candidate. |
| `GET /admin/ops/metrics` | Request counts, 5xx/4xx, auth failures, rate-limit violations, export failures, per-route p50/p95/p99 | Admin JWT + `MIS_VIEW`. In-memory, single process, resets on restart. |
| `GET /health`, `/readiness`, `/liveness` | Process alive / can serve / DB reachable | Public. `/readiness` returns 503 when the database is unreachable. |
| `audit_logs` table | Every security and financial action, append-only (DB trigger blocks UPDATE/DELETE) | Query directly. `action='rate_limit.exceeded'` rows are aggregated per IP+endpoint per minute. |
| `analytics_data_quality_exceptions` | Detected data-quality problems (Phase 13) | `GET /admin/analytics/data-quality` |
| `risk_signals` | Shared bank accounts, repeated failed payments, rapid multi-loan patterns (Phase 13) | `GET /admin/analytics/signals` |
| Application stdout | One structured line per request: `level=… request_id=… method=… route=… status=… duration_ms=…` | **Not persisted anywhere by default** — see Open Risks. |

---

## Standard Response Process

1. **Detect** — from a surface above.
2. **Declare** severity and record the start time.
3. **Preserve evidence** (SEV-1 financial/security — do this *before*
   containment where possible).
4. **Contain** — stop the bleeding without destroying evidence.
5. **Investigate** — establish root cause.
6. **Eradicate** — remove the cause.
7. **Recover** — restore normal service and verify.
8. **Communicate** — management, and compliance where regulatory
   reporting may be triggered.
9. **Post-incident review** — written, blameless, with actions.

---

## Evidence Preservation (mandatory for SEV-1 financial/security)

Before changing anything:

1. Capture the integrity report: `GET /admin/ops/integrity` — save the
   full JSON with a timestamp.
2. Snapshot the relevant `audit_logs` rows (they cannot be altered, but
   capture them so the investigation set is fixed).
3. Record affected identifiers: loan ids, payment ids, journal numbers,
   admin usernames, request ids.
4. Take a database backup **before** any corrective write (see
   Backup/Restore in the Phase 16 report — note that automated backups
   are **not currently configured**, so this is a manual step today).
5. **Never** correct financial data by direct SQL UPDATE/DELETE.
   Corrections go through the application's own reversal paths
   (`ReverseJournalEntry`, `ReverseExpense`) so the correction is itself
   auditable.

---

## Scenario Playbooks (item M)

### 1. Application instance failure

- **Detect**: `/liveness` not responding; requests failing at the proxy.
- **Contain**: restart the process. Only one instance exists today, so
  this is an outage, not a failover.
- **Recover**: confirm `/readiness` returns 200 (proves DB reconnected).
- **Validate**: run `GET /admin/ops/integrity`; confirm no in-flight
  financial operation was left inconsistent. Exports that were rendering
  at the time are **lost** — they run in-process — and must be
  re-requested.
- **Note**: the payment webhook is idempotent by `gateway_event_id`, so
  a gateway retry after a restart will not double-post.

### 2. Database failure

- **Detect**: `/readiness` returns 503 with `database: unreachable`.
- **Contain**: the application cannot serve meaningful traffic; keep it
  up so `/readiness` reports honestly to the proxy.
- **Recover**: there is **no replica and no automatic failover**. Recovery
  means restoring the database from backup (manual today) and restarting.
- **Validate**: after recovery run the full integrity report and compare
  customer/loan/payment/journal counts against the last known-good
  figures.

### 3. Storage failure (documents/exports)

- **Detect**: document download or export persistence errors.
- **Impact**: KYC documents and compliance evidence are AES-256-GCM
  encrypted on the local filesystem. Loss of that volume loses the
  documents; the database rows referencing them survive and will error
  on retrieval.
- **Recover**: restore the storage volume. **The encryption key
  (`DOCUMENT_ENCRYPTION_KEY`) must match** — a restored volume with a
  rotated key is unreadable.

### 4. Region / zone failure

- **Not applicable today.** Single deployment, no multi-region
  architecture exists. Documented as a gap, not a procedure.

### 5. Credential compromise

See §Compromised Credential Procedure below.

### 6. Accidental data deletion

- **Detect**: integrity check failures (orphaned journal lines,
  disbursement-vs-ledger disagreement), or a user report.
- **CRITICAL**: do **not** attempt to re-create deleted financial rows by
  hand. Journal numbers come from a Postgres sequence that never reuses
  a value, so a deleted entry leaves a permanent gap — that gap is
  evidence and must be preserved and explained, not filled.
- **Recover**: restore from backup to a **separate** database, extract
  the missing rows, and re-apply through the application's own posting
  paths so the re-application is audited.

### 7. Corrupt deployment

- **Detect**: health checks failing after a deploy; 5xx spike in
  `/admin/ops/metrics`.
- **Recover**: redeploy the previous binary. See Rollback in the Phase 16
  report — note that schema migrations are **forward-only**
  (`AutoMigrate`), so a rollback is safe only if the newer build made no
  incompatible schema change.

### 8. Payment integration outage

- **Detect**: `payment.failures` / `webhook.failures` counters rising;
  payments stuck in a pending state.
- **Contain**: the system is safe by design here — an EMI is only ever
  marked paid by a signature-verified webhook, so a gateway outage
  delays settlement rather than corrupting it.
- **Recover**: once the gateway recovers, replayed webhooks are
  deduplicated by `gateway_event_id`.
- **Validate**: run reconciliation; confirm no payment was double-posted.

### 9. Third-party API outage (KYC / bureau / CKYC / OTP)

- **Detect**: elevated 5xx on the relevant onboarding endpoints.
- **Contain**: affected onboarding steps fail; existing loans and
  collections are unaffected.
- **Note**: OTP outage blocks customer login entirely — there is no
  alternative authentication path.

---

## Compromised Credential Procedure (item AF)

For every case: **revoke → rotate → redeploy → validate → audit
historical usage**. Never print the old or new secret into a ticket,
chat, or this document.

| Credential | Revoke / Rotate | Blast radius | Historical audit |
|---|---|---|---|
| `ADMIN_JWT_SECRET` | Change the value and restart. All admin tokens become invalid immediately; admin sessions are additionally re-checked server-side per request, so revoking sessions in `admin_sessions` also works without a restart. | All admin access | `audit_logs` for admin actions in the exposure window |
| `JWT_SECRET` (customer) | Change and restart — invalidates all customer tokens; customers re-login via OTP. | All customer sessions | Customer-action audit rows |
| `EMPLOYEE_JWT_SECRET` | Change and restart. | Employee sessions | Employee-action audit rows |
| `PAN_ENCRYPTION_KEY` | **Cannot be rotated by changing the variable alone** — existing PAN ciphertext becomes undecryptable. Requires a decrypt-with-old / re-encrypt-with-new migration. Treat as a planned operation, never an emergency toggle. | All stored PANs | — |
| `DOCUMENT_ENCRYPTION_KEY` | Same constraint as above for stored documents. | All KYC/evidence documents | Document-access audit rows |
| Database credentials | Rotate at the database, update configuration, restart. | Full data access | Review DB-side logs if available |
| `RAZORPAY_KEY_SECRET` / `RAZORPAY_WEBHOOK_SECRET` | Rotate in the gateway dashboard and in configuration together. A stale webhook secret causes signature failures — visible as `webhook.failures`. | Payment initiation and webhook trust | `payment_events` for the window |

After any rotation: run `GET /admin/ops/integrity` and confirm no
financial invariant broke during the change.

---

## Financial Integrity Alarm Response

When `GET /admin/ops/integrity` reports `passed:false`:

| Failing check | Severity | First action |
|---|---|---|
| `journal_balance` | SEV-1 | Stop financial posting activity. The ledger does not balance; every report derived from it is unreliable. Preserve evidence, then involve accounting/CA. |
| `journal_sequence` | SEV-1 | Duplicate journal numbers mean the audit sequence is compromised. Preserve evidence; do not post further until understood. |
| `duplicate_disbursement` | SEV-1 | Money may have been paid out twice. Identify the loan, confirm with the bank, involve finance. |
| `orphaned_journal_lines` | SEV-1 | Ledger totals include amounts with no parent entry. |
| `closed_period` | SEV-2 | Postings exist after period close — accounting must review and decide treatment. |
| `negative_amounts` | SEV-2 | Impossible values present; identify the write path that produced them. |
| `disbursement_ledger_agreement` | SEV-2 | The operational table and the ledger disagree. Determine which side is incomplete. **Do not "fix" either to match the other** without accounting sign-off. |

**Never silently repair.** The integrity checks are read-only by design
and nothing in the codebase auto-corrects financial data.

---

## Security Incident Response

- **Repeated authentication failures**: `auth.failures` counter and
  `audit_logs` where `result='denied'`. Rate limiting already throttles
  brute force; a sustained pattern from one source warrants blocking at
  the proxy.
- **Rate-limit violations**: `audit_logs` where
  `action='rate_limit.exceeded'`. Each row aggregates a window per
  (IP, endpoint) and states how many further violations were suppressed.
- **Unauthorized financial action attempts**: 403s on admin financial
  routes, in `audit_logs`.
- **Webhook signature failures**: indicate either a rotated/mismatched
  secret or a forgery attempt. Check whether a rotation happened first.
- **Suspicious exports**: `report.export.requested` audit rows — check
  actor, report type, and frequency.

If a compromise is **confirmed** (not merely suspected): treat as SEV-1,
preserve evidence first, then follow the credential rotation table.

---

## Post-Incident Review

Within five working days of any SEV-1 or SEV-2, produce a written review:
timeline, impact (including any financial impact, quantified), root
cause, what detection worked and what did not, and specific corrective
actions with owners. Blameless — the target is the system, not the
operator.

For incidents with potential regulatory implications, compliance must
determine whether any reporting obligation is triggered. **This runbook
does not make that determination**, and nothing in this system does.
