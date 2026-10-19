#!/bin/sh
set -eu
# Runs only against the disposable verify.compose.yml database.
test "$PGDATABASE" = verification
umask 077
trap 'rm -f /tmp/verification.dump' EXIT
psql -v ON_ERROR_STOP=1 -c "CREATE TABLE IF NOT EXISTS restore_probe (id integer PRIMARY KEY, value text NOT NULL); INSERT INTO restore_probe VALUES (1, 'scratch restore verified') ON CONFLICT (id) DO NOTHING;"
pg_dump --format=custom --file=/tmp/verification.dump
createdb restored
pg_restore --exit-on-error --no-owner --dbname=restored /tmp/verification.dump
test "$(psql -d restored -Atc 'SELECT value FROM restore_probe WHERE id = 1')" = 'scratch restore verified'
source_count=$(psql -Atc 'SELECT count(*) FROM messages')
restored_count=$(psql -d restored -Atc 'SELECT count(*) FROM messages')
test "$source_count" = "$restored_count"
source_receipts=$(psql -Atc 'SELECT count(*) FROM message_receipts')
restored_receipts=$(psql -d restored -Atc 'SELECT count(*) FROM message_receipts')
test "$source_receipts" -gt 0
test "$source_receipts" = "$restored_receipts"
test "$(psql -d restored -Atc 'SELECT count(*) FROM message_receipts r LEFT JOIN messages m ON m.id=r.message_id WHERE m.id IS NULL')" = 0
echo 'Backup/restore drill passed (probe and message count match).'
