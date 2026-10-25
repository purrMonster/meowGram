# UST-2.1.4.1: ntfy protected-topic contract

## Pinned spike and owner reconciliation

Recommended fixture: released **ntfy v2.34.0** (2026-10-24), image
`binwiederhier/ntfy:v2.34.0@sha256:901e14503d3f605f445be0e68f2ac71f0aa73391897ce6ceb599f8c48d8b3792`.
This pins the upstream multi-platform image index; tests use Linux amd64.
[Release](https://github.com/binwiederhier/ntfy/releases/tag/v2.34.0) and
[versioned relay implementation](https://github.com/binwiederhier/ntfy/blob/v2.34.0/server/server.go).
The owner authorized this recommendation while inventory is unavailable.
**O2 remains pending reconciliation:** actual server image digest, installed iOS
app version and whether upstream relay is configured are unknown. This is no
authorization to upgrade or configure the owner's service.

## Dispatch and access contract

- One protected, opaque topic per human; no username, subject or conversation ID
  in its name. Default access denies everyone. Dedicated non-admin publisher has
  write-only access to configured topics; each recipient has read-only access to
  their topic. A publisher bearer token inherits that user's restricted ACL.
- Send `POST ${NTFY_BASE_URL}/${topic}` with `Authorization: Bearer <file token>`,
  `Title: meowGram`, and body `New message`. No text, sender, conversation label,
  UUID, membership count, attachment, click URL or action is sent. No client may
  supply a destination URL or inspect another human's topic/mapping.
- Eligibility comes from current human membership minus sender and muted humans.
  Recheck immediately before dispatch/retry; no replay notification. Scheduling
  remains bounded per conversation with a trailing signal; generic payloads do
  not contain the scheduling key. Provider failures never undo persistence.
- ntfy subscriptions outlive meowGram logout and FCM lease expiry. Removing the
  private mapping stops future dispatch. An already handed-off alert cannot be
  recalled. Real owner ACL setup and mapping lifecycle remain O3/O5 checks.

## Relay privacy boundary

The real pinned binary sends an **empty-body POST** to the upstream URL with
SHA256(`${NTFY_BASE_URL}/${topic}`) as the route and original message ID in
`X-Poll-ID`. The upstream creates a generic poll request; the iOS app fetches the
actual generic alert from the protected server with its own credentials.
The publisher Authorization header is not forwarded. A separately configured
upstream access token, if any, belongs to the ntfy operator and is outside
meowGram's publisher configuration.

The relay can observe source network address, timing, volume, stable hashed route,
message ID and ntfy User-Agent/version. Hashing is not anonymity; topic secrecy
and ACLs are both necessary. Generic relay fallback text is `New message`.
[Upstream iOS flow](https://docs.ntfy.sh/config/#ios-instant-notifications).
The fixture captures the outgoing wire request; it does not emulate APNs or claim
delivery to a locked phone. O4 requires the owner's real protected-topic iPhone.

## Configuration contract for UST-2.1.4.2 / 3.3

These names and defaults are published here; production code is implemented in
later stories and remains unchanged by this spike.

| Setting | Contract |
|---|---|
| `NTFY_ENABLED` | `false`; disabled requires no files or network access |
| `NTFY_BASE_URL` | Required when enabled; HTTPS fixed origin; no userinfo, query or fragment; reject redirects |
| `NTFY_TOKEN_FILE` | Required private read-only mounted publisher bearer token; never log it |
| `NTFY_RECIPIENTS_FILE` | Required private read-only JSON object mapping immutable human subject to opaque topic; no public API |
| `NTFY_TIMEOUT` | `5s`; positive bounded HTTP deadline |
| `PUSH_COALESCE_INTERVAL` | `30s`; bounded per-conversation scheduling plus trailing signal |
| Flutter `IOS_NOTIFICATION_PROVIDER` | Build-time `fcm` default, `ntfy` opt-in; reject unknown values |

Recipient topics use only ASCII letters, digits, `_` and `-` (1–64 characters),
with no path separators or percent encoding. Deployment supplies high-entropy
opaque values. Invalid enabled configuration fails closed. Files are host-local;
no real token, subject or mapping example is tracked. The publisher must limit
response reads, bound retries and redact errors; its implementation tests belong
to UST-2.1.4.2 after epic 2's provider interface reaches this epic.

In iOS ntfy mode, meowGram neither registers FCM nor generates duplicate local
alerts. The separate ntfy app handles its own login/subscription. Tapping opens
ntfy; no meowGram deep-link assumption. Android/macOS FCM behaviour stays tested.
Client implementation belongs to UST-2.1.3.3 after the approved integration sync.

## Executable evidence

Run `./deploy/scripts/verify-ntfy.ps1` or `sh deploy/scripts/verify-ntfy.sh` from
the repository. The driver uses the real pinned binary, scratch SQLite files,
random synthetic credentials/token/topics, and an in-process capturing relay.
Containers are read-only with tmpfs `/tmp`, `--network none`, and no published
ports, persistent data or real services. Setup output and secrets are suppressed.

Thirteen assertions per timezone cover anonymous denial, publisher read denial,
cross-user read/write denial, own-user write denial, authorized publish/read
positive controls, and exact relay route/ID/empty body/absent credentials.
This proves the synthetic ACL/wire contract, not production TLS, real ACLs,
client mode behaviour, provider filtering or phone delivery.
Fresh commands/counts: [story log](log/2026-10-25-UST-2.1.4.1.md).
