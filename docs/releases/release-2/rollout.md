# R2 owner rollout and recovery procedure

These are owner-run commands for an authorized maintenance window on cellar,
not evidence of deployment. Run from the checked-out release repository in a
POSIX shell. No live step has been run by the agent. Keep the same Compose project,
database volume and gitignored env files as the existing installation. Resolve
the owner checks before starting. The candidate is 2.0.0+4, tag **v2.0.0**;
the image version is **2.0.0**, and must actually have been published first.

## 1. Backup, off-node copy, and verify the actual dump

Schedule downtime and stop writers before counting and dumping; this makes the
count comparison meaningful. The existing db_backup.sh loops daily and atomically
renames a completed dump. Run a one-off instance and stop it after its first
completion (the regular backup service can remain configured).

```sh
set -eu
dc() { docker compose --env-file .env --env-file deploy/.env -f deploy/docker-compose.yml "$@"; }
# MEOWGRAM_VERSION must be set for Compose interpolation even before pulling.
export MEOWGRAM_VERSION=2.0.0
dc stop server
before_count=$(dc exec -T postgres sh -c 'psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -Atc "SELECT count(*) FROM messages"')
docker inspect meowgram-pre-r2-backup >/dev/null 2>&1 && { echo 'Backup container already exists; inspect it before proceeding'; exit 1; }
dc run -d --no-deps --name meowgram-pre-r2-backup db-backup
# Wait until `docker logs meowgram-pre-r2-backup` reports Database backup completed.
while ! docker logs meowgram-pre-r2-backup 2>&1 | grep -q 'Database backup completed:'; do sleep 2; done
dump=$(docker logs meowgram-pre-r2-backup 2>&1 | sed -n 's/^Database backup completed: //p' | head -n 1)
test -n "$dump" && test -s "deploy/backups/$dump"
docker stop meowgram-pre-r2-backup
docker rm meowgram-pre-r2-backup
# Set this to an owner-approved SSH destination, never a literal credential.
: "${OFFNODE_BACKUP_DEST:?set user@backup-host:/private/path}"
scp "deploy/backups/$dump" "$OFFNODE_BACKUP_DEST/"
sha256sum "deploy/backups/$dump"
# Compare this checksum with the off-node copy before proceeding.
```

If backup reports failure, stop and investigate; do not proceed to the upgrade.
Keep the completed filename, count and checksum in the private deployment record.
The generic verify_restore.sh only proves the synthetic fixture; it does NOT
validate this production dump. Restore the exact completed dump independently:

```sh
scratch="meowgram-dump-check-$(date -u +%Y%m%dT%H%M%SZ)"
docker run -d --name "$scratch" --network none \
  --tmpfs /var/lib/postgresql/data \
  -e POSTGRES_HOST_AUTH_METHOD=trust -e POSTGRES_DB=restore_check postgres:16-alpine
until docker exec "$scratch" pg_isready -U postgres -d restore_check >/dev/null 2>&1; do sleep 1; done
docker cp "deploy/backups/$dump" "$scratch:/tmp/upgrade.dump"
docker exec "$scratch" pg_restore -U postgres -d restore_check \
  --no-owner --no-acl --exit-on-error /tmp/upgrade.dump
restored_count=$(docker exec "$scratch" psql -U postgres -d restore_check -Atc 'SELECT count(*) FROM messages')
test "$before_count" = "$restored_count"
printf 'Verified message count: %s\n' "$restored_count"
docker rm -f "$scratch"
```

This named throwaway container has no published ports, network or persistent
volume. Remove only it. If a command fails, retain it for diagnosis and keep
the server stopped until recovery is understood. Do not delete the backup.

## 2. Pin the actual rollback images

```sh
server_id=$(docker inspect --format '{{.Image}}' "$(dc ps -aq server)")
web_id=$(docker inspect --format '{{.Image}}' "$(dc ps -aq web)")
docker image tag "$server_id" meowgram-server:pre-r2
docker image tag "$web_id" meowgram-web:pre-r2
printf 'Previous server ID: %s\nPrevious web ID: %s\n' "$server_id" "$web_id"
```

Record their release/source revision too. The rollback drill tests **v1.1.0**
(`4aa931951b72e4f6fb5be714feb746c4b4bc809c`); it does not establish that cellar's
currently running images are that release. If they differ, verify compatibility
before using keep-data rollback. Do not pull over or prune the local pre-r2 tags.

## 3. Pull and start, with bots off

Set `MEOWGRAM_VERSION=2.0.0` and `BOT_BASIC_AUTH_ENABLED=false` in the ignored
deployment env file. Then:

```sh
dc pull server web
dc up -d
dc ps
```

Production Compose has no build sections. Migrations 000005 and 000006 run at
server startup. They preserve human rows, add bot identity and idempotency, and
have non-destructive downs. Do not apply a down as a substitute for the recovery
procedure below. Local developers explicitly add `-f deploy/docker-compose.build.yml`
and use `up -d --build`; its `:dev` images are not cellar release artifacts.

## 4. Smoke and record digests

Use the actual HTTPS origin in a private variable (placeholder below):

```sh
MEOWGRAM_ORIGIN=https://example.home.arpa
curl --fail --silent --show-error "$MEOWGRAM_ORIGIN/healthz"
docker image inspect "ghcr.io/purrmonster/meowgram-server:$MEOWGRAM_VERSION" --format '{{json .RepoDigests}}'
docker image inspect "ghcr.io/purrmonster/meowgram-web:$MEOWGRAM_VERSION" --format '{{json .RepoDigests}}'
```

Log in via OIDC, confirm old history, send from phone and web, reconnect, and
confirm human messages have no `· Bot` label. Fill these fields only after rollout:

| Deployment record | Owner fills |
|---|---|
| Date / operator / checked-out release commit | NOT DEPLOYED |
| Previous release / server image ID / web image ID | NOT RECORDED |
| Backup filename / off-node checksum / compared count | NOT RUN |
| Pulled server digest / web digest | NOT PULLED |
| Health / login / old history / phone+web send | NOT RUN |

## Recovery A: keep data, only with zero bot messages

**Turning the flag off and starting v1.1.0 alone does not work.** At schema
version 6 it exits with `no migration found for version 6: read down for version
6: file does not exist`. Non-destructive down files do not solve the old binary's
missing migration files. Use the following only with the tested v1.1.0 image,
clean schema version 6 and **zero bot messages**, after stopping writers:

```sh
dc stop server
dc exec -T postgres sh -c 'psql -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" -d "$POSTGRES_DB"' <<'SQL'
BEGIN;
LOCK TABLE messages IN ACCESS EXCLUSIVE MODE;
DO $$ BEGIN
  IF NOT EXISTS (SELECT 1 FROM schema_migrations WHERE version=6 AND NOT dirty)
     OR (SELECT count(*) FROM schema_migrations) <> 1 THEN
    RAISE EXCEPTION 'Expected clean schema version 6; stop and investigate';
  END IF;
  IF EXISTS (SELECT 1 FROM messages WHERE sender_id IS NULL OR bot_principal_id IS NOT NULL) THEN
    RAISE EXCEPTION 'Bot messages exist: keep-data v1.1.0 rollback is unsafe';
  END IF;
END $$;
UPDATE schema_migrations SET version=4, dirty=false;
COMMIT;
SQL
docker compose --env-file .env --env-file deploy/.env \
  -f deploy/docker-compose.yml -f deploy/docker-compose.rollback.yml up -d server web
```

The overlay selects the **local** pre-r2 images and never pulls them; Basic stays
false. No tables or columns are dropped, and human messages/idempotency rows stay.
Smoke OIDC history and WebSocket send. The scratch drill proves this with the
actual previous binary. With a bot row, that binary can start after the forced
version change, but history/sync fails with HTTP 500 because sender_id is NULL.
Do not delete bot messages to circumvent the restriction.

Rolling forward after A: use the base Compose command (omit rollback overlay),
keep Basic false, `dc up -d server web`. The current image sees version 4 and
re-applies 000005/000006 safely; the drill checks version 6 clean and retained
human messages plus idempotency rows. Re-run the smoke checks.

## Recovery B: full pre-upgrade restore

This restores the pre-upgrade data and previous image. **All messages and other
changes since that backup are lost**, including bot messages. Obtain the owner's
explicit recovery authorization and verified backup first. Stop server and backup
service so nothing writes while replacing the DB. Do not delete the volume.

```sh
dc stop server db-backup
dc cp "deploy/backups/$dump" postgres:/tmp/pre-r2.dump
dc exec -T postgres sh -c 'dropdb --force -U "$POSTGRES_USER" "$POSTGRES_DB" && createdb -U "$POSTGRES_USER" -O "$POSTGRES_USER" "$POSTGRES_DB"'
dc exec -T postgres sh -c 'pg_restore -U "$POSTGRES_USER" -d "$POSTGRES_DB" --no-owner --no-acl --exit-on-error /tmp/pre-r2.dump'
dc exec -T postgres rm /tmp/pre-r2.dump
docker compose --env-file .env --env-file deploy/.env \
  -f deploy/docker-compose.yml -f deploy/docker-compose.rollback.yml up -d server web db-backup
```

Compare restored message count with the recorded pre-upgrade count, then smoke
login/history/new human sends. The fixture proves restore into a separate scratch
database; destructive replacement above is an owner procedure, never agent-run.

## Later, separately authorized bot enablement and pourOver pilot

After all [owner checks](owner-checks.md), create the dedicated LLDAP account and
direct group membership. Put the least-privilege reader password in an ignored
host-local file `deploy/secrets/lldap-reader-password` (mode 600, readable by
server UID 10001); mount the verified CA as `deploy/secrets/lldap-ca.pem`. Never
put credentials in command lines, tracked env files or logs. Configure:

```dotenv
BOT_BASIC_AUTH_ENABLED=false
BOT_LDAP_URL=ldaps://directory.example.home.arpa:636
BOT_LDAP_BASE_DN=ou=people,dc=example,dc=home,dc=arpa
BOT_LDAP_READER_DN=uid=meowgram-reader,ou=people,dc=example,dc=home,dc=arpa
BOT_LDAP_READER_PASSWORD_FILE=/run/secrets/lldap-reader-password
BOT_LDAP_ALLOWED_GROUP=cn=meowgram-bots,ou=groups,dc=example,dc=home,dc=arpa
BOT_LDAP_DIRECTORY_NAMESPACE=household-directory
BOT_LDAP_CA_FILE=/run/secrets/lldap-ca.pem
BOT_LDAP_TIMEOUT=5s
BOT_LDAP_MAX_CONCURRENT=8
BOT_TRUSTED_PROXY_CIDRS=<actual-proxy-peer-CIDRs>
API_RATE_PER_MINUTE=120
API_RATE_BURST=30
GLOBAL_RATE_PER_MINUTE=6000
GLOBAL_RATE_BURST=200
```

Only after approval change Basic to true and `dc up -d server`. HTTPS ingress
must preserve Authorization and overwrite X-Forwarded-Proto; CIDRs describe the
peer seen by the server, not the public client. Never trust all peers. Obtain the
principal sender_id from a controlled successful send using a private credential
file/client; the LDAP UUID and display name are not the bot principal ID.

Example/pourOver configuration (placeholders; adapt to pourOver's runtime):

```dotenv
MEOWGRAM_SYNC_URL=https://example.home.arpa/api/messages/sync
MEOWGRAM_SEND_URL=https://example.home.arpa/api/messages
MEOWGRAM_USERNAME=pourover
MEOWGRAM_PASSWORD_FILE=/private/pourover-password
MEOWGRAM_BOT_ID=<principal-sender_id>
MEOWGRAM_STATE_FILE=/private/pourover-state.json
MEOWGRAM_CHANNEL=
```

Run one instance from `server` with `go run ./examples/lounge-bot` for a controlled
smoke. Its default missing cursor processes **all historical messages**; before
pilot use a reviewed initial state at the agreed starting instant, with matching
Account (`syncURL|sendURL|uid|principalID`), Cursor.After (UTC RFC3339), Cursor.ID
empty and Pending null. Keep state across restarts. Do not delete pending sends.

Success: one new human lounge message gets one plain `@username` reply, visible
on phone/web as `· Bot` and never as self; the bot ignores its own kind+ID. Retry
the same key/body: identical response, one row; changed body: 409. Sync polls every
3s without steady-state 429; burst 429/directory 503 honor Retry-After. Unknown
channel_id remains 400; MEOWGRAM_CHANNEL must remain empty. Confirm group removal
denies the next request and restoring membership restores access.

To stop: stop the bot process, retain its state, set BOT_BASIC_AUTH_ENABLED=false
and `dc up -d server`. Human OIDC stays available; schema and messages stay.
**No image rollback is needed to stop the pilot.** After a bot message exists,
recovery A is no longer safe; prefer the current image with Basic disabled.

## Recorded follow-up, not implemented

R2.1 owner decision: should a newer binary start when the database schema is ahead
of its bundled migrations? Define compatibility guarantees and safe failure
behavior before changing golang-migrate startup policy. No bypass is added in R2.
