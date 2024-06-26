#!/usr/bin/env bash
export PGPASSWORD="meguca"
BACKUP_FILE="/meguca/backups/pg_backup_$(date +%Y%m%d%H%M).sql"

pg_dump -h localhost -U meguca -F c -v -f "$BACKUP_FILE" meguca
