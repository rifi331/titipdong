#!/usr/bin/env bash
# TitipDong — backup the Postgres database to a timestamped file.
#
# Use this BEFORE running risky SQL, before deploying new migrations, or just
# as a regular backup. Since your dev and production share the same DB, this is
# your safety net.
#
# Usage:
#   bash backup-db.sh                           # uses defaults below
#   DB_HOST=192.168.x.y DB_PASSWORD=xxx bash backup-db.sh
#
# Restore from a backup:
#   gunzip -c titipdong-YYYYMMDD-HHMMSS.sql.gz | \
#     PGPASSWORD='<pw>' psql -h <host> -U postgres -d titipdong

set -euo pipefail

DB_HOST="${DB_HOST:-__TRUENAS_IP__}"   # set DB_HOST env var, e.g. 192.168.x.y
DB_PORT="${DB_PORT:-5432}"
DB_USER="${DB_USER:-postgres}"
DB_NAME="${DB_NAME:-titipdong}"
OUT_DIR="${OUT_DIR:-./backups}"

# Read password from env, or fail loudly.
if [ -z "${DB_PASSWORD:-}" ]; then
    echo "ERROR: set DB_PASSWORD env var first." >&2
    echo "  Example: DB_PASSWORD='xxx' bash $0" >&2
    exit 1
fi

mkdir -p "$OUT_DIR"
TIMESTAMP=$(date +"%Y%m%d-%H%M%S")
OUT_FILE="$OUT_DIR/titipdong-$TIMESTAMP.sql.gz"

echo "Backing up $DB_NAME at $DB_HOST:$DB_PORT ..."
PGPASSWORD="$DB_PASSWORD" pg_dump \
    -h "$DB_HOST" \
    -p "$DB_PORT" \
    -U "$DB_USER" \
    -d "$DB_NAME" \
    --no-owner \
    --no-privileges \
    | gzip > "$OUT_FILE"

SIZE=$(du -h "$OUT_FILE" | cut -f1)
echo ""
echo "✓ Backup saved: $OUT_FILE ($SIZE)"
echo ""
echo "To restore:"
echo "  gunzip -c $OUT_FILE | PGPASSWORD='xxx' psql -h $DB_HOST -U $DB_USER -d $DB_NAME"
echo ""
echo "To list old backups:"
echo "  ls -lh $OUT_DIR/"
