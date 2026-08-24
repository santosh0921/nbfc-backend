#!/usr/bin/env bash
#
# Syncs the primary PostgreSQL database into a SEPARATE, standing
# PostgreSQL instance that serves as a live backup database.
#
# WHY A SEPARATE INSTANCE, NOT JUST A DUMP FILE: a dump file protects
# against data corruption/deletion but not against the primary's own
# postgres process or data directory becoming unavailable. A second,
# independently-running PostgreSQL server (its own process, its own data
# directory, its own port) survives a primary-instance failure that a
# dump sitting next to the primary's data directory would not.
#
# HONEST SCOPE LIMIT: this second instance runs on the SAME PHYSICAL
# MACHINE as the primary (Docker Desktop's engine could not be started in
# this environment — WSL2 is not functional here — so a container-based
# instance was not possible; see PHASE... reports). This protects against:
#   - primary PostgreSQL process crash or corruption
#   - primary data directory corruption
#   - accidental DROP/DELETE against the primary
# It does NOT protect against:
#   - disk failure (both instances share the same physical disk)
#   - host loss (both instances are the same host)
# True off-host protection still requires copying a dump to separate
# physical/cloud storage — see scripts/backup_postgres.sh's own header.
#
# WHAT THIS SCRIPT DOES:
#   1. Takes a fresh dump of the primary via backup_postgres.sh's mechanism.
#   2. Drops and recreates the target database on the backup instance.
#   3. Restores the fresh dump into it.
#   4. Verifies row counts match between primary and backup instance.
#   5. Writes a sync manifest (evidence, no credentials).
#
# This does NOT run automatically. Schedule it externally exactly like
# any other job in this system (see scripts/README_OPERATIONS.md) —
# never via an in-process timer.

set -Eeuo pipefail

fail() { echo "sync: ERROR: $*" >&2; exit 1; }

# --- Primary (source) ---
: "${DB_HOST:?DB_HOST is required}"
: "${DB_PORT:?DB_PORT is required}"
: "${DB_USER:?DB_USER is required}"
: "${DB_NAME:?DB_NAME is required}"
: "${DB_PASSWORD:?DB_PASSWORD is required}"

# --- Backup instance (target) ---
: "${BACKUP_DB_HOST:?BACKUP_DB_HOST is required (the standing backup PostgreSQL instance)}"
: "${BACKUP_DB_PORT:?BACKUP_DB_PORT is required}"
: "${BACKUP_DB_USER:?BACKUP_DB_USER is required}"
: "${BACKUP_DB_NAME:?BACKUP_DB_NAME is required}"
: "${BACKUP_DB_PASSWORD:?BACKUP_DB_PASSWORD is required}"

# --- Safety: the target must not be the primary ---
if [[ "$BACKUP_DB_HOST" == "$DB_HOST" && "$BACKUP_DB_PORT" == "$DB_PORT" ]]; then
  fail "REFUSING: backup target ($BACKUP_DB_HOST:$BACKUP_DB_PORT) is the same host:port as the primary ($DB_HOST:$DB_PORT)"
fi

PG_BIN="${PG_BIN:-}"
if [[ -n "$PG_BIN" ]]; then
  PG_DUMP="$PG_BIN/pg_dump"; PG_RESTORE="$PG_BIN/pg_restore"; PSQL="$PG_BIN/psql"
else
  PG_DUMP="$(command -v pg_dump || true)"; PG_RESTORE="$(command -v pg_restore || true)"; PSQL="$(command -v psql || true)"
fi
[[ -n "$PG_DUMP" && -n "$PG_RESTORE" && -n "$PSQL" ]] || fail "pg_dump/pg_restore/psql not found. Set PG_BIN."

# --- Concurrency lock (Phase 18 Step 10 finding) ---
#
# Two overlapping runs would both DROP/CREATE the target database and
# race their pg_restore against each other — the loser's restore lands
# in a database the winner is simultaneously dropping, which can leave
# the backup instance in a state where the row-count check still passes
# (because it just compares final counts) while the actual data is a
# mix of two restore attempts. `mkdir` is used as the lock primitive
# because it is atomic on every filesystem this script runs on
# (including the Windows filesystem under Git Bash), unlike a plain
# file existence check, and `flock` is not available in this
# environment (verified: absent from PATH).
#
# A stale lock (the previous holder crashed without cleaning up) is
# detected by PID liveness, not just age — an age-only check could
# still steal the lock from a slow-but-alive sync.
LOCK_DIR="${SYNC_LOCK_DIR:-${TMPDIR:-/tmp}/nbfc_backup_sync.lock}"
acquire_lock() {
  if mkdir "$LOCK_DIR" 2>/dev/null; then
    echo $$ > "$LOCK_DIR/pid"
    return 0
  fi
  local holder_pid
  holder_pid="$(cat "$LOCK_DIR/pid" 2>/dev/null || echo "")"
  if [[ -n "$holder_pid" ]] && kill -0 "$holder_pid" 2>/dev/null; then
    fail "REFUSING: another sync (PID $holder_pid) is already running (lock: $LOCK_DIR). This is expected under a naive overlapping schedule, not a bug — the previous run has not finished."
  fi
  # Holder PID is gone: a prior run crashed without cleanup. Safe to
  # reclaim, since nothing alive holds it.
  echo "sync: reclaiming stale lock left by dead PID $holder_pid"
  rm -rf "$LOCK_DIR"
  mkdir "$LOCK_DIR" || fail "could not acquire sync lock at $LOCK_DIR"
  echo $$ > "$LOCK_DIR/pid"
}
acquire_lock

WORKDIR="$(mktemp -d)"
trap 'rm -rf "$WORKDIR" "$LOCK_DIR"' EXIT
DUMP_FILE="$WORKDIR/sync.dump"

echo "sync: dumping primary $DB_HOST:$DB_PORT/$DB_NAME"
START=$(date -u +%s)

PGPASSWORD="$DB_PASSWORD" "$PG_DUMP" \
  --host="$DB_HOST" --port="$DB_PORT" --username="$DB_USER" --dbname="$DB_NAME" \
  --format=custom --compress=6 --no-owner --no-privileges --file="$DUMP_FILE"

[[ -s "$DUMP_FILE" ]] || fail "primary dump is empty — aborting sync, backup instance left untouched"
DUMP_SIZE=$(wc -c < "$DUMP_FILE" | tr -d ' ')
echo "sync: primary dump complete — $DUMP_SIZE bytes"

echo "sync: recreating $BACKUP_DB_NAME on backup instance $BACKUP_DB_HOST:$BACKUP_DB_PORT"
PGPASSWORD="$BACKUP_DB_PASSWORD" "$PSQL" -h "$BACKUP_DB_HOST" -p "$BACKUP_DB_PORT" -U "$BACKUP_DB_USER" -d postgres -v ON_ERROR_STOP=1 \
  -c "DROP DATABASE IF EXISTS \"$BACKUP_DB_NAME\";" \
  -c "CREATE DATABASE \"$BACKUP_DB_NAME\";" >/dev/null

echo "sync: restoring into backup instance"
set +e
PGPASSWORD="$BACKUP_DB_PASSWORD" "$PG_RESTORE" \
  --host="$BACKUP_DB_HOST" --port="$BACKUP_DB_PORT" --username="$BACKUP_DB_USER" \
  --dbname="$BACKUP_DB_NAME" --no-owner --no-privileges --jobs=4 "$DUMP_FILE" 2>"$WORKDIR/restore_errors.log"
RESTORE_RC=$?
set -e
echo "sync: pg_restore exit=$RESTORE_RC"

END=$(date -u +%s)
DURATION=$(( END - START ))

# --- Verification: row counts must match between primary and backup ---
q_primary() { PGPASSWORD="$DB_PASSWORD" "$PSQL" -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -tAc "$1"; }
q_backup()  { PGPASSWORD="$BACKUP_DB_PASSWORD" "$PSQL" -h "$BACKUP_DB_HOST" -p "$BACKUP_DB_PORT" -U "$BACKUP_DB_USER" -d "$BACKUP_DB_NAME" -tAc "$1"; }

echo
echo "=== ROW COUNT COMPARISON ==="
MISMATCH=0
for t in users loan_applications disbursements payments emi_installments \
         journal_entries journal_entry_lines audit_logs admin_users; do
  SRC=$(q_primary "SELECT COUNT(*) FROM $t;" 2>/dev/null || echo "n/a")
  DST=$(q_backup  "SELECT COUNT(*) FROM $t;" 2>/dev/null || echo "n/a")
  M="OK"; [[ "$SRC" != "$DST" ]] && { M="DIFF"; MISMATCH=$((MISMATCH+1)); }
  printf "%-24s source=%-10s backup=%-10s %s\n" "$t" "$SRC" "$DST" "$M"
done

AUDIT_TRIGGERS=$(q_backup "SELECT COUNT(*) FROM information_schema.triggers WHERE event_object_table='audit_logs';")
echo "audit_logs triggers on backup instance: $AUDIT_TRIGGERS"

MANIFEST_DIR="${SYNC_MANIFEST_DIR:-.}"
STAMP="$(date -u +%Y%m%dT%H%M%SZ)"
cat > "$MANIFEST_DIR/sync_${STAMP}.manifest.json" <<JSON
{
  "syncId": "sync_${STAMP}",
  "primaryHost": "${DB_HOST}",
  "primaryPort": "${DB_PORT}",
  "backupHost": "${BACKUP_DB_HOST}",
  "backupPort": "${BACKUP_DB_PORT}",
  "dumpSizeBytes": ${DUMP_SIZE},
  "durationSeconds": ${DURATION},
  "restoreExitCode": ${RESTORE_RC},
  "rowCountMismatches": ${MISMATCH},
  "auditTriggersOnBackup": ${AUDIT_TRIGGERS},
  "status": "$( [[ $RESTORE_RC -eq 0 && $MISMATCH -eq 0 ]] && echo SUCCESS || echo FAILED )"
}
JSON

echo
if [[ $RESTORE_RC -eq 0 && $MISMATCH -eq 0 ]]; then
  echo "sync: SUCCESS — backup database is in sync with primary (duration ${DURATION}s)"
else
  echo "sync: FAILED — restore_rc=$RESTORE_RC mismatches=$MISMATCH — see $WORKDIR/restore_errors.log"
  cat "$WORKDIR/restore_errors.log" >&2 || true
  exit 1
fi
