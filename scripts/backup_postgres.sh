#!/usr/bin/env bash
#
# PostgreSQL logical backup for the NBFC backend (Phase 16.1 Part 2).
#
# WHY pg_dump AND NOT A PROVIDER SNAPSHOT: the deployment this repository
# actually targets uses a self-managed PostgreSQL instance reached over
# host/port credentials (DB_HOST/DB_PORT/DB_USER/DB_PASSWORD in the
# environment). There is no cloud provider, no RDS/Aurora, and no
# managed-backup facility in this architecture — verified during Phase
# 16.1 discovery. A logical dump is therefore the correct mechanism.
#
# If this system is later moved to managed PostgreSQL (RDS, Aurora, Cloud
# SQL, Neon, Supabase, ...), the provider's own automated
# snapshot + point-in-time-recovery facility becomes the primary
# mechanism and this script becomes a secondary, portable export. Do NOT
# treat this script as a substitute for provider backups in that case.
#
# WHAT THIS SCRIPT DOES NOT DO — deliberately, because these depend on
# infrastructure that does not exist in this repository:
#   * It does not upload to off-host storage. A backup on the same disk
#     as the database is not a backup. See BACKUP_DESTINATION below.
#   * It does not schedule itself. Scheduling is external (see
#     scripts/README_OPERATIONS.md).
#   * It does not manage cloud KMS encryption keys.
#
# SECRETS: credentials are read from the environment only. Nothing is
# hard-coded, nothing is echoed, and PGPASSWORD is exported only for the
# lifetime of this process.

set -Eeuo pipefail

fail() { echo "backup: ERROR: $*" >&2; exit 1; }

: "${DB_HOST:?DB_HOST is required}"
: "${DB_PORT:?DB_PORT is required}"
: "${DB_USER:?DB_USER is required}"
: "${DB_NAME:?DB_NAME is required}"
: "${DB_PASSWORD:?DB_PASSWORD is required}"

# Directory backups are written to. MUST be on storage separate from the
# database volume for this to be a real backup.
BACKUP_DESTINATION="${BACKUP_DESTINATION:-./backups}"
RETENTION_DAYS="${BACKUP_RETENTION_DAYS:-30}"

# pg_dump must match the server's major version. Allow an explicit
# override for hosts where it is not on PATH (e.g. a Windows install).
PG_BIN="${PG_BIN:-}"
if [[ -n "$PG_BIN" ]]; then
  PG_DUMP="$PG_BIN/pg_dump"
else
  PG_DUMP="$(command -v pg_dump || true)"
fi
[[ -x "$PG_DUMP" || -f "$PG_DUMP" ]] || fail "pg_dump not found. Set PG_BIN to the PostgreSQL bin directory."

mkdir -p "$BACKUP_DESTINATION"

STAMP="$(date -u +%Y%m%dT%H%M%SZ)"
BASENAME="${DB_NAME}_${STAMP}"
DUMP_FILE="${BACKUP_DESTINATION}/${BASENAME}.dump"

export PGPASSWORD="$DB_PASSWORD"
trap 'unset PGPASSWORD' EXIT

echo "backup: starting  db=${DB_NAME} host=${DB_HOST}:${DB_PORT} format=custom"
START_EPOCH=$(date -u +%s)

# --format=custom: compressed, and restorable selectively with
# pg_restore (a plain SQL dump cannot be restored table-by-table).
# --no-owner/--no-privileges: the restore target may use a different
# role, and role mismatches are a common cause of restore failure.
"$PG_DUMP" \
  --host="$DB_HOST" --port="$DB_PORT" --username="$DB_USER" \
  --dbname="$DB_NAME" \
  --format=custom --compress=6 \
  --no-owner --no-privileges \
  --file="$DUMP_FILE"

END_EPOCH=$(date -u +%s)
DURATION=$(( END_EPOCH - START_EPOCH ))

[[ -s "$DUMP_FILE" ]] || fail "dump file is empty — backup FAILED"

SIZE_BYTES=$(wc -c < "$DUMP_FILE" | tr -d ' ')

# Checksum so a later restore can prove it used an unmodified artifact.
if command -v sha256sum >/dev/null 2>&1; then
  CHECKSUM="$(sha256sum "$DUMP_FILE" | awk '{print $1}')"
else
  CHECKSUM="$(certutil -hashfile "$(cygpath -w "$DUMP_FILE" 2>/dev/null || echo "$DUMP_FILE")" SHA256 2>/dev/null | sed -n 2p | tr -d ' \r')"
fi
echo "$CHECKSUM  ${BASENAME}.dump" > "${DUMP_FILE}.sha256"

# Manifest — the evidence record. Contains no credentials.
cat > "${BACKUP_DESTINATION}/${BASENAME}.manifest.json" <<JSON
{
  "backupId": "${BASENAME}",
  "database": "${DB_NAME}",
  "host": "${DB_HOST}",
  "port": "${DB_PORT}",
  "startedAtUtc": "${STAMP}",
  "durationSeconds": ${DURATION},
  "mechanism": "pg_dump --format=custom --compress=6",
  "sizeBytes": ${SIZE_BYTES},
  "sha256": "${CHECKSUM}",
  "retentionDays": ${RETENTION_DAYS},
  "status": "SUCCESS"
}
JSON

echo "backup: SUCCESS  id=${BASENAME}  size=${SIZE_BYTES} bytes  duration=${DURATION}s"
echo "backup: sha256=${CHECKSUM}"

# Retention. Applies ONLY to files this script created (name-matched),
# so it can never delete an unrelated file that happens to share the
# directory.
if [[ "$RETENTION_DAYS" -gt 0 ]]; then
  find "$BACKUP_DESTINATION" -maxdepth 1 -type f \
    -name "${DB_NAME}_*.dump*" -mtime "+${RETENTION_DAYS}" -print -delete \
    | sed 's/^/backup: retention removed /' || true
  find "$BACKUP_DESTINATION" -maxdepth 1 -type f \
    -name "${DB_NAME}_*.manifest.json" -mtime "+${RETENTION_DAYS}" -delete || true
fi

echo "backup: complete"
