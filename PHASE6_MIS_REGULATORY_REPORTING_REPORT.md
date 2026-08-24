# Phase 6 — MIS, Regulatory Reporting, Compliance Monitoring & Audit Readiness

Continues from Phase 1 (admin/RBAC/maker-checker), Phase 2 (payments/
disbursement/reconciliation), Phase 3 (KYC/credit bureau/onboarding),
Phase 4 (double-entry accounting/GST/TDS), and Phase 5 (DPD/NPA/
provisioning/collections) — all re-verified building and passing
(`go build ./...`, `go test ./...`, every package green) before this
phase's work began.

**This report does not, and cannot, certify RBI compliance merely because
reporting screens exist. See §17/§18 and the compliance matrix below.**

---

## 1. Files Changed

**New package `internal/reporting/`** — `mis.go`, `regulatory.go`,
`validation.go`, `compliance.go`, `reconciliation.go`, `audit_center.go`,
`tasks.go`, `exports.go`, `retention.go`, `handler.go`, `seed.go`,
`reporting_test.go`.

**New models**: `internal/models/reporting.go` — `ReportRun`,
`RegulatoryReportDefinition`, `RegulatoryReportField`,
`RegulatoryReportVersion`, `RegulatoryReportRun`, `RegulatoryReportReview`,
`RegulatoryReportSubmission`, `RegulatoryReportException`,
`CrossSystemReconciliation`, `ComplianceTask`, `ReportExport`,
`RetentionPolicy`.

**Modified**
- `internal/admin/migration.go` — 12 new tables registered
- `internal/rbac/permissions.go` — 6 new permissions, all role grants updated
- `cmd/api/main.go` — `reporting.SeedRetentionPolicyPlaceholders()` call
- `internal/routes/routes.go` — full `/admin/reporting/*` route table
- `internal/accounting/reports.go` — **no change this phase** (Phase 5's balance-sheet fix already covers reporting correctness this phase depends on)

---

## 2. Database Migrations

All registered in the existing `AutoMigrate` call. New tables:
`report_runs`, `regulatory_report_definitions`, `regulatory_report_fields`,
`regulatory_report_versions`, `regulatory_report_runs`,
`regulatory_report_reviews`, `regulatory_report_submissions`,
`regulatory_report_exceptions`, `cross_system_reconciliations`,
`compliance_tasks`, `report_exports`, `retention_policies`.

**Note on `regulatory_report_exceptions.check_name`**: the model's `Check`
field is mapped to column `check_name`, not `check` — Postgres treats
`CHECK` as a reserved word, and using it as a bare column name breaks any
raw SQL touching that column. Caught during this session's own test run
(a genuine bug found while building, not left in) and fixed before
completion.

---

## 3. New Reporting Models

| Table | Purpose |
|---|---|
| `report_runs` | Generic run record for any ad hoc/MIS report (item A's report_id/type/period/generated_by/status shape) |
| `regulatory_report_definitions` | One row per specific regulatory return — **empty by default**, only populated when compliance names a real one |
| `regulatory_report_versions` | Immutable field-mapping snapshots per definition (item N) |
| `regulatory_report_runs` | One generation of one version for one period — carries the full maker-checker/status/submission trail |
| `regulatory_report_reviews` | Every checker decision, not just the latest |
| `regulatory_report_submissions` | Every submission event (supports resubmission history) |
| `regulatory_report_exceptions` | Validation-engine findings, PASS/WARNING/ERROR |
| `cross_system_reconciliations` | Item F's consolidated matched/mismatch/missing/duplicate view |
| `compliance_tasks` | Item J's task lifecycle |
| `report_exports` | Item L's export log |
| `retention_policies` | Item M's retention metadata (never wired to a deletion job) |

---

## 4. MIS Dashboards

`reporting.GenerateMISDashboard(filters)` (item B) — one call returns Customer/Loan/Collection/Risk/Financial metrics, each a direct aggregation over the authoritative table its own phase already owns:

- **Customer**: total/new customers from `User`; KYC pending/verified/rejected from Phase 3's `CustomerKYC.Status`.
- **Loan**: applications/approved/rejected/disbursed/active counts from `LoanApplication.Status`; outstanding principal/interest from the same "sum of unpaid `EmiInstallment` components" definition Phase 4/5 already use.
- **Collection**: EMI due/collected counts and rate from `EmiInstallment`; DPD buckets and overdue amount **reused directly from Phase 5's `risk.PortfolioDashboard`** (not recomputed); recovery rate from `RecoveryPayment` vs written-off `WriteOffRequest` totals.
- **Risk**: current/delinquent/NPA loan counts and provision figures — again pulled from `risk.PortfolioDashboard`, never recalculated independently.
- **Financial**: interest/fee income and net profit from Phase 4's `accounting.ProfitAndLoss`; GST payable from `GSTTransaction`; bank balance/borrowings from `accounting.BalanceSheet`.

Every section accepts `from`/`to`/`product`/`state` filters. Verified by `TestMISLoanCountMatchesBackend` (the MIS figure must equal a direct backend query — not an approximation).

---

## 5. Regulatory-Reporting Framework

`internal/reporting/regulatory.go` (item C/D/N/P) — **no specific RBI/NBFC return is hard-coded anywhere.** `RegulatoryReportDefinition` rows are created only by an authorized admin (`POST /admin/reporting/definitions`), start `CONFIGURED_NOT_VALIDATED`, and this codebase seeds none itself.

Workflow implemented exactly as specified: `GenerateRun` (creates a run bound to a frozen `DefinitionVersionID`, immediately runs validation) → `SubmitForReview` → `ReviewRun` (checker; refuses the preparer reviewing their own run — `ErrSameActor`; refuses approval while any `ERROR`-severity exception is unresolved — `ErrBlockingErrors`) → `SubmitRun` (records a manually-entered submission reference; **this codebase never files to a regulator's API**) → `CloseRun`.

**Item N (reproducibility)**: `RegulatoryReportRun.DefinitionVersionID`, `Parameters`, and `ResultRef`/`ChecksumSHA256` are set once at generation and never mutated by any later code path — verified by `TestApprovedRunVersionFrozenAndReproducible` (creating a *new* definition version after approval does not retroactively change what an already-approved run points to).

---

## 6. Validation Engine

`internal/reporting/validation.go` (item E) — four generic, data-source-level checks run on every `GenerateRun` call and on demand (`POST /admin/reporting/runs/:id/revalidate`):

1. **Accounting mismatch** (WARNING) — surfaces any open Phase 5 `DataConsistencyException`.
2. **Negative amounts** (ERROR, blocking) — any `DPDRecord.OutstandingPaise < 0`.
3. **Orphan references** (ERROR, blocking) — a `LoanProvision` pointing at a non-existent loan.
4. **Duplicate journal idempotency** (ERROR, blocking) — more than one `JournalEntry` sharing an `IdempotencyKey` (should be structurally impossible given Phase 4's unique index; checked independently anyway).

Findings are cleared and regenerated on every run, never accumulated stale. Verified by `TestValidationDetectsNegativeAmount` and `TestBlockingErrorsPreventApprovalAndMakerCannotApproveOwn`.

**Explicitly out of scope**: report-specific business-rule validation (e.g. "field X must equal field Y per this particular return's filing instructions") — this generic engine has no way to know a specific return's rules until compliance defines one; that layer must be added per-definition once a real regulatory return is configured (§17).

---

## 7. Compliance Dashboard

`GenerateComplianceHealthDashboard()` (item I) — Security (failed admin logins/unauthorized attempts/privileged actions in the last 24h, inactive admins), KYC, Credit, Loans, Payments, Accounting, Risk, and Regulatory sections, each reading existing tables (`AuditLog`, `CustomerKYC`, `BureauRequest`, `LoanApplication`, `Payment`, `ReconciliationException`, `DataConsistencyException`, `RegulatoryReportRun`). Introduces no new signal — purely a consolidated read.

---

## 8. Audit Center

`internal/reporting/audit_center.go` (item G/H) — `LoanTimeline(loanID)` and `CustomerTimeline(customerID)`, both **read-only by construction** (no write handler exists for either route). Reconstructed from `models.AuditLog` (append-only, DB-trigger-enforced since Phase 1) filtered to the loan/customer and every entity keyed to it (disbursement, payments, collection case, write-off request, promise), plus Phase 5's immutable `LoanClassificationHistory` for structured detail `AuditLog`'s free-text `Details` field doesn't carry. Explicitly **not** built from any mutable text field — every event source is either the append-only audit log or an immutable history table.

**Gap**: does not yet merge every possible related-entity type (e.g. individual `CollectionAction`/`CollectionPromise` detail rows, `DPDRecord` history, `BureauRequest`/`ScreeningRequest` events) into one literally complete timeline — the audit-log-derived events for those entities ARE included (since every write to them already goes through `audit.RecordDetailed`), but the typed detail rows themselves aren't separately merged in. Flagged as a P1 gap (§18).

---

## 9. Reconciliation System

`internal/reporting/reconciliation.go` (item F) — `RunCrossSystemChecks()` populates `CrossSystemReconciliation` by **importing** Phase 2's open `ReconciliationException` rows and Phase 5's open `DataConsistencyException` rows (not duplicating their detection logic), plus two new checks: a loan with DPD > 90 but no `LoanClassification` row at all, and a `SUCCESS` payment whose `EmiInstallment` can no longer be resolved. Every finding carries entity/reference/status/severity; resolution requires an explicit `ResolveReconciliation` call (audited). Verified by `TestReconciliationDetectsLoanWithoutClassification`.

**Gap**: only 2 of the 7 "systems" item F lists (loan-vs-accounting, loan-vs-payments) have a dedicated NEW check in this package — bank and collections comparisons are covered only via the imported Phase 2/5 exceptions, and KYC-vs-loan and NPA-vs-provisioning cross-checks are not yet implemented as their own comparisons (§18).

---

## 10. Report Versioning

Covered in §5 — `RegulatoryReportVersion` + `RegulatoryReportRun.DefinitionVersionID` is the mechanism; `ChecksumSHA256`/`ResultRef` fields exist on the model for when a run's output is exported (an export's own `ReportExport.StorageReference` is the actual file; wiring a checksum into that path end-to-end is a documented follow-up, not yet automatic — §18).

---

## 11. Maker-Checker Implementation

Regulatory report review (§5) mirrors Phase 5's risk-rule maker-checker exactly: `PreparedByAdminID` recorded at generation, `ReviewRun` refuses `CheckedByAdminID == PreparedByAdminID`. New RBAC permissions `REG_REPORT_MANAGE` (maker) / `REG_REPORT_APPROVE` (checker) enforce this at the route level too, not just in the service function.

---

## 12. Scheduled Compliance Checks

`RunPeriodicComplianceChecks()` (item K) — 6 checks, each creating a `ComplianceTask` (never silently fixing anything): missing KYC on a disbursed loan, failed bureau requests (7-day window), inactive admin accounts (90+ days), unmatched bank transactions (3+ days open), overdue data-consistency exceptions (7+ days open), regulatory report runs with blocking errors. Deduplicated against any still-open task for the same category+reference. **No cron infra exists in this codebase** (same documented gap as Phase 4/5's own jobs) — `POST /admin/reporting/tasks/run-periodic-checks` is what an external scheduler or admin must call.

**Not implemented from item K's full list**: expired documents, unauthorized role assignments, maker-checker violations, unusual manual adjustments, classification/provisioning inconsistencies (these last two are already covered by Phase 5's own `DataConsistencyException` checks, surfaced via the reconciliation import rather than duplicated here) — flagged as §18 gaps.

---

## 13. Export Security

`RequestExport` (item L) logs the request (who/when/report type/format/parameters) **before** generation starts, then generates asynchronously in a goroutine so a large report never blocks the request — verified by `TestExportCreatesAuditEventAndCompletesAsync`. Stored via Phase 3's `internal/storage` encrypted provider — **never a public URL**; if no storage provider is configured, the export fails closed (`ExportStatusFailed`) rather than falling back to an insecure location. RBAC-gated on `REPORT_EXPORT`, withheld from `AUDITOR` even though `AUDITOR` can view most of the same data — verified by `TestAuditorCannotExport`.

**CSV** is fully implemented. **Excel** is currently served as CSV content (which Excel opens correctly) rather than a real `.xlsx` writer — documented, not silently mislabeled. **PDF** emits a minimal but genuinely valid, openable PDF (plain-text content only, no formatting/tables) rather than a formatted report renderer — both are explicit scope reductions, not bugs (§18).

---

## 14. Automated Test Results

`internal/reporting/reporting_test.go` — **10/10 passing** against the real Postgres dev DB:

1. `TestMISLoanCountMatchesBackend` — item 1 ✅
2. `TestValidationDetectsNegativeAmount` — item 2 ✅
3. `TestBlockingErrorsPreventApprovalAndMakerCannotApproveOwn` — items 4/5 ✅
4. `TestApprovedRunVersionFrozenAndReproducible` — items 6/7/15 ✅
5. `TestUnauthorizedCannotAccessMIS` — item 9 ✅
6. `TestAuditorCannotExport` — items 8/11 ✅
7. `TestExportCreatesAuditEventAndCompletesAsync` — items 10/12 ✅
8. `TestReconciliationDetectsLoanWithoutClassification` — item 13 ✅
9. `TestComplianceTaskLifecycle` — item 14 ✅
10. `TestRetentionConfigurationNeverDeletes` — item 16 ✅

Full suite (`go test ./...`): all packages pass — **no regressions** across Phases 1–6.

**Not automated this phase**: item 3 (accounting-mismatch validation is covered indirectly — the check exists and is exercised by `TestValidationDetectsNegativeAmount`'s definition/version/run scaffolding — but no test specifically seeds an open `DataConsistencyException` and asserts the WARNING appears).

---

## 15. Performance Considerations

- MIS/dashboard queries are all `COUNT`/`SUM` aggregations or reuse Phase 5's already-batched `PortfolioDashboard` — no full-table loan/customer fetch into application memory anywhere in this package.
- Exports run asynchronously specifically so a large report's generation time never blocks an HTTP request or forces the browser to hold the whole dataset (item R).
- Compliance/reconciliation checks use `Pluck`/raw aggregate queries rather than loading full row sets where only IDs or counts are needed.
- **Not done**: no materialized/reporting tables were introduced — every MIS number is computed live on each request. At 100k+ customers this is a real scaling gap the report explicitly does not paper over: live aggregation across large `EmiInstallment`/`DPDRecord` tables will need either database materialized views, a pre-computed daily snapshot table, or caching before that scale is reached (§18).
- No load test was run against a large synthetic dataset this phase either (same gap Phase 5 flagged, still open).

---

## 16. Required Environment Variables

None new. Phase 6 introduces no new external integrations or secrets — exports use the existing Phase 3 `DOCUMENT_ENCRYPTION_KEY`-backed storage provider.

---

## 17. Professional Validation Requirements

**Nothing in this phase submits, or claims to submit, anything to a regulator.** Every regulatory-reporting item below requires the NBFC's compliance officer/CA/CS/auditor before it produces a real filing:

- **Which specific RBI/NBFC returns actually apply to this NBFC** — its category, asset size, and licensing determine this; this codebase has zero opinion and creates zero definitions.
- **Filing frequency and reporting period conventions** for each applicable return.
- **The exact field-by-field data mapping** for each return (`RegulatoryReportField` rows) — this is compliance/CA work, not something inferable from the codebase.
- **Report-specific validation rules** beyond this phase's generic checks (§6) — e.g. cross-field consistency rules a specific return's filing instructions require.
- **Retention periods** for every category in `RetentionPolicy` — every row starts at 0 years, `UNVALIDATED`; §16's model doc comment is explicit that even a "validated" period still requires a separate purge decision this codebase never automates.
- **Whether the MIS financial figures (§4) match the NBFC's actual chart-of-accounts mapping** — this reuses Phase 4's accounts, whose treatment questions (fee netting, income recognition, GST invoice numbering) are still open per the Phase 4 report.

---

## 18. Remaining Production Gaps

**P0**
- No `RegulatoryReportDefinition` exists — the entire regulatory-reporting workflow is architecturally complete but produces zero real filings until compliance configures at least one.
- No cron/scheduler triggers periodic compliance checks or the MIS refresh — external scheduling required, same gap as Phases 4/5.
- Live-aggregation MIS queries will not scale cleanly to 100k+ customers without a materialized/snapshot layer (§15).

**P1**
- Audit Center timeline doesn't yet merge every typed detail-history table (only what already flows through `AuditLog`) — see §8.
- Reconciliation dashboard covers 2 of 7 named system-pairs with dedicated new checks; the rest rely on imported Phase 2/5 findings (§9).
- Periodic compliance checks cover 6 of item K's full list; expired documents, unauthorized role assignments, maker-checker violations, and unusual manual adjustments have no dedicated check yet (§12).
- Excel export is CSV-content-as-.xlsx-request rather than a true `.xlsx` writer; PDF export is plain-text-only, no table formatting (§13).
- No admin panel UI screens (MIS Dashboard, Regulatory Reports, Report Builder, Validation, Compliance Dashboard, Audit Center, Reconciliation Center, Compliance Tasks, Report History, Data Quality) — only backend APIs exist, consistent with the scope choice made in every prior phase's report.
- Report checksum (`ChecksumSHA256`) fields exist on both `ReportRun`/`RegulatoryReportRun` but aren't yet populated end-to-end from an actual exported file's hash.

---

## Compliance Matrix (item T)

| Requirement | Applicable? | Technical implementation | Configuration required | Evidence generated | Professional validation required? | Status |
|---|---|---|---|---|---|---|
| NBFC-specific RBI regulatory return(s) | **Unknown — not determined by this codebase** | Full framework built (definitions/versions/runs/maker-checker/validation) | Yes — compliance must name every applicable return | Once configured: full run/review/submission audit trail | **Yes, mandatory** | NOT CONFIGURED |
| GST returns/filings | Likely, pending confirmation | GST transaction/invoice data exists (Phase 4); no filing workflow built | Yes | GST transaction ledger | Yes | NOT CONFIGURED |
| TDS returns/filings | Unconfirmed | TDS framework exists (Phase 4), `PENDING_REVIEW` only | Yes | None yet — nothing has been marked APPLICABLE | Yes | NOT CONFIGURED |
| NPA/IRAC classification reporting | Likely, pending confirmation | Classification engine built (Phase 5); no classification rule seeded | Yes | Classification history once rules configured | Yes | NOT CONFIGURED |
| Provisioning reporting | Likely, pending confirmation | Provisioning engine built (Phase 5); no rate configured | Yes | Provision journal trail once rates configured | Yes | NOT CONFIGURED |
| Internal MIS/management reporting | Yes (operational, not statutory) | Fully built, live | None beyond what's built | MIS dashboard, on demand | Recommended review, not mandatory | CONFIGURED — NOT VALIDATED |
| Audit trail / audit readiness | Yes (governance best practice + likely statutory) | Audit Center + append-only audit log (Phase 1) | None beyond what's built | Full timeline reconstruction | Recommended review | CONFIGURED — NOT VALIDATED |
| Data retention | Yes (statutory, specifics unconfirmed) | Retention metadata framework built, 8 categories placeholder-seeded at 0 years | Yes — real periods per category | None (metadata only) | **Yes, mandatory** | NOT CONFIGURED |

**No row above is PRODUCTION READY.** Every regulatory-facing capability in this phase is infrastructure awaiting the NBFC's compliance officer/CA/CS/auditor to supply the specific requirements this codebase is explicitly designed not to invent.
