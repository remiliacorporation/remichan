#!/usr/bin/env bash
# Start Postgres inside Docker and execute the arguments

echo "Starting Backup Cronjob..."
crontab -l | { cat; echo "0 * * * * bash /meguca/scripts/backup_postgres.sh"; } | crontab -
cron

service postgresql start
echo "Waiting on PostgreSQL server..."
until pg_isready > /dev/null
do
    sleep 1
done

eval $@
