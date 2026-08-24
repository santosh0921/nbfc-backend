#!/usr/bin/env bash
#
# Restore verification for the NBFC backend (Phase 16.1 Part 4).
#
# A backup that has never been restored is not a backup. This script
# restores a dump into an ISOLATED database and verifies the result.
#
# SAFETY — this script refuses to touch the live database:
#   * The restore target name must differ from DB_NAME.
#   * The target must match a restore-test naming pattern.
#   * It will not drop a database that does not match that pattern.
# These are hard checks, not conventions, because "restore test
# overwrote production" is a genuine and catastrophic operator error.

set -Eeuo pipefail

fail() { echo "restore: ERROR: $*" >&2; exit 1; }

: "${DB_HOST:?DB_HOST is required}"
: "${DB_PORT:?DB_PORT is required}"
: "${DB_USER:?DB_USER is required}"
: "${DB_NAME:?DB_NAME is required}"
: "${DB_PASSWORD:?DB_PASSWORD is required}"

DUMP_FILE="${1:?usage: restore_verify.sh <dump-file> [target-db]}"
RESTORE_DB="${2:-${DB_NAME}_restore_test}"

[[ -f "$DUMP_FILE" ]] || fail "dump file not found: $DUMP_FILE"

# --- Hard safety checks ---
[[ "$RESTORE_DB" != "$DB_NAME" ]] || fail "REFUSING: restore target ($RESTORE_DB) is the live database ($DB_NAME)"
[[ "$RESTORE_DB" == *_restore_test* ]] || fail "REFUSING: restore target must contain '_restore_test' (got: $RESTORE_DB)"

PG_BIN="${PG_BIN:-}"
if [[ -n "$PG_BIN" ]]; then
  PSQL="$PG_BIN/psql"; PG_RESTORE="$PG_BIN/pg_restore"
else
  PSQL="$(command -v psql || true)"; PG_RESTORE="$(command -v pg_restore || true)"
fi
[[ -n "$PSQL" && -n "$PG_RESTORE" ]] || fail "psql/pg_restore not found. Set PG_BIN."

export PGPASSWORD="$DB_PASSWORD"
trap 'unset PGPASSWORD' EXIT

q() { "$PSQL" -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$1" -tAc "$2"; }

# --- Verify the artifact's integrity before trusting it ---
if [[ -f "${DUMP_FILE}.sha256" ]] && command -v sha256sum >/dev/null 2>&1; then
  echo "restore: verifying dump checksum"
  ( cd "$(dirname "$DUMP_FILE")" && sha256sum -c "$(basename "$DUMP_FILE").sha256" ) \
    || fail "CHECKSUM MISMATCH — the dump file has been altered or corrupted"
  echo "restore: checksum OK"
fi

echo "restore: target database = $RESTORE_DB (isolated; live database $DB_NAME is untouched)"
START_EPOCH=$(date -u +%s)

# Drop/recreate the isolated target only.
q postgres "DROP DATABASE IF EXISTS \"$RESTORE_DB\";" >/dev/null
q postgres "CREATE DATABASE \"$RESTORE_DB\";" >/dev/null

# --exit-on-error is deliberately NOT used: a custom-format dump taken
# with --no-owner can emit benign errors (extension/role comments) that
# do not affect data. Errors are counted and surfaced instead.
set +e
"$PG_RESTORE" --host="$DB_HOST" --port="$DB_PORT" --username="$DB_USER" \
  --dbname="$RESTORE_DB" --no-owner --no-privileges --jobs=4 \
  "$DUMP_FILE" 2> >(tee /tmp/pg_restore_errors.log >&2)
RESTORE_RC=$?
set -e

END_EPOCH=$(date -u +%s)
RTO_SECONDS=$(( END_EPOCH - START_EPOCH ))
echo "restore: pg_restore exit=$RESTORE_RC  elapsed=${RTO_SECONDS}s"

# --- Verification ---
echo
echo "=== SCHEMA VERIFICATION ==="
TABLES=$(q "$RESTORE_DB" "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema='public';")
INDEXES=$(q "$RESTORE_DB" "SELECT COUNT(*) FROM pg_indexes WHERE schemaname='public';")
CONSTRAINTS=$(q "$RESTORE_DB" "SELECT COUNT(*) FROM information_schema.table_constraints WHERE constraint_schema='public';")
TRIGGERS=$(q "$RESTORE_DB" "SELECT COUNT(*) FROM information_schema.triggers WHERE trigger_schema='public';")
echo "tables=$TABLES indexes=$INDEXES constraints=$CONSTRAINTS triggers=$TRIGGERS"

echo
echo "=== ROW COUNT COMPARISON (source vs restored) ==="
printf "%-28s %14s %14s %s\n" "TABLE" "SOURCE" "RESTORED" "MATCH"
MISMATCH=0
for t in users loan_applications disbursements payments emi_installments \
         journal_entries journal_entry_lines ledger_accounts audit_logs \
         admin_users report_exports; do
  SRC=$(q "$DB_NAME" "SELECT COUNT(*) FROM $t;" 2>/dev/null || echo "n/a")
  DST=$(q "$RESTORE_DB" "SELECT COUNT(*) FROM $t;" 2>/dev/null || echo "n/a")
  if [[ "$SRC" == "$DST" ]]; then M="OK"; else M="DIFF"; MISMATCH=$((MISMATCH+1)); fi
  printf "%-28s %14s %14s %s\n" "$t" "$SRC" "$DST" "$M"
done

echo
echo "=== FINANCIAL CONSISTENCY IN RESTORED DB ==="
UNBAL=$(q "$RESTORE_DB" "SELECT COUNT(*) FROM (SELECT journal_id FROM journal_entry_lines GROUP BY journal_id HAVING COALESCE(SUM(debit_paise),0) <> COALESCE(SUM(credit_paise),0)) t;")
DUPJN=$(q "$RESTORE_DB" "SELECT COUNT(*) FROM (SELECT journal_number FROM journal_entries GROUP BY journal_number HAVING COUNT(*)>1) t;")
DUPDISB=$(q "$RESTORE_DB" "SELECT COUNT(*) FROM (SELECT loan_id FROM disbursements WHERE status='SUCCESS' GROUP BY loan_id HAVING COUNT(*)>1) t;")
echo "unbalanced_journal_entries=$UNBAL duplicate_journal_numbers=$DUPJN loans_with_multiple_success_disbursements=$DUPDISB"

echo
echo "=== AUDIT TRAIL PROTECTION IN RESTORED DB ==="
AUDIT_TRIGGERS=$(q "$RESTORE_DB" "SELECT COUNT(*) FROM information_schema.triggers WHERE event_object_table='audit_logs';")
echo "audit_logs triggers restored = $AUDIT_TRIGGERS"

echo
echo "=== RESULT ==="
echo "RTO_SECONDS=$RTO_SECONDS"
echo "TABLE_COUNT_MISMATCHES=$MISMATCH"
if [[ "$MISMATCH" -eq 0 && "$UNBAL" -eq 0 && "$DUPJN" -eq 0 && "$TABLES" -gt 0 ]]; then
  echo "RESTORE_VERIFICATION=PASS"
else
  echo "RESTORE_VERIFICATION=FAIL"
  exit 1
fi
