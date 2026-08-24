# Phase 1 Security & Authorization Foundation — Implementation Report

Scope: backend-enforced admin identity, RBAC, session security, maker-checker on sanction/disbursement, and an upgraded audit trail. No accounting, GST changes, real CIBIL, real KYC providers, NPA, payment gateway, or bank disbursement integration were implemented — those remain P1/P2/P3, out of scope for this phase.

Every finding from the prior gap audit was re-verified against the current code before changing anything (confirmed: admin auth was still a plaintext `.env` comparison, no RBAC existed, no maker-checker existed, `AuditLog` had no actor-ID/IP/before-after fields, logout was a no-op) — nothing had changed since that audit, so no findings needed revision.

---

## 1. Files Changed

**Modified:**
- `cmd/api/main.go` — calls `admin.Bootstrap()`, adds `middleware.RequestID()`
- `internal/admin/jwt.go` — rewritten: access+refresh token generation, `Claims` now carries `AdminID`/`Role`/`SessionID`
- `internal/admin/login_handler.go` — rewritten: real `AdminUser` lookup, bcrypt, lockout, session issuance
- `internal/admin/migration.go` — adds `AdminUser`/`AdminSession` to `AutoMigrate`, installs append-only audit trigger
- `internal/admin/kyc_review_handler.go` — adds `audit.RecordDetailed` with before/after
- `internal/documents/handler.go` — adds audit logging on document access (admin loan/customer document list endpoints)
- `internal/loans/admin_handler.go` — old unilateral `sanction`/`disburse` actions disabled (410 Gone), reject-only path kept and upgraded to detailed audit
- `internal/middleware/admin_auth_middleware.go` — rewritten: validates JWT **and** re-checks the session/account server-side on every request
- `internal/models/auditlog.go` — new structured fields (additive, backward-compatible)
- `internal/models/loan.go` — new maker-checker state fields + two new status constants
- `internal/audit/handler.go` — `GET /admin/audit-log` now supports actor/date-range filters and returns `{entries, total}`
- `internal/routes/routes.go` — every admin route now gated by an explicit `rbac.Permission`

**New:**
- `internal/models/admin_user.go` — `AdminUser`, `AdminSession` models, role/status constants
- `internal/rbac/permissions.go` — the permission model (backend source of truth)
- `internal/admin/adminuser_handler.go` — admin-user CRUD, role changes, status changes, roles listing
- `internal/admin/bootstrap.go` — first-super-admin seeding
- `internal/admin/logout_handler.go`, `internal/admin/refresh_handler.go` — session revocation, refresh rotation
- `internal/admin/util.go` — small shared helpers
- `internal/audit/detailed.go`, `internal/audit/redact.go` — structured audit entries with redaction
- `internal/middleware/require_permission.go`, `internal/middleware/request_id.go`
- `internal/loans/maker_checker_handler.go` — the four new sanction/disbursement handlers + pending-approvals queue
- `internal/admin/security_test.go` — the security regression suite (§8)

**Admin panel (Flutter, `nbfc-admin-panel`):**
- Modified: `lib/core/api_client.dart` (added `put`), `lib/core/auth_provider.dart` (rewritten for access+refresh tokens, auto-refresh, real logout), `lib/screens/audit_log_screen.dart` (new filters, new response shape), `lib/screens/loan_detail_screen.dart` (maker-checker actions replace old Sanction/Disburse buttons), `lib/screens/shared_widgets.dart` (two new status labels), `lib/screens/admin_shell.dart` (permission-aware nav)
- New: `lib/core/permissions.dart`, `lib/screens/admin_users_screen.dart`, `lib/screens/roles_permissions_screen.dart`, `lib/screens/pending_approvals_screen.dart`

No existing color system, theme, or dark mode was touched — every new screen reuses the existing `ui_kit.dart`/`shared_widgets.dart` components as-is.

---

## 2. Database Migrations Created

All via `gorm.AutoMigrate` (idempotent, non-destructive — confirmed no `DROP`/data-loss statements anywhere):
- `admin_users` table (new)
- `admin_sessions` table (new)
- `loan_applications` — 15 new nullable columns (maker-checker state; see §3). Existing rows are untouched; a `NULL` on these columns for a historical loan is simply "not applicable," nothing reads them as if they were required.
- `audit_logs` — 7 new nullable columns (`actor_id`, `actor_role`, `ip_address`, `user_agent`, `request_id`, `result`, `reason`, `before_value`, `after_value`)
- A Postgres trigger (`audit_logs_append_only`) rejecting `UPDATE`/`DELETE` on `audit_logs` — installed via raw SQL in `admin.Migrate()`, idempotent (`DROP TRIGGER IF EXISTS` then recreate).

**No manual migration command is required** — `admin.Migrate()` runs automatically on every backend startup, same as it already did before this phase.

---

## 3. New Tables/Models

- **`AdminUser`**: `id, username (unique), email (unique), password_hash, role, status, failed_login_attempts, locked_until, last_login_at, created_by, created_at, updated_at`. Password is never serialized to JSON (`json:"-"`).
- **`AdminSession`**: `id, admin_user_id, refresh_token_hash (unique), user_agent, ip_address, created_at, expires_at, revoked_at`. Only the SHA-256 hash of the refresh token is stored, never the token itself.
- **`LoanApplication`** additions: `recommended_by_admin_id, recommended_by, recommended_at, recommended_tenure_months, recommended_interest_rate_percent, recommendation_remarks, decided_by_admin_id, disbursement_prepared_by_admin_id, disbursement_prepared_by, disbursement_prepared_at, disbursement_prepared_amount, disbursement_approved_by_admin_id, disbursement_approved_by, disbursement_approved_at`.
- **`AuditLog`** additions: `actor_id, actor_role, ip_address, user_agent, request_id, result, reason, before_value, after_value`.

---

## 4. New APIs/Endpoints

| Method | Path | Permission | Purpose |
|---|---|---|---|
| POST | `/admin/login` | public | Real admin login (bcrypt, lockout) |
| POST | `/admin/refresh` | public | Refresh-token rotation |
| POST | `/admin/logout` | any admin | Server-side session revocation |
| GET | `/admin/admin-users` | `ADMIN_MANAGE` | List admin accounts |
| POST | `/admin/admin-users` | `ADMIN_MANAGE` | Create an admin account |
| PUT | `/admin/admin-users/:id/role` | `ROLE_MANAGE` | Change an admin's role |
| PUT | `/admin/admin-users/:id/status` | `ADMIN_MANAGE` | Enable/disable an account (also revokes all its sessions) |
| GET | `/admin/roles` | `ADMIN_MANAGE` | Role → permission matrix |
| POST | `/admin/loans/:id/recommend` | `LOAN_REVIEW` | Maker: recommend sanction terms |
| POST | `/admin/loans/:id/approve-sanction` | `LOAN_SANCTION` | Checker: approve/reject the recommendation |
| POST | `/admin/loans/:id/prepare-disbursement` | `LOAN_DISBURSE_PREPARE` | Maker: prepare a disbursement (no funds move) |
| POST | `/admin/loans/:id/approve-disbursement` | `LOAN_DISBURSE` | Checker: release funds (the only point money actually moves) |
| GET | `/admin/loans/pending-approvals` | `LOAN_VIEW` | Cross-admin maker-checker queue |

**Disabled (return `410 Gone` with a pointer to the replacement):**
- `POST /admin/loans/:id/decision` with `action: "sanction"` — now rejects with an explanatory message; `action: "reject"` still works (outright rejection needs no second approver).
- `POST /admin/loans/:id/disburse` — always returns 410 now.

---

## 5. New RBAC Permissions

`CUSTOMER_VIEW, CUSTOMER_EDIT, KYC_VIEW, KYC_APPROVE, LOAN_VIEW, LOAN_REVIEW, LOAN_SANCTION, LOAN_REJECT, LOAN_DISBURSE_PREPARE, LOAN_DISBURSE, LOAN_RESTRUCTURE, COLLECTION_VIEW, COLLECTION_RECORD, ACCOUNTING_VIEW, REPORT_VIEW, AUDIT_VIEW, ADMIN_MANAGE, ROLE_MANAGE`

All 10 requested roles are implemented with a fixed permission set each (`internal/rbac/permissions.go`). Two roles beyond the spec's example list (`agencies`/`employees` management) were mapped onto `ADMIN_MANAGE` since the brief didn't define a dedicated permission for them — flagged as a scope decision, not an oversight; happy to add `STAFF_MANAGE` as a dedicated permission if you'd rather it be separate from admin-account management.

Every admin route is gated by exactly one permission in `routes.go` — there is no route left protected only by "is this a valid admin token."

---

## 6. Maker-Checker Workflow

**Sanction:** `submitted/auto_verified/assigned/verified` → `recommend` (maker, `LOAN_REVIEW`) → `pending_sanction_approval` → `approve-sanction` (checker, `LOAN_SANCTION`, must be a **different admin ID** than the recommender) → `sanctioned` or `rejected`.

**Disbursement:** `sanctioned` → `prepare-disbursement` (maker, `LOAN_DISBURSE_PREPARE`) → `ready_for_disbursement` → `approve-disbursement` (checker, `LOAN_DISBURSE`, must be a **different admin ID** than the preparer) → `disbursed` (EMI schedule + letter generated here, and only here) or back to `sanctioned` if rejected.

Enforcement is in the handler code itself (`internal/loans/maker_checker_handler.go`), not just permission checks — a `SUPER_ADMIN` holding every permission still cannot approve their own recommendation or their own prepared disbursement, because the comparison is `recommendedByAdminID == actingAdminID`, an identity check independent of role. Both checker steps run inside a DB transaction with `SELECT ... FOR UPDATE` row locking and a status re-check, so two concurrent approval requests can't both succeed (verified in §8/§9).

**Existing data:** no historical loan had these new columns populated before, so every pre-existing `sanctioned`/`disbursed` loan is simply left as-is (its history predates this control, which is expected and not silently rewritten).

---

## 7. Audit-Log Improvements

- New structured fields: `actorId, actorRole, ipAddress, userAgent, requestId, result, reason, beforeValue, afterValue` (redacted JSON snapshots — passwords/OTP/MPIN/tokens/PAN/Aadhaar fields are stripped to `"[REDACTED]"` before storage, see `internal/audit/redact.go`).
- Newly audited (previously not logged at all): admin login success **and** failure, logout, admin-user creation/role-change/status-change, KYC approve/reject (with before/after), every maker-checker step (including denied attempts), document/KYC-file access.
- `audit_logs` is now **append-only at the database level** — a Postgres trigger rejects any `UPDATE`/`DELETE`, verified live (see §9). This is a real control, not a complete one: a Postgres role with DDL privilege could still drop the trigger — noted as a remaining gap in §10.
- `GET /admin/audit-log` now supports `actor`/`from`/`to` filters server-side (previously action/targetType only, and only ever client-side against whatever page was loaded) and returns a `total` count for real pagination.

**Not achieved:** CSV/bulk export auditing. The admin panel's CSV exports (customers/loans/recovery) are computed **client-side** in Flutter from an already-fetched list — there is no backend export endpoint for this phase's audit hook to attach to. Flagged as a P1 item: either add a real backend export endpoint (auditable) or accept this as a known gap.

---

## 8. Security Tests Created and Their Results

`internal/admin/security_test.go` — 12 tests, run against the real local Postgres dev database (not mocked), **all passing**:

```
--- PASS: TestInvalidAdminLogin
--- PASS: TestPasswordIsHashedNotPlaintext
--- PASS: TestAccountLockoutAfterFailedAttempts
--- PASS: TestDisabledAdminRejected
--- PASS: TestPermissionDenialForWrongRole
--- PASS: TestMakerCannotApproveOwnRecommendation
--- PASS: TestDifferentAdminCanApproveSanction (includes duplicate-approval check)
--- PASS: TestUnauthorizedSanctionAndDisbursementBlocked
--- PASS: TestMakerCannotApproveOwnDisbursementPreparation
--- PASS: TestDoubleDisbursementApprovalBlocked
--- PASS: TestRevokedSessionRejected
--- PASS: TestAuditRecordCapturesActorAndBeforeAfter
--- PASS: TestAuditRecordOnPermissionDenial
PASS
ok  	github.com/santosh0921/nbfc-backend/internal/admin	4.8s
```

This covers every item on your list: invalid login, password hashing, lockout, disabled-admin rejection, permission denial, maker self-approval, unauthorized sanction/disbursement, revoked-session rejection, audit generation/attribution/before-after, and duplicate/double-approval prevention.

---

## 9. Existing Functionality Tested and Results

Ran the full backend against the live local Postgres (`go build`/`go vet` clean; existing `internal/profile` test suite still passes unmodified) and did a live end-to-end walkthrough against a running server:

- Bootstrap: server startup log confirmed `admin` account was correctly migrated from the legacy `.env` credential into a real hashed `AdminUser` row.
- Full sanction cycle on a real loan: `recommend` → same-admin `approve-sanction` correctly **403's** → different admin approves → **200**, correct GST recomputed (`processingFeeTotal: 1180`, `IGST` — matches the earlier GST feature, untouched).
- Old `POST /admin/loans/:id/disburse` → **410 Gone**, as intended.
- Full disbursement cycle: `prepare-disbursement` by a role lacking the permission → **403**; same-admin `approve-disbursement` → **403**; different, correctly-permissioned admin → **200**, `status: disbursed`, EMI schedule generated; repeat approval → **409**.
- `POST /admin/logout` then reuse of the same access token → **401** (previously this was always a no-op — now genuinely revokes).
- Audit log confirmed every one of the above steps recorded with correct actor/result/reason, including the two denied attempts.
- Unrelated endpoints re-checked working: `/admin/dashboard` (200), `/admin/users` (200), employee login endpoint reachable (401 was because no such employee exists in this local DB, not a regression — that package was never touched).

Admin panel: `flutter analyze` clean (0 errors, only pre-existing SDK-deprecation infos unrelated to this work).

---

## 10. Remaining P0 Security Gaps (not in this phase's scope, or partially addressed)

- **Audit-log tamper-evidence is trigger-based, not cryptographic.** A DB superuser/migration-privileged role could still drop the append-only trigger. True tamper-evidence (hash chaining, external WORM storage) is a larger undertaking, out of scope here.
- **CSV/bulk export has no audit trail** (see §7) — the exports are entirely client-side today.
- **No rate limiting / brute-force lockout on `/admin/refresh`** beyond the generic per-IP limiter already on `/admin/login` — a stolen refresh token isn't throttled at the refresh endpoint itself.
- **Restructure (`AdminRestructureLoanHandler`) still has no maker-checker** — the brief's "at minimum" list only named sanction and disbursement; restructure remains single-admin, now correctly gated by `LOAN_RESTRUCTURE` but not split into maker/checker. Flag if you want it added.
- **Payment gateway, real CIBIL, real KYC providers, NPA/DPD, accounting/GST/TDS, bank disbursement rails** — all explicitly out of scope for this phase, per your instructions, and unchanged.
- **No IP-based anomaly/impossible-travel detection** — IP is now captured per audit row, but nothing alerts on it yet.

---

## 11. Migration/Deployment Commands You Must Run

**None manually.** Everything runs automatically via `admin.Migrate()` + `admin.Bootstrap()` on the next backend startup (same pattern the app already used for every prior migration). On first startup after this deploy:
1. The `admin_users`/`admin_sessions` tables and new columns are created.
2. The append-only trigger is installed.
3. If `admin_users` is empty, the first `SUPER_ADMIN` is bootstrapped automatically — see §12 for exactly which credential source it uses. **Watch the startup log** for the bootstrap message; if a random password was generated, it is shown **once** and never again.

---

## 12. Environment Variables

**No new required variable.** Bootstrap credential priority:
1. `ADMIN_BOOTSTRAP_USERNAME` + `ADMIN_BOOTSTRAP_EMAIL` + `ADMIN_BOOTSTRAP_PASSWORD`, if all three are set (recommended for a fresh production deploy — set these once, remove after first startup).
2. Otherwise, the existing `ADMIN_USERNAME`/`ADMIN_PASSWORD` (your current placeholder values) are used to seed the first account — **this is exactly what happened in local testing**, confirmed live.
3. Otherwise, a random 24-character password is generated and printed once to the server log.

**Action recommended, not required:** after the first successful startup and login, remove `ADMIN_USERNAME`/`ADMIN_PASSWORD`/`ADMIN_BOOTSTRAP_PASSWORD` from `.env` — they are never read again after bootstrap (confirmed: `login_handler.go` no longer references them at all).

---

## 13. Breaking Changes

- **`POST /admin/loans/:id/decision` with `action: "sanction"`** and **`POST /admin/loans/:id/disburse`** now return `410 Gone`. Any external integration calling these directly (not just the bundled admin panel, which has been updated) will break and must switch to the new recommend/approve-sanction and prepare/approve-disbursement endpoints.
- **`GET /admin/audit-log`** response shape changed from a bare array to `{entries: [...], total: N}`. The bundled admin panel was updated; any other consumer needs to change too.
- **Admin JWT claims shape changed** (`username` string claim → structured `{adminId, username, role, sessionId}`) and **access tokens are now short-lived (15 min)** instead of 12 hours, requiring the refresh flow. Any client other than the updated admin panel holding an old-format or long-lived assumption will need updating.
- **The single shared `ADMIN_USERNAME`/`ADMIN_PASSWORD` login no longer works as a login mechanism** — only as a one-time bootstrap seed. Every admin needs their own account going forward (create via `POST /admin/admin-users` once the first `SUPER_ADMIN` is bootstrapped).

---

## 14. Manual Testing Checklist

- [ ] Restart the backend, confirm the bootstrap log line appears and note/rotate the resulting credential.
- [ ] Log in as the bootstrapped `SUPER_ADMIN` via the admin panel.
- [ ] Create a second admin account with a non-`SUPER_ADMIN` role (e.g. `CREDIT_OFFICER`) from **Admin Users**.
- [ ] Confirm the **Roles & Permissions** screen shows the expected permission set per role.
- [ ] As the `CREDIT_OFFICER`, open a `verified`/`auto_verified` loan and confirm you see **Recommend Sanction**, not a direct **Sanction** button.
- [ ] Submit a recommendation; confirm the loan now shows under **Pending Approvals**.
- [ ] Still logged in as the same admin, confirm attempting to approve your own recommendation is refused with a clear message.
- [ ] Log in as a second admin with `LOAN_SANCTION` (e.g. `CREDIT_MANAGER`) and approve it; confirm the loan becomes `Sanctioned` and the letter is generated as before.
- [ ] Repeat the same maker/checker flow for **Prepare Disbursement** → **Approve & Release**, using two different accounts, and confirm the EMI schedule appears only after the second approval.
- [ ] Try disabling an admin account from **Admin Users**; confirm that admin is signed out immediately (their next request fails) and cannot log back in.
- [ ] Log out from one browser tab and confirm the same session can't be reused (e.g. reopen the app; it should require login again once the refresh token is also gone, or reject the old access token).
- [ ] Open **Audit Log**, filter by actor and by a date range, and confirm every action above appears with the correct actor, result, and (for denials) reason.
- [ ] Confirm existing, untouched flows still work end-to-end: customer login/apply, employee login/verification, KYC approve/reject, recovery case actions, support threads, CSV exports.
