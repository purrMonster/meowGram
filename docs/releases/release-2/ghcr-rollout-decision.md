# Owner-approved GHCR deployment decision — 2026-10-22

Implementation belongs to UST-2.5.2, after the epic 2.5 parent-sync owner merge
and API acceptance. This records requirements, not completed deployment work.
No live changes, package visibility changes, credentials, DNS or tags are authorized.

## Compose and release artifacts

- In deploy/docker-compose.yml, server uses
  `ghcr.io/purrmonster/meowgram-server:${MEOWGRAM_VERSION:?set MEOWGRAM_VERSION}`;
  web uses `ghcr.io/purrmonster/meowgram-web:${MEOWGRAM_VERSION:?set MEOWGRAM_VERSION}`.
- No :latest. Move both build sections, including web build args, to
  deploy/docker-compose.build.yml as an explicit local-development override.
- Leave deploy/verify.compose.yml unchanged.
- Add `MEOWGRAM_VERSION=2.0.0` to .env.example with a placeholder comment stating
  it must match a published tag. Do not claim the image exists until verified.
- In the release PR bump client/pubspec.yaml to `2.0.0+4`. The existing release.yml
  version job requires `v<pubspec semantic version>`: this release needs v2.0.0,
  not v2.0.0+4. Creating/publishing that tag remains an owner action.

## Owner-run cellar upgrade procedure to write and verify in UST-2.5.2

1. Run db_backup.sh, copy the completed dump off-node, and verify that dump restores
   in isolation before upgrading. The current db_backup.sh is a recurring loop;
   the procedure must identify a completed dump. The current verify_restore.sh
   only dumps/restores the synthetic /verification DB: do not represent that as
   validation of the copied dump. Provide a safe isolated dump-input restore
   procedure/harness without modifying verify.compose.yml or touching live data.
2. Tag the actual currently running local server/web images as
   `meowgram-server:pre-r2` and `meowgram-web:pre-r2`. Record their image IDs and
   previous release/version; do not guess which image cellar currently runs.
3. Keep BOT_BASIC_AUTH_ENABLED=false. Set MEOWGRAM_VERSION=2.0.0, run compose pull,
   then up -d with the explicit production Compose/env files. Migrations
   000005/000006 run on server start. No builds on cellar.
4. Smoke-test /healthz, login, existing history, phone send and web send.
5. Record the actual pulled server/web image digests in the runbook's deployment
   record after the owner performs the rollout. Before rollout, leave explicit
   unfilled fields rather than inventing digests.

## Rollback restriction

Owner decision: point Compose at the local pre-r2 tags (or the previous published
version), then up -d. Document an explicit image override so local tags are not
mistakenly expanded under the GHCR repository prefix. Keep Basic disabled.
This rollback is valid only while NO bot message exists. Verify that precondition;
a false flag alone does not prove it. After any bot message, do not recommend the
pre-R2 image path. Preserve migrations 000005/000006; their downs are non-destructive.
UST-2.5.2 must state the previous image release and its schema compatibility or
clearly state that compatibility has not been established. Do not delete bot data
or clear migration state to make an old image work.

## GHCR access owner check

Add a timed owner-checks.md item: make GHCR packages meowgram-server and
meowgram-web public, OR create a read:packages token for cellar. Expected result:
cellar can pull the exact version of both packages. Token provisioning/login and
visibility changes remain owner actions; never print or commit the token.

Before recommending public visibility, inspect BOTH release images' configuration,
environment, history and all saved layers (including files removed by later layers)
for secrets and deployment-specific hostnames. Source/Dockerfile review alone is
not image inspection. Record image IDs/digests, inspection scope and findings;
placeholder/example hostnames are distinct from deployment-specific hostnames.
Do not claim images are cleared or recommend public access before this check.

The existing API pilot still requires reader/group visibility, verified LDAPS,
proxy Authorization pass-through and overwritten X-Forwarded-Proto, explicit
trusted CIDRs and owner-created bot accounts. pourOver keeps MEOWGRAM_CHANNEL
empty. Enabling Basic/pilot is a later explicit owner step, after the pre-bot
upgrade and rollback checks above.
