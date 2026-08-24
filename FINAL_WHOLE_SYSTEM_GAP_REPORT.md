# FINAL WHOLE-SYSTEM GAP REPORT — Phases 1–16

Consolidated view of everything incomplete, untested, or requiring
external validation across all sixteen phases.

**Completing sixteen phases does NOT mean this NBFC is legally or
regulatorily compliant.** It does not mean VAPT passed, that an external
audit occurred, that backups work, or that the system is production
ready. This report exists to state precisely what is and is not true.

---

## How to read the status labels

| Label | Meaning |
|---|---|
| **IMPLEMENTED** | Code exists and builds |
| **TESTED** | Automated tests exercise it and pass |
| **VERIFIED** | Independently re-checked against real behaviour or real data (not just "a prior report said so") |
| **NOT TESTED** | Exists or is claimed, but no evidence was produced |
| **ASSUMED** | Believed true from design/inspection, never proven |
| **REQUIRES EXTERNAL VALIDATION** | Cannot be settled by engineering — needs CA, legal, compliance, management, or an independent assessor |

Severity categories used below: **CRITICAL**, **HIGH**, **MEDIUM**,
**LOW**, **INFORMATIONAL**, **BUSINESS VALIDATION**, **CA/ACCOUNTING
VALIDATION**, **LEGAL/REGULATORY VALIDATION**, **NOT TESTED**.

---

## Section 1 — CRITICAL (production blockers)

| # | Gap | Phase | Status | Why it blocks |
|---|---|---|---|---|
| C1 | **No automated database backups** | 16 | NOT TESTED / absent | Total data loss risk. Nothing configured, nothing verified. |
| C2 | **Restore never tested** (item K calls it mandatory) | 16 | NOT TESTED | `pg_dump`/`pg_restore`/`psql` unavailable; Docker daemon not running. Backups are worthless unproven. |
| C3 | **No job scheduler — daily financial/compliance jobs never run** | 16 | VERIFIED absent | Risk job, compliance sweep, reconciliation, data-quality and integrity checks are manual-POST only. Verified: no ticker/cron anywhere in `cmd/api`. |
| C4 | **No alerting, no centralized logging, no on-call** | 16 | Absent | Detection surfaces exist; nothing watches them or notifies anyone. Logs go to stdout and do not survive restart. |
| C5 | **No deployment pipeline, no CI, no rollback test** | 16 | Absent | No Dockerfile, no IaC, no `.github/`. Deployment is manual; rollback never exercised. |
| C6 | **Zero compliance/CA configuration** | 5,10,11 | REQUIRES EXTERNAL VALIDATION | Classification rules, provisioning rules, TDS rules, retention periods, NBFC profile all deliberately unseeded. The system cannot produce regulatory output until qualified people configure them. |
| C7 | **No independent VAPT** | 14 | REQUIRES EXTERNAL VALIDATION | Scope document ready; no engagement performed. |
| C8 | **RPO/RTO undefined** | 16 | BUSINESS VALIDATION | No recovery targets set, so current capability cannot be judged sufficient or not. |
| C9 | **Production credentials not configured** (payment gateway, KYC, bureau, OTP) | 1,2,3 | 👤 EXTERNAL | EMI online payment is disabled without them; OTP outage blocks all customer login. |
| C10 | **Live PAN verification is still a stub** | 1 | IMPLEMENTED (stub) | Real KYC cannot be performed. Carried unresolved since Phase 1. |

## Section 2 — HIGH

| # | Gap | Phase | Status |
|---|---|---|---|
| H1 | **Open `disbursement_ledger_agreement` integrity failure** — dev DB shows ₹380,000 (disbursement table) vs ₹715,000 (ledger), difference ₹335,000 | 16 | VERIFIED failing; cause believed to be accumulated test data but **not proven**. Must be re-run on production data and resolved with accounting. |
| H2 | **Analytics performance at target transaction volume unproven** | 13,15,16 | NOT TESTED. Only 2,000 customers seeded; no loans/payments/journals at volume. ~2 ms analytics figures reflect near-empty tables. |
| H3 | **Per-route security coverage incomplete** | 14,16 | 291/291 routes INVENTORIED; ~20 scenario-level security tests. Most individual admin routes NOT TESTED for IDOR/parameter manipulation. |
| H4 | **No high availability** | 16 | Single instance. Rate limiter and export workers are per-instance state that breaks under horizontal scaling. Instance failure/replacement NOT TESTED. |
| H5 | **No database replica or failover** | 16 | Absent. Recovery means restore-from-backup, which is itself untested (C2). |
| H6 | **DR documented but never drilled** | 16 | Nine playbooks written; zero exercised. Documentation is not a test. |
| H7 | **`audit_logs` bloat is permanent and attacker-influenceable** | 16 | VERIFIED: one soak produced 3.56M rows / 1,029 MB, 99.9% rate-limit noise. Table is append-only by design, so it **cannot be pruned** through the application. Amplification fixed; the accumulated data remains. |
| H8 | **No least-privilege verification for the application DB user** | 16 | ASSUMED. Cannot be checked from application code; INFRA must confirm the app user is not a superuser. |
| H9 | **Encryption-key rotation has no tooling** | 16 | PAN/document keys cannot be rotated by changing the variable — existing ciphertext becomes undecryptable. No decrypt/re-encrypt migration exists. |
| H10 | **No disk/storage monitoring** | 16 | Absent. Database or document-volume exhaustion would be discovered by outage. |

## Section 3 — MEDIUM

| # | Gap | Phase | Status |
|---|---|---|---|
| M1 | Full pagination sweep incomplete — `/admin/users`, `/admin/loans`, `/admin/payments` done; other list endpoints not audited | 15 | IMPLEMENTED partially |
| M2 | In-flight exports lost on restart (in-process workers) | 15,16 | Known limitation |
| M3 | Export execution cap does not kill a running render (Go cannot safely kill a goroutine) — bounds pending time and frees the slot only | 16 | IMPLEMENTED with documented limitation |
| M4 | Metrics are in-memory, single-process, reset on restart, not aggregated across instances | 16 | IMPLEMENTED with stated limitation |
| M5 | No migration versioning, no down-migrations (`AutoMigrate` forward-only) | 16 | Known constraint |
| M6 | Deep offset pagination degrades on large tables; keyset paging not implemented | 15 | Known limitation |
| M7 | Out-of-process load testing never performed — all figures exclude network/TLS/proxy overhead | 15,16 | NOT TESTED |
| M8 | DB-side metrics (locks, deadlocks, slow-query stats) never captured | 15,16 | NOT TESTED — no `psql` access |
| M9 | Soak beyond 30 minutes not performed (30 min pre-fix + 10 min post-fix were run) | 16 | NOT TESTED beyond that |
| M10 | Frontend build, performance, and security review not exercised | 15,16 | NOT TESTED |
| M11 | External admin panel must be updated for Phase 15 breaking changes (`/admin/loans` shape; `/admin/users` `total` semantics) | 15,16 | ⚪ N/A in-repo (no admin frontend here); real for any external panel |
| M12 | Phase 11/12 Excel and PDF report gaps — PDF management reports (7 types) never built; several named workbooks unwired | 11,12 | NOT IMPLEMENTED |
| M13 | Report snapshot/versioning for approved MIS reports | 12 | NOT IMPLEMENTED |
| M14 | Cash flow reporting foundation | 12 | NOT IMPLEMENTED |
| M15 | Statistical forecasting | 13 | NOT IMPLEMENTED (deliberately — labelling requirements not met) |
| M16 | Automated data-lineage exception detection for regulatory returns | 11 | Blocking mechanism exists; detection queries do not |
| M17 | Admin-initiated refund workflow | 9 | NOT IMPLEMENTED |
| M18 | `PostTDSDeduction`/`finance.MarkPaid` not automatically linked | 10 | NOT IMPLEMENTED |
| M19 | GST input-tax-credit tracking | 8 | NOT IMPLEMENTED |
| M20 | `go test -race` never run | 9,16 | NOT TESTED |

## Section 4 — LOW

| # | Gap | Phase | Status |
|---|---|---|---|
| L1 | `Referrer-Policy` / `Permissions-Policy` headers missing | 14,16 | NOT IMPLEMENTED |
| L2 | No OpenAPI/Swagger inventory (route inventory is generated instead) | 14,16 | NOT IMPLEMENTED |
| L3 | Various alert triggers unbuilt (large failed-payment spike, repeated webhook failure, regulatory-return pending/rejected) | 9,11 | NOT IMPLEMENTED |
| L4 | Product/geography-level revenue, expense, and profitability allocation | 13 | Correctly NOT built — no cost allocation exists and inventing one was explicitly forbidden |
| L5 | Channel and customer-type/risk-category segmentation | 13 | Correctly NOT built — no such data exists anywhere |

## Section 5 — NOT TESTED (explicit)

Areas where no evidence was produced, listed so none is mistaken for
verified:

- Database restore (C2) — **mandatory per Phase 16 item K**
- Disaster recovery drill (H6)
- Deployment rollback (C5)
- Instance failure / replacement / traffic redistribution (H4)
- TLS certificate expiry/mismatch behaviour — no TLS terminator exists
- Live SSRF probing against infrastructure (code-review only)
- Analytics at target transaction volume (H2)
- Large report generation at volume (caps exist; behaviour at volume unproven)
- Soak beyond 30 minutes (M9)
- `go test -race` (M20)
- Frontend build/lint/tests (M10)
- Production smoke test — no production environment exists
- DB-internal metrics (M8)
- Most individual admin routes for IDOR (H3)

## Section 6 — BUSINESS VALIDATION REQUIRED

| # | Item | Owner |
|---|---|---|
| B1 | RPO and RTO targets | MGMT |
| B2 | NBFC profile: legal category, scale-based-regulation layer, deposit-acceptance status, registrations | MGMT/COMP |
| B3 | Management alert thresholds (Phase 13 seeds zero by design) | MGMT |
| B4 | Budget targets (Phase 13 seeds zero by design) | MGMT/FIN |
| B5 | Which loan products/channels actually exist — the system has only a free-text `Category` and no channel concept | MGMT |
| B6 | On-call rotation and incident ownership | MGMT |

## Section 7 — CA / ACCOUNTING VALIDATION REQUIRED

| # | Item | Notes |
|---|---|---|
| A1 | **Asset classification rules** (Phase 5) | Zero `ClassificationRule` rows seeded. Boot log warns explicitly. No loan can be classified until approved rules exist. |
| A2 | **Provisioning rules** (Phase 5) | Zero rows. Same constraint. |
| A3 | **Delinquency/DPD bucket definitions** (Phase 5) | Seeded as *examples*, explicitly "NOT validated against any regulatory framework". |
| A4 | **TDS rules — rates, thresholds, sections** (Phase 10) | Zero `TDSRule` rows. No TDS is computed until configured. |
| A5 | **PAT / corporate income tax** (Phase 13) | Deliberately not computed. TDS is withholding on income paid *to* the NBFC and must never be used as a tax charge. Requires a CA-reviewed provision. |
| A6 | **Funding cost completeness** (Phase 13) | Derivable from ledger account 5200 but reads zero until real borrowings are entered. |
| A7 | **The open ₹335,000 disbursement-vs-ledger discrepancy** (H1) | Must be resolved with accounting; neither side may be "fixed" to match the other without sign-off. |
| A8 | **Accounting period close policy** | Mechanism enforced; policy (when to close, who approves) is an accounting decision. |
| A9 | **Retention periods** (Phase 11) | Placeholders seeded at 0 years, marked UNVALIDATED. |

## Section 8 — LEGAL / REGULATORY VALIDATION REQUIRED

| # | Item |
|---|---|
| R1 | Whether this NBFC's actual regulatory obligations are met — **no technical control establishes this** |
| R2 | Which RBI requirements apply given the NBFC's category and layer |
| R3 | Applicable regulatory returns: names, frequencies, due dates, formats |
| R4 | Whether uploaded policy documents are legally sufficient (the system tracks *that* a policy was uploaded, never whether its content suffices) |
| R5 | Data retention and erasure obligations, including how they interact with the append-only audit log |
| R6 | Whether any incident triggers a regulatory reporting obligation (the runbook explicitly does not make this determination) |
| R7 | Concentration limits — Phase 13 reports concentration as an *indicator* and deliberately never calls anything a breach |
| R8 | Fair-practices, digital-lending, and outsourcing obligations |

## Section 9 — Security findings across all phases

| # | Finding | Phase | Severity | Status |
|---|---|---|---|---|
| S1 | JWT signing algorithm not pinned at any of 4 verification sites | 14 | MEDIUM | **FIXED**, regression-tested (`TestJWTAlgNoneRejected`) |
| S2 | CSV/Excel formula injection in exports | 14 | MEDIUM | **FIXED**, single choke-point sanitizer, regression-tested |
| S3 | **DoS amplification via synchronous rate-limit audit writes** | 16 | MEDIUM | **FIXED** — 198 denials → 1 audit row. Consequences measured: 3.56M rows / 1 GB from one soak (H7). Post-fix soak confirms the cure: heap 904 MiB → 11.8 MiB, flat instead of climbing |
| S4 | 6 Go stdlib CVEs (net/url, crypto/tls, net/http, encoding/xml, encoding/asn1, x/net/idna) | 14→16 | MEDIUM | **FIXED** — `toolchain go1.26.6` pinned; `govulncheck` now reports 0 |
| S5 | Journal numbering via `COUNT(*)+1` — any deletion caused permanent posting outage; MAX+1 still reused numbers | 15 | HIGH (data integrity) | **FIXED** — Postgres sequence, regression-tested |
| S6 | `/admin/users` N+1 + unbounded fetch (12.97 s p50 at 2,000 customers) | 15 | HIGH (availability) | **FIXED** — 1.507 ms p50, 5,850 req/sec |

**Verified-not-assumed controls** (re-checked against current code rather
than trusted from earlier reports): admin session revocation on
logout/disable, maker≠checker on expense/sanction/return approval,
append-only audit log via database trigger, Razorpay webhook HMAC
verification, AES-256-GCM with separate PAN/document keys, three
distinct JWT secrets enforced at startup, per-endpoint rate limiting,
security headers, closed-period posting rejection, exactly-once financial
posting under concurrency.

## Section 10 — What is genuinely solid

Stated for balance, since the list above is necessarily a list of
problems:

- **Financial concurrency correctness is VERIFIED**, not assumed: 16
  concurrent identical postings → exactly 1 journal entry; 12 concurrent
  duplicate webhooks → 1 row; 10 concurrent disbursements on one loan →
  1 row; 24 concurrent postings all balanced.
- **Audit immutability is enforced at the database level**, not by
  application convention.
- **The system refuses to start unsafely** in production — missing keys,
  `DEMO_MODE=true`, or unset CORS all prevent boot.
- **Nothing fabricates regulatory or financial data.** Across Phases
  5, 10, 11, 12, 13 the consistent pattern is to seed zero rows and warn
  loudly rather than invent rules, thresholds, budgets, or classifications.
- **Dependencies are clean** (0 CVEs).
- **Load and spike behaviour is good**: 5,000 concurrent, 0% errors,
  clean recovery; 30-minute soak with no goroutine leak.

## Section 11 — Recommended next actions, in order

1. **Configure and test backups.** Automated backup + a real restore
   test with the validation procedure in the Phase 16 report §10. Nothing
   else matters if this is missing. (C1, C2)
2. **Add an external scheduler** invoking the existing authenticated job
   endpoints, with its own failure monitoring. (C3)
3. **Stand up alerting and centralized logging**, wired to
   `/admin/ops/metrics` and `/admin/ops/integrity`, plus an on-call
   rotation. (C4)
4. **Resolve the open ₹335,000 integrity discrepancy** against
   production data with accounting. (H1)
5. **Build the deployment pipeline** with a rollback test. (C5)
6. **Engage CA/compliance** to configure classification, provisioning,
   TDS, retention, and the NBFC profile. (C6, Section 7)
7. **Commission an independent VAPT** using `PHASE14_VAPT_SCOPE.md`.
   (C7)
8. **Define RPO/RTO**, then verify current capability against them. (C8)
9. **Seed realistic transaction volume** and re-measure analytics before
   any scale claim. (H2)
10. **Obtain legal/regulatory review** — Section 8 in full. (R1–R8)

---

## Final statement

Across sixteen phases this system has accumulated a substantial,
well-tested application layer with genuinely verified financial
correctness properties, and an operational layer that is largely absent.
Six real defects were found and fixed in the last three phases alone —
four of them only discovered by actively looking rather than trusting
earlier phases' own reports, which is the main lesson worth carrying
forward.

**Overall: NOT READY for production.** The remaining work is
predominantly infrastructure provisioning and qualified human
configuration, not application development.

**No claim is made — and none should be inferred — that this system is
RBI compliant, legally compliant, VAPT passed, externally audited,
backed up, disaster-recovery tested, or production ready.** Every one of
those requires validation by parties outside this engineering work.
