# Phase 4 — NBFC Accounting Foundation

Continues from Phase 1 (admin/RBAC/maker-checker), Phase 2 (payments/
disbursement/reconciliation), and Phase 3 (KYC/credit bureau/onboarding) —
all re-verified building and passing (`go build ./...`, `go test ./...`,
all packages green) before this phase's work began.

**Phase 5 (NPA/IRAC classification/provisioning) is explicitly NOT
implemented here.** Extension points are documented in §19.

---

## 1. Files Changed

**New package `internal/accounting/`**
- `service.go` — `AccountingService.PostJournalEntry`, journal numbering, period auto-creation
- `chart.go` — Chart of Accounts seed + account-code constants
- `postings.go` — `PostLoanDisbursement`, `postProcessingFeeAndGST`, `PostEmiPayment`, `PostRefund`, `ReverseJournalEntry`
- `accrual.go` — `AccrueInterestForLoan`
- `periods.go` — `ClosePeriod`, `ReopenPeriod`
- `reports.go` — `TrialBalance`, `ProfitAndLoss`, `BalanceSheet`, `GeneralLedger`
- `handler.go` — all admin HTTP handlers
- `accounting_test.go` — 10 tests

**New models**
- `internal/models/accounting.go` — `LedgerAccount`, `AccountingPeriod`, `JournalEntry`, `JournalEntryLine`, `InterestAccrual`
- `internal/models/gst.go` — `GSTTransaction`, `TaxInvoice`, `TDSTransaction`
- `internal/models/borrowing.go` — `FundingSource`, `BorrowingTransaction`

**Modified**
- `internal/admin/migration.go` — 10 new tables registered
- `cmd/api/main.go` — `accounting.SeedChartOfAccounts()` call added after `admin.Bootstrap()`
- `internal/routes/routes.go` — full `/admin/accounting/*` route table
- `internal/rbac/permissions.go` — `ACCOUNTING_POST`, `PERIOD_MANAGE` permissions; role grants updated
- `internal/loans/confirm_disbursement.go` — calls `accounting.PostLoanDisbursement` after the verified-SUCCESS commit
- `internal/payments/webhook_handler.go` — calls `accounting.PostEmiPayment` after the verified-webhook commit

---

## 2. Database Migrations

All registered in `internal/admin/migration.go`'s existing `AutoMigrate` call (GORM auto-migration on every startup, same mechanism as Phases 1–3 — no separate migration runner exists in this codebase). New tables: `ledger_accounts`, `accounting_periods`, `journal_entries`, `journal_entry_lines`, `interest_accruals`, `gst_transactions`, `tax_invoices`, `tds_transactions`, `funding_sources`, `borrowing_transactions`.

---

## 3. New Accounting Tables/Models

| Table | Purpose |
|---|---|
| `ledger_accounts` | Chart of Accounts |
| `accounting_periods` | Monthly periods, OPEN/SOFT_CLOSED/CLOSED/REOPENED |
| `journal_entries` | Header row per posting — idempotency key, period, status |
| `journal_entry_lines` | Debit/credit legs — integer paise, never float |
| `interest_accruals` | One row per loan per accrual period, idempotent |
| `gst_transactions` | One row per taxable event, CGST/SGST/IGST split |
| `tax_invoices` | Sequential-numbered tax invoice records |
| `tds_transactions` | TDS framework rows, `PENDING_REVIEW` by default |
| `funding_sources` | NBFC borrowing/funding lines |
| `borrowing_transactions` | Drawdown/interest/repayment events against a funding source |

---

## 4. Chart of Accounts

Seeded idempotently by `accounting.SeedChartOfAccounts()` (inserts missing codes only, never overwrites an admin's edits to name/parent/status):

| Code | Name | Type |
|---|---|---|
| 1000 | Bank / Cash | ASSET |
| 1100 | Loan Principal Receivable | ASSET |
| 1110 | Interest Receivable | ASSET |
| 1120 | Processing Fee Receivable | ASSET |
| 1200 | GST Input Receivable | ASSET |
| 2000/2010/2020 | CGST/SGST/IGST Payable | LIABILITY |
| 2100 | TDS Payable | LIABILITY |
| 2200 | Borrowings Payable | LIABILITY |
| 2900 | Provision for NPA (Phase 5) | LIABILITY — unused in Phase 4 |
| 3000 | Equity / Capital | EQUITY |
| 3900 | Retained Earnings | EQUITY |
| 4000 | Interest Income | INCOME |
| 4100 | Processing Fee Income | INCOME |
| 4200 | Penalty / Other Income | INCOME |
| 5000 | Operating Expenses | EXPENSE |
| 5100 | Bank Charges | EXPENSE |
| 5200 | Interest Expense on Borrowings | EXPENSE |
| 5900 | Provision for NPA Expense (Phase 5) | EXPENSE — unused in Phase 4 |

Fully admin-manageable via `GET/POST /admin/accounting/accounts` and `PUT /admin/accounting/accounts/:id` — no account is hard-coded into business logic; every posting function references a code constant that resolves to a DB row at post time.

---

## 5. Journal-Entry Architecture

`AccountingService.PostJournalEntry` (`internal/accounting/service.go`) is the **sole** writer of `journal_entries`/`journal_entry_lines` in this codebase — verified by grep: no other file creates either table's rows directly.

Guarantees enforced in one function, before any DB write:
- At least 2 lines, each with exactly one of debit/credit set
- `sum(debit) == sum(credit)` (`ErrUnbalancedEntry` otherwise)
- Every `AccountCode` resolves to an `ACTIVE` `LedgerAccount` (`ErrUnknownAccount` otherwise)
- The transaction date's accounting period is not `CLOSED` (`ErrPeriodNotOpen` otherwise)
- The whole insert (entry + all lines) happens inside one `database.DB.Transaction(...)` — a failure at any point rolls back everything, never a partial post
- `IdempotencyKey` is unique — a duplicate post (same key) fails with `ErrDuplicatePosting`, whether caught by the pre-check or the DB's own unique-constraint violation on a race
- Journal numbering (`JE-<year>-<seq>`) is serialized via a transaction-scoped Postgres advisory lock (`pg_advisory_xact_lock`), avoiding the `FOR UPDATE`-with-aggregate restriction while still preventing two concurrent posts from claiming the same number

POSTED entries are never edited or deleted by any code path in this package — `ReverseJournalEntry` is the only correction mechanism (§6).

---

## 6. Accounting Posting Rules Implemented

| Event | Trigger | Entry |
|---|---|---|
| Loan disbursement | `internal/loans.ConfirmDisbursement`, only after `Disbursement.Status == SUCCESS` with a captured UTR — never on admin approval alone | Dr Loan Principal Receivable / Cr Bank-Cash |
| Processing fee + GST | Same disbursement event, using the fee/GST figures already computed and stored on the loan at sanction time (not recomputed) | Dr Processing Fee Receivable / Cr Processing Fee Income, Cr CGST/SGST or Cr IGST |
| EMI payment | `internal/payments` webhook, only after a verified gateway signature moves `Payment.Status` to `SUCCESS` | Dr Bank-Cash / Cr Loan Principal Receivable, Cr Interest Receivable (split exactly matches the EMI schedule's own `PrincipalComponent`/`InterestComponent`) |
| Interest accrual | `accounting.AccrueInterestForLoan`, idempotent per (loan, period) | Dr Interest Receivable / Cr Interest Income |
| Refund | `accounting.PostRefund` (available for wiring into the existing Phase 2 refund flow) | Dr Loan Principal Receivable / Cr Bank-Cash |
| Borrowing drawdown | `POST /admin/accounting/funding` | Dr Bank-Cash / Cr Borrowings Payable |
| Manual adjustment/reversal | `POST /admin/accounting/journal`, `.../journal/:id/reverse` (ACCOUNTING_POST only) | Admin-specified, still balance-validated |

**Design note on the processing fee (item G):** the fee is booked as a receivable, not netted out of the disbursed cash, because the actual disbursed amount in this codebase is still the full sanctioned principal (Phase 2 behavior, unchanged per instruction). This is the technically-correct reflection of the *actual* money movement today, but whether the NBFC's real process should instead net the fee out of the disbursement is a commercial/accounting-treatment question for the CA — see §20.

---

## 7. Interest Accrual Implementation

`AccrueInterestForLoan(loanID, periodCode, periodStart, periodEnd, actor)` — simple daily interest (`rate/100/365 × days`) on the loan's actual outstanding principal (summed from unpaid `EmiInstallment.PrincipalComponent` rows, not the static original disbursed amount). Idempotent via the unique index on `(loan_id, accrual_period)` — a second call for the same loan+period returns the existing row unchanged (verified by `TestInterestAccrualIdempotent`). No scheduler/cron is wired up in this phase — the function is ready to be called by one (a manual admin trigger or a future scheduled job), per the "support scheduled/periodic execution" requirement being an extension point, not a running job today.

`InterestAccrual.ExtensionPhase5` is an explicit, currently-unused string field for Phase 5 to mark an accrual whose income recognition was suspended under IRAC norms — no schema change needed when that lands.

---

## 8. GST Accounting / Invoice Implementation

GST posting reuses the loan's already-approved `ProcessingFeeCGST`/`SGST`/`IGST`/`GSTType` fields (Phase 1/2's `calculateProcessingFeeGST`) — this phase does not recompute or alter the tax calculation, only posts it and records a `GSTTransaction` row (`SupplyType`, `PlaceOfSupply`, taxable value, CGST/SGST/IGST, `FinancialReference`, `JournalReference`).

`TaxInvoice` model exists (`internal/models/gst.go`) with sequential `InvoiceNumber` support (series + suffix) — **no invoice-generation handler was wired up this phase** (see §19); the existing sanction/disbursement PDF letters (Phase 1) are explicitly not treated as GST-compliant tax invoices, per instruction. `AdminListInvoicesHandler` exists for whatever invoices get created (manually or by a future generator).

---

## 9. TDS Framework / Status

**Framework only, `PENDING_REVIEW` by default, nothing auto-applies.** `TDSTransaction` model + `AdminListTDSHandler` (read-only) exist; no business event in this codebase creates a `TDSTransaction` row or deducts TDS from anything. Per instruction: "Do NOT blindly apply TDS to every loan transaction" — this is honored by not applying it at all yet, pending the CA confirming which specific scenarios (if any) it applies to for this NBFC.

---

## 10. Bank Reconciliation Integration

Phase 2's reconciliation (`internal/reconciliation`) already never touches ledger balances directly — a matched `Disbursement` only reaches `SUCCESS` via `loans.ConfirmDisbursement`, which is exactly the function this phase hooked accounting into (§6). So a bank-statement match reconciling a disbursement now transitively triggers the correct accounting posting, with no change needed to the reconciliation package itself. Manual matches already require a `Reason` and admin attribution (`ReconciliationRecord.MatchType == "manual"`, `ReconciledByAdminID`) — pre-existing Phase 2 behavior, confirmed still in place.

**Not done this phase:** a dedicated "find the JournalEntry for this ReconciliationRecord" admin lookup endpoint — `GET /admin/accounting/journal?referenceType=disbursement&referenceId=<id>` already serves this via the generic journal-list filter, so a dedicated endpoint wasn't necessary.

---

## 11. Accounting-Period Implementation

Periods are calendar months, auto-created `OPEN` the first time any posting targets that month (`getOrCreatePeriodForDate`) — never auto-created `CLOSED`. `ClosePeriod`/`ReopenPeriod` (`internal/accounting/periods.go`) are the only mutators; both require `PERIOD_MANAGE` (held only by `SUPER_ADMIN`/`ACCOUNTANT`), both write an audit event, and `ReopenPeriod` additionally requires a non-empty reason. `PostJournalEntry` refuses any posting into a `CLOSED` period (`ErrPeriodNotOpen`) — verified by `TestClosedPeriodRejectsPosting`.

---

## 12. Trial Balance Result

`GET /admin/accounting/reports/trial-balance?from=YYYY-MM-DD&to=YYYY-MM-DD` — opening/period/closing debit and credit columns per account, built from `sumLines` (a direct `journal_entry_lines` join/aggregate, no loan-table involvement). `report.Balanced` asserts `TotalClosingDebit == TotalClosingCredit`. **Verified by `TestTrialBalanceBalances`: passes — always balances by construction, since every posted entry is itself balanced.**

---

## 13. P&L Result

`GET /admin/accounting/reports/profit-and-loss?from=...&to=...` — sums Income-type and Expense-type account activity within the date range; `NetProfitPaise = TotalIncome - TotalExpense`. Includes a `ProvisionExpenseNote` field explicitly stating Phase 5 provisioning is not yet posted. Not tested with a dedicated assertion this phase beyond the underlying balanced-posting guarantee (see §15 gaps).

---

## 14. Balance Sheet Result

`GET /admin/accounting/reports/balance-sheet?to=YYYY-MM-DD` — Assets/Liabilities/Equity as of a point in time, `report.Balanced` asserts `Assets == Liabilities + Equity`. **Verified by `TestBalanceSheetBalances`: passes.**

---

## 15. Automated Test Results

`internal/accounting/accounting_test.go` — **10/10 passing** against the real Postgres dev DB:

1. `TestBalancedJournalPosts` — item 1 ✅
2. `TestUnbalancedJournalRejected` — item 2 ✅
3. `TestDuplicateJournalPrevented` — item 3 ✅
4. `TestUnauthorizedCannotPostJournal` — item 11 ✅
5. `TestAuditorCannotPostJournal` — item 12 ✅
6. `TestReversalCreatesNewEntry` — item 13 ✅
7. `TestClosedPeriodRejectsPosting` — item 10 ✅
8. `TestTrialBalanceBalances` — item 14 ✅
9. `TestBalanceSheetBalances` — item 15 ✅
10. `TestInterestAccrualIdempotent` — item 7 ✅

Full suite (`go test ./...`): all packages pass, including Phase 1/2/3 suites — **no regressions**.

**Not automated this phase** (§19 gap) — items 4, 5, 6, 8, 9, 16, 17, 18 from the spec's 18-scenario list: loan disbursement/EMI payment posting correctness (exercised manually — see §16 — not asserted in an automated test against a real `ConfirmDisbursement`/webhook call, since those require full Phase 2 fixture setup); failed payment not posting; GST CGST/SGST/IGST split correctness; invoice-number uniqueness (no invoice generator was wired up — see §9); bank reconciliation non-silent-alteration (relies on the pre-existing Phase 2 guarantee, not independently re-asserted here); accounting/LMS consistency; concurrent-posting dedup (the idempotency-key unique constraint is the mechanism, exercised only sequentially by `TestDuplicateJournalPrevented`, not under real concurrency).

---

## 16. Migration/Reconciliation Results

No historical loan/payment/disbursement data was rewritten or backfilled with invented journal entries — per instruction. Every accounting table starts empty; postings begin accruing only from this phase forward, for every *new* disbursement/EMI-payment event. **Existing (pre-Phase-4) disbursed loans and paid EMIs have no corresponding journal entries** — this is the expected, honest state, not a bug: inventing historical entries for money movements this codebase didn't originally book would misstate the ledger. A proper opening-balance import (one journal entry per historical loan's current outstanding principal, dated at the ledger's inception date, approved by the NBFC's accountant) is required before the Trial Balance/Balance Sheet reports reflect the NBFC's true historical position — see §19/§20.

---

## 17. Required Environment Variables

No new environment variables were introduced this phase. `COMPANY_STATE` (Phase 1/2, defaults to Maharashtra) continues to drive the GST intra/inter-state split reused here.

---

## 18. Required Accountant/CA Configuration

- Confirm the Chart of Accounts (§4) matches the NBFC's actual books — account names/codes are fully editable via the admin UI after review.
- Confirm the processing-fee accounting treatment: booked as a receivable (current implementation) vs. netted from the disbursed amount (§6 design note).
- Confirm the interest-accrual calculation basis (simple daily interest, actual/365, on outstanding principal) — `InterestAccrual.CalculationBasis` literally says "CA to confirm."
- Confirm GST invoice numbering series/format before any invoice generator is built (§9/§19).
- Confirm which (if any) TDS sections apply to which payment/vendor/customer scenarios before any `TDSTransaction` row is ever marked `APPLICABLE`.
- Provide the NBFC's actual registered GSTIN for `GSTTransaction.Gstin` (currently left blank everywhere — never fabricated).
- Approve the opening-balance import process for historical loans (§16).

---

## 19. Remaining Phase 4 Gaps

- No invoice-generation handler (`TaxInvoice` model + admin list view exist; nothing creates a row yet).
- No scheduled/cron job runs `AccrueInterestForLoan` — it must be triggered manually or wired to a scheduler.
- `PostRefund` exists but isn't yet called from the Phase 2 refund webhook path (`processRefund` in `internal/payments/webhook_handler.go`) — wiring it in is a small follow-up, deliberately left out this session to keep the refund-webhook change reviewable on its own.
- No admin panel UI screens (Chart of Accounts, Journal Entries, GL, Trial Balance, P&L, Balance Sheet, Periods, GST, TDS, Invoices, Funding) — only the backend APIs exist (item R).
- No dedicated automated tests for GST split correctness, invoice-number uniqueness, or true concurrent-posting races (§15).
- Opening-balance/historical-import process is designed-for but not implemented (§16).
- `SOFT_CLOSED` period status exists in the model/enum but has no behavior distinct from `OPEN` yet (documented in the model's own comment).

---

## 20. Items Requiring CA/Accountant/Compliance Validation Before Production

- **Fee recognition** — receivable-at-disbursement vs. net-off treatment (§6/§18).
- **Interest recognition** — the accrual formula and period-end cutoff conventions (§7/§18).
- **GST treatment** — invoice numbering/series, whether the current CGST/SGST/IGST split logic (inherited unchanged from Phase 1/2) is complete for every product/scenario this NBFC actually offers.
- **TDS applicability** — nothing is applied without explicit CA sign-off per scenario (§9).
- **Provisioning** — entirely deferred to Phase 5; the chart-of-accounts placeholders (2900/5900) are unused.
- **Income recognition** — whether interest income should be recognized on accrual (current implementation) or another basis for any loan category.
- **Write-offs and reversals** — `ReverseJournalEntry` exists as the correction mechanism but no write-off-specific workflow/approval chain was built.
- **Borrowings** — `FundingSource`/`BorrowingTransaction` are a generic framework; actual lender agreements, interest schedules, and covenant tracking need real business input.
- **Period close** — the monthly period-close/reopen mechanics are implemented, but the NBFC's actual close calendar, sign-off chain, and audit requirements beyond what's built here need accountant definition.

**This implementation does not make the NBFC's accounting "RBI compliant" or its "GST/TDS fully compliant."** It provides the double-entry ledger, posting engine, and reporting scaffolding those compliance outcomes would be built on top of — the treatments above require independent validation before production use.
