# Phase 16 — Production Go-Live Checklist

**STATUS LEGEND**
`✅ DONE` — implemented and verified in this codebase
`⚠️ PARTIAL` — exists but incomplete or unverified
`❌ BLOCKER` — must be resolved before go-live
`⚪ N/A` — not applicable to this system as built
`👤 EXTERNAL` — requires action outside this codebase (infrastructure, business, or a third party)

**OWNER LEGEND**: ENG (engineering), INFRA (whoever operates the hosting
and database), FIN (finance/accounting), CA (chartered accountant),
COMP (compliance), MGMT (management), SEC (security/external assessor).

---

## 1. Infrastructure

| # | Item | Owner | Status | Evidence | Blocker |
|---|---|---|---|---|---|
| 1.1 | Production hosting provisioned | INFRA | 👤 EXTERNAL | No IaC, Dockerfile, or deployment manifest exists in this repo | Yes — nothing defines the production environment |
| 1.2 | Container/build artifact defined | ENG/INFRA | ❌ BLOCKER | No Dockerfile or build pipeline in repo | Yes |
| 1.3 | CI/CD pipeline | ENG | ❌ BLOCKER | No `.github/` or any CI config exists | Yes |
| 1.4 | Load balancer / reverse proxy | INFRA | 👤 EXTERNAL | Not configured in repo | Yes |
| 1.5 | Multiple application instances | INFRA | ❌ BLOCKER | Single-process design; in-memory rate limiter is per-instance (limits multiply if scaled out) | Yes, if HA is required |
| 1.6 | Trusted proxy configuration | ENG | ✅ DONE | `router.SetTrustedProxies(nil)` in `cmd/api/main.go` | No |

## 2. Database

| # | Item | Owner | Status | Evidence | Blocker |
|---|---|---|---|---|---|
| 2.1 | Production PostgreSQL provisioned | INFRA | 👤 EXTERNAL | — | Yes |
| 2.2 | `DB_SSLMODE=require` | INFRA | ⚠️ PARTIAL | Configurable and documented in `.env.example`; defaults to `disable` for local | Yes — must be set |
| 2.3 | Connection pool sized | ENG | ✅ DONE | 25 max / 10 idle / 30m lifetime, `internal/database` | No — but re-size per instance count |
| 2.4 | Statement timeout | ENG | ✅ DONE | `statement_timeout=30000` in DSN | No |
| 2.5 | Performance indexes applied | ENG | ✅ DONE | 5/5 verified at boot (`CreatePerformanceIndexes`) | No |
| 2.6 | DB user least privilege | INFRA | ⚪ NOT VERIFIED | Cannot be checked from application code | Yes — verify app user is not superuser |
| 2.7 | Disk/storage monitoring | INFRA | ❌ BLOCKER | Not configured | Yes |
| 2.8 | Replica / failover | INFRA | ❌ BLOCKER | None exists; no automatic failover | Yes, if RTO requires it |

## 3. Secrets

| # | Item | Owner | Status | Evidence | Blocker |
|---|---|---|---|---|---|
| 3.1 | No secrets committed | ENG | ✅ DONE | `.env.example` contains only `change_this_…` placeholders |No |
| 3.2 | Three distinct JWT secrets set | INFRA | ⚠️ PARTIAL | App refuses to start if unset/equal — enforced, but production values must be generated | Yes |
| 3.3 | Distinct PAN + document encryption keys | INFRA | ⚠️ PARTIAL | Enforced at startup; real keys must be generated | Yes |
| 3.4 | Secrets externalized, not logged | ENG | ✅ DONE | Read from env; no secret logged (verified Phase 14 §23, §26) | No |
| 3.5 | Rotation procedure documented | ENG | ✅ DONE | `PHASE16_INCIDENT_RESPONSE_RUNBOOK.md` §Compromised Credential Procedure | No |
| 3.6 | Encryption-key rotation plan | ENG | ⚠️ PARTIAL | Documented as requiring decrypt/re-encrypt migration; **no migration tooling exists** | Yes, before any key rotation is needed |

## 4. TLS / DNS

| # | Item | Owner | Status | Evidence | Blocker |
|---|---|---|---|---|---|
| 4.1 | HTTPS terminated | INFRA | 👤 EXTERNAL | Application serves plain HTTP; TLS is the proxy's job | Yes |
| 4.2 | Certificate + auto-renewal | INFRA | 👤 EXTERNAL | — | Yes |
| 4.3 | HSTS header | ENG | ✅ DONE | `internal/middleware/security_headers.go` (conditional on HTTPS) | No |
| 4.4 | Security headers | ENG | ✅ DONE | CSP, X-Frame-Options, X-Content-Type-Options present | No |
| 4.5 | `Referrer-Policy` / `Permissions-Policy` | ENG | ❌ MISSING | Phase 14 finding, still open | No (LOW) |
| 4.6 | `CORS_ALLOWED_ORIGINS` set to real origins | INFRA | ⚠️ PARTIAL | App refuses to start in production without it | Yes |
| 4.7 | Production domains documented | INFRA | 👤 EXTERNAL | Not defined in repo | Yes |

## 5. Application Configuration

| # | Item | Owner | Status | Evidence | Blocker |
|---|---|---|---|---|---|
| 5.1 | `APP_ENV=production`, `DEMO_MODE=false` | INFRA | ✅ ENFORCED | `ValidateProductionSafety` refuses to start otherwise | Yes — must be set |
| 5.2 | Startup fails safely on missing config | ENG | ✅ DONE | Verified — refuses to boot without keys/CORS in production | No |
| 5.3 | Go toolchain pinned to 1.26.6 | ENG | ✅ DONE | `toolchain go1.26.6` in `go.mod`; closes 6 stdlib CVEs | No |

## 6. Backups & Restore

| # | Item | Owner | Status | Evidence | Blocker |
|---|---|---|---|---|---|
| 6.1 | Automated backups configured | INFRA | ❌ BLOCKER | None configured; no backup tooling in repo | **YES** |
| 6.2 | Backup retention policy | INFRA/COMP | ❌ BLOCKER | Not defined | Yes |
| 6.3 | Backup encryption | INFRA | ❌ BLOCKER | Not defined | Yes |
| 6.4 | Backup access controls | INFRA | ❌ BLOCKER | Not defined | Yes |
| 6.5 | **Restore test performed** | INFRA/ENG | ❌ **NOT TESTED** | `pg_dump`/`pg_restore`/`psql` unavailable in this environment; Docker daemon not running | **YES — item K calls this mandatory** |
| 6.6 | Post-restore validation procedure | ENG | ✅ DOCUMENTED | Counts + integrity checks defined in the Phase 16 report §10 | No |

## 7. Disaster Recovery

| # | Item | Owner | Status | Evidence | Blocker |
|---|---|---|---|---|---|
| 7.1 | DR scenarios documented | ENG | ✅ DONE | Runbook §Scenario Playbooks (9 scenarios) | No |
| 7.2 | RPO target agreed | MGMT | ❌ BLOCKER | Not defined by the business | Yes |
| 7.3 | RTO target agreed | MGMT | ❌ BLOCKER | Not defined by the business | Yes |
| 7.4 | DR drill executed | INFRA | ❌ NOT TESTED | Never performed | Yes |
| 7.5 | Credential compromise procedure | ENG | ✅ DONE | Runbook §Compromised Credential Procedure | No |

## 8. Monitoring & Alerting

| # | Item | Owner | Status | Evidence | Blocker |
|---|---|---|---|---|---|
| 8.1 | Metrics exposed | ENG | ✅ DONE | `GET /admin/ops/metrics` — counters, gauges, per-route p50/p95/p99 | No |
| 8.2 | Health/readiness/liveness | ENG | ✅ DONE | Correct semantics, verified | No |
| 8.3 | Request correlation IDs | ENG | ✅ DONE | `RequestID()` middleware; propagated to audit rows | No |
| 8.4 | Structured request logging | ENG | ✅ DONE | One line per request with request_id, no PII/secrets | No |
| 8.5 | Metrics survive restart / aggregate across instances | ENG | ❌ MISSING | In-memory, single-process, resets on restart | Yes, for real monitoring |
| 8.6 | Centralized log aggregation | INFRA | ❌ BLOCKER | Logs go to stdout only; not persisted anywhere | Yes |
| 8.7 | Automated alerting configured | INFRA | ❌ BLOCKER | No alerting system exists; thresholds not wired to any notifier | **YES** |
| 8.8 | On-call rotation | MGMT | ❌ BLOCKER | None | Yes |

## 9. Financial Integrity

| # | Item | Owner | Status | Evidence | Blocker |
|---|---|---|---|---|---|
| 9.1 | Automated integrity checks | ENG | ✅ DONE | `GET /admin/ops/integrity` — 7 checks incl. debit=credit | No |
| 9.2 | Checks never auto-repair | ENG | ✅ VERIFIED | `TestIntegrityChecksNeverModifyFinancialData` | No |
| 9.3 | Journal sequence integrity | ENG | ✅ DONE | Postgres sequence (Phase 15 fix) + standing check | No |
| 9.4 | Integrity checks scheduled | ENG/INFRA | ❌ MISSING | Endpoint exists; nothing calls it on a schedule | Yes |
| 9.5 | Current integrity state clean | FIN/CA | ⚠️ **1 HIGH OPEN** | `disbursement_ledger_agreement` fails in the dev DB (₹380,000 vs ₹715,000) — believed to be accumulated test-data artefact, **must be re-run and resolved against production data** | Yes — verify on production data |
| 9.6 | Exactly-once financial posting | ENG | ✅ VERIFIED | Phase 15 concurrency tests (16 concurrent → 1 entry) | No |

## 10. Security

| # | Item | Owner | Status | Evidence | Blocker |
|---|---|---|---|---|---|
| 10.1 | Dependency CVE scan clean | ENG | ✅ DONE | `govulncheck`: 0 vulnerabilities after 1.26.6 pin | No |
| 10.2 | Route inventory complete | ENG | ✅ DONE | 291 routes, `PHASE16_ROUTE_INVENTORY.md` | No |
| 10.3 | All routes security-tested | SEC | ⚠️ PARTIAL | Representative subset only — inventory ≠ testing | Yes, for VAPT confidence |
| 10.4 | Independent VAPT | SEC | ❌ NOT DONE | Scope document ready (`PHASE14_VAPT_SCOPE.md`); no engagement performed | Yes |
| 10.5 | Rate limiting | ENG | ✅ DONE | Per-endpoint + blanket admin limit; audit amplification fixed this phase | No |
| 10.6 | Audit log immutable | ENG | ✅ VERIFIED | DB trigger blocks UPDATE/DELETE | No |
| 10.7 | Encryption at rest (PAN, documents) | ENG | ✅ DONE | AES-256-GCM, separate keys | No |

## 11. Payments / KYC / External

| # | Item | Owner | Status | Evidence | Blocker |
|---|---|---|---|---|---|
| 11.1 | Razorpay production credentials | INFRA | 👤 EXTERNAL | Not configured; EMI online payment disabled without them | Yes |
| 11.2 | Webhook signature verification | ENG | ✅ DONE | HMAC verified before payload trusted | No |
| 11.3 | Webhook idempotency | ENG | ✅ VERIFIED | Unique `gateway_event_id`; concurrency-tested | No |
| 11.4 | Live PAN verification provider | ENG | ❌ MISSING | Still a stub (carried from Phase 1) | Yes, for real KYC |
| 11.5 | Bureau / CKYC / screening credentials | INFRA | 👤 EXTERNAL | Provider abstractions exist; credentials not configured | Yes |
| 11.6 | OTP provider credentials | INFRA | 👤 EXTERNAL | Required — OTP outage blocks all customer login | Yes |

## 12. Reports & Exports

| # | Item | Owner | Status | Evidence | Blocker |
|---|---|---|---|---|---|
| 12.1 | Export resource limits | ENG | ✅ DONE | Rows/date-range/size/concurrency/execution caps, all configurable | No |
| 12.2 | Exports asynchronous | ENG | ✅ DONE | Goroutine + status tracking | No |
| 12.3 | Formula-injection defence | ENG | ✅ DONE | Phase 14 fix, single choke point | No |
| 12.4 | Export survives restart | ENG | ❌ MISSING | In-process; in-flight exports lost on restart | No (MEDIUM) |
| 12.5 | Large-report scalability proven | ENG | ⚠️ NOT PROVEN | Caps now exist but not tested at volume | No |

## 13. Performance

| # | Item | Owner | Status | Evidence | Blocker |
|---|---|---|---|---|---|
| 13.1 | Critical N+1 fixed | ENG | ✅ DONE | 12.97s → 1.5ms p50 (Phase 15) | No |
| 13.2 | List endpoints paginated | ENG | ⚠️ PARTIAL | `/admin/users`, `/admin/loans`, `/admin/payments` done; full sweep not completed | No |
| 13.3 | Load tested | ENG | ✅ DONE | 5,000 concurrent, 0% errors | No |
| 13.4 | Soak tested | ENG | ✅ DONE (30 min) | See Phase 16 report §27 | No |
| 13.5 | Analytics proven at transaction volume | ENG | ❌ NOT PROVEN | No loans/payments/journals seeded at scale | Yes, before claiming scale |

## 14. Frontend

| # | Item | Owner | Status | Evidence | Blocker |
|---|---|---|---|---|---|
| 14.1 | Admin panel exists | — | ⚪ N/A | No admin frontend in this repo (customer + employee Flutter apps only) | — |
| 14.2 | Phase 15 breaking changes handled | ENG | ⚪ N/A in-repo / 👤 EXTERNAL | No in-repo consumer of `/admin/loans` or `/admin/users`; **any external admin panel must be updated** | Yes, if an external panel exists |
| 14.3 | Flutter app builds | ENG | ⚠️ NOT RUN | Flutter toolchain not exercised this phase | No |

## 15. Deployment & Rollback

| # | Item | Owner | Status | Evidence | Blocker |
|---|---|---|---|---|---|
| 15.1 | Documented deploy procedure | ENG | ❌ MISSING | No pipeline or documented process | Yes |
| 15.2 | Migration safety | ENG | ⚠️ PARTIAL | `AutoMigrate` is forward-only, additive; indexes created CONCURRENTLY. **No migration versioning, no down-migrations** | Yes |
| 15.3 | Rollback tested | ENG | ❌ NOT TESTED | Never performed | Yes |
| 15.4 | Zero-downtime deployment | INFRA | ❌ NOT SUPPORTED | Single instance — deployment means downtime. **Do not claim zero downtime.** | No, but must be understood |

## 16. Business / Regulatory Validation

| # | Item | Owner | Status | Blocker |
|---|---|---|---|---|
| 16.1 | NBFC profile confirmed | COMP/MGMT | ❌ NOT DONE — zero rows seeded by design | Yes |
| 16.2 | Asset classification rules approved | CA/COMP | ❌ NOT DONE — zero `ClassificationRule` rows | Yes |
| 16.3 | Provisioning rules approved | CA/COMP | ❌ NOT DONE | Yes |
| 16.4 | TDS rules configured | CA | ❌ NOT DONE — zero `TDSRule` rows | Yes |
| 16.5 | Compliance requirements activated | COMP | ❌ NOT DONE | Yes |
| 16.6 | Retention periods configured | COMP | ❌ NOT DONE — placeholders are 0 years, UNVALIDATED | Yes |
| 16.7 | Management thresholds configured | MGMT | ❌ NOT DONE — zero rows by design | No |
| 16.8 | Independent legal/regulatory review | LEGAL | ❌ NOT DONE | **Yes — no technical control substitutes for this** |

---

## Go-Live Blocker Summary

**Hard blockers (must resolve before serving real customers):**

1. No automated database backups — and **no restore test has ever been performed**.
2. No alerting, no centralized logging, no on-call.
3. No deployment pipeline, no rollback test, no production infrastructure definition.
4. RPO/RTO undefined by the business.
5. Zero compliance/CA configuration (classification, provisioning, TDS, retention, NBFC profile).
6. No independent VAPT.
7. Production credentials for payment gateway, KYC, bureau, and OTP not configured.
8. Live PAN verification still a stub.
9. The one open HIGH integrity finding must be re-run and resolved against production data.

**This checklist does not certify regulatory compliance.** Items in §16
require independent legal, CA, and compliance determination that no part
of this system provides.
