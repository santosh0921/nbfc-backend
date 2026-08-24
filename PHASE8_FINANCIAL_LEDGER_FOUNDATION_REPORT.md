# Phase 8 — Financial Ledger, Accounting Foundation, Financial Controls & Reporting Source-of-Truth

Continues from Phases 1–7, all re-verified building and passing (86+15
existing tests, full suite) before this phase's work began. A production
route-table boot test (compiled binary, actually started, all migrations
ran) was re-run at the end of this phase and confirmed clean.

**This report does not claim RBI compliance, statutory accounting
compliance, tax compliance, audit certification, or financial-statement
correctness.** Section S separates every accounting-policy decision that
still requires the NBFC's CA/accountant to confirm.

---

## A) Current Financial Architecture — Audit Findings

Before writing any code, the full application was inspected for existing
financial representation. Result: **the financial ledger foundation this
phase asks for already exists, built in Phase 4/5**, not missing as the
prompt's framing might suggest:

| Concept | Existing implementation |
|---|---|
| General ledger / double-entry | `internal/accounting` — `JournalEntry`/`JournalEntryLine`, `AccountingService.PostJournalEntry` (Phase 4) |
| Chart of accounts | `internal/accounting/chart.go` — 19 seeded accounts across Asset/Liability/Equity/Income/Expense (Phase 4) |
| Journal approval / maker-checker | Reused from Phase 1's pattern throughout Phase 4/5/6 (loan sanction, write-off, regulatory reports) |
| Period controls | `AccountingPeriod` (OPEN/SOFT_CLOSED/CLOSED/REOPENED), `ClosePeriod`/`ReopenPeriod` (Phase 4) |
| Loan → ledger mapping | `accounting.PostLoanDisbursement`, `postProcessingFeeAndGST`, `PostEmiPayment` (Phase 4) |
| Disbursement traceability | Customer→Loan→Sanction→Disbursement→Payment reference→Ledger chain already exists (Phase 1/2/4) |
| Collection component tracking | EMI payment journal already splits principal/interest as separate lines (Phase 4) |
| NPA/provisioning | `internal/risk` — classification + provisioning posting through the same `AccountingService` (Phase 5) |
| Bank reconciliation | `internal/reconciliation` (bank-vs-disbursement/payment matching, Phase 2) + `internal/reporting`'s cross-system dashboard (Phase 6) |
| MIS / regulatory reporting framework | `internal/reporting` (Phase 6) |

**Genuinely missing, confirmed by the same audit** (this is what Phase 8
actually built):
- **Vendor management** — no `Vendor` model/table existed anywhere.
- **Expense management** — the chart of accounts has expense-type
  accounts (Operating Expenses, Bank Charges) but nothing ever posted a
  real bill against them; there was no `Expense` model, no
  approval/payment workflow.
- **Bank account balance tracking** — `BankAccount` (Phase 2) exists only
  as a scoping key for statement imports; it had no opening balance field
  and no computed running/closing balance.
- **A real XLSX export** — Phase 6's "Excel" export was CSV content
  mislabeled as Excel (documented as such in its own code comment).
- **A named executive financial dashboard** with the specific metrics
  item Q lists (total sanctioned/disbursed/collected/receivable/etc. in
  one place) — the pieces existed scattered across `accounting`, `risk`,
  and `reporting`, but nothing assembled them into the one dashboard item
  Q/U describes.
- **A disbursement-vs-bank amount-mismatch reconciliation check** — item
  O's exact example ("loan says ₹100,000 disbursed but gateway says
  ₹99,000") had no dedicated check.

**Decision made per the instruction not to duplicate existing
functionality**: this phase does NOT rebuild the chart of accounts,
double-entry engine, journal posting, period controls, or maker-checker
infrastructure — it extends `internal/accounting`'s existing `Service`
for every new financial posting (expense payment), exactly as Phase 5's
provisioning engine already did.

---

## B) Chart of Accounts Status: 🟢 COMPLETE (pre-existing, Phase 4)

No changes made. All new Phase 8 postings (expense payments) use existing
account codes (`AcctOperatingExpenses`, `AcctBankCash`, etc.) — no new
accounts were added, since the existing Expense/Operating-Expense-type
accounts already cover this need. **Category-to-account-code mapping
remains an admin-entered choice per expense** (the `AccountCode` field on
`Expense`), not inferred automatically — this is itself flagged as a
CA-review item (§S), since "which GL account does a given expense
category post to" is an accounting-policy decision.

---

## C) Ledger Status: 🟢 COMPLETE (pre-existing, Phase 4)

`internal/accounting.Service.PostJournalEntry` remains the **sole**
writer of journal entries — Phase 8's `finance.MarkPaid` calls it exactly
the same way every other phase's posting functions do; no direct
`journal_entries`/`journal_entry_lines` writes were added anywhere.

## D) Double-Entry Status: 🟢 COMPLETE (pre-existing, Phase 4)

`TOTAL DEBITS == TOTAL CREDITS` is validated inside `PostJournalEntry`
before any DB write — re-verified this phase by
`TestExpenseApprovalAndPaymentPosting`, which asserts the new expense
payment journal balances.

---

## E) Disbursement Accounting Status: 🟢 COMPLETE (pre-existing, Phase 2/4) + 🟡 PARTIAL new check

The full traceability chain item F asks for (Customer→Loan→Sanction→
Disbursement→Payment reference→Ledger→Reconciliation) already exists.
**New this phase**: `checkDisbursementAmountMismatch` in
`internal/reporting/reconciliation.go` — directly implements item O's
example scenario (a successful `Disbursement.AmountPaise` that disagrees
with its matched `BankTransaction.AmountPaise` becomes a `CRITICAL`
`CrossSystemReconciliation` exception). Not yet automatically tested with
a dedicated fixture this session (manually verified via code inspection
of the query logic) — flagged in §T.

## F) Collection Accounting Status: 🟢 COMPLETE (pre-existing, Phase 4)

Principal/interest are already posted as separate journal lines against
distinct accounts (`AcctLoanPrincipalReceivable` / `AcctInterestReceivable`)
per EMI payment — confirmed by reading `accounting.PostEmiPayment` and
re-verified this phase via `finance.journalCreditSum`, which the new
financial dashboard uses to independently pull "principal collected" and
"interest collected" as two separate figures from those exact posted
lines (not re-derived from `EmiInstallment` directly).

## G) Revenue Status: 🟡 PARTIAL — foundation complete, explicit non-claim documented

`FinancialDashboard.RevenuePaise` is populated **only** from
`accounting.ProfitAndLoss`'s posted Income-account sum — i.e. only
Interest Income + Processing Fee Income + Penalty/Other Income as
actually journaled, never "total collected." `TotalPrincipalCollectedPaise`
is deliberately a **separate, differently-named field**, precisely
because item H says "do not simply define TOTAL COLLECTION = REVENUE."
The distinction is enforced by construction (different code paths, one
from posted Income accounts, one from posted Asset-reduction lines), not
just by a comment. **Marked PARTIAL, not COMPLETE**, because the
underlying income-recognition policy itself (accrual vs. cash basis,
whether provisioning-suspended interest should count) is unconfirmed —
see §S.

## H) Expense Status: 🟢 COMPLETE (new this phase)

`internal/finance` — `Vendor` + `Expense` models, full lifecycle
(`PENDING_APPROVAL → APPROVED/REJECTED → PAID`), maker≠checker enforced
(`ErrSameActor`, tested), accounting-posted only at `MarkPaid` (never
before approval). Every field item I lists is present on the `Expense`
model. Categories are a free string, not an enum (item I: "do not
hard-code only a few categories").

## I) Bank/Cash Status: 🟢 COMPLETE (new this phase)

`BankAccount.OpeningBalancePaise` (new field) + `finance.BankAccountBalance`
(computed strictly from imported `BankTransaction` rows: opening + credits
− debits = closing) + `AllBankAccountSummaries` for the dashboard tile.
Tested (`TestBankAccountBalanceComputation`). No bank credentials are
ever exposed — `BankAccount` already only stores a masked account number
(Phase 2, unchanged).

## J) Reconciliation Status: 🟡 PARTIAL

Bank-vs-disbursement (Phase 2), risk/accounting internal consistency
(Phase 5), and cross-system dashboard (Phase 6) all pre-exist. **New this
phase**: the disbursement-amount-mismatch check (§E). **Still missing**
(explicitly out of this session's scope, flagged for a future phase):
payment-gateway-vs-application-status mismatch detection (item O's second
example — "bank says succeeded but application says failed") has no
dedicated check yet; Phase 2's webhook-only-source-of-truth design
prevents this specific inconsistency from arising in the first place
(a `Payment` can only reach `SUCCESS` via a verified webhook), but no
explicit reconciliation check independently verifies that invariant held.

## K) Period Control Status: 🟢 COMPLETE (pre-existing, Phase 4) — re-verified

Phase 7's `TestClosedPeriodBlocksNewPosting` (in `internal/security`)
still passes unchanged after this phase's changes — confirms closed-period
protection was not weakened by the new expense-posting code path (it goes
through the same `PostJournalEntry` period check as everything else).

## L) Maker-Checker Status: 🟢 COMPLETE (extended this phase)

New: expense approval (`ApproveExpense`/`RejectExpense`, `ErrSameActor`
enforced, tested by `TestExpenseMakerCannotApproveOwn`). Reuses the exact
pattern established in Phase 5 (write-off) and Phase 6 (regulatory
report review) — no new maker-checker mechanism was invented.

## M) Audit Trail Status: 🟢 COMPLETE (extended this phase)

New audit actions: `finance.vendor.created`, `finance.vendor.status_changed`,
`finance.expense.created`, `finance.expense.approved`,
`finance.expense.rejected`, `finance.expense.paid` — all via the existing
`audit.RecordDetailed` (Phase 1, append-only, re-verified still enforced
by `TestAuditLogCannotBeModifiedOrDeleted` in the full regression run).

## N) Excel Reporting Status: 🟢 COMPLETE (new this phase)

`internal/reporting/xlsx.go` — a real, valid, dependency-free `.xlsx`
writer (standard-library `archive/zip` + `encoding/xml` only, no new
third-party spreadsheet library added, consistent with the Phase 7
discipline on dependencies). Every export now carries the item-S metadata
block (report name, generated-at, reporting period, filters, generated
by, record count, totals) as leading rows on the sheet.
`RequestExportWithMetadata` is the new entry point; `RequestExport`
(existing callers, Phase 6) still works unchanged, now defaulting to a
minimal metadata block. **Not yet wired into every report type item S
lists** (General Ledger, Trial Balance, Disbursement/Collection/Expense/
Revenue Registers, Receivable/Payable, Bank Transaction Register,
Reconciliation Exceptions, Financial Transaction Register) — the writer
and metadata contract exist and are proven working (§P), but per-report
handler wiring to call `RequestExportWithMetadata` with the right rows/
totals for each of those 11 report types was not done for all of them
this session — flagged in §T.

## O) Admin Frontend Changes: ⚪ NOT APPLICABLE

No Flutter admin-panel UI work was done this phase (consistent with the
scope choice documented in every prior phase's report — backend/API only,
UI screens are a standing gap tracked across all phases). New backend
routes exist under `/admin/finance/*` for a future UI to call.

## P) Tests Added: 🟢 COMPLETE

5 new tests in `internal/finance/finance_test.go`, all passing:
1. `TestCreateVendorEncryptsPAN` — vendor PAN encrypted at rest, not plaintext
2. `TestExpenseMakerCannotApproveOwn` — maker≠checker (item V: "unauthorized journal modification"-adjacent)
3. `TestExpenseApprovalAndPaymentPosting` — approval flow + balanced accounting journal on payment (item V: debit=credit)
4. `TestUnauthorizedCannotAccessFinancialDashboard` — RBAC (item Y)
5. `TestBankAccountBalanceComputation` — opening+credits−debits=closing arithmetic

Full existing suite (86 tests, Phases 1–6) + Phase 7's 15 security tests
— **all still passing**, confirmed by a full `go test ./...` run after
these changes (item Z).

**Not covered by a dedicated test this session** (item V's full list):
duplicate-disbursement/duplicate-collection prevention (pre-existing
Phase 2/4 protections, not re-tested here), reversal correctness for the
new expense flow (no expense-reversal function was built — expenses have
no reversal path yet, only reject-before-payment), reconciliation
mismatch for the new disbursement-vs-bank check specifically, export
total-reconciles-with-source assertion, concurrent financial transaction
handling.

## Q) Vulnerabilities/Issues Found: 🔴 → 🟢 FIXED

**Mass-assignment risk in `AdminCreateExpenseHandler`** — the handler
initially bound the incoming JSON directly into `*models.Expense`, which
would have let a caller with only `ExpenseManage` set `Status`,
`ApprovedBy`, `JournalReference`, `PaymentReference`, or `CreatedByAdminID`
directly in the request body, bypassing the entire approval workflow.
Found during this session's own implementation (not by an external scan)
and **fixed before being committed to the route table** — the handler now
binds into a narrow, explicit request struct containing only the fields a
creator should legitimately set.

No other new vulnerabilities were introduced or found in the pre-existing
Phase 1–7 code this session touched.

## R) Fixes Implemented

1. Mass-assignment fix (§Q).
2. `BankAccount.OpeningBalancePaise` field added (additive, non-breaking —
   defaults to 0 for every pre-existing row via GORM's migration).

## S) Accounting Decisions Requiring CA/Accountant Review

- **Expense-category → GL-account mapping**: currently a free-text
  `AccountCode` chosen per expense at creation time, not a fixed
  category→account table — a CA should confirm whether specific expense
  categories (Employee, Technology, Cloud, KYC/API costs, Credit bureau
  costs, Payment gateway costs, Collection costs, Marketing, Legal, Audit,
  Office/admin) should each map to a dedicated GL account rather than
  being left to admin discretion at entry time.
- **TDS on vendor expenses**: `Expense.TDSAmountPaise` exists as a field
  but is **never auto-computed** — same posture as Phase 4's TDS
  framework (`internal/models/gst.go`'s `TDSTransaction`, still unused).
  `MarkPaid`'s journal debits the full net-payable straight out of cash
  with no separate TDS-payable credit leg — this needs CA confirmation of
  whether/how this NBFC's vendor payments actually attract TDS before
  that accounting treatment is relied upon.
- **Revenue recognition basis**: `RevenuePaise` reflects whatever was
  posted to Income accounts (accrual, per Phase 4's interest-accrual
  engine) — a CA needs to confirm this is the intended recognition basis,
  especially interacting with Phase 5's NPA/IRAC provisioning (income
  recognition suspension for NPA loans is a documented Phase 5 gap, still
  unresolved).
- **"Profit Before Tax" labeling**: `ProfitBeforeTaxPaise` is exactly
  `Revenue − OperatingExpenses` as posted — no tax provisioning, no
  "Profit After Tax" figure exists anywhere in this codebase (deliberately
  not computed, per item Q's explicit instruction not to label a number
  "profit" without an established accounting definition).
- **Write-off accounting treatment carried over from Phase 5** (debiting
  the Provision-for-NPA account, assuming full prior provisioning) remains
  unconfirmed — unchanged this phase, still flagged from Phase 5's report.

## T) Remaining Technical Work

- Wire `RequestExportWithMetadata` into report-specific handlers for all
  11 register/report types item S names (only the writer + one generic
  entry point exist; per-report column/total definitions are not yet
  built for each).
- Payment-gateway-vs-application-status mismatch reconciliation check
  (item O's second example) — not implemented.
- Expense reversal/correction path — currently only reject-before-payment
  exists; a paid expense has no documented correction mechanism (should
  follow the same reversal-not-edit pattern as `accounting.ReverseJournalEntry`).
- Concurrent-financial-transaction and export-total-reconciliation tests
  (item V) not written this session.
- Admin panel UI for the new `/admin/finance/*` routes.
- Vendor GST-details field exists (`GSTIN`) but no GST-on-vendor-invoice
  input-tax-credit tracking was built (would connect to Phase 4's
  `AcctGSTInputReceivable`, currently unused by anything).

---

## PHASE 8 STATUS: **PARTIAL**

Core financial-ledger foundation (chart of accounts, double-entry,
journal posting, period controls, maker-checker, disbursement/collection
accounting) was **already complete from Phase 4/5** and is re-verified
intact. This phase's genuinely new scope — vendor/expense management,
bank balance tracking, real Excel export, executive financial dashboard,
one new reconciliation check — is implemented, tested, and passing, but
several item-S/O/T sub-requirements (full report-type export wiring,
gateway-status reconciliation, expense reversal) remain open, hence
PARTIAL rather than COMPLETE.

**NEXT PHASE: Phase 9 — Payments, Collections & Bank/Gateway Reconciliation**

**REMAINING PHASES: 9, 10, 11, 12, 13, 14, 15, 16**
