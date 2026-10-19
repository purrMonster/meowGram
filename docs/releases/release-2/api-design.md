# Release 2 API and identity design

Status: proposed implementation contract, 2026-10-15.
Product scope is recorded in [the release plan](README.md).

The [2026-10-17 directory investigation](directory-identity.md) refines this
proposal: verified LDAPS for LLDAP v0.6.3 (StartTLS is not supported by that
fixture), separate bot principal/identity tables with a nullable bot message
FK, and a required wire sender-kind discriminator. The owner reports deployed
version 0.6.3. Live transport and the implementation's PostgreSQL/client tests
remain gates.

## Authentication boundary

| Surface | Human bearer token | Bot Basic Auth |
|---|---|---|
| POST message endpoint (new) | Accepted after existing token validation | Accepted only after LLDAP password and group checks |
| GET sync endpoint (existing) | Existing behavior retained | Same directory checks on every page request |
| WebSocket ticket and WebSocket connection | Existing ticket flow retained | Rejected; no ticket or socket granted |
| Push-device registration and other protected endpoints | Existing policy retained | Rejected |
| Future media endpoints | Define under media workstream | Not enabled by the bot group |

Dispatch by the Authorization scheme. A failed bearer token must never fall
back to Basic. Reject ambiguous/multiple credentials and malformed or empty
Basic usernames/passwords before contacting LDAP. Authentication errors must
not disclose whether an account exists. Do not introduce a Basic login UI in
Flutter, URL credentials, cookies, or password-bearing query parameters.

The external API requires HTTPS. Retain loopback-only backend publication and
the trusted reverse-proxy boundary. Do not trust arbitrary forwarded headers
as evidence of TLS. The implementation must document how insecure requests are
rejected before credentials are processed at ingress.

## Directory validation

1. Apply bounded ingress/authentication budgets before directory work. Use
   bounded concurrency, connection deadlines and request cancellation so failed
   authentication cannot exhaust LDAP or the Go server.
2. Resolve exactly one account using a least-privilege directory reader if the
   deployment requires one. Escape LDAP filters and distinguished names using
   library facilities; never interpolate unchecked usernames into filters.
3. Validate the supplied password with a user bind over verified LDAPS or
   mandatory StartTLS. Reject empty passwords and never fall back to plaintext
   or skip certificate validation. Do not rebind a shared concurrent connection.
4. Check direct membership in the configured group using directory data. Do not
   assume nested groups, disable flags or schema attributes exist; establish
   the exact supported query in the directory compatibility story.
5. Resolve a stable bot principal and authorize this request. No positive cache
   in v1; a subsequent request after a directory group removal is visible must
   fail. An operation already authorized may finish; this is not retroactive
   cancellation or erasure of messages already read.

LLDAP unavailability, ambiguous entries or membership lookup failures must
never permit bot operations. Dependency failures return a sanitized 503;
invalid credentials return 401; valid credentials without membership return
403. Bearer-authenticated human operations must not acquire an LDAP dependency.

Every bot has a separate account. Group removal is the defined access-revocation
mechanism. Password rotation and account deletion also need integration tests.
Any additional account-disable feature is subject to actual directory support.
Never log Authorization, passwords, bind secrets, raw LDAP responses or request
objects containing credentials. Audit outcomes using a stable principal where
known and sanitized reason codes.

## Identity and persistence

Current users and messages use Authelia subject identity, including the
`messages.sender_id` foreign key to `users.authelia_sub`. An LDAP username must
not simply be inserted as an Authelia subject. Nor should a matching username
or email automatically join a bot to an existing human profile.

Introduce an additive provider-qualified identity mapping to a local principal.
Preserve existing OIDC subjects, message IDs and sender associations. Choose
the concrete schema after reviewing migrations and all wire-format consumers;
document how bot sender IDs coexist with existing OIDC sender IDs without
collisions. A stable directory identifier must survive a rename and distinguish
a deleted/recreated account. If the deployed schema cannot supply that, resolve
an explicit provisioning mapping before shipping.

Access revocation must not delete local profiles or cascade-delete messages.
Display names are labels, not identity keys. Bots should be recognizable in the
timeline without breaking existing message parsing or UUID deduplication.

## HTTP message contract

Paths below are proposed defaults and remain configurable. Only sync exists
today; this document does not claim POST is implemented.

### Send

`POST /api/messages` with `Content-Type: application/json`, Authorization, and a
required `Idempotency-Key` header containing a caller-generated UUID.

```json
{"text_content":"The household backup completed."}
```

Allow only the documented payload fields. Reject caller-supplied sender,
timestamp and message ID fields. Reuse the WebSocket text validation and
4,000-character limit with identical Unicode/whitespace behavior; set an HTTP
body-size bound compatible with that limit. HTTP and WebSocket sends share the
same per-principal message budget and persistence/broadcast service.

On a new committed message return 201 with `id`, `sender_id`, `username`,
`text_content` and `created_at`, matching the existing message representation.
`created_at` is server-generated UTC. A duplicate retry returns 200 with the
original message and never inserts, broadcasts or notifies again.

Scope keys to the authenticated principal. Store the key, validated request
fingerprint and result atomically with the message. Concurrent requests with
the same key and content produce one record. The same key with different
content returns 409. Authentication and group validation run before replaying
any stored response, so a revoked bot cannot replay its way around the gate.

Proposed retention: at least 7 days, configurable. Clients retry the same
operation with the same key only within that window; after expiry, replay may
create a new message. Document this boundary and bound storage cleanup. An
idempotent replay consumes ingress/API budgets but not another message-write
budget. Define deterministic transaction behavior for concurrent requests.

Commit before enqueueing to the hub. A successful response confirms durable
storage, not delivery to every client. If the process fails after commit,
retries recover the same result and sync recovers the message. Failed database
writes produce no broadcasts. Existing Hub ownership, WritePump isolation and
slow-client eviction remain intact.

### Read

Reuse `GET /api/messages/sync?after={timestamp}&after_id={uuid}`. Responses stay
JSON arrays, ordered by the existing timestamp/UUID cursor, capped at 500.
When `X-Has-More: true`, continue using both fields from the last message.
An empty result is `[]`; preserve the current cursor and poll with backoff.

For first retrieval, callers supply a chosen lower-bound timestamp; an epoch
timestamp can request all retained modern chat history. Persist the cursor
only after processing a page and deduplicate by message UUID. This endpoint is
catch-up polling, not arbitrary full-text search or a subscription protocol.

### Errors

Use a stable JSON error code and safe message, aligned with existing handlers.

| HTTP status | Meaning |
|---|---|
| 400 | Invalid JSON, fields, cursor or idempotency key |
| 401 | Missing, malformed or invalid authentication; standards-compliant challenge |
| 403 | Authenticated bot lacks the required group |
| 409 | Idempotency key reused with different content |
| 413 / 415 | Body too large / unsupported content type |
| 429 | Budget exhausted; honor Retry-After |
| 500 / 503 | Internal persistence failure / unavailable dependency; no sensitive details |

Return `Cache-Control: no-store` on authenticated message responses and errors.
Do not widen CORS for bot scripts. Web OAuth callers need the new idempotency
request header allowed by the configured-origin policy. Basic is disabled by
default and is never added globally to existing authentication middleware.

## Proposed configuration

These settings are design proposals, not currently supported environment
variables. Final names/defaults must be added together to server config,
`.env.example`, Compose wiring where applicable, and the runbook when built.
LDAP settings are server-only and must not enter client config/discovery.

| Proposed setting | Default / requirement |
|---|---|
| MESSAGE_SEND_ENDPOINT | `/api/messages` |
| BOT_BASIC_AUTH_ENABLED | `false` |
| BOT_LDAP_URL | Required when enabled; encrypted transport only |
| BOT_LDAP_STARTTLS | Deferred: v0.6.3 fixture does not support StartTLS; initially require LDAPS |
| BOT_LDAP_BASE_DN | Required directory search base |
| BOT_LDAP_READER_DN | Required if authenticated searches are needed |
| BOT_LDAP_READER_PASSWORD_FILE | Host-local secret file if a reader credential is needed |
| BOT_LDAP_ALLOWED_GROUP | Required when enabled; suggested `meowgram-api-bots` |
| BOT_LDAP_DIRECTORY_NAMESPACE | Required stable directory-instance identity; not derived from its hostname |
| BOT_LDAP_CA_FILE | Optional private CA file; system trust otherwise |
| BOT_LDAP_TIMEOUT | Proposed 5 seconds per authentication operation |
| BOT_LDAP_MAX_CONCURRENT | Proposed 8 directory authentication operations |
| MESSAGE_IDEMPOTENCY_RETENTION | Proposed 168 hours |

Existing global/API/message budgets remain applicable. Final directory search
and group/ID attribute settings depend on the verified schema; they must be
explicitly documented rather than embedding deployment-specific values in code.
No real hostnames, DNs or secret values belong in tracked examples.
