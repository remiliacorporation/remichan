#!/usr/bin/env bash
export PGPASSWORD="meguca"
BACKUP_FILE="backups/pg_backup_$(date +%Y%m%d).sql"


LATEST_FILE=$(find backups/ -name "pg_backup_*.sql" -type f -printf "%T@ %p\n" | sort -n | tail -1 | cut -d' ' -f2-)

if [[ -f $LATEST_FILE ]]; then
    pg_restore -h localhost -p 5432 -U meguca -d meguca -v -c "$LATEST_FILE"
else
    echo "No backup file found."
fi