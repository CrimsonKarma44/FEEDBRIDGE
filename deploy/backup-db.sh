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

# Cleanup: drop cached detection results (link_repositories) that are older
# than 30 days and that no subscription uses. Runs after the dump, so the
# rows are still in the last 7 backups. GetFeed reads these rows for active
# subscriptions once their Redis entry expires, so referenced rows stay.
# Hard DELETE on purpose: a GORM soft delete would keep the unique url and
# block re-inserting the same URL later.
sudo -u postgres psql -d feedbridge -v ON_ERROR_STOP=1 -qAt -c "
WITH gone AS (
    DELETE FROM public.link_repositories lr
    WHERE lr.created_at < now() - interval '30 days'
      AND NOT EXISTS (
          SELECT 1 FROM public.subscriptions s
          WHERE s.url = lr.url
             OR lr.feed_links ? s.url
      )
    RETURNING 1
)
SELECT 'link_repositories cleanup: ' || count(*) || ' rows deleted' FROM gone;"
