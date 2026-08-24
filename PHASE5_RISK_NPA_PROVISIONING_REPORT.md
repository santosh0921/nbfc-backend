# Phase 5 — Loan Portfolio Risk, DPD, NPA/Classification & Provisioning Foundation

Continues from Phase 1 (admin/RBAC/maker-checker), Phase 2 (payments/
disbursement/reconciliation), Phase 3 (KYC/credit bureau/onboarding), and
Phase 4 (double-entry accounting) — all re-verified building and passing
(`go build ./...`, `go test ./...`, every package green) before this
phase's work began.

**This report does not, and cannot, certify that NPA classification,
provisioning, income recognition, or collection practices are RBI
compliant. See §17.**

---

## 1. Files Changed

**New package `internal/risk/`** — DPD engine, delinquency buckets,
classification engine, provisioning engine, daily job, dashboard,
exceptions, alerts, admin handlers, tests.

**New package `internal/collections/`** — collection case service,
promise-to-pay, admin handlers.

**New package `internal/writeoff/`** — write-off/recovery service, admin
handlers.

**New models**
- `internal/models/risk.go` — `DPDRecord`, `DelinquencyBucket`, `ClassificationRule`, `LoanClassification`, `LoanClassificationHistory`, `ProvisioningRule`, `LoanProvision`, `RiskJobRun`, `RiskAlert`
- `internal/models/collections.go` — `CollectionCase`, `CollectionAssignment`, `CollectionAction`, `CollectionNote`, `CollectionPromise`, `WriteOffRequest`, `RecoveryPayment`
- `internal/models/risk_exception.go` — `DataConsistencyException`

**Modified**
- `internal/admin/migration.go` — 18 new tables registered
- `internal/rbac/permissions.go` — 7 new permissions, new `COLLECTION_MANAGER` role, all role grants updated
- `internal/models/admin_user.go` — `AdminRoleCollectionManager` constant
- `cmd/api/main.go` — `risk.SeedDefaultBuckets()` call
- `internal/routes/routes.go` — full `/admin/risk/*`, `/admin/collections/*`, `/admin/writeoffs/*` route table
- `internal/payments/webhook_handler.go` — calls `collections.CheckPromiseAfterPayment` after the verified-payment commit
- `internal/accounting/reports.go` — **bug fix, not a Phase 5 feature**: `BalanceSheet` now folds unclosed Income/Expense net (a "Current Year Earnings" line) into Equity — Phase 4 never exercised a posting that touched a P&L account without an equal-and-opposite Asset/Equity leg, so this gap in the accounting equation was latent until Phase 5's provisioning postings (Dr Expense / Cr Liability) surfaced it. See §6.

---

## 2. Database Migrations

All registered in the existing `AutoMigrate` call (same GORM-auto-migration-on-startup mechanism as Phases 1–4). New tables: `dpd_records`, `delinquency_buckets`, `classification_rules`, `loan_classifications`, `loan_classification_histories`, `provisioning_rules`, `loan_provisions`, `risk_job_runs`, `risk_alerts`, `collection_cases`, `collection_assignments`, `collection_actions`, `collection_notes`, `collection_promises`, `write_off_requests`, `recovery_payments`, `data_consistency_exceptions`.

---

## 3. DPD Engine

`risk.CalculateDPDForLoan(loanID, asOf)` (`internal/risk/dpd.go`) — for every installment due on or before `asOf`: `dueAmount` (from the EMI schedule), `paidAmount` (the full installment amount if `EmiInstallment.Status == paid`, else 0 — see §18 for the partial-payment limitation this reflects), `outstanding = dueAmount - paidAmount` (clamped ≥ 0), `dpd = max(0, days since due date)` if outstanding > 0, else 0.

Idempotent via the unique index on `(schedule_id, calculation_date)` — a second call for the same loan+date is a no-op (verified by `TestDPDCalculationIdempotent`). All amounts stored as integer paise. `FindBucketForDPD` maps a DPD value to the highest-priority `ACTIVE` `DelinquencyBucket` whose range contains it, or `nil` if none configured — never a default guess.

**Delinquency buckets are seeded** (`risk.SeedDefaultBuckets`, idempotent) with the exact example structure named in the spec (CURRENT/EARLY_OVERDUE/DELINQUENT/HIGH_RISK/NPA at 0/1-30/31-60/61-90/91+ days) — explicitly labeled `RuleVersion: "bucket-seed-v1-UNAPPROVED-EXAMPLE"` and logged as unvalidated at startup. These ranges are fully admin-editable via `POST /admin/risk/buckets`.

---

## 4. Classification Engine

`risk.ClassifyLoan(loanID, product, loanType, dpd, asOf, actor)` (`internal/risk/classification.go`) picks the most specific `APPROVED` `ClassificationRule` matching (product, loanType, dpd) as of `asOf`. **A `DRAFT` or `RETIRED` rule is never eligible** — enforced by the SQL filter itself, not a runtime check, satisfying "unapproved rule cannot be used in production" (verified by `TestUnapprovedClassificationRuleNotUsed`).

If no `APPROVED` rule matches, the function returns `ErrNoApprovedRule` and **leaves the loan's classification exactly as it was** — it never defaults to `STANDARD` or any other guess. Every classification change writes an immutable `LoanClassificationHistory` row (old/new classification, reason, rule version, calculated-by) in the same transaction as the current-state `LoanClassification` update, so the two can never disagree.

**`ManualOverrideClassification`** (item D: "never allow a normal credit/operations user... without appropriate authorization") requires both a non-empty `reason` and a non-empty `approvedBy` — refused otherwise. The HTTP handler additionally gates on the new `CLASSIFICATION_OVERRIDE` permission, held only by `SUPER_ADMIN`/`CREDIT_MANAGER` (verified by `TestUnauthorizedCannotOverrideClassification` and `TestManualOverrideCreatesHistory`).

**No `ClassificationRule` rows are seeded.** Classification labels (STANDARD/SUBSTANDARD/DOUBTFUL/LOSS) exist only as example strings in code comments — the table starts and stays empty until compliance configures and approves real rules via the maker-checker admin flow (`POST /admin/risk/classification-rules` to draft, `POST .../:id/approve` to approve — refuses the same admin doing both).

---

## 5. Provisioning Engine

`risk.CalculateProvision(loanID, classification, asOf, actor)` (`internal/risk/provisioning.go`) finds the `APPROVED` `ProvisioningRule` for the classification, computes `exposure = outstanding principal` (same "sum of unpaid installments' principal component" definition Phase 4's interest accrual already uses — deliberately kept consistent), and `provisionAmount = exposure × rate`.

Idempotent per `(loan_id, calculation_date)` (verified by `TestDuplicateProvisioningPrevented`). If classification changes since the last calculation, the previous `ACTIVE` provision is marked `REVERSED` and its accounting entry is reversed via `accounting.ReverseJournalEntry` — **never edited** — before the new provision posts (verified by `TestProvisionReversalOnReclassification`).

**No `ProvisioningRule` rows are seeded** — same reasoning as classification rules. Until compliance configures and approves rates, `CalculateProvision` returns `ErrNoApprovedProvisioningRule` and posts nothing.

---

## 6. Accounting Integration

Every provisioning journal goes through `accounting.Service.PostJournalEntry` — this package never writes `journal_entries`/`journal_entry_lines` directly. Uses the exact chart-of-accounts codes Phase 4 already seeded and reserved for this purpose (`AcctProvisionForNPA` = 2900, `AcctProvisionExpenseNPA` = 5900) — no new account codes were needed, confirming Phase 4's "design the ledger so Phase 5 can post without redesigning it" requirement was actually met.

**Bug found and fixed**: `accounting.BalanceSheet` (Phase 4) computed Equity only from Equity-type account postings, silently assuming every journal entry's non-Asset/Liability legs would net to zero — true for Phase 4's own test fixtures (which never posted a P&L-only leg without an offsetting Asset/Equity leg) but false in general. Phase 5's provisioning entries (Dr Expense / Cr Liability) exposed this: `TestBalanceSheetBalances` started failing the moment a real provisioning entry existed. Fixed by having `BalanceSheet` fold the cumulative unclosed Income−Expense net into Equity as an explicit "Current Year Earnings" line — the standard double-entry convention for an interim balance sheet, and correct until a real period-close-to-Retained-Earnings journal exists (not built in either phase — see §16).

---

## 7. Collection Workflow

`internal/collections` (item I/J/K) — `CollectionCase` opened automatically by the daily job (`collections.TouchCase`) whenever a loan has a positive overdue amount, starting at an operational status derived from DPD (`CURRENT`/`REMINDER`/`OVERDUE`/`COLLECTION_ASSIGNED` — explicitly NOT the regulatory classification). `AssignCase` (requires `COLLECTION_MANAGE`), `EscalateCase`, `RecordAction` (never touches any financial field — no amount/balance parameter exists in its signature), `CreatePromise`.

**Promise-to-pay never self-settles**: `CheckPromiseAfterPayment` is called only from the verified Razorpay webhook path, after a payment reaches `SUCCESS` — never from a timer, never from a customer claim. `MarkBrokenPromises` (called by the daily job) flags any `OPEN` promise past its date as `BROKEN` and raises an alert — it never marks one `PAID`.

---

## 8. Portfolio Dashboard

`risk.PortfolioDashboard` (`internal/risk/dashboard.go`, `GET /admin/risk/dashboard`) — every figure is a server-side SQL aggregation (`COUNT`, `SUM`, a `DISTINCT ON` latest-DPD-per-loan query) over `loan_applications`/`dpd_records`/`loan_provisions`/etc., never a client-side sum. Returns total portfolio loans, outstanding principal, current/overdue loan counts, per-bucket counts and overdue amounts, NPA loan count/amount, provision required, write-off/recovery totals. Filters: `product` (loan category), `state` (via `CustomerAddress.CurrentState`), `from`/`to` (disbursement date range).

**Documented gap**: `DPDBucket`/`Classification`/`CollectionOfficerAdminID` filters exist in the `DashboardFilters` struct but are not yet applied to the query (§16). Branch/location filtering is not implemented — no `BranchID` field exists anywhere in the loan/customer data model to filter by (§16).

---

## 9. Risk Rules / Configuration

Item S's maker-checker requirement is implemented for both `ClassificationRule` and `ProvisioningRule`: draft (`RISK_RULE_MANAGE`) → approve (`RISK_RULE_APPROVE`), with the approving admin required to differ from the drafting admin (checked in the handler, same pattern as Phase 1's loan sanction). `DelinquencyBucket` rows are simpler (no approval gate — item C frames buckets as operational/example structure, not the regulatory classification item D is strict about) but are still fully admin-editable, not hard-coded.

---

## 10. Reconciliation Checks

`risk.RunConsistencyChecks(asOf)` (`internal/risk/exceptions.go`, item T) — four checks, each writing a `DataConsistencyException` row (deduplicated: only one `OPEN` exception per type+loan at a time):

1. **Negative outstanding** — any `DPDRecord.OutstandingPaise < 0`
2. **Paid EMI still overdue** — an installment marked `paid` whose latest `DPDRecord` still shows a positive outstanding balance
3. **Duplicate provisioning** — more than one `LoanProvision` row for the same loan+date
4. **Missing journal reference** — an `ACTIVE` provision with a nonzero amount but no `JournalReference`

Run automatically at the end of every daily job execution, and on demand via `POST /admin/risk/exceptions/run-checks`. Verified by `TestConsistencyCheckDetectsNegativeOutstanding`.

**Not implemented** (§16): ledger/loan mismatch and duplicate-payment-allocation checks specifically — the former needs a defined "expected ledger balance for this loan" formula the CA hasn't specified yet; the latter is structurally prevented already by Phase 2's `PaymentReference` unique index rather than needing a detection check.

---

## 11. Scheduled Jobs

`risk.RunDailyRiskJob(runDate)` (`internal/risk/job.go`, item F) — idempotent at the run level via the unique index on `RiskJobRun.RunDate`: a `COMPLETED` run for a date is never reprocessed. A `FAILED`/`PARTIAL_FAILURE` run is safely retried against the same row (individual loan processing is itself idempotent, so a retry never double-posts anything that already succeeded). Streams every `DISBURSED` loan through `gorm.FindInBatches` (batch size 200) rather than loading the whole book into memory (item V). One loan's failure is caught and recorded without aborting the run; the run ends `PARTIAL_FAILURE` (never silently `COMPLETED`) if any loan failed, and raises a `JOB_FAILED` critical alert.

**No actual cron/scheduler infra exists in this codebase** (same gap Phase 4 flagged for interest accrual) — `POST /admin/risk/jobs/run` is what an external scheduler (or an admin, manually) must call once a day. Wiring a real cron is a P1 gap (§16).

---

## 12. RBAC Changes

7 new permissions: `RISK_VIEW`, `RISK_RULE_MANAGE`, `RISK_RULE_APPROVE`, `CLASSIFICATION_OVERRIDE`, `COLLECTION_MANAGE`, `WRITE_OFF_REQUEST`, `WRITE_OFF_APPROVE`. New role `COLLECTION_MANAGER` (item L).

| Role | Risk view | Rule manage/approve | Classification override | Collection manage | Write-off request/approve |
|---|---|---|---|---|---|
| SUPER_ADMIN | ✅ | ✅/✅ | ✅ | ✅ | ✅/✅ |
| CREDIT_MANAGER | ✅ | manage only | ✅ | – | request only |
| COMPLIANCE_OFFICER | ✅ | approve only | – | – | approve only |
| COLLECTION_OFFICER | ✅ | – | – | – | – |
| COLLECTION_MANAGER | ✅ | – | – | ✅ | – |
| ACCOUNTANT | ✅ (read-only) | – | – | – | – |
| AUDITOR | ✅ (read-only) | – | – | – | – |

Matches item L exactly: collection officers view/record/PTP only; managers additionally assign/escalate; accountants view but cannot manipulate collection status; auditors are read-only everywhere.

---

## 13. Audit Events

New audit actions: `risk.classification.changed`, `risk.classification.manual_override`, `risk.provision.calculated`, `risk.provision.reversed`, `collection.case.opened`, `collection.assigned`, `collection.escalated`, `collection.action`, `collection.ptp.created`, `collection.ptp.paid`, `collection.ptp.broken`, `writeoff.requested`, `writeoff.approved`, `writeoff.rejected`, `writeoff.recovery`. Every manual override (classification override, write-off approval) records actor, reason, old/new value, and an approver reference — verified by `TestManualOverrideCreatesHistory`.

---

## 14. Automated Test Results

`internal/risk/risk_test.go` — **11/11 passing** against the real Postgres dev DB:

1. `TestDPDCalculationCorrect` — item 1 ✅
2. `TestDPDZeroForPaidInstallment` — items 2/4 ✅
3. `TestDPDCalculationIdempotent` — item 5 ✅
4. `TestUnapprovedClassificationRuleNotUsed` — item 7 ✅
5. `TestApprovedRuleClassifiesAndProvisions` — items 6/9/12 ✅
6. `TestDuplicateProvisioningPrevented` — item 10 ✅
7. `TestProvisionReversalOnReclassification` — item 11 ✅
8. `TestUnauthorizedCannotOverrideClassification` — item 8 ✅
9. `TestManualOverrideCreatesHistory` — item 19 ✅
10. `TestDailyJobIdempotentAtRunLevel` — item 16 ✅
11. `TestConsistencyCheckDetectsNegativeOutstanding` — item 20 ✅

A pre-existing Phase 4 bug (`BalanceSheet` not accounting for unclosed P&L) was found and fixed during this work — `internal/accounting`'s own test suite (10 tests) still passes after the fix.

Full suite (`go test ./...`): all packages pass — **no regressions** across Phases 1–5.

**Not automated this phase** (§16 gap) — items 3, 13, 14 (partial), 15, 17, 18 from the spec's 20-scenario list:
- **Item 3 (partial payment calculation)**: the underlying LMS has no partial-EMI-payment capability at all (`internal/payments` always creates a gateway order for the exact full installment amount — confirmed by inspection, not assumed). `DPDRecord`'s `due/paid/outstanding` fields are architected generically enough to represent a partial payment if one ever existed, but no test could exercise this honestly without fabricating LMS behavior that doesn't exist. Flagged as a real gap, not silently skipped.
- **Item 13 (collection officer cannot modify financial balances)**: enforced by construction (`CollectionAction`'s request/response shape has no amount/balance field, and `RecordAction` never touches accounting tables) rather than by a dedicated negative-permission test.
- **Items 14/15 (write-off authorization / doesn't delete history)**: `internal/writeoff` implements the maker≠checker refusal (`ErrSameActor`) and the RBAC gate, and never issues a `DELETE` against `LoanApplication`/`Payment`/`EmiInstallment` anywhere in the package (confirmed by inspection) — no dedicated automated test was written this session due to time; this is the clearest near-term test-coverage gap to close first.
- **Item 17 (failed job retry)**: the retry-in-place code path exists (`RunDailyRiskJob` resets a `FAILED`/`PARTIAL_FAILURE` row rather than creating a new one) but isn't independently asserted — `TestDailyJobIdempotentAtRunLevel` only covers the `COMPLETED`-skip path.
- **Item 18 (dashboard matches backend calculations)**: the dashboard is built from the same queries/tables the other engines write to (by construction, not coincidence — see §8), but no dedicated cross-check test compares dashboard output against independently-computed expected figures.

---

## 15. Performance Considerations

- The daily job never loads the full loan book into memory — `gorm.FindInBatches` streams `DISBURSED` loans in batches of 200 (`internal/risk/job.go`'s `batchSize` constant).
- `DPDRecord`, `LoanProvision` unique indexes double as both the idempotency mechanism and query-performance indexes for the daily-per-loan lookups the job and dashboard both do.
- The dashboard's "latest DPD per loan" query uses Postgres `DISTINCT ON` (a single indexed pass) rather than one query per loan.
- **Not done**: no explicit composite index audit was performed against a 100k+-row synthetic dataset — the schema uses the same indexing conventions as Phases 1–4 (foreign-key columns indexed, unique constraints doubling as lookup indexes) but hasn't been load-tested at that scale. Flagged as a P1 gap for before that portfolio size is actually reached (§16).

---

## 16. Required Environment Variables

None new. Phase 5 introduces no new external integrations or secrets.

---

## 17. Rules Requiring CA/Compliance Approval

**Nothing in this list has been applied to a single real loan** — every classification/provisioning table is empty by design until these are configured and approved:

- **Delinquency bucket DPD ranges** — seeded as an explicit, labeled example (`bucket-seed-v1-UNAPPROVED-EXAMPLE`), not a validated policy.
- **Classification rule thresholds and labels** (STANDARD/SUBSTANDARD/DOUBTFUL/LOSS and their DPD ranges) — must be configured against the exact regulatory framework applicable to this NBFC (RBI's NBFC-specific IRAC norms, which differ from bank IRAC norms and have changed over recent years — the exact current version applicable to this NBFC's category must be confirmed, not assumed by this code).
- **Provisioning rates per classification** — entirely unconfigured; no percentage anywhere in this codebase.
- **Income-recognition treatment for NPA loans** — not implemented at all this phase (interest continues to accrue normally regardless of classification); Phase 4's `InterestAccrual.ExtensionPhase5` field exists specifically so Phase 5/6 can suspend recognition once the applicable rule is confirmed, but nothing does so yet.
- **Write-off accounting treatment** — `internal/writeoff`'s journal (debiting the Provision-for-NPA account rather than a fresh bad-debt expense) assumes the loan was already fully provisioned; this assumption needs explicit CA confirmation (§ code comment in `internal/writeoff/service.go`).
- **The "large overdue amount" alert threshold** (₹50,000, hardcoded in `internal/risk/process.go`) — an illustrative placeholder, explicitly commented as not a validated policy figure.
- **Collection practices/communication cadence** — the operational collection-status ladder (CURRENT→REMINDER→OVERDUE→...) is a workflow convenience, not a claim about what collection contact practices are legally permitted or required for this NBFC's customers.

---

## 18. Remaining P0/P1 Gaps

**P0**
- No `ClassificationRule`/`ProvisioningRule` rows configured — the classification/provisioning engines currently produce no output for any loan (by design, but it means Phase 5's value is entirely latent until compliance acts).
- No real cron/scheduler triggers the daily job — an external scheduler must call `POST /admin/risk/jobs/run`.
- Partial-EMI-payment support doesn't exist at the LMS level, limiting how fully the DPD engine's `paid_amount`/`outstanding_amount` fields can be exercised in practice today.

**P1**
- Dashboard `DPDBucket`/`Classification`/`CollectionOfficerAdminID` filters accepted but not applied to the query.
- No branch/location data model exists to support that dashboard filter.
- No admin panel UI screens (Portfolio Risk Dashboard, Loan Risk section, Risk Rules config, Collections workspace, Write-off queue) — only backend APIs exist, consistent with the same scope choice made in Phases 3/4's reports.
- No period-close-to-Retained-Earnings journal exists in `internal/accounting` — `BalanceSheet`'s "Current Year Earnings" line is the correct interim workaround, but a real close process is still a Phase 4/6 follow-up.
- Ledger/loan-balance and duplicate-payment-allocation consistency checks (two of the six named in item T) are not yet implemented.
- Alert de-duplication (`checkBucketChangeAlert`/`checkClassificationAlert`) uses an in-process map, not a durable DB-backed suppression window — resets on every process restart, so a restart could re-alert for accounts already known to be in a given bucket.
- No load/performance test has been run against a large (100k+ loan) synthetic portfolio.
- Write-off/recovery has no dedicated automated test (implemented and manually reasoned-through, per §14).

---

## 19. Manual Testing Checklist

```
go run ./cmd/api
```

1. **Buckets seed on boot**: confirm `GET /admin/risk/buckets` returns the 5 example buckets, each labeled with `RuleVersion` containing `UNAPPROVED-EXAMPLE`.
2. **No classification without a rule**: for a disbursed loan with an overdue installment, `POST /admin/risk/jobs/run` then `GET /admin/risk/loans/:loanId` — confirm `classification` is empty (no rule configured yet).
3. **Draft → approve a classification rule**: `POST /admin/risk/classification-rules` as one admin (DRAFT), `POST .../:id/approve` as a *different* admin — confirm success; retry approval as the *same* admin who drafted it — confirm `403`.
4. **Re-run the job**: `POST /admin/risk/jobs/run` again — confirm the overdue loan now gets classified per the approved rule, and `LoanClassificationHistory` has a new row.
5. **Provisioning**: draft+approve a `ProvisioningRule` for that classification, re-run the job — confirm a `LoanProvision` row and a balanced journal entry (`GET /admin/accounting/journal?referenceType=loan_provision`).
6. **Idempotency**: run the job a third time for the same day — confirm no new `DPDRecord`/`LoanProvision` rows are created.
7. **Collections**: confirm a `CollectionCase` was auto-opened for the overdue loan; assign it as a `COLLECTION_MANAGER`; attempt the same assignment as a `COLLECTION_OFFICER` — confirm `403`.
8. **PTP**: create a promise via `POST /admin/collections/cases/:id/promises`; manually run the job with a `date` past the promised date — confirm the promise flips to `BROKEN` and an alert appears in `GET /admin/risk/alerts`.
9. **Write-off**: `POST /admin/writeoffs` as a `CREDIT_MANAGER`, approve as `COMPLIANCE_OFFICER` — confirm the loan's history/payments are still fully present in the DB afterward.
10. **Exceptions**: `POST /admin/risk/exceptions/run-checks`, confirm the Exceptions list is empty on a clean DB; manually insert a bad `DPDRecord` (negative outstanding) and re-run — confirm it's flagged.
11. **Dashboard**: `GET /admin/risk/dashboard` and cross-check totals against `GET /admin/risk/loans/:loanId` for a couple of individual loans.
