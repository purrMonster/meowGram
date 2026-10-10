#!/bin/sh
set -eu

umask 077
retention_days="${BACKUP_RETENTION_DAYS:-14}"
case "$retention_days" in
	''|*[!0-9]*) echo "BACKUP_RETENTION_DAYS must be a positive integer" >&2; exit 2 ;;
esac
if [ "$retention_days" -lt 1 ]; then
	echo "BACKUP_RETENTION_DAYS must be a positive integer" >&2
	exit 2
fi

mkdir -p /backups

while :; do
	timestamp="$(date -u +%Y%m%dT%H%M%SZ)"
	temporary="/backups/.meowgram-${timestamp}.dump.tmp"
	backup="/backups/meowgram-${timestamp}.dump"
	if pg_dump --no-password --format=custom --file="$temporary"; then
		chmod 600 "$temporary"
		mv "$temporary" "$backup"
		find /backups -type f -name 'meowgram-*.dump' -mtime "+${retention_days}" -delete
		echo "Database backup completed: $(basename "$backup")"
	else
		rm -f "$temporary"
		echo "Database backup failed; retrying in five minutes" >&2
		sleep 300
		continue
	fi
	sleep 86400
done
