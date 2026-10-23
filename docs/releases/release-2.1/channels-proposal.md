# R2.1 channels — code-informed proposal

Revised 2026-10-23 from main e094dfa. Planning only. Fixed owner decisions:
channel_id, UUID internally/opaque string to clients, null lounge, existing history
retained, per-channel bot authorization and 403 for inaccessible channels.
[Scope and pending decisions](README.md) govern this proposal.

## Code map

Paths are repository-relative; symbols make the findings reproducible.

| Area | Current behavior | Required change |
|---|---|---|
| server/internal/chat/hub.go Run/deliver/pushWorker | Global broadcast, separate direct-history queue, one push signal and 30-second throttle | Scope both queues; bounded per-channel activity scheduler |
| chat/message_service.go and http_send.go | Shared validation/budget, commit before publish, no replay publication | Carry channel, authorize replay and new writes separately |
| chat/client.go ReadPump/WritePump/SendHistory | Raw/JSON text, global 50-message history, batched queued frames | Bind channel; check every final frame, including batched history |
| handler/sync.go, send.go and route files | Unfiltered sync, strict text-only body, default/custom aliases | Explicit scope in repository calls; accept only new channel_id field |
| handler/websocket.go and ws_ticket.go | Ticket binds user; register then history then pumps | Bind scope at issuance; reauthorize at consumption/history/delivery |
| handler/push_devices.go and repository/device_repository.go | 24-hour bearer registration; global active tokens without returned user identity | Identity-aware recipient queries; capability defaults for old devices |
| server/cmd/server/main.go | One Hub, FCM publisher, shared route budgets | Wire membership and provider coordinator without broadening Basic |
| repository/message_repository.go GetRecent/GetMessagesAfter | No channel predicate; timestamp/UUID paging | Filter before LIMIT; parenthesize cursor OR under channel predicate |
| repository/idempotency.go CreateIdempotent | Principal/key lock, raw-text SHA256, exact body; authorizeWrite skipped on replay | Access check on replay; compatible versioned fingerprints |
| repository/bot_repository.go and botauth/http.go | Stable uid principal; fresh LDAP/global eligibility and budgets | Independent typed channel grants; keep bot route isolation |
| server/migrations and database/db.go | Versions 5/6 preserve data on down; migrate.Up on startup | Approve startup ADR before 000007; test actual old binary |
| fcm/service.go PublishActivity | All active tokens, 500/page, global lounge collapse tags | Authorized recipients and per-channel scheduling |
| client/lib/src/models/chat_message.dart | No channel; ignores unknown JSON fields | Nullable channel in parse/serialize/equality and routing |
| bloc/chat_bloc.dart | One timeline/roster, async cache writes, WS sends | Per-channel state; discard stale account/channel generations |
| services/sync_service.dart | Newest cache cursor before hydration; no cache skips REST; one in-flight sync | Independent completed-page cursors, channel query and explicit 403 state |
| storage/local_message_repository.dart | One meowgram_messages_v1 Hive box, UUID keys, 2,000-row target; UI isolate | Account/server/channel namespaces and restart-safe migration |
| services/background_message_service.dart | Opt-in Android only; one secure-storage cursor; no Hive | Scoped independent cursors and membership refresh; not an iOS strategy |
| push/fcm_service.dart and device_registration_client.dart | Android/iOS/macOS registration and logout cleanup | New capability; skip iOS FCM for free-signing deployment |
| screens/responsive_layout.dart, chat_screen.dart, widgets/sidebar.dart | One Bloc, static lounge, zero unread; 800px layout | Real list, selection, minimal membership controls and unread state |

Extend chat safety/service tests; handler send/sync/ticket/device tests; real-DB
repository migrations/idempotency; botauth LDAP; FCM fake sender; client sync/cache,
Bot/UI/device/release regressions. server/integration/api_acceptance_test.go uses
actual binary/OIDC/LDAPS; rollback_test.go uses the old binary. deploy/scripts/verify*.sh
and PowerShell counterparts remain scratch-only entry points. Replace R2's named
channel_id_400 assertion with acceptance of this field and continued rejection of
all other unknown fields; do not broadly relax JSON decoding.

## Data and API contract

Add channels, typed memberships and audit rows; nullable messages.channel_id FK
and channel/time/UUID indexes. Preserve old sender IDs, times, UUIDs, keys and
responses. Null never means all channels. Membership FKs use human authelia_sub
(text) or bot_principals.id (UUID), exactly one with a matching kind; use per-kind
unique indexes. Do not let cascades erase private history/audit or last-owner
protection. Every new migration has partial-up, repeated-up and down/up tests.

POST send: omitted/null means lounge; UUID string selects channel. Empty string,
malformed/wrong type or duplicate channel fields return 400. GET sync: omission
or empty channel_id= means lounge; UUID selects channel; literal null, malformed
or repeated parameters return 400. Authenticate, authorize every page including
empty pages; unknown well-formed/nonmember returns 403, storage failure 503 with
Retry-After. Preserve 500-row cap, timestamp/UUID ordering, offsets, [] empty body,
X-Has-More, no-store and aliases. No implicit all-channel message endpoint.

New send/sync/history/live messages explicitly carry channel_id, including null
(no omitempty). Spoofed sender fields remain rejected. Members read pre-join
history under ADR 002. Key uniqueness remains principal-kind/ID/key across channels:
changing channel must not create a second operation. Under the transaction lock,
authorize before replay and insertion, separately from the write-budget callback.
Serialize membership revocation with protected operations. Check requested AND
stored message access before returning bytes; removed members cannot retrieve a
response by changing channel_id. Authorized changed-channel/text key reuse is 409;
inaccessible requests are 403 first.

Upgrade complication: old hashes cover raw text, and old stored bodies lack
channel_id. Recommend keeping null-channel hashes R2-compatible and using a
versioned, unambiguous canonical UUID + length-delimited raw-text encoding for
non-null channels. Add format metadata if necessary; no rewriting old rows.
Omission/null are equivalent. Preserve old unexpired replay bytes under the narrow
owner exception requested in README; all new responses carry the field. Missing
old channel fields decode as lounge. Retention is configurable, not necessarily
seven days. Test legacy retry, new lounge retry under old-binary recovery, >=20
duplicates, restart/expiry, failed writes and no duplicate publication/push.

## Old-client privacy argument and falsification test

For private channel x, egress requires current typed membership in x. Legacy
transport selection is always null, even when its user belongs to x. This is a
proposed invariant, not a claim that current code already enforces channels.

| v2.0.0 path | Server restriction | Negative proof |
|---|---|---|
| Sync without channel_id, default/custom aliases | channel_id IS NULL before cursor and LIMIT | Interleave 1,001 equal-time private rows with lounge; no private row on any page |
| Old ticket/socket/reconnect | Ticket defaults to null; recent 50 and live only null | Private insert before and after connection never reaches old decoder |
| Direct history/presence/notices | Same scope; no private labels or membership roster in lounge | Private join/leave/archive cannot produce private lounge metadata |
| Existing/renewed old push registrations | Absent capability means lounge-only | Private activity triggers zero legacy-only token deliveries |
| Old Android background sync | Omitted-filter path remains lounge | Legacy worker never turns private data into a local notification |
| Old cache/model | Only lounge data reaches transport | Actual tag v2.0.0 decoder/cache fixtures persist no private ID/text |

Fixture: lounge plus A/B, humans in A-only/B-only/both/neither, bots, legacy and
expired devices. Seed synthetic canaries, exercise real routes, capture HTTP,
decoded socket batches, provider calls and legacy persisted cache. Assert absence
of forbidden IDs/content AND presence of allowed traffic, so disabled delivery
cannot pass the test. Repeat on reconnect, revocation during history/pages/push,
and replay. Run unchanged v2.0.0 fixtures as well as new clients. Client filtering
is never authorization.

## Socket routing, resync and revocation

Recommend one selected channel per socket. New ticket requests optionally include
channel_id; bind it into the one-use ticket and recheck membership on consumption.
Reject URL overrides. Old requests stay lounge. Switching obtains a new socket;
late results carry account/channel generation and are discarded. Raw-text sends
remain supported in the bound scope; structured sends must agree with it. Malformed
channel JSON must not fall back to raw text or the lounge. Bots stay HTTP-only.

Hub alone owns/ closes registry buffers. Route live, direct history, presence and
notices by channel; preserve non-blocking 256-frame eviction. No SQL/provider HTTP
inside Hub. Reconnect scopes its 50-message burst, then fills the gap from the
completed cursor captured before hydration; deduplicate by UUID. Inactive channels
use bounded staggered HTTP catch-up on resume and while visible (proposed configurable
30-second interval, respecting shared API budgets). Badges need not be instantaneous;
this avoids an all-channel socket subscription protocol in R2.1.

Revocation events alone cannot invalidate frames already batched by WritePump.
Propose a per-channel delivery coordinator outside Hub, membership generations and
a revocation barrier. Deny new grants, invalidate pending work, wait for bounded
in-flight writes/provider calls, commit removal, then acknowledge. Each final
writer checks generation on EVERY frame, including extra frames in a batch. HTTP
reads/sends/replays share authorization ordering; repository transactions use a
common channel lock for membership changes and protected operations. DB errors
fail closed; timeout cannot report successful revocation. Rebuild state before
serving after restart. Specify lock ordering in the first routing story and test
20-way send/revoke/replay, stalled writes/providers, history queues and restarts.

This assumes the current single-server deployment. Multiple replicas require a
separate distributed invalidation/barrier design, not silent process-local caching.
After successful removal no new dispatch may begin; data already handed to the
network/provider while authorized can arrive later. Previously downloaded history
and offline caches cannot be recalled. Never promise retroactive erasure.

## Push and iPhone ntfy

Replace the anonymous global signal with bounded channel activity records. A hot
channel cannot suppress another; retain a trailing signal and configurable 30-second
default. Resolve current recipients immediately before dispatch under the same
revocation ordering. Replays produce no signal. Provider payload: title meowGram,
body New message; no text, sender, channel ID/name, roster or count. Generic device
collapse tag; internal channel scheduling preserves fairness. Opening the app
refreshes membership and syncs authorized channels.

FCM registrations gain a versioned channel-aware capability; absent means lounge
only. Capability is transport support, never an access grant. Recipient queries
retain user identity/expiry and check every page/batch. Preserve invalid-token and
logout handling; exclude sender from new channel alerts by typed identity. Legacy
lounge behavior stays compatible. Expired/reassigned tokens cannot receive private
activity.

ntfy uses its separately installed iOS app, not meowGram APNs. Owner provisions one
unique opaque topic per immutable human sub, read-only subscription and write-only
publisher ACLs with anonymous access denied. Random topic names alone are not auth.
Use a private owner-managed recipient mapping file and separate mounted publisher
token file; no arbitrary client-supplied URLs, topic/identity mapping in public Git,
or secrets in public client config. No general provisioning UI.

Proposed config: NTFY_ENABLED=false, NTFY_BASE_URL, NTFY_TOKEN_FILE,
NTFY_RECIPIENTS_FILE, NTFY_TIMEOUT=5s, PUSH_COALESCE_INTERVAL=30s. Implement strict
HTTPS fixed-origin requests, no redirects, safe topic segments, duplicate mapping
rejection, bounded queue/timeouts/retries and sanitized errors. Reload by restart
initially; disabled requires no credentials. Owner removes a mapping to stop user
alerts. meowGram logout does not log out the separate ntfy app; explain this at
setup. This mapping has its own lifecycle, independent of FCM's 24-hour device
lease; channel recipients still require current membership on every dispatch.

Provider failure cannot roll back a committed message or block Hub. Retry with
bounds and reauthorization; hints can be coalesced/lost after restart, while sync
recovers durable messages. No exactly-once push claim. Skip iOS FCM registration
for this deployment and suppress duplicate meowGram background alerts when ntfy
is selected; retain Android/macOS tests. Notifications open ntfy; automatic deep
link into meowGram is not assumed.

Official ntfy documentation says self-hosted iOS instant delivery uses upstream
poll requests with message ID and hashed topic URL; the iOS app fetches the actual
alert from the self-hosted server. The relay still observes timing/routing metadata.
Its configuration is external and owner-only. Installed server/iOS versions are
unknown: the provider spike must record exact versions and pin the fixture digest.
Protected-topic fetch and locked-phone delivery need real-device proof.
Source checked 2026-10-23: [ntfy iOS configuration](https://docs.ntfy.sh/config/#ios-instant-notifications).

## Flutter cache, cursors, unread and offline

Namespace by server origin + OIDC issuer/sub + channel (explicit internal null
sentinel). Separate timeline, completed REST cursor, read watermark and channel-bound
draft. Live/history must not advance a completed cursor past an unsynced gap. Save
fetched page and cursor recoverably; UTC/time then UUID tie-break, dedup by UUID.

Use a new Hive box. Legacy missing-channel rows migrate to lounge with IDs/text/times
preserved, restart-safe marker and verification. The old box has no account provenance:
import only with attributable persisted session/server context; otherwise quarantine
untouched and resync, explaining why. This policy needs the owner decision in README.
Never place private rows in v1 storage. Test interrupted import, server/account switch
and downgrade. Cached lounge renders before network; known same-account channel
caches render offline with stale membership/unread state clearly indicated.

Refresh channel list on resume/reconnect and before send. On 403 clear active
view, invalidate queued results/drafts and purge affected new cache/cursors. Never
retry private text in lounge. Keep unsent text visibly failed/channel-bound; do not
add an automatic offline-send queue. Cancel stale-account asynchronous writes.

Unread counts are device-local, not cross-device read receipts. Count distinct
non-self messages after persisted read watermark; mark read only while selected
and foreground. First visit establishes a current baseline; catching up shows
syncing instead of false zero. Counters survive cache eviction; replay/history
cannot double-count. Set/test per-channel and total account cache caps while
preserving sync state. Inactive channel refresh is bounded by the API budget.

New Android background checks enumerate current authorized channels and store
account/channel cursors separately in secure storage, never opening Hive in that
isolate. Clear stale cursor/list state on logout/server/account change. iOS uses
ntfy, not this worker. Retain 800px layout, SafeArea/viewInsets and Bot labels.

## What the original proposal underestimated

- Replay authorization callback skips replays; legacy fingerprints/exact bodies
  require upgrade policy, not just adding a field.
- Direct history, batched writes, presence/notices and buffered revocation all need
  isolation; a Hub map is insufficient.
- Push queries discard user identity; even content-free global alerts reveal
  private activity. Old registrations need explicit defaults.
- No account-partitioned cache exists today; foreground/background cursors, unread
  counts and management controls are new work.
- ntfy and actual-v2.0.0 recovery were absent from the original 7–12 days.
  Replace with [20–31 base / 25–39 contingency days](delivery-plan.md).
- Additive schema alone cannot make rollback private: first non-null message is
  the unsafe boundary. See [ADR 001](adr-001-schema-compatibility.md).
