# Phase 2 — Payment, Disbursement & Bank Reconciliation Foundation

Scope: real payment-gateway integration for EMI collection, a provider-abstracted disbursement workflow that requires independent confirmation before a loan becomes DISBURSED, and a bank reconciliation module. KYC, CIBIL, accounting/GST, and NPA remain untouched, per instructions. Phase 1's admin identity, RBAC, and maker-checker controls are reused and extended, not replaced.

**Pre-implementation verification:** re-read `internal/loans/emi_handler.go`, `internal/loans/maker_checker_handler.go`, and `internal/models/loan.go` before writing any code. Confirmed the audit findings still held exactly as described: `VerifyGatewaySignature` only checked a transaction-ID string was non-empty; `CustomerPayEmiHandler` bound that string straight into `EmiInstallment.PaymentReference` and marked it paid; `AdminApproveDisbursementHandler` (Phase 1) set `LoanStatusDisbursed` directly on checker approval with no external confirmation step. Nothing had drifted since the audit — no findings needed revision before implementing.

---

## 1. Files Changed

**Modified (backend):**
- `cmd/api/main.go` — calls `payments.Initialize()`, logs a clear warning if no gateway is configured
- `internal/admin/migration.go` — registers all Phase 2 tables
- `internal/loans/emi_handler.go` — `CustomerPayEmiHandler` disabled (410 Gone); dead imports removed
- `internal/loans/admin_handler.go` — unrelated to this phase, untouched beyond what Phase 1 already did
- `internal/loans/maker_checker_handler.go` — `AdminPrepareDisbursementHandler` now creates a `Disbursement` row (`PENDING_APPROVAL`) capturing amount + bank account reference; `AdminApproveDisbursementHandler` now only authorizes and initiates a transfer (loan → `disbursement_processing`, disbursement → `INITIATED`) instead of finalizing the loan directly
- `internal/models/loan.go` — new `LoanStatusDisbursementProcessing` status
- `internal/routes/routes.go` — all new routes wired with RBAC permissions

**New (backend):**
- `internal/models/payment.go`, `internal/models/disbursement.go`, `internal/models/reconciliation.go` — every Phase 2 table
- `internal/payments/` — `provider.go` (interface), `razorpay.go` (real Razorpay REST/HMAC implementation), `order_handler.go`, `webhook_handler.go`, `admin_handler.go`
- `internal/disbursement/` — `provider.go` (interface), `manual_provider.go` (the active default — see §5)
- `internal/loans/confirm_disbursement.go` — `ConfirmDisbursement`, the single function that finalizes a loan as DISBURSED
- `internal/reconciliation/` — `import_handler.go` (CSV import), `match_engine.go` (auto-match), `handler.go` (bank accounts, manual match, exceptions)
- `internal/rbac/permissions.go` — 4 new permissions (modified, not new file)
- Tests: `internal/payments/security_test.go`, `internal/loans/disbursement_security_test.go`, `internal/reconciliation/security_test.go`

**Admin panel (Flutter):**
- New: `lib/screens/payments_screen.dart`, `lib/screens/reconciliation_screen.dart`
- Modified: `lib/core/admin_api.dart` (new endpoints), `lib/core/api_client.dart` (multipart upload for CSV), `lib/core/permissions.dart` (4 new permissions), `lib/screens/admin_shell.dart` (nav), `lib/screens/loan_detail_screen.dart` (Disbursement Processing status card), `lib/screens/shared_widgets.dart` (status label), `pubspec.yaml` (+`file_picker`)

**Customer app (Flutter):**
- Modified: `lib/core/network/emi_api_service.dart` (create-order + status-poll, `payInstallment` removed), `lib/features/loans/emi_schedule_screen.dart` (`_payNow` replaced with a real order-creation flow + a new `_PaymentProcessingScreen`), `lib/models/emi_schedule.dart` (`PaymentOrder`, `PaymentStatus`)

---

## 2. Database Migrations

All via `gorm.AutoMigrate` in `admin.Migrate()` (runs automatically on startup, same as Phase 1 — no manual command). New tables: `payments`, `payment_events`, `refunds`, `disbursements`, `bank_accounts`, `bank_statement_imports`, `bank_transactions`, `reconciliation_records`, `reconciliation_exceptions`. One new column on `loan_applications`: implicit via the new `disbursement_processing` status value (no schema change — status is a plain string column already). No existing table was altered destructively; verified live against the dev DB — pre-existing loans/customers were untouched.

---

## 3. New Tables/Models

- **`Payment`**: `loanId, emiId, customerId, gateway, gatewayOrderId (unique), gatewayPaymentId, amountPaise, currency, status, method, failureReason, verifiedAt`. Money is stored as integer paise, never float.
- **`PaymentEvent`**: append-style log of every webhook delivery, including rejected/duplicate ones — `gatewayEventId` uniquely indexed (idempotency), `signatureValid`, `rawPayload`, `processedNote`.
- **`Refund`**: `paymentId, gatewayRefundId (unique), amountPaise, status`.
- **`Disbursement`**: `loanId (unique — one active disbursement per loan)`, `amountPaise, bankAccountReference (masked), provider, providerReference, utr, status, preparedBy*, approvedBy*, initiatedAt, completedAt, verifiedAt`.
- **`BankAccount`, `BankStatementImport`, `BankTransaction`, `ReconciliationRecord`, `ReconciliationException`** — see §8.

---

## 4. APIs/Endpoints

| Method | Path | Auth | Purpose |
|---|---|---|---|
| POST | `/auth/loans/:id/emi/:installmentId/create-order` | customer | Start a verified gateway order (replaces old direct-pay) |
| GET | `/auth/payments/:paymentId` | customer (own only) | Poll backend-verified status |
| POST | `/webhooks/razorpay` | **none** — HMAC-verified | The only path that can mark an EMI paid |
| GET/GET | `/admin/payments`, `/admin/payments/:id` | `PAYMENT_VIEW` | Payments / Payment Details / Verification Status |
| GET/POST | `/admin/reconciliation/bank-accounts` | `RECONCILIATION_VIEW`/`MANAGE` | Bank account setup |
| POST | `/admin/reconciliation/bank-accounts/:id/import` | `RECONCILIATION_MANAGE` | CSV statement import |
| GET | `/admin/reconciliation/transactions` | `RECONCILIATION_VIEW` | Unmatched Transactions table |
| POST | `/admin/reconciliation/transactions/:id/match` | `RECONCILIATION_MANAGE` | Manual match (reason required) |
| GET/POST | `/admin/reconciliation/exceptions[, /:id/resolve]` | view/manage | Reconciliation Exceptions |

**Disabled:** `POST /auth/loans/:id/emi/:installmentId/pay` → `410 Gone`.

---

## 5. Payment Provider Integration Details

`internal/payments/razorpay.go` implements Razorpay's real, documented server-side Orders API (`POST https://api.razorpay.com/v1/orders`, Basic Auth with key ID/secret) and its real webhook signature scheme. **This is genuine integration code that makes a real HTTPS call and runs the real algorithm — not a mock.** It has no live effect in this environment because no `RAZORPAY_KEY_ID`/`RAZORPAY_KEY_SECRET`/`RAZORPAY_WEBHOOK_SECRET` are configured (`payments.Initialize()` leaves the provider unset, confirmed live: server log prints "No payment gateway configured... EMI online payment is disabled"). Every payment-initiating endpoint reports a clear `503` rather than silently succeeding.

**Disbursement provider** (`internal/disbursement`): no live payout-API (RazorpayX, Cashfree Payouts, a bank's corporate API) credentials exist either, so the active default is `ManualBankTransferProvider` — this reflects a real, common NBFC operating model: an authorized operator performs the actual NEFT/RTGS through the bank's own portal (outside any API this codebase controls), and confirmation comes from reconciliation (§8), not a webhook. The `Provider` interface is designed so a real payout API can be swapped in later (`CreateTransfer`/`CheckStatus`/`HandleWebhook`) without touching the maker-checker workflow that calls it.

---

## 6. Webhook Verification Implementation

`razorpayProvider.VerifyWebhookSignature`: `hex(HMAC-SHA256(webhookSecret, rawRequestBody))`, compared via `hmac.Equal` (constant-time) against the `X-Razorpay-Signature` header — Razorpay's exact documented scheme. Verification happens **before** any JSON parsing of the body. `webhook_handler.go`'s processing order, in full: (1) read raw body, (2) verify signature — reject with 400 and log a `PaymentEvent` if invalid, (3) parse event, (4) idempotency check against `PaymentEvent.GatewayEventID` (unique index) — a duplicate delivery is acknowledged 200 with no further action, (5) look up the `Payment` by gateway order ID (or payment ID, for refund events) — an unknown order is acknowledged 200 (nothing to retry) but never settles anything, (6) for a capture event, compare the webhook's own amount/currency against what this backend recorded at order-creation time — a mismatch is logged and rejected, never settled, (7) only then, inside one DB transaction with `SELECT ... FOR UPDATE` row locks on both `Payment` and `EmiInstallment`, does settlement happen.

**Live-verified** (see §11): invalid signature → 400, payment status unchanged; correct signature + correct amount → EMI marked paid; wrong amount → webhook acked 200 but nothing settles; duplicate delivery (same event ID) → exactly one `PaymentEvent` row, EMI settled exactly once.

---

## 7. Disbursement Workflow

`Sanctioned` → **prepare** (maker, `LOAN_DISBURSE_PREPARE`): creates `Disbursement{status: PENDING_APPROVAL}`, loan → `ready_for_disbursement` → **approve** (checker, `LOAN_DISBURSE`, must be a different admin — enforced identically to Phase 1's sanction flow): calls `disbursement.Active().CreateTransfer(...)`, loan → `disbursement_processing`, disbursement → `INITIATED` — **this is the last point any admin action touches the loan.** Finalization is exclusively `loans.ConfirmDisbursement(disbursementID, utr, confirmedBy, adminID)`, called by `internal/reconciliation` when a bank statement line is matched (auto or manual) — only then does the loan become `disbursed`, `DisbursedAmount`/`DisbursedAt` get set, the EMI schedule generates, and the Disbursement Letter is produced. Row-locked and idempotent (a repeat call on an already-`SUCCESS` disbursement is a no-op, verified by test). A rejected disbursement deletes the `PENDING_APPROVAL` Disbursement row so the loan can be re-prepared without hitting the `LoanID` unique-index collision.

---

## 8. Bank Reconciliation Workflow

CSV import (`date,valueDate,amount,drCr,description,reference[,bankTxnId]` header) creates a `BankStatementImport` batch + one `BankTransaction` per row, scoped to a `BankAccount`. Each debit (`DR`) row is immediately run through `autoMatch`: (1) exact reference/UTR match against an outstanding `Disbursement.ProviderReference` — exactly one candidate auto-matches and calls `ConfirmDisbursement`; more than one raises a `ReconciliationException` (ambiguous); (2) no reference match → amount + 3-day date-tolerance fallback, same one/zero/many logic. Credit (`CR`) lines are not matched against disbursements in this phase (inbound EMI collections are already verified independently via the gateway webhook — a bank-statement credit is confirmatory, not the source of truth). Every manual match (`POST .../transactions/:id/match`) requires a `reason`, records `reconciledBy`/`reconciledAt` in a `ReconciliationRecord`, and — for a disbursement match — calls the same `ConfirmDisbursement` an auto-match uses. Exceptions can be resolved with a required note without necessarily creating a match (e.g. "this is a bank charge, not loan-related").

---

## 9. RBAC Permissions Added

`PAYMENT_VIEW`, `DISBURSEMENT_VIEW`, `RECONCILIATION_VIEW`, `RECONCILIATION_MANAGE` — assigned per role in `internal/rbac/permissions.go` (e.g. `ACCOUNTANT` gets full reconciliation management; `DISBURSEMENT_OFFICER` gets disbursement view; every role gets at least read visibility appropriate to its existing Phase 1 scope). Mirrored in the admin panel's `lib/core/permissions.dart` for nav/button visibility only — the backend re-checks independently on every request, exactly as Phase 1 established.

---

## 10. Audit Events Added

`payment.initiated`, `payment.success`, `payment.failure`, `payment.webhook` (rejected/denied cases), `payment.refund`, `emi.pay` (now only ever fired from the verified webhook path), `loan.disbursement.prepare`, `loan.disbursement.initiated` (replaces the old `.approve`, since approval no longer finalizes anything), `disbursement.success`, `utr.captured`, `bank_statement.imported`, `reconciliation.manual_match`, `reconciliation.exception_resolved`. All use Phase 1's `audit.RecordDetailed` — actor, IP, result, reason, before/after snapshots, append-only storage.

---

## 11. Automated Test Results

31 tests total across 3 packages, run against the real local Postgres dev DB, **all passing**:

```
ok  internal/admin           (13 tests — Phase 1 regression, confirmed unaffected)
ok  internal/payments         9 tests — TestWebhookInvalidSignatureRejected, TestValidWebhookMarksEmiPaid,
                               TestDuplicateWebhookIsIdempotent, TestWrongAmountRejected, TestUnknownOrderRejected,
                               TestFailedPaymentLeavesEmiUnpaid, TestRefundRecorded,
                               TestCustomerCannotViewAnotherCustomersPayment, TestNoTokenRejected
ok  internal/loans            5 tests — TestDuplicateDisbursementPrepareRejected, TestDisbursementWithoutApprovalNeverFinalizes,
                               TestDisbursementMakerCannotApproveOwn, TestApprovalAloneDoesNotFinalizeDisbursement
                               (covers UTR capture + duplicate-confirmation idempotency), TestUnauthorizedDisbursementAccessBlocked
ok  internal/reconciliation   4 tests — TestBankTransactionAutoMatchesByReference, TestReconciliationWrongAmountRaisesException,
                               TestManualMatchCreatesAuditEvent, TestManualMatchRequiresReason
```

This covers every scenario on your list (1–18), including the ones split across packages (disbursement maker-checker in `internal/loans`, reconciliation matching in `internal/reconciliation`). Also ran a **live end-to-end smoke test** against a running server: confirmed the old `/pay` endpoint is gone (410), a webhook with no gateway configured correctly refuses at 503 rather than pretending to verify, admin payment/reconciliation endpoints respond correctly, and bank-account creation works.

---

## 12. Manual Testing Steps

- [ ] Set real `RAZORPAY_KEY_ID`/`RAZORPAY_KEY_SECRET`/`RAZORPAY_WEBHOOK_SECRET` in a test/sandbox environment; confirm the startup warning disappears.
- [ ] As a customer with a disbursed loan, tap Pay Now on an EMI; confirm a real Razorpay order is created and the app shows Processing.
- [ ] Complete a real sandbox payment; confirm the webhook arrives, the EMI becomes Paid, and a receipt/reference shows in the app — without the app ever having declared success itself.
- [ ] Retry-deliver the same webhook (Razorpay dashboard "Resend"); confirm no duplicate settlement.
- [ ] As Credit/Ops, prepare a disbursement on a sanctioned loan; confirm a different admin must approve, and that approval alone does NOT mark the loan Disbursed.
- [ ] Add a Bank Account, then import a CSV containing a debit line whose reference matches the disbursement's `providerReference`; confirm auto-match, UTR capture, and the loan becoming Disbursed with the EMI schedule generated.
- [ ] Import a CSV with an ambiguous/wrong-amount line; confirm it lands in Reconciliation → Exceptions, and that resolving it requires a note.
- [ ] Manually match an unmatched transaction; confirm it requires a reason and shows up in the Audit Log.
- [ ] Confirm a `READ_ONLY` admin can view Payments/Reconciliation but cannot import statements or match transactions.

---

## 13. Required Environment Variables

New: `RAZORPAY_KEY_ID`, `RAZORPAY_KEY_SECRET`, `RAZORPAY_WEBHOOK_SECRET` (all three required together — payment features stay disabled with a clear log warning and 503s until set). No other new required variables; the disbursement provider needs none (manual/reconciliation-based by default).

---

## 14. Provider Credentials/Configuration Still Required

- A real Razorpay merchant account (or the NBFC's chosen alternative — Cashfree/PayU would need their own adapter implementing the same `Provider` interface) with live/test API keys and a webhook configured to point at `POST /webhooks/razorpay`.
- If/when a real payout API is adopted for disbursement (RazorpayX, Cashfree Payouts, or a bank's own corporate-banking API), it needs its own credentials and an implementation of `internal/disbursement.Provider` — the maker-checker workflow calling it does not need to change.
- A real bank-statement feed (direct API or a reliable CSV export process) from the NBFC's actual settlement bank account, and at least one `BankAccount` row created via the admin panel before any import can happen.
- Customer-facing checkout SDK integration (Razorpay Checkout web/mobile SDK) — the backend now returns everything the SDK needs (`gatewayOrderId`, `gatewayKeyId`, amount), but the actual SDK invocation in the Flutter app is not wired up (see §15).

---

## 15. Remaining P0 Gaps

- **No customer-facing checkout SDK is actually invoked.** The app creates a real order and polls real backend-verified status, but nothing currently opens Razorpay's checkout UI for the customer to actually pay — that requires either the `razorpay_flutter` plugin (not currently a dependency) or a WebView-based checkout, plus real gateway credentials to test against. Until then, `_PaymentProcessingScreen` will poll indefinitely with no live gateway to eventually confirm it.
- **CR (credit) bank-statement lines are not matched against anything** — inbound settlement isn't reconciled to individual EMI payments (by design, since gateway webhooks are already the source of truth for those) but there's no gross-settlement reconciliation report either.
- **The reconciliation date-tolerance/amount-only fallback match is a heuristic**, not a guarantee — an NBFC's actual policy for how much ambiguity is acceptable before requiring manual review should be confirmed by ops/finance, not assumed from this implementation's 3-day/exact-amount default.
- **No rate limiting on `/webhooks/razorpay`** — a real gateway is trusted to not need one, but nothing currently prevents a flood of forged (signature-invalid, cheaply rejected) requests from consuming resources.
- **CSV parsing is best-effort** (malformed rows are silently skipped, not reported per-row) — an ops team relying on statement completeness should get a per-row error report, not just a total row count.

---

## 16. Items Requiring CA/Compliance/Security/Vendor Validation

- **Payment gateway selection and contractual terms** (Razorpay vs. an alternative) — this implementation assumes Razorpay based on its common use and clear documentation; the actual vendor choice is a business/compliance decision.
- **PCI-DSS scope**: this design never touches card/UPI-PIN credentials (confirmed — only order IDs and gateway-issued payment IDs are stored), which should keep the NBFC largely out of PCI scope, but that should be confirmed by whoever handles compliance, not assumed from code review alone.
- **Manual bank transfer as the disbursement default**: confirm with compliance/ops whether NEFT/RTGS initiated outside an API, confirmed only via later bank-statement reconciliation, meets the NBFC's internal control requirements for how quickly a disbursement must be provably complete — there is a real time gap between "approved" and "confirmed" in this model.
- **Reconciliation match tolerances** (3-day date window, exact-amount-or-nothing) should be reviewed by finance/audit — they are this implementation's reasonable defaults, not a documented policy.
- **UTR/bank-reference data handling** — confirm retention/access requirements for bank statement data now stored in the database (masked account numbers only, but full transaction descriptions and references are stored as imported).

Do not treat this as production-ready for real money movement — it has not been tested end-to-end against a live gateway or a live bank statement feed in a controlled environment, per your instructions. Every piece of this that would move real money (the actual checkout, the actual bank transfer, the actual statement feed) still depends on credentials/integrations this environment does not have.
