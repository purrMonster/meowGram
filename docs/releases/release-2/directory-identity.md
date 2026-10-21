# UST-2.3.1: directory compatibility and sender identity

Updated: 2026-10-17. Status: isolated LLDAP v0.6.3 compatibility verified;
identity design selected for implementation. The owner confirmed deployed
LLDAP 0.6.3; live transport/configuration remain unverified. This is not an authentication feature
or an applied database migration.

## Reproduce the directory evidence

From the repository root, run either:

```powershell
./deploy/scripts/verify-directory.ps1
```

```sh
sh deploy/scripts/verify-directory.sh
```

The image pins LLDAP v0.6.3 by digest. Python probe packages come from its
Alpine package repository; those packages are not byte-for-byte pinned.
The scratch verification workflow runs this probe in its own `directory` job.
Build requires network access; execution uses `--network none` and only
container loopback. No host ports, host data mounts, Docker socket, production
environment files, or external services are used. The non-root probe generates
test passwords, signing material and a short-lived certificate inside `/tmp`
tmpfs. LLDAP's database is also there. `--rm` removes the container; the wrapper
retains only the reusable build image. Provider/tool output is suppressed and
only fixed check labels are printed. Fixture administration uses HTTP on
container loopback, not a production authentication transport.

Verified checks:

| Check | Result / implication |
|---|---|
| User password bind with trusted certificate | Verified LDAPS works |
| Unknown certificate and incorrect password | Rejected |
| `lldap_strict_readonly` reader | Resolves `uid`, `entryUUID`, `cn`, `memberOf` |
| Direct group membership | `memberOf` contains the allowed group's full DN |
| Group removal | Next fresh bind/search observes removal |
| Password rotation | Old password fails; replacement works |
| Display-name update | `entryUUID` remains stable |
| Delete/recreate same login | New UUID; bot membership is not inherited |
| StartTLS | Unavailable in this fixture; no bind or plaintext fallback attempted |
| Login-ID rename | No rename field in the provider's update API; not claimed as supported |

The fixture checks provider behavior, not meowGram authentication middleware.
No claim is made yet about HTTP status handling, LDAP request budgets, revocation
of in-flight requests, or human OAuth regressions in an implemented bot path.

## Directory contract for the next implementation

Use verified LDAPS for this provider version. Do not implement a selectable
StartTLS path based only on the original proposal: it needs a separately tested
provider/transport. TLS hostname/chain checks remain mandatory. Credentials
must never reach an unencrypted connection after negotiation fails.

Resolve exactly one user under `ou=people,<base DN>` with an escaped `(uid=...)`
filter. Read `entryUUID`, `uid`, `cn` and `memberOf` explicitly. Bind the user
on a separate connection using the resolved DN; do not interpolate unchecked
input into a DN or share a rebound connection. Then re-read membership and
UUID using the reader, requiring the UUID to match the resolved account, and
authorize against a configured full group DN. Use DN-aware comparison, not
substring matching. No nested-group semantics are assumed. Check these facts
on every request, including idempotent replays and every sync page.

A reader in `lldap_strict_readonly` is sufficient for these tested queries;
meowGram must not require the admin account used to provision this fixture.
That group grants directory read visibility, not just access to one bot group;
the owner must accept that scope or provide a separately verified restriction.
The application needs no GraphQL administration or password-changing capability.

Identity key: `(provider='lldap', directory_namespace, entryUUID)`. Namespace
is a configured, stable directory-instance identifier, not the hostname or
user-supplied input. Changing a hostname must not create new principals;
replacing a directory is an explicit identity-migration decision. Missing or
ambiguous UUIDs fail closed. Never substitute a login, email or display name.

Display-name changes preserve identity. Login-ID renames are unsupported by
this tested administrative contract; do not simulate one by deleting/recreating
and silently linking the new account. If login-ID rename is later required,
verify provider support or design an explicit owner-reviewed mapping workflow.
The v0.6.2 source derives UUIDs from name and creation time; the v0.6.3 recreation probe
establishes ordinary lifecycle behavior, not immunity to a malicious directory
administrator or restoration of an old directory database.

## Selected persistence design

Current schema: `users.id` is a local UUID, but `messages.sender_id` references
`users.authelia_sub` with `ON DELETE CASCADE`. Push ownership also uses OIDC
subjects. Human messages, subjects, devices and foreign keys must retain their
existing associations. Do not insert fabricated LDAP subjects into `users`.

For UST-2.3.3, introduce separate bot principals and directory identities:

| Object | Proposed columns and constraints |
|---|---|
| `bot_principals` | UUID primary key, display name, creation time; no password or auth cache |
| `bot_directory_identities` | Provider, stable directory namespace, external UUID as a composite primary key; bot principal UUID FK with `ON DELETE RESTRICT`; unique principal mapping for API v1 |
| `messages.bot_principal_id` | Nullable UUID FK to bot principals, `ON DELETE RESTRICT` |
| Existing `messages.sender_id` | Preserve all values and its OIDC FK; relax `NOT NULL` for new bot rows only |
| Sender check | Exactly one of `sender_id` and `bot_principal_id` is non-null |

This adds tables and a column and relaxes one constraint without rewriting
existing sender values or backfilling rows. Existing rows satisfy the sender
check. Validate constraints in a controlled migration; lock duration still
needs measurement. This design is intentionally not installed as a numbered
migration in this investigation. Implementation must test it against populated
scratch PostgreSQL before shipping.

Existing human reads/sends continue through the current user identity path.
Bot sends insert a null OIDC sender and the resolved bot principal UUID.
History/sync queries must join both sender sources and emit a non-null wire
sender for each. Local principal/profile rows survive removal of group access
or deletion from the directory. Authorization is freshly checked, never
inferred from the existence of a local profile. A recreated directory account
maps to a new principal even when its login matches an old bot or a human.

Separate bot principals keep this narrow exception out of human provisioning,
push ownership and WS tickets. A universal principal conversion would require
broader changes to existing OIDC references; it is unnecessary for API v1.

## Wire and Flutter compatibility

Human `sender_id` remains the exact OIDC subject. Bot `sender_id` is its local
principal UUID, accompanied by `sender_kind: "bot"`. Human messages emit
`sender_kind: "human"`; missing kind is treated as legacy human data.
Identity comparisons must use **kind plus ID**, since a string prefix or a
random UUID alone cannot prove separation from arbitrary valid OIDC subjects.

Before enabling bots, update all these consumers together:

- Go `Message` and `WSMessage` serialization, history and sync queries.
- Flutter `ChatMessage` parsing, serialization/cache, equality and `isMine`.
- Foreground `ChatBloc` and background notification self-message filtering.
- Message bubble bot labeling, retaining UUID-based message deduplication.

Tests must deliberately use the same string for a human subject and bot UUID
to prove neither self-styling nor notification suppression confuses them.
Older clients ignore the new discriminator and cannot provide that guarantee;
rollout therefore requires updated clients before enabling bot access.
Sender-kind support belongs in the identity implementation PR, not an optional
cosmetic follow-up. Timestamps, message UUIDs and pagination remain unchanged.

After bot messages exist, an older server may not handle null legacy sender
columns. The rollback is disabling bot Basic Auth while retaining the compatible
new server and data. Do not deploy an older binary or run a destructive down
migration without a separately verified compatibility plan.

## Remaining gates

1. Version match established: the owner reported LLDAP 0.6.3 on 2026-10-17 and
   the pinned v0.6.3 fixture passed. Reverify when the provider version changes.
2. Confirm authorized production LDAPS availability, trust and reader scope
   during rollout. No live identity/deployment changes are authorized here.
3. In 2.3.3, prove populated-schema migration, concurrent principal creation,
   OIDC/bot collision isolation, history retention and client sender-kind behavior.
4. In 2.3.2, prove bounded LDAP work, HTTP route isolation and error handling.

Source references: [pinned provider configuration](https://github.com/lldap/lldap/blob/v0.6.3/lldap_config.docker_template.toml),
[update schema](https://github.com/lldap/lldap/blob/v0.6.3/schema.graphql),
[UUID construction in v0.6.2](https://github.com/lldap/lldap/blob/v0.6.2/crates/domain/src/types.rs),
and [fixture provisioning tooling](https://github.com/lldap/lldap/blob/v0.6.3/example_configs/bootstrap/bootstrap.md).
The execution results above are from this repository's probe on 2026-10-17.

Repository regression verification also passed on 2026-10-17 through
`deploy/scripts/verify.ps1`: Go race tests and vet, PostgreSQL integration and
dump/restore, Flutter analysis and tests, release web build and production web
smoke checks. The existing optional Cupertino font warning remains. Both
wrapper syntax checks and release-document local-link checks passed. Native
Android/Apple builds were not rerun; no production configuration was tested.
