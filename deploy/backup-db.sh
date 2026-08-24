#!/usr/bin/env bash
# Daily backup of FEEDBRIDGE data tables. Keeps the last 7 dumps.
# Install as root cron:  0 4 * * * /opt/feedbridge/backup-db.sh >> /var/log/feedbridge-backup.log 2>&1
set -euo pipefail

BACKUP_DIR=/var/backups/feedbridge
RETAIN=7
STAMP="$(date +%F_%H%M)"

install -d -m 0700 "$BACKUP_DIR"

sudo -u postgres pg_dump -d feedbridge \
    -t subscriptions -t link_repositories \
    | gzip > "${BACKUP_DIR}/feedbridge_${STAMP}.sql.gz"

# Retention
ls -1t "${BACKUP_DIR}"/feedbridge_*.sql.gz | tail -n +$((RETAIN + 1)) | xargs -r rm --

echo "backup written: ${BACKUP_DIR}/feedbridge_${STAMP}.sql.gz"
