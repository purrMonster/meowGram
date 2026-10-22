# UST-2.3.4 — stable bot usernames

Parent/PR target: feat/r2-epic-2.3-bot-identity. Task branch:
feat/ust-2.3.4-stable-bot-usernames. Complete before the identity epic release PR.
Owner-approved API-only R2 scope and pourOver R10 requirement, 2026-10-20.

## Contract

Directory authentication explicitly requests uid, entryUUID and memberOf. It
requires exactly one valid uid, unchanged across the password bind. The returned
Identity.Username is the directory value, not cn and not the spelling submitted
in Basic Auth. Missing/ambiguous/oversized uid fails closed with a sanitized
unavailable response. No fallback to a display label is allowed. The existing
100-character database limit is enforced without trimming or truncation.

RepositoryResolver adapts the verified identity into BotRepository.Resolve and
returns Principal.Username from the persisted profile. Use this adapter when
wiring the future message API gate. Resolve inserts the login uid only on first
provisioning; subsequent requests return the existing username without an UPDATE.
Namespace plus directory entryUUID remains the identity key. Deleting/recreating
a login produces a different principal even if its username is reused.

Human and bot usernames are not checked for uniqueness against each other.
pourOver must identify itself by sender_kind + sender_id. Neither display labels
nor matching usernames link a bot to an OIDC user. Human provisioning, usernames,
authentication and message persistence are unchanged.

## Migration and compatibility

No migration, schema rewrite or display_name column is needed. cn is not persisted
by this path. Existing profiles retain their stored username, including any
cn-derived labels created by earlier pre-release experiments; this change does
not infer or backfill a login ID from a display name. Basic activation remains
blocked until API integration, so this task does not claim to repair live bot
profiles. Before a pilot, inspect any pre-existing experimental bot profiles
under owner authorization; do not delete identities/history or silently rewrite
them. Fresh provisioning uses uid. A future explicit remediation of legacy labels
would need its own reviewed mapping/data-change plan.

The supported LLDAP 0.6.3 fixture does not support login-ID rename. A display-name
rename is supported and must never change username. If another provider later
supports uid renames, the provisioned username remains stable until an explicit
rename policy is approved. This task does not add a new provider or rename API.

## Verification and runbook fold

- TestDirectoryUsesVerifiedUIDNotDisplayName: changed cn and differently cased
  Basic input still yield canonical uid.
- TestDirectoryRejectsInvalidOrChangingUID: missing, empty, multivalued, oversized,
  whitespace/control and bind-time-changing uid are rejected without fallback.
- TestGateStableUsernameAndRecreatedPrincipal: httptest gate + real PostgreSQL
  + RepositoryResolver preserve username, isolate recreated identities and allow
  a same-name human without changing the human row or sender kind.
- TestLLDAP: real pinned LLDAP display rename preserves uid and entryUUID;
  recreation changes entryUUID and keeps the reused login name.
- TestBotIdentityMigrationAndLifecycle: immutable stored username/history, legacy
  timestamps, sender constraints, and 20 concurrent first-provisioning attempts.

At the epic documentation fold, update the runbook's reader attributes from
cn to uid and describe immutable login usernames (not refreshed display labels).
Before the authorized pilot the reader must expose uid as well as entryUUID and
memberOf. No endpoint/config variable changes and no public bot route activation.
