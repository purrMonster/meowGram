# R2.1 conversations — lounge + groups design

Owner model APPROVED, 2026-10-23. API/implementation design for review; no code.
Conversation is the product term; **channel_id remains the wire field/parameter**.
[ADR 002](adr-002-membership.md) defines equal membership rights and reuse.
[ADR 001](adr-001-schema-compatibility.md) defines approved startup/recovery policy.

## 1. Current code map

Paths are repository-relative, inspected for the original plan and retained here
with revised change targets. None of these proposed behaviors is implemented yet.

| Area / symbols | Current behavior | Required change |
|---|---|---|
| server/internal/chat/hub.go Run/deliver/pushWorker | Global live/direct queues; global push signal/30s throttle | Route history/live/presence/notices by conversation; close revoked sockets through Hub; scoped coalescing |
| chat/message_service.go, http_send.go | Shared budget/validation; commit then publish; replay no publication | Carry channel_id; check access on replay and new write |
| chat/client.go ReadPump/WritePump/SendHistory | Raw/JSON text; global recent 50; writer batches frames | Bind conversation, safe structured send validation; terminate revoked sockets without draining private queues |
| handler/sync.go, send.go, sync_route.go | Unfiltered sync, text-only strict POST, default/custom aliases | Nullable scope plus explicit member stream; reject all other unknown body fields |
| handler/ws_ticket.go, websocket.go, auth/ws_ticket.go | One-use user ticket, register/history/pumps | Bind scope; per-principal/group invalidation; consume-time membership check |
| handler/push_devices.go, repository/device_repository.go | 24h device lease; globally returns tokens without user identity | Retain recipient/capability; user mute/member filtering, default old capability lounge-only |
| server/cmd/server/main.go | One Hub, FCM publisher, shared auth/API budgets | Wire conversations/state/routes/provider fan-out; never broaden Basic |
| repository/message_repository.go GetRecent/GetMessagesAfter | Global timestamp/UUID queries | Scope BEFORE cursor/LIMIT; merged authorized stream uses EXISTS, no duplicate rows |
| repository/idempotency.go CreateIdempotent | Raw-text hash; principal/key lock; exact bytes; callback skips replay | Separate membership check; old-null compatible/non-null versioned fingerprints |
| repository/bot_repository.go, botauth/http.go | Stable typed bot identity; fresh LDAP/group check; budgets | Keep global gate; DB group access also requires a remaining human; member stream within budgets |
| database/db.go, migrations 5/6 | migrate.Up; preserving down files | Approved rejection policy; additive conversations/memberships/personal state/channel column |
| fcm/service.go | All active tokens in 500 batches, lounge collapse tag | Current eligible unmuted recipients minus sender; new capability; generic payload |
| client/lib/src/models/chat_message.dart | No channel field | Nullable channel parse/serialize/equality; legacy absent -> lounge |
| bloc/chat_bloc.dart | One timeline/roster, async cache writes | Conversation state/drafts/read markers; client account/conversation generations |
| services/sync_service.dart | Newest-cache cursor before burst, skips empty cache, one in-flight sync | Completed-page cursors; merged member cursor; per-group backfill and explicit 403 |
| storage/local_message_repository.dart | One v1 Hive box, UUID keys, 2000-row target, UI isolate | New account/server/conversation storage; quarantine unowned cache |
| services/background_message_service.dart | Android opt-in; single secure cursor; no Hive | scope=member checkpoint plus per-conversation positions; suppress own/muted alerts |
| push/fcm_service.dart, device_registration_client.dart | Android/iOS/macOS FCM | Capability and iOS ntfy mode; Android/macOS retained |
| screens/responsive_layout.dart, chat_screen.dart, widgets/sidebar.dart | Static lounge/zero unread; one Bloc; 800px layout | Lounge pinned, groups/activity/unread/mute; picker/start/named/rename/member/leave flows |

Tests: chat safety/service; handler send/sync/tickets/device; repository DB/migration/
idempotency; botauth real LDAP; FCM fakes; client sync/cache/notifications/UI regressions.
server/integration/api_acceptance_test.go and rollback_test.go run actual binaries;
deploy/scripts/verify*.sh/.ps1 remain scratch-only. Replace channel_id_400 with
acceptance of the single new field while retaining all other strict validation.

## 2. Schema and authorization

Add conversations(id UUID, name nullable, created_at UTC, created_by typed);
typed memberships with exactly one human-sub/bot-ID FK, kind check and per-kind
unique indexes; per-human mute/read state including implicit lounge; nullable
messages.channel_id FK and (channel_id,created_at,id) indexes. No membership row for
lounge. Owner refinement (2026-10-25): human deletion cascades their memberships
and state, with existing R2 sender-message deletion unchanged; bot/conversation
references restrict deletion (ADR 002). Preserve all existing
IDs/times/history/keys and response bytes. No roles/audit/archive fields now.
Future roles/archive/bot initiation stay additive, not implemented.

Readable(caller,x): null lounge requires valid human auth or current LDAP-eligible
bot; a group requires current typed membership AND at least one human member.
Unknown well-formed UUID and denied group both return identical 403. Inaccessible
groups are omitted from conversation list and member stream, but explicit group
requests return 403 even when empty. Auth failure 401; malformed input 400;
storage/directory failure sanitized 503 + positive integer Retry-After.
Errors retain the R2 JSON shape {"error":"forbidden","message":"request not permitted"} for 403; unknown well-formed groups use identical status/body. All routes no-store. Every page and replay reauthorizes; group membership does
not replace global LDAP eligibility. No display-name-derived grants.

Mutations/reuse follow ADR 002's one-transaction locks, exact current-set comparison,
deterministic duplicate-match selection and no uniqueness constraint on sets.
Membership changes increment a per-principal membership revision transactionally,
including bot/human access changes when the last human leaves. This is sync
invalidation metadata, not roles/audit or a per-frame authorization barrier.

## 3. Exact HTTP route and shape contract

Paths below are default routes, configurable via existing send/sync/ticket settings
and proposed CONVERSATIONS_ENDPOINT=/api/conversations and
PRINCIPALS_ENDPOINT=/api/principals. State/member routes are children of the
configured conversation prefix. Validate reserved-path collisions; config.go,
.env.example, Compose and runbook fold must agree. IDs in examples are placeholders.

Typed reference: {"kind":"human","id":"opaque-sub"} or {"kind":"bot","id":"<uuid>"}.
Member result adds "display_label"; no credentials/directory internals.
Names trim to NULL if empty and otherwise allow 1–100 Unicode runes. Selected IDs
must exist; reject repeated typed refs (caller selected once is folded into caller).
Human IDs remain exact strings, bot UUIDs canonicalized.

Message response M:
{"id":"<uuid>","sender_id":"<id>","sender_kind":"human|bot","username":"<label>",
"text_content":"...","created_at":"<UTC RFC3339>","channel_id":null|"<uuid>"}.
Only legacy replay bytes omit channel_id, under the approved exception.

| Method/default route | Request | Success response |
|---|---|---|
| POST /api/messages | Idempotency-Key; {"text_content":"...","channel_id":null or UUID}, channel optional | 201 M; replay exact stored status/body |
| GET /api/messages/sync | after required; after_id optional; selectors below | 200 array of M; oldest (created_at,id), cap 500; X-Has-More true on full page; [] empty |
| GET /api/conversations | limit default 50/max100; cursor optional opaque | 200 {"conversations":[C...],"next_cursor":null or opaque} |
| POST /api/conversations | {"name":"nonblank","members":[typed refs]} | 201 C, always fresh named group |
| POST /api/conversations/start-chat | {"members":[typed refs]} (no name field) | 200 {"conversation":C,"created":true or false}; same shape for new/reused |
| PATCH /api/conversations/{channel_id} | {"name":"text" or null}; empty trimmed -> null | 200 C |
| GET /api/conversations/{channel_id}/members | limit default50/max100; cursor optional | 200 {"members":[typed refs+display_label],"next_cursor":null or opaque} |
| POST /api/conversations/{channel_id}/members | one typed reference | 204 (existing member is idempotent no-op) |
| DELETE /api/conversations/{channel_id}/members/{kind}/{id} | no body; path ID percent-encoded, human opaque segments handled safely | 204; caller must be current human member; absent target no-op |
| POST /api/conversations/{channel_id}/leave | no body | 204; remove authenticated caller only |
| PUT /api/conversations/{channel_id-or-lounge}/mute | {"muted":true or false} | 204; state only for authenticated human caller |
| PUT /api/conversations/{channel_id-or-lounge}/read | {"message_id":"<uuid>"} | 204; same-conversation persisted message, advance max watermark |
| GET /api/principals | q (1–100 chars), limit default20/max50, opaque cursor optional | 200 {"principals":[typed refs+display_label],"next_cursor":null or opaque} |
| POST /api/ws-ticket | optional {"channel_id":null or UUID}; old empty body retained | 200 {"ticket":"<opaque>","expires_in":30}; existing expiry retained |

C = {"channel_id":null|UUID,"kind":"lounge"|"group","name":null|string,
"member_summary":{"count":integer,"members":[up to 10 typed refs+display_label]},
"last_activity_at":UTC timestamp|null,"unread_count":nonnegative integer,
"muted":boolean}. Lounge first on first page only, then groups by last activity
descending, UUID tie-break; no-message group activity uses created_at. Lounge
member summary represents provisioned human/bot principals without live
LDAP enumeration; count is not an authorization grant. No group name is invented
for unnamed groups; UI derives a label from member summary. All management/list/
picker/state routes are human-bearer-only; Basic rejected regardless of method/
aliases. Every human member may mutate the group; no owner tests. Literal lounge
path token is reserved for personal state only, maps to null, never a stored UUID.
Unknown fields/duplicate JSON fields and repeated query parameters reject with 400.

Picker returns bounded minimal typed IDs/labels from existing provisioned records,
rate-limited; it never provisions accounts or exposes LDAP credentials. Selecting a
bot does not guarantee it is presently LDAP-eligible. Group creation returns 400
for unknown principal selection; no group existence oracle via nonmember requests.

Sync selectors (default and custom aliases identical):
- No channel_id and no scope -> lounge only, WHERE channel_id IS NULL before
  parenthesized cursor predicate and LIMIT.
- channel_id=<uuid> -> one authorized group.
- scope=member -> lounge plus all currently readable groups in ONE merged stream.
- channel_id plus scope, repeated parameters, empty selector, literal null, invalid
  UUID, unknown scope or duplicate after/after_id -> 400. Omit selectors for lounge.
  This supersedes the previous proposal accepting empty channel_id=.

Send omission/null -> lounge; empty string, wrong type, malformed/duplicate field
->400. Accept only text_content and channel_id; spoofed fields remain rejected.

For bot trigger policy without granting bots a conversation-list endpoint, sync
rows additionally carry conversation_member_count (integer for group, null for
lounge), from the current authorized page snapshot. This derived metadata is not
persisted message identity and is not added to exact send replay bytes. Sync rows
for human callers additionally carry conversation_muted boolean for background
notification filtering. These explicit read-path fields avoid inferring membership
size from observed senders and do not widen Basic beyond sync/send.

## 4. Merged cursor, unread and idempotency

Member sync orders globally by (created_at,id), not one limit per conversation.
Use EXISTS authorization predicates to avoid duplicate rows. Maintain one merged
checkpoint per account/server/scope, plus per-conversation completed cursors.
Never use the maximum of independent conversation cursors as the merged cursor.
Commit page data and checkpoint recoverably before moving on. 20 polls/minute plus
at least 10 replies must fit one bot's budgets without steady 429; paginate/retry
without advancing on failure.

Membership addition can expose history older than a global checkpoint. Return
X-Membership-Version (opaque revision) on member pages, including []. Clients keep
it with the merged cursor. A changed/missing revision on upgrade triggers a replay
from epoch, deduplicating UUIDs; humans may instead backfill explicitly listed new
groups with their own cursors. A change mid-pagination restarts from epoch under
the new revision. If a response races a membership change, the next request detects
it; never advance a checkpoint that would permanently skip newly readable history.
Removed groups are filtered on every page; no-human groups disappear. Read
metadata/counts reflect that page's authorized snapshot, not a perpetual guarantee.
Expose the new header in CORS only for existing approved origins.

Unread is server-derived per human/conversation: count non-self messages strictly
after a persisted read watermark, default epoch (retained history initially unread).
Read updates take maximum of old/new position, never future or cross-group IDs.
Return accurate counts even if local cache is pruned; update only when visible/
foreground and message actually rendered. Mute does not mark read or reduce badges.
Offline badges are last-known/stale. Refresh list on resume, switch and bounded
foreground poll (proposed configurable 30s) and after management/read/mute changes.

Idempotency remains (kind,id,key) across ALL conversations. Under transaction lock,
authorize requested and stored channel before replay or insert. Removed member
cannot retrieve old bytes by selecting lounge. Unauthorized ->403 first; otherwise
changed text/channel ->409. Fresh LDAP on every bot retry. Write-budget callback
remains new-write-only, separate from authorization. Null hash stays R2 raw-text
compatible. Non-null hash uses versioned canonical UUID + length-delimited raw text.
Pre-upgrade responses replay EXACT stored bytes; missing channel means lounge.
No row rewrite. >=20 duplicates -> one row/publication/push; restart, expiry,
partial failures and stopped-Hub durable success retained.

## 5. Socket revocation and legacy privacy

Ticket binds conversation; recheck on use. Old empty requests bind lounge. Reject
URL scope override. One socket per selected conversation; switch obtains a new
ticket/socket. Incoming structured send must agree with bound scope; malformed
JSON must not silently fall back to lounge. Client account/conversation generations
discard late callbacks; these are UI guards, NOT server per-frame barriers.

On remove/leave, commit membership change, invalidate that principal's outstanding
group tickets, then send a Hub control event to close all their sockets for it.
A last-human departure invalidates/closes ALL group tickets/sockets. Recheck on
ticket consumption and coordinate registration with invalidation to close a
consume/register race. Hub alone owns registries and closes queues; socket removal
also cancels writer/connection so buffered frames are not deliberately drained.
Acknowledge management success after invalidation/Hub closure handling. On failure
report retriable failure while persisted HTTP authorization still denies access.
No SQL or provider calls in Hub; retain 256-frame eviction and sole writer/pings.

No per-frame membership generation barrier. HTTP checks membership per request;
an authorized in-flight HTTP response/write or provider delivery may finish after
removal. Already sent/downloaded data cannot be recalled. Test this explicit boundary
without claiming revocation erases old bytes. Multi-replica deployments require
distributed ticket/socket invalidation and are not supported by this design.

| Legacy path | Required isolation | Falsification check |
|---|---|---|
| Default/custom sync without channel_id/scope | Lounge predicate before cursor/LIMIT | 1001 equal-time A/B rows interleaved with lounge never leak |
| Old ticket/reconnect/initial 50/live | Implicit null scope | Private canaries inserted before/after connect absent |
| Presence/notices/direct history | Conversation-scoped routing | No group labels/members/notices on lounge socket |
| Capability absent, including renewed registration | Lounge-only push | Zero private-activity calls to old device |
| Android background worker | Its omitted selectors stay lounge-only | No group-derived local alert |
| v2.0.0 decoder/cache | Server delivers lounge only | Actual old fixtures persist no group IDs/text |

Keep lounge + A/B, humans A-only/B-only/both/neither, bots, old/new/expired devices,
equal timestamps across page boundaries. Capture real-binary HTTP/socket/provider
output and actual v2.0.0 decoder/cache/background fixtures. Assert forbidden canaries
ABSENT and allowed traffic PRESENT; broken delivery must not pass. Repeat after
reconnect, history, revocation and replay. Explicit member scope is opt-in only;
never use client filtering as authorization.

## 6. Push and ntfy

Recipients: current readable members (all humans for lounge), minus typed sender,
minus per-user muted conversation. Current leases and channel-aware capability for
FCM; absent capability permits lounge only. Bots share group read/send flow but
receive no device/ntfy registration API. ntfy maps only human recipients.

Per-conversation bounded coalescing: default configurable 30s plus trailing signal;
busy group cannot suppress another. Pending signals retain enough sender/event
eligibility to avoid suppressing a legitimate other-sender alert in a mixed burst.
Recheck membership/mute/lease immediately before each dispatch/retry; do not snapshot
recipients at enqueue time. Revoked/no-human/muted targets must not get new sends.
Provider in-flight calls cannot be recalled. Replay emits no signal.

Content-free payload: meowGram / New message, no sender/text/group label/ID/member
count. Generic device collapse tag; scheduling is still per-conversation. Provider
failure cannot undo persistence or block Hub; bounded retries, no exactly-once push
promise. Sync repairs durable message gaps.

ntfy starts with owner versions + pinned synthetic server/relay fixture. Separate
iOS ntfy app, protected per-user opaque topic, publisher write-only/user read-only
ACL, anonymous denied. Private host-mounted immutable-human-sub/topic mapping and
separate token file, no arbitrary client URL or public identity/topic mapping.
Proposed NTFY_ENABLED=false, NTFY_BASE_URL, NTFY_TOKEN_FILE, NTFY_RECIPIENTS_FILE,
NTFY_TIMEOUT=5s, PUSH_COALESCE_INTERVAL=30s; validate HTTPS/fixed origin/no redirects,
topic syntax, bounded requests and sanitized errors. Disabled needs no secret.
Owner-only upstream configuration; relay timing/routing metadata remains observable.
Contract reference: [ntfy iOS configuration](https://docs.ntfy.sh/config/#ios-instant-notifications),
reviewed in original planning session; installed versions still require owner input.

ntfy subscription is separate from meowGram logout/FCM lease; removing private
mapping stops dispatch. iOS ntfy mode skips FCM registration and duplicate local
alerts; Android/macOS remain tested. ntfy tap opens ntfy; no assumed meowGram deep
link. Real protected-topic locked-phone delivery is an owner check, not a fake pass.

## 7. Flutter and pourOver

Lounge pinned first; groups ordered by last activity, unread/mute indicators and
typed Bot labels. Implement named group, start-chat picker/reuse, rename/clear,
add/remove/leave and mute. No role/archive UI.

New cache namespace: server origin + issuer/sub + conversation. Legacy v1 rows
attributable to current account/server import only into lounge; otherwise quarantine
untouched and resync (APPROVED). Restart-safe import, no UUID/time/text rewriting;
never group rows in v1 box. Cached first frame precedes network. New cache per-group
and account caps retain completed/read cursor state independently.

Per-conversation drafts/offline/403 state; cancel stale-generation writes and
requests on switch/account change. On removal/403 clear display and affected cache,
disable sends, keep any unsent draft visibly unavailable; never retry into lounge.
Reconnect captures completed cursor before 50-message burst, dedup by UUID and sort
UTC/time then UUID. Live/history cannot advance REST cursor past an unsynced gap.

Android background uses scope=member with merged checkpoint and per-conversation
secure-storage positions, separate from UI Hive. Filter own/muted alerts using
page metadata; refresh membership revision on empty pages too. Clear account/server
state on logout/switch. Legacy installs remain lounge-only. No iOS background poll.
Preserve SafeArea/viewInsets and 800px behavior.

[pourOver note](pourover-note.md): another repository, docs only. Poll scope=member,
reply with triggering message channel_id, require the trigger prefix in every
conversation (including two-member groups), and require a human to add it first.
Owner resolution 2026-10-27: prefix-free behavior needs a future approved bot
discovery mechanism; no bot initiation or wider Basic Auth scope in R2.1.

### Device and socket envelope details

Existing PUT /api/push/devices accepts {"token":"<device-token>",
"capabilities":{"conversations":true}} for upgraded clients; omitted capabilities
means lounge-only. DELETE retains {"token":"<device-token>"}. Both return 204 and
remain bearer-only with configured PUSH_DEVICE_ENDPOINT and 24-hour lease. Reject
unknown capability names/types; an old renewal explicitly resets capability to
lounge-only so an app downgrade cannot inherit private activity support.

WS chat/history frames are M plus type=chat/history; system/presence/error frames
also carry the bound channel_id, and presence users retain their existing shape.
No private group metadata in lounge frames. Ticket request channel validation matches
POST send; consumption cannot change it. New client payload {"text_content":"...",
"channel_id":null or UUID} must match the socket; old raw/text-only payloads use
its bound scope. Network handoff and old-client local notification behavior cannot
be retroactively changed: server push respects mute for old devices, but an old
background worker may still locally notify on readable lounge text. New clients
suppress muted local notifications; mute is not a read filter.
