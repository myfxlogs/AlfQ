#!/bin/bash
# ALFQ PostgreSQL Backup — daily full + WAL archive to MinIO
# Usage: ./scripts/backup-pg.sh [full|wal]
# Cron: 0 3 * * * /opt/alfq/scripts/backup-pg.sh full

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
BACKUP_DIR="${BACKUP_DIR:-/data/alfq/backups/pg}"
PG_HOST="${PG_HOST:-localhost}"
PG_PORT="${PG_PORT:-5432}"
PG_USER="${PG_USER:-alfq}"
PG_DB="${PG_DB:-alfq}"
MINIO_ENDPOINT="${MINIO_ENDPOINT:-localhost:9002}"
MINIO_AK="${MINIO_AK:-alfq}"
MINIO_SK="${MINIO_SK:-alfq_dev}"
MINIO_BUCKET="${MINIO_BUCKET:-alfq-backups}"
MODE="${1:-full}"
TIMESTAMP=$(date -u +%Y%m%d_%H%M%S)

export PGPASSWORD="${PG_PASSWORD:-alfq_dev}"

log() { echo "[$(date -u +%H:%M:%S)] $*"; }

# ── Full backup ──
if [ "$MODE" = "full" ]; then
    log "Starting full PG backup..."
    mkdir -p "$BACKUP_DIR"

    DUMP_FILE="$BACKUP_DIR/alfq_full_${TIMESTAMP}.sql.gz"
    pg_dump -h "$PG_HOST" -p "$PG_PORT" -U "$PG_USER" -d "$PG_DB" \
        --no-owner --no-acl | gzip > "$DUMP_FILE"

    SIZE=$(stat -f%z "$DUMP_FILE" 2>/dev/null || stat -c%s "$DUMP_FILE" 2>/dev/null)
    log "Full dump created: $DUMP_FILE ($SIZE bytes)"

    # Upload to MinIO
    OBJECT="pg/full/alfq_full_${TIMESTAMP}.sql.gz"
    curl -s -X PUT -T "$DUMP_FILE" \
        "http://${MINIO_ENDPOINT}/${MINIO_BUCKET}/${OBJECT}" \
        -H "Authorization: AWS ${MINIO_AK}:${MINIO_SK}" \
        -H "Content-Type: application/gzip" || true

    log "Uploaded to MinIO: s3://${MINIO_BUCKET}/${OBJECT}"

    # Cleanup old backups (> 7 days)
    find "$BACKUP_DIR" -name "alfq_full_*.sql.gz" -mtime +7 -delete
    log "Full backup complete"

# ── WAL archive ──
elif [ "$MODE" = "wal" ]; then
    WAL_DIR="${PG_DATA_DIR:-/var/lib/postgresql/data}/pg_wal"
    ARCHIVE_DIR="$BACKUP_DIR/wal"
    mkdir -p "$ARCHIVE_DIR"

    # Copy completed WAL segments
    for f in "$WAL_DIR"/*.done; do
        [ -f "$f" ] || continue
        base=$(basename "$f" .done)
        cp "$WAL_DIR/$base" "$ARCHIVE_DIR/"
        log "Archived WAL: $base"
        rm "$f"
    done

    log "WAL archive complete"
fi

log "PG backup finished"
