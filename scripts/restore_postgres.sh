#!/usr/bin/env bash
export PGPASSWORD="meguca"
BACKUP_FILE="backups/pg_backup_$(date +%Y%m%d).sql"

 pg_restore -h localhost -p 5432 -U meguca -d meguca -v -c $1