# R2.1 channels proposal — planning only

Owner-requested planning, 2026-10-20. No schema, API or Flutter changes are
implemented or authorized for implementation in this step. R2 remains one lounge.
The wire field is **channel_id**, matching pourOver; recommend PostgreSQL UUID
internally, serialized as an opaque string. Clients must not infer meaning from it.

## Model and compatibility

- channels: UUID id, name, creator principal, created_at and optional archived_at.
  Names are labels, not identity. Avoid implicit global name-based access grants.
- channel_memberships: channel_id FK, principal_kind (human/bot), matching human
  or bot principal FK (exactly one, like message senders), role (owner/member),
  created_at, added_by. Unique channel + typed principal. Prevent mixed/missing
  principal references and keep member-management audit history.
- messages.channel_id: nullable UUID FK to channels. **NULL means the existing
  shared lounge**, not an all-channels wildcard. A non-null value selects one
  channel. No separate lounge row, no rewriting old sender IDs or timestamps.
- Existing rows retain NULL via an additive nullable-column migration; existing
  lounge history, UUID ordering and cache remain intact. New indexes support
  (channel_id, created_at, id), including lounge queries. Migration must be
  repeatable after partial application/down-up and tested against legacy rows.
  No destructive down migration. Default/omitted channel remains the lounge.

Human OIDC access and the bot's global LLDAP group are prerequisites, not channel
membership grants. Human and bot IDs are distinguished by sender_kind + sender_id;
matching usernames do not grant access. Channel membership applies equally to both.

## Send and sync

Send extends the R2 JSON body with optional channel_id. Omitted or null means
lounge; a UUID string selects a channel. Resolve and authorize server-side before
idempotent replay or write; never trust a client-selected sender. Include channel_id
in the idempotency fingerprint: reuse of a principal/key with a different channel
is 409. Membership removal must also prevent replay of an old successful response.

Sync accepts optional channel_id query filter. Omission means lounge only. A
well-formed channel for which the authenticated caller is not a member returns
**403**, never 200 [] and never a silently filtered success. Recommended consistent
403 for unknown well-formed channel IDs avoids an existence distinction; malformed
IDs return 400. Preserve R2 timestamp/UUID pagination, ≤500 rows and X-Has-More
within the selected channel. A member with no new messages gets 200 [].
Every page checks current membership. Keep a separate persisted cursor per channel;
changing channels must not reuse another channel's cursor. Wire messages carry
channel_id (null for lounge); legacy missing fields deserialize as lounge.

## Bot membership management

Recommend authenticated human channel-owner management endpoints to add/remove
typed principal IDs and list memberships, with authorization and audit tests.
Bots cannot self-enroll, invite users, create channels or manage memberships.
Global bot group membership is still required on every Basic request; its removal
blocks access even if channel membership remains. Removing channel membership
does not remove a bot from the lounge or other channels. Channel owners cannot
grant global LDAP eligibility. Prevent removal of the last human owner without
an explicit ownership transfer; provisioning/administrative policy needs review
before implementation, not live credential changes in this proposal.

## WebSocket and push isolation

Retain the sole Hub registry owner and per-client ReadPump/WritePump. Add
authorized per-channel subscriptions for humans; bots continue HTTP polling.
Publish only to subscribed, currently authorized members of the message's channel.
The lounge subscription preserves current behaviour. On membership removal,
invalidate subscriptions and stop further fan-out; use revocation events plus an
authoritative check at delivery boundaries to avoid stale authorization. Test
revocation races and reconnects without blocking the hub or weakening eviction.

Push recipients must belong to that channel and have current valid device leases;
re-check membership before provider dispatch. Scope coalescing by channel and
preserve content-free notifications. Already-delivered notifications/downloaded
messages cannot be recalled. Channel membership must never expand provider topics
or leak recipients/content to unrelated channels. Historical reads remain guarded.

## Flutter impact and migration

Add channel selection/list, membership-aware navigation, unread state and per-channel
sync subscriptions/cursors. Extend ChatMessage with nullable channel_id, partition
account caches by channel and UUID, and route pending sends to their original
channel. Never silently retry a removed channel's send into the lounge. Show a
clear forbidden state on 403, cancel its queued sends/subscription, and clear its
visible/cache content where feasible. Cached bytes cannot be retroactively erased
from an uncontrolled client. Keep first-frame cached lounge rendering and adaptive
layout/keyboard handling. On upgrade, existing cache entries lacking channel_id
remain in the lounge; do not duplicate or reorder them. Notification navigation
must authorize the destination. Old clients remain lounge-only; channel events
must never enter their implicit lounge subscriptions.

## Acceptance and estimate

Test migration legacy preservation and down/up reruns; two channels plus lounge
with equal timestamps/pagination; 403 nonmembers; independent cursors; sender/key
isolation and cross-channel conflicts; authorization before replay; bot management
denials; human membership operations; revocation between pages and during fan-out;
no cross-channel socket/push leakage; old client/cache compatibility and offline UI.

| Work (including tests) | Focused days |
|---|---:|
| Schema, membership rules and management endpoints | 1.5–2.5 |
| Send/sync filtering, replay and authorization | 1–2 |
| WebSocket/push isolation and revocation | 1.5–2.5 |
| Flutter channel navigation/cache/cursors and states | 2–3 |
| Integrated regression, documentation and rollout | 1–2 |
| Base | 7–12 |
| With 25% contingency, rounded up to half-days | 9–15 |

Eight-hour days: base 56–96 hours; contingency budget 72–120 hours. This is
separate from R2 and R3. Depends on the integrated R2 message API; implementation
branch/release planning comes later. No media dependency or R2 scope expansion.
