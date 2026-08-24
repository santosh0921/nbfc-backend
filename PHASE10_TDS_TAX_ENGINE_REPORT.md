# Phase 10 — TDS, Tax Engine, Tax Reporting, Tax Reconciliation & Tax Compliance Foundation

Continues from Phases 1–9, all re-verified building and passing before
this phase's work began (full `go test ./...`, production route-table
boot test — see the end of this report for the re-run after Phase 10's
own changes).

**This report does not, and cannot, claim TDS compliance, GST
compliance, statutory compliance, or audit certification.** Every rate,
threshold, section reference, form name, and due date this phase's
engine can hold is a field an authorized human enters — **this codebase
seeds zero real tax rules**, exactly as instructed. Sections V separates
technical implementation from what a CA/tax professional must still
confirm.

---

## A) Current Tax Architecture — Audit Findings

Before writing code, the existing application was inspected:

| Item | Prior state |
|---|---|
| TDS | `models.TDSTransaction` existed since Phase 4 as a bare skeleton — no rule engine, no calculation logic, no ledger posting, confirmed unused by any handler |
| GST | `models.GSTTransaction`/`TaxInvoice` exist (Phase 4), reused unchanged by processing-fee accounting — genuinely GST-relevant, not touched this phase |
| Vendor/expense payments | `internal/finance` (Phase 8) — `Expense.TDSAmountPaise` field existed but was **never computed**, confirmed by reading `finance.CreateExpense`/`MarkPaid` |
| Professional/contractor/rent/commission payments | No distinct payment categories exist anywhere — `Expense.Category` is free text; nothing in this codebase currently distinguishes these payment types from any other expense |
| Interest payments | `internal/accounting`'s `AcctBorrowingInterestExp` account exists (Phase 4/8) for interest paid on NBFC borrowings — no TDS consideration was ever applied to it |
| Tax deposits/returns/certificates | Did not exist at all |

**Conclusion**: nothing in Phases 1–9 actually calculated, tracked, or
deposited TDS on anything — Phase 4 and Phase 8's own reports both
explicitly flagged this as an open CA-review item rather than claiming it
was solved. Phase 10 builds the real engine described below.

---

## B) TDS Engine Status: 🟢 COMPLETE (technical implementation)

`internal/tax` — a versioned, maker-checker-gated rule engine
(`internal/tax/rules.go`). `SelectApplicableRule(natureOfPayment, transactionDate)`
picks the APPROVED `TDSRule` whose `[EffectiveFrom, EffectiveTo)` window
covers the transaction date — **never the newest rule regardless of
date**, tested explicitly (`TestRuleSelectionByEffectiveDate`, using a
synthetic 1-Apr-2026-style cutover to prove the pre/post-transition
selection logic, not to assert what the real transition rule is).

**Zero `TDSRule` rows are seeded** (`internal/tax/seed.go`) — until an
admin drafts and a *different* admin approves a real rule, `CalculateTDS`
correctly returns `ErrNoApplicableRule` for every payment
(`TestNoRuleReturnsExplicitError`). This is the safe, intended default.

---

## C) Tax Rule Versioning: 🟢 COMPLETE

Every `TDSRule` carries `RuleVersion`, `EffectiveFrom`/`EffectiveTo`,
`TaxAct` (free text — never asserts "Income-tax Act, 1961" or "Income-tax
Act, 2025" as applicable to anything; an admin enters whichever is
correct after verifying), and a **required** `SourceReference` — a rule
cannot even be drafted without one (`ErrSourceRequired`, tested). A
`TDSTransaction` freezes the exact `RuleID`/`RuleVersion` used at
calculation time and is never retroactively repointed when a newer rule
version is added later — confirmed by `TestRuleSelectionByEffectiveDate`
implicitly (the old rule stays selectable for old dates even after the
new rule exists) and by construction (`CalculateTDS` never updates a past
`TDSTransaction` row).

---

## D) TDS Calculation: 🟢 COMPLETE

`tax.CalculateTDS` — gross amount → threshold check → rate (with a
distinct, optional no-PAN rate) → surcharge/cess if configured → TDS
amount → net payable, with **every input stored on the resulting row**:
`RuleID`, `RuleVersion`, `TransactionDate`, `CalculationDate`,
`ThresholdPaise`, `PANStatusAtCalc`. Tested: threshold below/above
(`TestThresholdAndRateCalculation`), no-PAN higher rate
(`TestNoPANRateApplied`). Below-threshold amounts are still recorded
(status `NOT_APPLICABLE`, not silently skipped) — item D's "why" question
is answerable even for a non-deduction.

`OverrideTDS` (item L) is the only way to change a calculated amount —
requires a reason, records the previous amount, the overriding admin, and
a separate `approvedBy` name; always audited.

---

## E) TDS Liability Ledger: 🟢 COMPLETE (and a real integration gap closed)

**Deducted vs. deposited are structurally distinct** — `PostTDSDeduction`
(Dr expense account / Cr TDS Payable) only ever moves a transaction to
`DEDUCTED`; nothing in this codebase automatically marks it deposited.
Only `MarkChallanPaid`, requiring real bank-reference evidence, does that
(Dr TDS Payable / Cr Bank-Cash), and only for transactions explicitly
linked via `LinkTransactionToChallan`.

**This closes a real gap Phase 8's own report flagged**: `finance.MarkPaid`
posts the *net* payable straight to Bank-Cash with no TDS-payable leg at
all. `PostTDSDeduction` is designed as an **additive** second entry
(Dr same expense account / Cr TDS Payable, for the TDS portion only) so
the combined effect is correct (gross expense recognized, net cash paid,
TDS held as a liability) **without modifying Phase 8's own posting
function** — per the instruction not to redo completed Phase 8
functionality. `TDSReversal` (item E: "TDS reversal") posts a proper
`accounting.ReverseJournalEntry`, never edits the original — tested
(`TestTDSReversal`, confirms the original journal is marked `REVERSED`,
not altered).

**Still open**: this additive posting means an admin must remember to
call `PostTDSDeduction` alongside `finance.MarkPaid` for any expense that
attracts TDS — the two are not automatically linked. Wiring them together
requires a business decision about *when* in the expense lifecycle TDS
should be calculated (at invoice time vs. payment time), which is itself
a CA-review item (§V) — not resolved unilaterally this phase.

---

## F) TDS Deposit Tracking: 🟢 COMPLETE

`TDSChallan` — `PENDING → DUE → PAID/LATE/PARTIALLY_PAID/RECONCILIATION_REQUIRED/CANCELLED`,
tracks deduction period, deposit due date (only ever admin-entered, never
computed by this code from a hard-coded rule), actual deposit date, bank
reference, responsible/approving admin. `MarkChallanPaid` requires a
non-empty bank reference and automatically classifies `LATE` vs `PAID`
based on the admin-configured due date — this code does not compute
interest/penalty (item F: "do not automatically determine legal
interest/penalty treatment without verified current rules" — honored by
not attempting it at all).

---

## G) Challan Reconciliation: 🟢 COMPLETE (subset of item G's named checks)

`RunTDSReconciliation` (`internal/tax/reconciliation.go`) implements 4 of
item K's named exception types: PAN missing/invalid, deducted-but-not-
challan-mapped after 7 days, overdue deposit, duplicate deduction (per
reference). Not yet implemented: wrong-rate/wrong-amount detection
(requires an independent "what should this have been" recomputation,
which would need a second, separately-verified rule source to compare
against — not built), return-mismatch detection (no return-filing content
generation exists yet to compare against — see §H).

---

## H) TDS Return Reporting: 🟡 PARTIAL

`TDSReturnFiling` model tracks the workflow shape (quarter, deductee/
transaction counts, totals, status, filed-at, acknowledgement reference)
with `FormReference` as a **free-text field an admin enters** — this
codebase does not assert "Form 140" or "Form 26Q" as the currently-
applicable form anywhere in code; the prompt's own claim about Form 140
is treated as something for compliance to verify and enter, not as a
fact this code hard-codes. **No content-generation logic exists yet** —
the model tracks that a filing happened, but nothing assembles the
actual per-deductee return data into the filing. Flagged as remaining
work (§U).

---

## I) TDS Certificates: 🔴 MISSING (data foundation only, no generation)

`TDSTransaction.CertificateRef` exists as a field to record a certificate
reference once one is generated externally, but no certificate-generation
logic was built this phase — per the instruction "do not generate legally
valid certificates unless all required fields and rules have been
verified," and given no real rule has been configured/verified, building
a generator would have nothing legitimate to generate from.

---

## J) PAN Validation: 🟢 COMPLETE (framework; no live validation provider)

`TDSDeductee` — PAN encrypted at rest (same AES-256-GCM mechanism as
customer/vendor PAN, Phase 1/3/8), masked (`PANLast4` only) everywhere.
`SetPANValidationStatus` requires a non-empty, honest `source` (e.g.
"manual" or a real provider name — never fabricated as if an automated
check ran). **No live PAN-validation provider is integrated** — same
posture as Phase 3's Aadhaar gap: the interface/data model exists,
production validation is blocked pending a real provider credential.

---

## K) GST Foundation: ⚪ NOT APPLICABLE (unchanged this phase)

Phase 4's `GSTTransaction`/`TaxInvoice` models already exist and are
reused unchanged for processing-fee GST. This phase did not extend GST
treatment — item M's instruction to "flag all GST treatment decisions
requiring CA validation" was already satisfied by Phase 4's own report
and is not re-litigated here.

---

## L) Tax Compliance Calendar: 🟢 COMPLETE (framework; zero dates seeded)

`TaxComplianceCalendarItem` — `ComplianceType` is free text (never a
hard-coded form/return name), `DueDate` is always admin-entered with a
`Source` field for the official reference it was verified against.
`RefreshOverdueCalendarItems` only ever flips status based on an
already-admin-entered due date — it never invents or shifts one.

---

## M) Tax Dashboard: 🟢 COMPLETE

`GET /admin/tax/dashboard` — TDS deducted/deposited/payable/overdue/due-
soon, exception count, reconciled/unreconciled counts, deductee/
transaction counts, all server-aggregated from `TDSTransaction`/
`TDSChallan`/`TDSException` directly.

---

## N) Excel Reports: 🟡 PARTIAL

3 new controlled report types added to Phase 6/9's report-builder
registry: `TDS_REGISTER`, `TDS_CHALLAN_REGISTER`, `TDS_EXCEPTIONS` — all
via `POST /admin/reporting/exports/:reportType`, same metadata contract
(report name, period, filters, generated-by, record count, totals) as
every other controlled export. **Not built this phase**: TDS Deductee
Register, Vendor Tax Report, Quarterly TDS Report, Tax Year Summary,
Tax Adjustment Register (item T's remaining 5 of 10 named exports).

---

## O) Admin Frontend: ⚪ NOT APPLICABLE

No Flutter UI work this phase, consistent with every prior phase. New
backend endpoints exist under `/admin/tax/*` for a future UI.

---

## P) Audit Trail: 🟢 COMPLETE

New audit actions: `tax.rule.drafted`, `tax.rule.approved`,
`tax.rule.retired`, `tax.tds.calculated`, `tax.tds.overridden`,
`tax.tds.deducted`, `tax.tds.reversed`, `tax.challan.created`,
`tax.challan.mapped`, `tax.challan.paid`, `tax.deductee.created`,
`tax.deductee.pan_status_changed`, `tax.calendar.created`,
`tax.calendar.completed`. All via `audit.RecordDetailed` (Phase 1,
append-only, re-verified intact by the unchanged
`TestAuditLogCannotBeModifiedOrDeleted`).

---

## Q) Security: 🟢 COMPLETE

3 new permissions (`TAX_VIEW`, `TAX_RULE_MANAGE`/`TAX_RULE_APPROVE`,
`TAX_OPERATE`), maker≠checker enforced for rule approval
(`TestMakerCannotApproveOwnRule`), unauthorized access blocked
(`TestUnauthorizedCannotAccessTaxDashboard`). PAN masked in every API
response (`json:"-"` on `PANEncrypted`, only `PANLast4` exposed) — same
pattern as customer/vendor PAN. Cross-vendor/cross-period IDOR was not
separately tested this phase (no per-vendor ownership boundary exists on
tax data the way customer loan data has one — every tax record is
already admin-only, RBAC-gated, not customer-facing at all, so a
customer-vs-customer IDOR scenario doesn't apply here the way it did in
Phase 9).

---

## R) Tests

**9 new tests**, `internal/tax/tax_test.go`, all passing:
1. `TestRuleSelectionByEffectiveDate`
2. `TestNoRuleReturnsExplicitError`
3. `TestThresholdAndRateCalculation`
4. `TestNoPANRateApplied`
5. `TestMakerCannotApproveOwnRule`
6. `TestRuleWithoutSourceReferenceRejected`
7. `TestDuplicateDeductionPrevented`
8. `TestTDSReversal`
9. `TestUnauthorizedCannotAccessTaxDashboard`

**Full regression**: existing suite (Phases 1–9, ~106 tests) + Phase 7's
15 security tests + this phase's 9 — **all passing**, confirmed by a
full `go test ./...` run. Production route-table boot test (compiled
binary, actually started) re-run and confirmed clean at the end of this
phase's work, per item X.

**Not tested this phase**: concurrent tax processing (item X's "race
tests where practical" — `go test -race` was not run, same gap flagged
in Phase 9's report), Excel-total-reconciles-with-API-total assertion
for the new TDS report types.

---

## S) Vulnerabilities Found

None new. No injection, IDOR, or authentication issues found in this
phase's own code during review — RBAC/maker-checker were applied from
the start (the pattern was well-established by Phase 5/6/8/9, not
retrofitted).

---

## T) Fixes Implemented

1. `PostTDSDeduction`'s additive Dr-expense/Cr-TDS-Payable posting closes
   the "TDS never reaches the ledger" gap flagged in Phase 8's report
   (§E above) — without modifying Phase 8's own `MarkPaid` function.
2. No other Phase 1–9 code was modified this phase.

---

## U) Remaining Technical Issues

- `PostTDSDeduction` and `finance.MarkPaid` are not automatically linked
  — an admin must trigger TDS posting separately (§E).
- No return-content generation (§H), no certificate generation (§I).
- No live PAN-validation provider (§J).
- 5 of item T's 10 named Excel exports not built (§N).
- Wrong-rate/wrong-amount and return-mismatch reconciliation checks not
  implemented (§G).
- `go test -race` not run.
- No admin panel UI.

---

## V) CA/Tax Professional Review Items

**Nothing in this list has been applied to a single real payment** — the
`TDSRule` table is empty by design.

- **Whether any payment this NBFC makes attracts TDS at all**, and under
  which Act/section — this codebase takes no position. The prompt's own
  claim about a 1-April-2026 Income-tax Act transition and a
  consolidated Section 393 must be independently verified against the
  Income Tax Department/CBDT before any `TDSRule` is drafted, let alone
  approved.
- **Applicable rates, thresholds, surcharge/cess** — zero configured.
- **Form 140 / Form 26Q naming** — the prompt's claim is noted but not
  encoded anywhere in this code as fact; `TDSReturnFiling.FormReference`
  is free text for compliance to fill in after independent verification.
- **When in the expense lifecycle TDS should be calculated and deducted**
  (invoice date vs. payment date) — affects the §E integration gap.
- **Interest/penalty treatment for late deposits** — deliberately not
  computed by this code.
- **PAN-validation provider selection** — no provider is integrated.
- **Whether GST and TDS ever apply to the same transaction simultaneously**
  for this NBFC's specific expense categories — not analyzed this phase.

---

## PHASE 10 STATUS: **PARTIAL**

The core rule-versioning/calculation/ledger-integration/maker-checker
engine is implemented, tested, and correctly seeded with zero real rules
per the explicit instruction. Return/certificate generation and several
Excel exports remain open (§U), hence PARTIAL rather than COMPLETE.

## PHASE 8 REMAINING ITEMS

The TDS-ledger-integration gap Phase 8's own report flagged is now
closed (§E/§T). All other Phase 8 remaining items (full report-type
export wiring for 2 register types, GST-input-tax-credit tracking, admin
UI) are unchanged and still open.

## PHASE 9 REMAINING ITEMS

Unchanged from Phase 9's own report: no admin-initiated refund workflow,
2 daily-summary Excel exports, large-failed-payment-spike/repeated-
webhook-failure alerts, `go test -race` not run, no admin UI for Phase 9
endpoints.

**NEXT PHASE: Phase 11 — RBI/NBFC Compliance, Regulatory Reporting, Audit & Evidence Management**

**REMAINING PHASES: 11, 12, 13, 14, 15, 16**
