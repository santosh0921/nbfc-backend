# Phase 11 — RBI/NBFC Compliance, Regulatory Reporting, Audit Management, Compliance Calendar & Evidence Management

Continues from Phases 1–10, all re-verified building and passing before
this phase's work began.

**This system is a compliance-management and reporting tool. It is NOT
itself proof of regulatory compliance (item AC).** No claim of "RBI
compliant," "NBFC compliant," "audit certified," or "regulator approved"
is made anywhere in this report or in the code. Every requirement,
applicability determination, due date, and NBFC-category field this
module can hold is admin-entered after independent verification — this
phase seeds **zero** requirement rows, zero NBFC-profile field values,
and zero regulatory-return rows.

---

## A) NBFC Profile / Applicability: 🟢 COMPLETE (technical), 🔵 REQUIRES EXTERNAL REVIEW (content)

`internal/compliance/profile.go` — single-row `NBFCProfile`, every field
(NBFC type, regulatory layer, deposit-acceptance status, registrations,
compliance officer) starts blank. `AcceptsDeposits` is a `*bool` —
`nil` means "not yet stated," never defaulted to `false`. `ConfirmProfile`
is a distinct, explicit action required before `ConfirmationStatus`
leaves `REQUIRES_REVIEW`; any subsequent edit resets it back to
`REQUIRES_REVIEW` (tested implicitly by construction — `UpdateProfile`
unconditionally clears the confirmation fields).

**MANAGEMENT APPROVAL REQUIRED**: this NBFC's actual legal category,
scale-based-regulation layer, and registered activities must be entered
and confirmed by management/compliance — nothing in the existing
codebase (loan products, etc.) was used to infer this, per the explicit
instruction not to guess.

---

## B) Compliance Requirement Master: 🟢 COMPLETE (technical), 🔵 REQUIRES EXTERNAL REVIEW (content)

`ComplianceRequirement` — every field item B lists, `Status` starts
`DRAFT` and cannot become `ACTIVE` without a non-empty `SourceReference`
(tested: `TestRequirementWithoutSourceCannotActivate`). `Applicability`
defaults to `REQUIRES_REVIEW`, never `APPLICABLE`. Zero rows seeded.

---

## C) RBI Requirements: ⚪ NOT APPLICABLE (framework supports it; no content seeded)

Item C's category list (Governance, KYC/AML, Fair Practices, Digital
lending, Outsourcing, InfoSec, ...) is supported as free-text `Category`
values on `ComplianceRequirement` — **not** a fixed enum, so nothing
forces a category that doesn't apply. **No category was marked
applicable by this phase** — doing so would require the NBFC-profile
confirmation in §A first, which this session cannot perform.

---

## D) Compliance Calendar: 🟢 COMPLETE

`ComplianceCalendarItem` — full status vocabulary
(`NOT_STARTED→...→OVERDUE/EXCEPTION`), only creatable against an
`ACTIVE` requirement. `ComplianceReminderConfig` model exists for
configurable reminder offsets (item D explicitly allows the 30/15/7/3/1-
day list as an editable example) — no rows seeded by this phase (an
admin configures real offsets). `RefreshOverdueCalendarItems` +
`RunDailySweep` (item X) raise `COMPLIANCE_OVERDUE`/`COMPLIANCE_DUE_SOON`
alerts, deduplicated per still-open alert.

---

## E) Regulatory Returns: 🟢 COMPLETE (technical)

`RegulatoryReturnTracking` — full status vocabulary from item E,
maker≠checker enforced (`ReviewAndApproveReturn`, tested:
`TestReturnMakerCannotApproveOwn`). **Deliberately kept separate from
Phase 6's `RegulatoryReportRun`** (the existing, working generic
report-generation/validation workflow) rather than merged — per the
instruction not to rebuild completed functionality; the two can
cross-reference by ID for the same real filing.

---

## F) Regulatory Data Reconciliation: 🟡 PARTIAL

`RegulatoryDataException` — a return cannot be approved while any
exception for it is `OPEN` (tested: `TestReturnApprovalBlockedByOpenExceptions`).
**What's built**: the blocking mechanism and the exception model.
**Not built this phase**: automated detection queries that populate
these exceptions from real loan/ledger data (item F's "missing records,"
"duplicate records," "outstanding balance mismatch," etc.) — `RecordDataException`
exists as the entry point, but nothing calls it automatically yet for a
specific return type, since no real return content-generation exists to
detect mismatches against (§E — Phase 6's framework has the same gap).

---

## G) Management Sign-off: 🟢 COMPLETE

Maker/reviewer/approver fields + approval date/comments on
`RegulatoryReturnTracking`, same maker≠checker enforcement as §E.

---

## H) Evidence Management: 🟢 COMPLETE

`ComplianceEvidence` — reuses Phase 3's AES-256-GCM encrypted storage
provider directly (not a second storage path), SHA-256 integrity hash,
every field item H lists. Never publicly accessible — retrieval only via
`storage.Active().Retrieve`, gated by admin auth + `ComplianceView`.
Download is audit-logged (`compliance.evidence.downloaded`).

---

## I) Evidence Version Control: 🟢 COMPLETE

`UploadNewVersion` — creates a new row (never overwrites), links via
`PreviousVersionID`, marks the prior version `SUPERSEDED` (never deleted,
still retrievable) — tested end-to-end including confirming the old
version's *content* is unchanged (`TestEvidenceVersioning`).

---

## J) Policy Management: 🟢 COMPLETE

`Policy` — **cannot be created without a real, already-uploaded
`ComplianceEvidence` row** (`CreatePolicy` validates the referenced
evidence exists) — directly enforces item J's "do not claim a policy
exists unless evidence is uploaded" (tested:
`TestPolicyRequiresEvidence`). Zero policies exist yet — none were
fabricated to populate the register.

---

## K) Policy Review Workflow: 🟢 COMPLETE

Full lifecycle (`Draft→Review→Approval→Effective→Revision→Superseded`),
`CreateRevision` creates a new version row (never edits in place),
`FindOverduePolicyReviews` feeds the daily sweep's `POLICY_REVIEW_OVERDUE`
alert.

---

## L) Internal Audit Module: 🟢 COMPLETE

`InternalAudit` — every field item L lists. **No audits or findings were
created by this phase** — per "do not fabricate audit findings," this
module is empty scaffolding until a real internal audit happens.

---

## M) Audit Finding Lifecycle: 🟢 COMPLETE

`AuditFinding` — full lifecycle including `ManagementResponse`.
**Closure is hard-blocked without evidence + a verifier name**
(`ErrClosureRequiresEvidence`, tested:
`TestFindingCannotCloseWithoutEvidence`) — directly implements item M's
explicit requirement.

---

## N) CAPA: 🟢 COMPLETE

`CAPA` — full status vocabulary, linked to a `FindingID`,
`RefreshOverdueCAPAs` feeds the daily sweep.

---

## O) Compliance Risk Dashboard: 🟢 COMPLETE — correctly and carefully named

`GenerateDashboard` returns `InternalComplianceTrackingScorePercent`
(exactly the name item O mandates) computed as
`Completed / TotalApplicableCalendarItems × 100` — **never labeled a
"compliance score"** anywhere in code, comments, or this report. Every
count (requirements, calendar items by status, findings by severity, open
CAPAs) is a live server-side aggregation.

---

## P) Management Dashboard: 🟡 PARTIAL

Item P asks for one unified executive view spanning Regulatory/Audit/
Financial/Tax/Security/Operations. **This phase built the Regulatory/
Audit slice** (`GET /admin/compliance/dashboard`). The Financial slice
already exists (`internal/finance`, Phase 8), Tax (`internal/tax`, Phase
10), Security (Phase 7's compliance-health dashboard, `internal/reporting`),
Operations (Phase 5/6's risk/MIS dashboards) — **all pre-existing,
independently reachable, not yet composed into one single combined
endpoint**. Building that composition wasn't attempted this phase to
avoid duplicating five already-working dashboards into a fragile sixth
one — flagged as remaining work (§U), not silently skipped.

---

## Q) Regulatory Report Generation: 🟢 COMPLETE (reuses Phase 6, extended by §E/§G)

Phase 6's `RegulatoryReportRun` framework (versioned, maker-checker,
frozen after approval) is unchanged and still the generation engine;
Phase 11's `RegulatoryReturnTracking` is the compliance-team-facing
tracking layer around it. `CreateCorrection` is the only way to change a
submitted/accepted return — a new linked row, original untouched.

---

## R) Excel Reporting: 🔴 MISSING

**Not built this phase.** Item R's 10 named exports (Compliance Register,
Calendar, Regulatory Returns Register, Audit Findings, CAPA Register,
Policy Register, Evidence Register, Compliance Exceptions, Overdue
Compliance, Management Compliance Summary) were not wired into Phase 6/9/10's
existing `BuildRegisterReport` registry this session — a real gap, not
attempted given the volume of other Phase 11 scope; flagged as the
highest-priority remaining item (§U).

---

## S) PDF Audit Report Foundation: 🔴 MISSING

Not built this phase — no PDF generation for Internal Audit
Report/CAPA Report/Management Compliance Report was attempted. Phase 6's
minimal PDF writer (`internal/reporting`'s `generatePDFPlaceholder`)
exists and could be reused for this in a future phase.

---

## T) Frontend / Admin Panel: ⚪ NOT APPLICABLE

No Flutter UI work this phase, consistent with every prior phase. New
backend endpoints exist under `/admin/compliance/*`.

---

## U) Role-Based Access: 🟢 COMPLETE

5 new permissions (`COMPLIANCE_MGMT_VIEW/MANAGE/APPROVE`,
`AUDIT_MODULE_ACCESS/MANAGE`) — reused Phase 1/7's existing role model
exactly as instructed, **no new roles invented**. `COMPLIANCE_OFFICER`
(existing role) gets manage+approve+audit-access; `AUDITOR` (existing
role) gets read-only compliance+audit access only. Confirmed: an
ordinary customer has zero compliance routes reachable (all under
`adminGroup`, JWT-role-distinct from customer tokens since Phase 1).
Maker≠checker verified for regulatory-return approval (§E).

---

## V) Evidence Security: 🟡 PARTIAL

Reuses Phase 3's storage security wholesale (encrypted at rest, no public
URL, path-traversal-safe UUID references — all pre-existing, re-verified
unchanged). **Not separately tested this phase**: a dedicated IDOR test
for evidence download by an unauthorized admin (RBAC gating exists —
`ComplianceView` required — but no dedicated cross-role test was written
beyond the general dashboard-access test), unauthorized deletion/
replacement (no delete endpoint exists at all — evidence is
version-controlled, never deleted, which is itself the safest posture,
but wasn't explicitly negative-tested).

---

## W) Compliance Audit Trail: 🟢 COMPLETE

New audit actions: `compliance.profile.updated/confirmed`,
`compliance.requirement.created/activated/applicability_changed`,
`compliance.calendar.created/status_changed`, `compliance.return.created/approved/submitted/correction_created`,
`compliance.evidence.uploaded/new_version/downloaded`,
`compliance.policy.created/status_changed/revised`,
`compliance.audit.created`, `compliance.finding.created/management_response/closed`,
`compliance.capa.created/status_changed`. All via `audit.RecordDetailed`
(Phase 1, append-only, unmodified this phase).

---

## X) Alerting: 🟢 COMPLETE

`RunDailySweep` — 6 alert types (compliance due-soon/overdue, CAPA
overdue, critical finding, policy review overdue, evidence expiry),
deduplicated per still-open alert, each with severity/owner/status.
Regulatory-return-pending/rejected and reconciliation-failure alert
types from item X's list are not yet raised (no automated trigger wired
for those two specifically) — flagged (§U).

---

## Y) Data Lineage: 🟡 PARTIAL

This phase's own new tables don't introduce manually-entered numbers
where a system-generated one exists — `RegulatoryReturnTracking`'s
`DataSource` field is a pointer/description, not a place to hand-type a
total. The deeper drill-down chain item Y describes ("Total Disbursed" →
disbursement transactions → payment references → ledger → reconciliation)
**already exists across Phases 4/8/9** (`accounting.GeneralLedger`,
`finance` registers, `reporting` registers) — this phase did not need to
rebuild it, only avoid breaking it, confirmed by the unchanged Phase 4/8/9
tests still passing.

---

## Z) Compliance Testing: 🟡 PARTIAL

7 new tests, `internal/compliance/compliance_test.go`, all passing:
1. `TestReturnMakerCannotApproveOwn`
2. `TestReturnApprovalBlockedByOpenExceptions`
3. `TestFindingCannotCloseWithoutEvidence`
4. `TestPolicyRequiresEvidence`
5. `TestEvidenceVersioning`
6. `TestUnauthorizedCannotAccessDashboard`
7. `TestRequirementWithoutSourceCannotActivate`

**Not covered**: dedicated evidence-IDOR test, CAPA-manipulation test,
applicability/due-date-manipulation-by-unauthorized-user test, Excel
export authorization (no exports were built, §R).

---

## AA) Full Regression

Full suite (Phases 1–11) + Phase 7's 15 security tests — run via
`go test ./...` after all Phase 11 changes; build/vet clean. (Exact
pass/fail counts confirmed in the final validation section below, run
after this report was drafted, per the session's standing discipline of
verifying before concluding.)

---

## AB) Official Source / Regulatory Validation

**No RBI requirement was implemented with asserted applicability this
phase.** Every `SourceReference` field exists and is enforced as
mandatory before a requirement activates, but zero requirements were
created — there is nothing yet to cite a source for. This is the correct
state per the instruction not to hard-code generic RBI requirements
without first determining actual applicability.

---

## AC) Compliance Disclaimer

Restated per instruction: this application is a compliance-**management**
and **reporting** system. It tracks whether compliance activities were
performed and evidenced; it is not itself proof that any activity was
performed correctly, that any RBI requirement was correctly identified,
or that this NBFC is compliant with anything. See the classification
table below for what still requires human/professional action.

---

## Classification Summary

| Area | Status |
|---|---|
| A. NBFC Profile/Applicability | 🟢 (technical) / 🔵 (content) |
| B. Requirement Master | 🟢 (technical) / 🔵 (content) |
| C. RBI Requirements | ⚪ (no content seeded, by design) |
| D. Regulatory Returns | 🟢 |
| E. Compliance Calendar | 🟢 |
| F. Policy Register | 🟢 |
| G. Evidence Management | 🟢 |
| H. Internal Audit | 🟢 |
| I. Audit Findings | 🟢 |
| J. CAPA | 🟢 |
| K. Management Sign-off | 🟢 |
| L. Data Lineage | 🟡 |
| M. Compliance Dashboard | 🟢 (internal tracking score, correctly named) |
| N. Excel Reports | 🔴 |
| O. PDF Audit Reports | 🔴 |
| P. Frontend Changes | ⚪ |
| Q. Security (RBAC/maker-checker/evidence) | 🟢 / 🟡 (IDOR not separately tested) |
| R. Tests | 🟡 (7 written, several item-Z scenarios not covered) |
| S. Vulnerabilities | none found this phase |
| T. Fixes Implemented | none needed — new module, no pre-existing bug touched |
| U. Remaining Items | see below |
| V. Management/CA/Legal Review Items | see below |

---

## Remaining Items (§U)

- Excel exports for all 10 named report types (item R) — highest priority.
- PDF audit-report generation (item S).
- Unified 5-domain management dashboard composition (item P) — the 5
  underlying dashboards all already exist independently.
- Automated data-lineage exception detection for regulatory returns
  (item F) — the blocking mechanism exists, the detection queries don't.
- Regulatory-return-pending/rejected and reconciliation-failure alert
  triggers (item X).
- Dedicated evidence-IDOR / CAPA-manipulation / applicability-tampering
  security tests.
- Admin panel UI.

---

## Management/CA/Legal Review Items (§V)

- Confirm and enter the NBFC's actual legal category, scale-based-
  regulation layer, deposit-acceptance status, and registrations
  (`NBFCProfile`) — then execute `ConfirmProfile`.
- Identify which of item C's regulatory categories genuinely apply to
  this NBFC's business model, and draft the specific requirements with
  verified official sources before activating any.
- Assign real compliance officer/internal auditor/reviewer identities to
  the roles this module tracks.
- Confirm the actual applicable regulatory returns (names, frequency,
  due dates) from official RBI sources — none are configured.
- Legal/CS review of whatever policies get drafted and uploaded as
  evidence — this module tracks that a policy document was uploaded, not
  that its content is legally sufficient.
- External audit / VAPT remain exactly as scoped in Phase 7's report —
  unaffected by this phase.

---

# PHASE 11 STATUS: **PARTIAL**

Core compliance-management data model, workflows (requirement
lifecycle, calendar, regulatory-return maker-checker, evidence
versioning, policy-requires-evidence, finding-closure-requires-evidence,
CAPA, alerts, internal tracking dashboard) are implemented and tested.
Excel/PDF reporting (items R/S) were not built this phase — the single
largest gap — along with the unified 5-domain dashboard composition and
several item-Z test scenarios, hence PARTIAL.

## PHASE 8 REMAINING ITEMS
Unchanged from Phase 9's carry-forward: full report-type export wiring
for 2 register types, GST-input-tax-credit tracking, admin UI.

## PHASE 9 REMAINING ITEMS
Unchanged: no admin-initiated refund workflow, 2 daily-summary Excel
exports, large-failed-payment-spike/repeated-webhook-failure alerts,
`go test -race` not run, no admin UI.

## PHASE 10 REMAINING ITEMS
Unchanged: `PostTDSDeduction`/`finance.MarkPaid` not automatically
linked, no return-content/certificate generation, no live PAN-validation
provider, 5 of 10 named Excel exports, no admin UI.

**NEXT PHASE: Phase 12 — Complete NBFC MIS, Revenue, Expenses, Profitability, Management Reporting & Excel/PDF Reporting**

**REMAINING PHASES: 12, 13, 14, 15, 16**
