# R2.1 API contract freeze

UST-2.1.1.1 fixtures: [HTTP matrix](../../../server/contracts/r21/http.json),
[execution and seed contract](../../../server/contracts/r21/README.md).
The approved [design](channels-proposal.md) remains authoritative for later
concurrency, persistence, pagination, revocation and recovery implementation,
subject to the owner correction below.

The cases freeze method, route, raw body/query, actor and expected status.
Legacy selector omission stays lounge-only. Conflicting/repeated parameters,
literal null and malformed UUIDs reject with 400. Unknown and inaccessible groups
use identical 403 bodies. Message rows on sync, send, history and live delivery
contain message fields plus `channel_id` only. Sync headers are explicitly named:
`X-Membership-Version` on member pages including empty pages, and `X-Has-More`.

Owner correction, 2026-10-23: `conversation_member_count` and
`conversation_muted` are absent from every message row. Conversation name,
member count, mute and unread state belong to `GET /api/conversations` and its
members route as appropriate, never to the stored message response. Per-viewer
state would become stale in an idempotent stored replay. This supersedes the
earlier plan's row metadata and its bot prefix assumption based on sync-row
member counts. Bots retain send/sync access only; no conversation-list or
management access is added to obtain that metadata.

Owner resolution, 2026-10-27: prefix always required in lounge and all groups,
including two-member groups. Prefix-free replies need a future, owner-approved
bot discovery mechanism (would widen the Basic Auth scope / AGENTS.md).
See the [resolved contract log](log/2026-10-27-UST-2.1.1.1-prefix-resolution.md).

Replay fixtures can perform successful setup sends with the same per-case
idempotency key before the asserted request. Changed text or another readable
channel must conflict with the stored request; authorization of both requested
and stored channels precedes conflict or replay. The revoked stored-channel case
returns 403 even when the retry selects lounge. The seed contract names the groups.

Default fixture integrity tests run in existing Go suites. Tagged `r21_contracts`
HTTP replay is pending implementation and expected to fail against R2.0;
it is not enabled in CI. A compile check is not an HTTP compliance pass.
See the linked README for the per-case disposable reset/actor/seed adapter.
No production endpoint or migration is added by this story.
