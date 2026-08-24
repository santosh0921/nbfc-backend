# Phase 12 — Complete NBFC MIS, Financial Reporting, Revenue, Expenses, Profitability, Management Dashboard, Excel/PDF Reporting and Growth Tracking

Continues from Phases 1–11, all re-verified building and passing before
this phase's work began (`go build ./...`, `go vet ./...`, `go test ./...`
all green across every package, including the new `internal/mis`
package — 2.883s, `ok`).

**Explicit disclaimers (restated verbatim per the phase spec — these are
never violated anywhere in this codebase):**

- Revenue ≠ Collections. Collections include principal recovery, which is
  a balance-sheet movement, not income (§I, §F).
- Profit ≠ Cash balance. Profit is an accrual accounting concept; cash
  balance is a point-in-time bank/cash position (§U).
- Profit ≠ Collections − Expenses. Profit (PBT) = Revenue − Operating
  Expenses − Funding Cost, where Revenue is interest/fee income actually
  recognized, not gross cash collected (§L).
- TDS ≠ Revenue. TDS is a statutory withholding on income paid to the
  NBFC by others, tracked in Phase 10, never treated as the NBFC's own
  revenue line (§W).
- Loan disbursement ≠ Expense. Disbursement is an asset (loan
  receivable) being created, not a cost (§E, §L).
- Principal collection ≠ Income. Principal repayment reduces the loan
  receivable asset; only the interest/fee component of a collection is
  income (§F, §I).

Every calculation in `internal/mis` is a **composition layer**, not a
re-derivation — it calls the exact functions Phases 5, 8, 10, and 11
already built and tested, and embeds their return structs directly. This
makes §B ("single source of truth") a structural guarantee: the
executive dashboard's financial numbers cannot silently drift from the
standalone finance dashboard's numbers because they are, literally, the
same function call. `TestExecutiveDashboardMatchesFinanceDashboard`
exists to catch a future regression of that guarantee, not to compensate
for its absence.

---

## A) Reporting Architecture Audit: 🟢 COMPLETE

Reviewed every existing dashboard/report generator before writing new
code: `finance.GenerateFinancialDashboard` (Phase 8), `risk.PortfolioDashboard`
(Phase 5), `tax.GenerateTaxDashboard` (Phase 10), `compliance.GenerateDashboard`
(Phase 11), `reporting.AdminMISDashboardHandler` / `AdminComplianceHealthHandler`
(Phase 6). No disconnected/incorrect calculation was found requiring a
fix — every one of these already computes from real ledger/loan/payment
source data (Phase 8/9's own reports already established this
discipline). Phase 12 therefore composes rather than replaces.

## B) Single Reporting Source of Truth: 🟢 COMPLETE

`ExecutiveDashboard` (`internal/mis/executive.go`) embeds
`finance.FinancialDashboard`, `risk.PortfolioDashboardReport`,
`tax.TaxDashboard`, and `compliance.ComplianceTrackingDashboard`
verbatim from their own generator functions. No number is computed
twice. Hierarchy (Loan → Disbursement → Payments → Ledger →
Reconciliation → Tax → MIS) matches existing module dependencies; MIS
depends on all of them, none of them depend on MIS.

## C) Executive Dashboard: 🟢 COMPLETE (backend)

`GenerateExecutiveDashboard(from, to)` returns business counts (real
customers/loans/applications/approved/rejected, plus a derived
"closed loan" count — see Errors/Fixes note below since no
`LoanStatusClosed` exists in this codebase's state machine) alongside
the embedded Financial/Portfolio/Tax/Compliance sections. `/admin/mis/executive`.

## D) Date Filters: 🟢 COMPLETE

`internal/mis/fy.go` — `DateRangePreset` supports
today/yesterday/this_week/this_month/previous_month/this_quarter/
previous_quarter/current_fy/previous_fy, defaulting to `this_month`
rather than unbounded "all time" on an unrecognized preset. `custom`
is handled by the handler accepting explicit `from`/`to` query params.
Financial Year is implemented as the India-standard April–March window
(`FinancialYearRange`) — a calendar convention, not a tax-policy call
requiring CA sign-off, so it was safe to hard-code directly (unlike
Phase 10's TDS rates, which are seeded empty for compliance to enter).

## E) Disbursement Report: 🟢 COMPLETE (data), 🟡 PARTIAL (export)

Real disbursement data already surfaced via `finance.GenerateFinancialDashboard`
and the new `LOAN_REGISTER` Excel export (`internal/reporting/mis_registers.go`).
A dedicated aggregate-only disbursement report endpoint was not added
separately this phase — the data is available via `/admin/mis/executive`
and `/admin/mis/growth`.

## F) Collection MIS: 🟢 COMPLETE

Growth report's "Collection" metric sums actual bank/cash debits
against `EMI_PAYMENT` journal entries (`journalDebitSum`), never
conflated with revenue — kept as a structurally separate metric from
"Revenue," which is computed via `accounting.ProfitAndLoss` instead.
Principal recovery is never labeled revenue anywhere in this module.

## G) Portfolio MIS: 🟢 COMPLETE (reused from Phase 5, unchanged)

`risk.PortfolioDashboard` embedded directly in the executive dashboard.
No new regulatory classification rules were invented — Phase 5's own
report already carries the "REQUIRES COMPLIANCE/ACCOUNTING VALIDATION"
posture for asset classification (unseeded `ClassificationRule` rows,
confirmed still true by this session's boot log:
`⚠️ No ClassificationRule/ProvisioningRule rows are seeded`).

## H) DPD/Collection Quality Buckets: 🟢 COMPLETE (reused from Phase 5)

Unchanged this phase — Phase 5's delinquency buckets remain explicitly
labeled "example DPD ranges — NOT validated against any regulatory
framework" (confirmed by this session's boot log). Operational DPD and
regulatory asset classification remain two distinct, unmerged concepts,
as established in Phase 5.

## I) Revenue Engine: 🟢 COMPLETE

`revenueSum(from, to)` and `expenseSum(from, to)` both call
`accounting.ProfitAndLoss(from, to)` — the same P&L engine used
throughout Phase 8, never `Total Collections`. `Revenue = Collections`
is never asserted anywhere in this module.

## J) Expense MIS: 🟢 COMPLETE (reused from Phase 8/9)

`expenseSum` reuses the existing P&L expense total; category-level
detail already exists via Phase 8/9's `EXPENSE_REGISTER` export — not
rebuilt.

## K) Funding MIS: ⚪ DEPENDENCY / NOT CURRENTLY AVAILABLE

No funding/borrowing-cost data source exists anywhere in Phases 1–11
(confirmed by inspection — no lender/facility/interest-cost model). Per
the explicit instruction not to fabricate funding data, this is marked
unavailable rather than estimated. `Funding Cost` is therefore **not**
subtracted in any PBT calculation this phase produces — profitability
figures in this module represent Revenue − Operating Expenses only, and
are labeled as such, not as a complete PBT until funding cost is wired
in.

## L) Profitability: 🟡 PARTIAL

`GrowthReport`'s "Profit Before Tax" metric = Revenue − Expenses (both
from `accounting.ProfitAndLoss`). **This is explicitly NOT a complete
PBT** per §K (funding cost unavailable) — the field name is kept as
"Profit Before Tax" for continuity with the growth-metric list the spec
names, but this report itself documents that funding cost is missing;
treat it as Operating Profit until Funding MIS exists. PAT (post-tax) is
**not** computed this phase — Phase 10's tax module computes TDS
withholding, not the NBFC's own corporate income tax liability, so no
source of truth for "Tax" (as a PAT input) exists. Marking accordingly
rather than fabricating a PAT figure.

## M) Unit Economics: 🟢 COMPLETE

`internal/mis/unit_economics.go` — Revenue/Customer, Revenue/Active
Loan, Average Loan Size, Average Collection, Average Expense/Active Loan
all computed from real data. CAC, Collection-Cost-per-Rupee,
Tech-Cost-per-Customer, KYC-Cost-per-Customer, and
Credit-Bureau-Cost-per-Application are explicitly marked
`NOT AVAILABLE — REQUIRED DATA SOURCE MISSING` (tested:
`TestUnitEconomicsMarksUnavailableExplicitly`) since no marketing-spend,
channel-attribution, or per-service-cost data exists anywhere in this
codebase.

## N) Growth Analytics: 🟢 COMPLETE

`internal/mis/growth.go` — MoM/QoQ/YoY handled uniformly by shifting the
comparison window back by the same duration as the current window
(works for any custom range, not just calendar months).
`SafePercentChange` returns `nil` (not `+Inf`/`NaN`/a silently-clamped
number) when the prior period is zero — tested
(`TestSafePercentChangeHandlesZeroDenominator`).

## O) Loan Funnel: 🟢 COMPLETE

`internal/mis/funnel.go` — Applications → KYC Completed → Credit
Evaluated → Approved → Sanctioned → Disbursed, using only states that
already exist in this codebase's loan/KYC state machines (no invented
stage). Conversion rates use `ratioPercent`, nil-safe on a zero
denominator — tested (`TestFunnelHandlesEmptyPeriod`). Note: "Sanctioned"
is currently the same count as "Approved" — this codebase's state
machine does not distinguish an approval decision from the sanction
event as two separate transitions, documented in-code rather than
silently presented as if they were independently meaningful.

## P) Collection Performance: 🟢 COMPLETE (via §F/§N)

Operational collection totals (growth report) are kept structurally
separate from revenue/accounting metrics — no shared field, no implicit
conversion between the two.

## Q) Customer Metrics: 🟢 COMPLETE

Business counts are aggregate totals only. The one PII-adjacent export
this phase added (`CUSTOMER_REGISTER`) masks mobile numbers
(`maskMobile` — last 4 digits only) and includes no PAN/Aadhaar/address.

## R) Product Profitability: ⚪ NOT RELIABLY CALCULABLE

No product-level cost-allocation exists anywhere in this codebase (no
loan product carries an attributed share of operating/funding cost).
Per the spec's own instruction not to allocate shared costs arbitrarily,
this was correctly not built rather than approximated.

## S) Branch/Location Reporting: ⚪ NOT APPLICABLE

No branches exist in this codebase's data model. Per the spec's own
instruction, no fake branches were created.

## T) Financial Year Report: 🟡 PARTIAL

`FinancialYearRange` gives the correct Apr–Mar boundary and label
(tested: `TestFinancialYearRange`, confirms FY2026-27 = 2026-04-01 →
2027-03-31). The full opening-portfolio + disbursements − collections −
write-offs = closing-portfolio structured summary was **not** assembled
as a single report this phase — the boundary utility exists, the
roll-forward composition does not yet.

## U) Cash Flow Foundation: 🔴 NOT BUILT

No cash-inflow/outflow or operating/financing/investing-activity
reporting was added this phase. Flagged as the largest data-model gap
remaining after Funding MIS (§K).

## V) Reconciliation Dashboard: 🟢 COMPLETE (reused from Phase 9, unchanged)

Not modified this phase — Phase 9's reconciliation module remains the
source of truth; MIS does not duplicate it.

## W) Tax Dashboard: 🟢 COMPLETE

`tax.GenerateTaxDashboard` embedded directly in the executive dashboard
— no TDS figure is recalculated in `internal/mis`; Phase 10 remains the
sole source of truth.

## X) Compliance Dashboard: 🟢 COMPLETE

`compliance.GenerateDashboard()` embedded directly — no second
compliance database or scoring logic was created.

## Y) Management Report Pack: 🔴 NOT BUILT

The 17-section configurable report pack (Executive Summary through Risk
Summary) was not assembled as a single deliverable this phase. All the
underlying data sections exist individually (via the executive
dashboard's embedded structs, growth report, and funnel report) but are
not yet composed into the named pack format.

## Z) Excel Reporting: 🔴 LARGELY NOT BUILT

Only 2 of the 20 named workbooks were added this phase: `LOAN_REGISTER`
and `CUSTOMER_REGISTER` (`internal/reporting/mis_registers.go`), wired
into the existing `BuildRegisterReport` dispatch alongside Phase 8/9/10's
registers (`EXPENSE_REGISTER` and `TDS_EXCEPTIONS`/TDS registers already
existed from those phases and satisfy part of item Z's naming without
new work). The remaining ~16 workbooks (Sanction Register, Outstanding
Register, DPD Report, Revenue Register, Vendor Register, Funding
Register, Profitability Report, Product Performance, Customer Growth,
Loan Growth, Reconciliation, Compliance, Audit Findings, Executive MIS,
and others) are not wired.

## AA) PDF Management Reports: 🔴 NOT BUILT

No PDF generation was attempted this phase for any of the 7 named
report types. Phase 6's minimal `generatePDFPlaceholder` in
`internal/reporting` remains unused for this purpose.

## AB) Report Snapshots: 🔴 NOT BUILT

No versioned/frozen snapshot mechanism for approved MIS reports exists.
Phase 6's `RegulatoryReportRun` and Phase 11's `RegulatoryReturnTracking`
have adjacent freeze-after-approval patterns that a future phase could
reuse, but nothing new was built for general MIS snapshots this phase.

## AC) Frontend / Executive Admin Dashboard: ⚪ NOT APPLICABLE (scope)

No Flutter UI work this phase, consistent with every prior phase's
established backend-only scope (Phases 7–11 all noted the same gap).

## AD) Drill-Down: ⚪ NOT APPLICABLE (no frontend to drill from — see AC)

## AE) Report Filtering: 🟡 PARTIAL

Only date-range filtering (preset or custom) was added to the new MIS
endpoints. Product/branch/customer-segment/loan-status/payment-status/
collection-channel/reconciliation-status filters were not added
specifically to the new endpoints, though `risk.DashboardFilters`
(Phase 5, embedded via §G) already supports several of these for the
portfolio section.

## AF) Performance: 🟢 COMPLETE (by design)

All new endpoints return aggregates/summaries computed server-side via
SQL `SUM`/`COUNT`, never raw transaction rows loaded client-side. No
synchronous million-row export path was added this phase (the only new
exports, §Z, are register-style with existing pagination/limit patterns
from Phase 8/9).

## AG) Data Consistency Testing: 🟡 PARTIAL

`TestExecutiveDashboardMatchesFinanceDashboard` verifies the finance
section byte-for-byte matches the standalone finance dashboard for an
identical period — the central §B guarantee. Collection=payment-
source=ledger, expenses=expense-ledger, tax=Phase-10-source, and
compliance=Phase-11-source consistency are structurally guaranteed by
the same embedding design (§B) but were not each given an individual
test this phase.

## AH) Report Security: 🟡 PARTIAL

`TestUnauthorizedCannotAccessExecutiveDashboard` confirms a
COLLECTION_OFFICER role gets 403 on `/admin/mis/executive` — Phase 6's
`rbac.MISView` permission is reused, no new permission was invented.
IDOR, cross-period access abuse, and unauthorized-Excel-export tests
specific to the two new Phase 12 registers were not written this phase.

## AI) Auditability: 🟡 PARTIAL

Report *generation* metadata (who/when/filters/period) is available
implicitly via standard request logging, but no `audit.RecordDetailed`
call was wired into any MIS dashboard/report GET handler this phase
(unlike Phase 11's evidence/policy/finding actions, which are all
explicitly audited) — read-only dashboard views were not added to the
audit trail.

## AJ) Final Validation: 🟢 COMPLETE

`go build ./...`, `go vet ./...` (no output — clean), `go test ./...`
— every package with tests, including the new `internal/mis` package,
reports `ok`. Production binary boot-tested
(`go build -o nbfc-api12.exe ./cmd/api` + timed run) — server starts
cleanly, connects to the database, migrates all tables, seeds Chart of
Accounts/delinquency buckets/retention placeholders, and reaches
`🚀 Server starting on port 8080` with the 5 new `/admin/mis/*` routes
registered, no errors.

## AK) Classification Summary

| Section | Status |
|---|---|
| A. Reporting Architecture Audit | 🟢 |
| B. Single Source of Truth | 🟢 |
| C. Executive Dashboard | 🟢 (backend only) |
| D. Date Filters | 🟢 |
| E. Disbursement Report | 🟢 / 🟡 (export) |
| F. Collection MIS | 🟢 |
| G. Portfolio MIS | 🟢 |
| H. DPD Buckets | 🟢 |
| I. Revenue Engine | 🟢 |
| J. Expense MIS | 🟢 |
| K. Funding MIS | ⚪ dependency, not available |
| L. Profitability | 🟡 |
| M. Unit Economics | 🟢 |
| N. Growth Analytics | 🟢 |
| O. Loan Funnel | 🟢 |
| P. Collection Performance | 🟢 |
| Q. Customer Metrics | 🟢 |
| R. Product Profitability | ⚪ not reliably calculable |
| S. Branch Reporting | ⚪ not applicable |
| T. Financial Year Report | 🟡 |
| U. Cash Flow Foundation | 🔴 |
| V. Reconciliation Dashboard | 🟢 (reused) |
| W. Tax Dashboard | 🟢 (reused) |
| X. Compliance Dashboard | 🟢 (reused) |
| Y. Management Report Pack | 🔴 |
| Z. Excel Reporting | 🔴 (2/20 wired) |
| AA. PDF Reports | 🔴 |
| AB. Report Snapshots | 🔴 |
| AC. Frontend | ⚪ not applicable (scope) |

---

## Remaining Items (this phase)

- Cash Flow Foundation (§U) — highest-priority gap.
- Management Report Pack composition (§Y).
- Remaining ~16 Excel workbooks (§Z).
- PDF management reports, all 7 types (§AA).
- Report snapshot/versioning mechanism (§AB).
- Financial Year roll-forward report composition (§T) — boundary utility
  exists, full opening/closing portfolio summary doesn't.
- Additional filters beyond date range on the new endpoints (§AE).
- Per-domain data-consistency tests beyond the one finance-dashboard
  test (§AG).
- IDOR/cross-period/export-abuse security tests for the two new
  registers (§AH).
- Audit-log wiring for MIS dashboard/report generation (§AI).
- Admin panel UI / drill-down (§AC/§AD).
- Funding MIS remains a genuine external dependency — no data source
  exists; profitability figures in this module are Operating Profit
  (Revenue − Expenses), not complete PBT, until it is wired in.

---

# PHASE 12 STATUS: **PARTIAL**

The core composition-layer architecture, executive dashboard, growth
analytics, loan funnel, unit economics, and financial-year date
utilities are implemented and tested, with the single-source-of-truth
guarantee (§B) enforced structurally rather than by convention. Excel
exports, PDF reports, the management report pack, cash flow reporting,
and report snapshots — items Z, AA, Y, U, AB — are the major gaps,
hence PARTIAL.

## PHASE 8 REMAINING ITEMS
Unchanged: full report-type export wiring for 2 register types,
GST-input-tax-credit tracking, admin UI.

## PHASE 9 REMAINING ITEMS
Unchanged: no admin-initiated refund workflow, 2 daily-summary Excel
exports, large-failed-payment-spike/repeated-webhook-failure alerts,
`go test -race` not run, no admin UI.

## PHASE 10 REMAINING ITEMS
Unchanged: `PostTDSDeduction`/`finance.MarkPaid` not automatically
linked, no return-content/certificate generation, no live PAN-validation
provider, 5 of 10 named Excel exports, no admin UI.

## PHASE 11 REMAINING ITEMS
Unchanged: Excel exports for all 10 named report types, PDF
audit-report generation, unified 5-domain management dashboard
composition, automated data-lineage exception detection for regulatory
returns, regulatory-return-pending/rejected and reconciliation-failure
alert triggers, dedicated evidence-IDOR/CAPA-manipulation/applicability-
tampering security tests, admin panel UI.

**NEXT PHASE: PHASE 13 — GROWTH ANALYTICS, RISK ANALYTICS, PORTFOLIO INTELLIGENCE & MANAGEMENT DECISION SYSTEM**

**REMAINING PHASES: 13, 14, 15, 16**
