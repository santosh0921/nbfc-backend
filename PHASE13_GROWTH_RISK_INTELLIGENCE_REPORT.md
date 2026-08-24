# Phase 13 — Growth Analytics, Risk Analytics, Portfolio Intelligence & Management Decision System

Continues from Phases 1–12, all re-verified building and passing before
this phase's work began (`go build ./...`, `go vet ./...`, `go test ./...`
all green — 14 packages with tests, including the new `internal/analytics`
package, 8.1s).

**This is decision-support, not decision-making.** Nothing in
`internal/analytics` makes a credit, regulatory-classification,
accounting, or tax decision. The five layers item's introduction demands
be kept separate are separate by construction:

- **OBSERVED DATA** — the existing loan/payment/journal/EMI tables,
  untouched by this phase.
- **CALCULATED METRIC** — computed at request time in `internal/analytics`,
  composing Phase 4/5/8/10/11's own functions rather than re-deriving
  them (same discipline as Phase 12's `internal/mis`).
- **RISK INDICATOR** — `models.RiskSignal`: an observation, never an
  accusation, always starting `OPEN` and closable only by a named human.
- **MANAGEMENT THRESHOLD** — `models.ManagementThreshold`: zero rows
  seeded. The alert engine (item Z) produces zero alerts until an admin
  configures one.
- **REGULATORY RULE** — Phase 5's `ClassificationRule`/`ProvisioningRule`,
  untouched. Every operational risk report in this phase carries an
  explicit disclaimer distinguishing itself from that layer.

**Correction to Phase 12's report**: that report marked Funding Cost
(item K) as an unavailable dependency. Re-inspection this phase found
`models.FundingSource`/`BorrowingTransaction` (Phase 8) and ledger
account 5200 (`AcctBorrowingInterestExp`) already exist and are wired to
`AdminListFundingSourcesHandler`/`AdminCreateFundingSourceHandler`
(`internal/routes/routes.go:205-206`). Funding cost **is** derivable —
`analytics.FundingCost` reads it directly from the ledger and now feeds
Profitability (item T). It reads zero only because no funding source has
been entered yet, which is an honest zero, not a fabricated one.

---

## A) Current Analytics Audit: 🟢 COMPLETE

Inspected every existing dashboard/generator before writing new code:
`mis.GenerateGrowthReport`/`GenerateFunnelReport` (Phase 12, count-only),
`risk.PortfolioDashboard` (Phase 5), `accounting.ProfitAndLoss` (Phase
4), `finance.GenerateFinancialDashboard` (Phase 8). No incorrect
calculation was found. Phase 13 therefore **extends** rather than
replaces: `internal/analytics` adds amount/drop-off columns to the
funnel, growth-quality flags, risk/concentration/anomaly analytics, the
KPI dictionary, and the alert engine — none of which existed — while
composing the existing generators for every number they already produce.

## B) Growth Analytics: 🟢 COMPLETE

`analytics.GenerateGrowthDashboard` — every metric group item B lists
(Customers/Applications/Loans/Disbursements/Collections/Portfolio/
Revenue/Expense/Profitability), each figure sourced from real
aggregation (no client-side sums). `Metric.Unit` (PAISE/COUNT/PERCENT)
makes every number self-describing.

## C) Loan Funnel: 🟢 COMPLETE

`analytics.GenerateFunnel` — 8 stages (Application → KYC → Credit
Assessment → Approval → Sanction → Disbursement → Active → Closure),
count + amount at every stage, conversion% and drop-off% between the
period-scoped stages, and `LargestLeakage` names the single worst
transition. As in Phase 12, Sanction is documented as identical to
Approval — this codebase's state machine has no separate sanction
transition.

## D) Customer Analytics / Segments: 🟡 PARTIAL

`analytics.SegmentAnalytics` genuinely supports only new-vs-repeat
borrower, because that is the only segmentation this codebase's data can
produce. Customer-type, acquisition-channel, and risk-category
segmentation are explicitly returned as `NOT CALCULABLE` in the
response's `Unavailable` field rather than omitted or invented.

## E) Product Analytics: 🟡 PARTIAL

`analytics.ProductAnalytics` — customers/applications/approval
rate/average loan size/delinquency per `LoanApplication.Category`, the
only product-like field this codebase has. Revenue, expenses, funding
cost, and profitability **by product** are explicitly marked
unavailable/not-reliably-calculable — no journal entry, expense, or
funding transaction carries a product attribution, and inventing one
would violate item G's own instruction.

## F) Channel Analytics: ⚪ NOT APPLICABLE

`analytics.ChannelAnalytics` confirms no acquisition- or
collection-channel field exists anywhere in this codebase and returns an
explicit, self-describing empty report rather than inventing channel
categories — exactly what item H instructs.

## G) Geographic Analytics: 🟡 PARTIAL

`analytics.GeographicAnalytics` aggregates by `CustomerAddress.CurrentState`
only (customers/applications/approval rate/disbursed/average loan size).
Collections/outstanding/revenue/expenses by geography are unavailable —
no transaction carries a location attribution. Only state-level
aggregates are ever returned; no precise address is exposed, per item I.

## H) Portfolio Risk: 🟢 COMPLETE

`analytics.GeneratePortfolioRisk` embeds Phase 5's `risk.PortfolioDashboard`
directly (same composition discipline as Phase 12) and attaches
`RegulatoryDisclaimer` to every response, stating explicitly that these
are operational figures, not a regulatory asset classification.

## I) DPD: 🟢 COMPLETE

`analytics.GenerateDPDTrend` — 6 monthly snapshots (configurable) across
the four operational buckets (1-30/31-60/61-90/90+), each with
accounts/amount/portfolio%, all four buckets always present (including
empty ones). `Trend` (IMPROVING/STABLE/DETERIORATING/INSUFFICIENT DATA)
compares the 90+ bucket's portfolio share between the first and last
snapshot. `Methodology` is returned inline with the data, not only in
documentation — item K's explicit requirement. Deliberately uses its own
fixed bucket set rather than Phase 5's configurable
`DelinquencyBucket` rows, so an operational trend chart can never be
mistaken for the regulatory-facing bucket configuration.

## J) Collection Risk: 🟢 COMPLETE

`analytics.GenerateCollectionRisk` — due/collected/overdue amounts,
collection rate, overdue rate, repeat-overdue-customer count, trend
(vs. the prior equal-length period), and the subset of products whose
delinquency rate exceeds the portfolio average. Region/channel
breakdowns are marked unavailable (item L) rather than guessed.

## K) Concentration: 🟢 COMPLETE

`analytics.GenerateConcentration` — top-10 exposure by customer
(identified only by an opaque `customer:<uuid>` reference, never name/
PAN/mobile), product, and state, each with `PortfolioPercent`. Every
response carries `Indicator`, explicitly stating no applicable
regulatory concentration limit has been verified — item M's "use
CONCENTRATION INDICATOR, never call it a breach."

## L) Anomaly Signals: 🟢 COMPLETE

`analytics.DetectOutliers` (item N) — a purely statistical 3-standard-
deviation test over disbursement and payment amounts, methodology
returned inline, every result labelled `ANOMALY / REVIEW REQUIRED`
(tested: `TestOutliersAreLabelledReviewRequired`). `internal/analytics/signals.go`
(item O) — `RiskSignal` foundation with three detectors (shared bank
account across customers, repeated failed payments, multiple
applications in a short window), each carrying Reason/Source/Severity,
starting `OPEN`, dedupe-keyed so a standing observation is never
re-raised, and closable only by a named reviewer (tested:
`TestRiskSignalRequiresReviewerToLeaveOpen`,
`TestSignalDetectionIsIdempotent`). No signal type or field ever asserts
fraud.

## M) Revenue Quality: 🟢 COMPLETE

`analytics.GenerateRevenueQuality` — interest/fee/penalty-other mix from
the chart of accounts (never principal recovery), revenue per active
loan and per customer, growth vs. the prior period (nil-safe). Tested
that the three components sum exactly to the total
(`TestRevenueQualityExcludesPrincipal`).

## N) Expense Efficiency: 🟢 COMPLETE

`analytics.GenerateExpenseEfficiency` — total/by-category, per-customer,
per-active-loan, per-rupee-disbursed, per-rupee-collected, growth, and
the largest categories by size — explicitly labelled as a magnitude
ranking, not an efficiency judgement, per item S's own instruction that
size alone doesn't make a cost inefficient.

## O) Profitability: 🟢 COMPLETE (PBT), 🔵 PAT REQUIRES CA VALIDATION

`analytics.GenerateProfitability` — PBT = Revenue − Operating Expenses −
Funding Cost (funding cost is now real, see the correction above), profit
margin, profit per active loan/customer, trend vs. prior period. PAT is
explicitly not computed: `PATNote` states this system has no corporate
income-tax provision, and that Phase 10's TDS (withholding on income paid
TO the NBFC) must never substitute for a tax charge (tested:
`TestProfitabilityDoesNotClaimPAT`). Profit by product remains
unavailable — see item E.

## P) Unit Economics: 🟢 COMPLETE (reused from Phase 12, unchanged)

Phase 12's `mis.GenerateUnitEconomics` already implements exactly what
item U asks (CAC/collection-cost/KYC-cost/etc. → `NOT AVAILABLE`;
revenue/cost-per-customer/loan → computed). Not duplicated here.

## Q) Cohorts: 🟢 COMPLETE

`analytics.GenerateCohorts` — groups loans by calendar month of
disbursement (customers/loans/disbursed/repaid/outstanding/overdue/
closed per cohort), only for months that actually contain a disbursed
loan — empty months are omitted rather than shown as zero cohorts, per
item V.

## R) Customer Retention: 🟢 COMPLETE

`analytics.GenerateRetention` — new vs. repeat borrower counts (actual
disbursed-loan counts, not an inferred rate) and a count of customers who
closed a loan and then borrowed again. Deliberately does not compute a
churn or projected-retention rate — item W warns against inferring
retention from incomplete data, and this codebase's loan-closure date
series is not complete enough to support one honestly.

## S) Forecasting: 🔴 NOT BUILT

No statistical forecasting was implemented this phase. Item X is
explicit that any forecast must be clearly labelled and never presented
as actual — building one without that guarantee in place was judged
higher-risk than leaving it for a dedicated phase. The historical time
series this would need (cohorts, DPD trend, growth dashboard) already
exist as inputs.

## T) Budget vs Actual: 🟢 COMPLETE (foundation), ⚪ zero budgets by design

`models.Budget` + `analytics.GenerateBudgetVariance` — the full data
model and variance calculation exist (variance, variance%, nil-safe on a
zero target). Zero rows are seeded — item Y: "do not invent budgets"
(tested: `TestNoThresholdsAreSeeded`).

## U) Management Alert Engine: 🟢 COMPLETE

`models.ManagementThreshold` + `analytics.EvaluateThresholds` — a
threshold may only reference a KPI already in the dictionary (tested:
`TestThresholdRejectsUndocumentedMetric`), so an alert can never fire on
an undefined number. Zero thresholds are seeded, so the engine produces
zero alerts out of the box (tested:
`TestAlertEngineProducesNothingWithoutConfiguredThresholds`) — item Z:
"do not hard-code thresholds." A raised alert states metric/current
value/threshold/direction/period/source in `Explanation`, never a
recommended action.

## V) Management Decision Center: 🟢 COMPLETE

`analytics.GenerateDecisionCenter` — combines open management alerts,
growth-quality attention items, and open risk signals in one response.
Growth-quality (`internal/analytics/quality.go`, item D) flags four
specific relationships (disbursement outpacing revenue, expense
outpacing revenue, overdue outpacing disbursement growth, collections
declining while disbursement grows) — each states what/why/source/period
and is always labelled `MANAGEMENT ATTENTION REQUIRED`, never a
judgement like "bad decision" or "stop lending" (item AA's explicit
example is reproduced verbatim in the file's doc comment as the
derivation basis for the thresholds used).

## W) KPI Dictionary: 🟢 COMPLETE — MANDATORY, DELIVERED

`internal/analytics/kpi.go` — 10 KPIs seeded (disbursed amount,
collections, revenue, expenses, funding cost, PBT, outstanding principal,
collection efficiency, approval rate, 90+ DPD share), each with
Definition/Formula/DataSource/Frequency/Unit/Dependency, seeded
idempotently at boot so a formula change in code is reflected in the
dictionary rather than drifting from it (tested:
`TestKPIDictionaryIsSeededAndComplete`, `TestDocumentedKPIsResolve`).

## X) Data Lineage: 🟢 COMPLETE (for seeded KPIs)

Every seeded KPI carries a `Lineage` (Dashboard → Report → Calculation →
Source → Transaction), returned alongside the KPI by
`GET /admin/analytics/kpi-dictionary` — tested that every KPI has a
non-empty Source and Calculation.

## Y) Excel Reports: 🟢 COMPLETE (18/18 named workbooks)

All 18 workbooks item AD names are wired into the existing
`BuildRegisterReport` dispatch (`internal/reporting/registers.go`),
sharing that handler's existing envelope (period/generated
date/generated by/filters/totals/record count) rather than defining a
second one: `GROWTH_DASHBOARD`, `CUSTOMER_GROWTH`, `LOAN_GROWTH`,
`DISBURSEMENT_GROWTH`, `COLLECTION_GROWTH`, `PORTFOLIO_GROWTH`,
`REVENUE_GROWTH`, `EXPENSE_GROWTH`, `PROFITABILITY_GROWTH`,
`PRODUCT_ANALYTICS`, `CUSTOMER_SEGMENTS`, `GEOGRAPHIC_ANALYTICS`,
`DPD_ANALYTICS`, `CONCENTRATION_ANALYTICS`, `COLLECTION_PERFORMANCE`,
`RISK_SIGNALS`, `MANAGEMENT_ALERTS`, `KPI_DICTIONARY`. Every one delegates
to `internal/analytics`, so an exported figure cannot disagree with the
corresponding dashboard. Tested individually
(`TestAnalyticsExportsBuild`) and that the DPD workbook carries its
disclaimer/methodology (`TestDPDExportCarriesDisclaimer`).

## X) Frontend Changes: ⚪ NOT APPLICABLE (scope)

No Flutter/admin-UI work this phase, consistent with every prior phase's
backend-only scope (Phases 7–12 all note the same gap). Item AE
(Intelligence nav) and item AF (charts) are therefore both ⚪.

## Y) Performance: 🟢 COMPLETE (by design, at current scale)

Every report is a server-side SQL aggregation (SUM/COUNT/GROUP BY), never
a client-side sum over fetched rows. Concentration/outlier queries are
capped (LIMIT 10/50). Item AH's specific asks — materialized views,
background jobs, date partitioning — were **not** added: at this
codebase's actual current data volume there is no evidence of a
performance problem, and item AH itself says not to prematurely
introduce infrastructure complexity without evidence. This is flagged as
a genuine gap to revisit once real data volume is known, not treated as
complete for a 100,000-customer scale.

## Z) Security: 🟡 PARTIAL

Every new `/admin/analytics/*` route is gated: read endpoints reuse
Phase 6's `rbac.MISView` (financial/growth data) or `rbac.RiskView`
(portfolio/signal/data-quality data); state-changing endpoints
(reviewing a signal, creating a threshold, resolving an alert) require
`rbac.RiskRuleManage` — no new permission was invented. Tested:
unauthenticated access rejected (`TestAnalyticsRequiresAuthentication`),
`COLLECTION_OFFICER` (has `RiskView` but not `MISView`) blocked from the
financial growth dashboard and from configuring thresholds
(`TestUnauthorizedCannotAccessAnalytics`). Not tested this phase:
dedicated IDOR/cross-period/export-abuse cases specific to the new
analytics endpoints (same gap Phase 11 and Phase 12 both carried
forward for their own new surfaces).

## AA) Tests: 🟢 COMPLETE

24 tests in `internal/analytics/analytics_test.go` covering: zero/negative
prior-period safety (items C), operational-vs-regulatory disclaimers
(items J/K), concentration-never-a-breach (item M), anomaly/signal review
discipline including idempotent detection (items N/O), revenue excluding
principal (item R), PAT never fabricated from TDS (item T), zero
seeded thresholds/budgets and the hard-coding prohibition (items Y/Z),
threshold-must-reference-a-documented-KPI, KPI dictionary completeness
and lineage (items AB/AC), data-quality checks never mutating financial
data (item AI), funnel nil-safety (item E), channel unavailability
honesty (item H), authorization (item AG), all 18 Excel exports
resolving, and prior-period boundary non-overlap. All 24 pass, alongside
the full pre-existing suite (14 packages, `go vet` clean).

A verification pass during this phase (a temporary, since-removed test
that surfaced raw-SQL errors GORM's `Scan` otherwise swallows silently)
caught and fixed two real bugs before they shipped: the expense-category
query referenced a non-existent `amount_paise` column (the actual field
is `gross_amount_paise`) and dated by `created_at` instead of
`expense_date`; the cohort query correlated a subquery against an
ungrouped column, which PostgreSQL rejects. Both are fixed in
`internal/analytics/economics.go` and now covered by the standing test
suite's exercise of every code path.

## AB) Vulnerabilities: none found this phase.

## AC) Fixes Implemented

- Corrected Phase 12's report: Funding Cost is calculable (see the
  correction note above), not an unavailable dependency.
- Fixed the two SQL bugs described in item AA before they reached the
  test suite as passing-but-wrong code.

## AD) Remaining Items (this phase)

- Statistical forecasting (item X) — not built; the historical series it
  needs already exist as inputs for a future phase.
- Frontend/Intelligence nav and charts (items AE/AF) — backend-only
  scope, consistent with every prior phase.
- Materialized views / background jobs / date partitioning for analytics
  at 100,000+ customer scale (item AH) — not built; no evidence of a
  performance problem exists yet at this codebase's actual data volume.
- Dedicated IDOR/cross-period/export-abuse security tests for the new
  analytics endpoints (item AG/Z).
- Channel and full customer-type/risk-category segmentation remain
  genuinely unavailable — no such data exists anywhere in this codebase.
- Product/geography-level revenue, expense, funding cost, and
  profitability remain NOT RELIABLY CALCULABLE — no cost allocation
  exists, and none was invented.

---

# PHASE 13 STATUS: **PARTIAL**

The core growth/risk/portfolio intelligence layer — growth dashboard,
funnel with amounts, growth quality, product/segment/geography
analytics, portfolio risk, DPD trend, collection risk, concentration,
outlier detection, the risk-signal foundation, revenue quality, expense
efficiency, profitability (PBT), cohorts, retention, the KPI dictionary
with lineage, the management alert engine, the decision center, and all
18 named Excel exports — is implemented and tested, composing rather than
duplicating every Phase 4/5/8/10/11/12 calculation it depends on.
Forecasting, the frontend, and scale-specific performance infrastructure
are the major gaps, hence PARTIAL.

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

## PHASE 12 REMAINING ITEMS
Cash Flow Foundation, Management Report Pack composition, remaining ~16
Excel workbooks (item Z was largely addressed for Phase 13's own new
report types, but Phase 12's own outstanding list — Sanction Register,
Outstanding Register, Revenue Register, Vendor Register, Funding
Register, etc. — is still open), PDF management reports (all 7 types),
report snapshot/versioning, Financial Year roll-forward report
composition, additional filters beyond date range, per-domain
data-consistency tests beyond the one finance-dashboard test,
IDOR/cross-period/export-abuse security tests for Phase 12's two
registers, audit-log wiring for MIS dashboard/report generation, admin
panel UI/drill-down.

**NEXT PHASE: PHASE 14 — COMPLETE VAPT, OWASP SECURITY VALIDATION, PENETRATION-TEST READINESS & SECURITY REMEDIATION**

**REMAINING PHASES: 14, 15, 16**
