# ADR 002 — lounge + groups, human-managed, Teams-style reuse

Status: APPROVED by owner review of PR #51, 2026-10-23. Replaces the previous
channel-owner proposal entirely. Implementation remains a later authorized step.

## Rights and lifecycle

Lounge is implicit/null, with no memberships/rename/delete. Groups use UUID IDs
and human users.authelia_sub (text) or bot bot_principals.id (UUID) memberships.
No grants by matching usernames/display labels. Current members read all retained
history, including pre-join, and send. Bots additionally need fresh global LDAP
eligibility. Creator provenance grants no special role.

Any human member may rename, add/remove humans or bots, and leave. Bots participate
only; no group creation/management/principal lookup/initiation. No owner/admin
roles, last-owner protection, audit rows, archive, hard deletion, invitations or
block lists. Future bot initiation requires a separate AGENTS Basic-scope decision;
future roles/archive must be additive, not built now.

Creation requires caller plus at least one distinct selected principal. To reconcile
free leave with minimum creation size, validate >=2 at creation; subsequent removal
may shrink a retained group. A surviving human keeps ordinary read/manage/send
rights and can add others. If the last human leaves/is removed, keep all conversation,
message and remaining bot rows but deny all group access, including bot sync/send/
push. Invalidate every group ticket/socket then. No recovery UI. Concurrent last-human
removals produce this inaccessible state, never a protected-last-owner error.

## Exact-set reuse

Start-chat has no name; normalize omitted/null/trimmed-empty name to unnamed where
names are accepted. Caller is automatically included. Validate existing typed IDs,
reject duplicate selections and require >=2 distinct final members.

One transaction takes an advisory transaction lock over a canonical hash of the
sorted typed set (length-delimited kind/id; preserve human sub, canonicalize bot
UUID). Re-check current memberships and name IS NULL inside the lock; return exact
match or atomically insert conversation and memberships. At least 20 simultaneous
identical starts return one UUID and insert one group. Hash collisions only serialize
unrelated operations; exact set comparison is authoritative.

Name/member mutations share that coordination: acquire old/new set-hash locks in
deterministic order, then conversation-row lock, re-read and retry if the observed
set changed. Start-chat rechecks candidate rows after locking. Test bounded retries,
deadlocks and start/create racing rename/remove/add. No schema uniqueness on sets.

Named creation always creates fresh, including identical sets. Naming an unnamed
group removes it from matching; clearing the name makes it matchable again.
Membership changes change its next match. If history of edits produces several
unnamed matches, choose earliest (created_at,id) deterministically; do not merge,
delete or rewrite them. Concurrent future starts choose that same match.

## Personal state and schema

Human user-state keyed by account and conversation stores muted=false and read
watermark (UTC timestamp/UUID). Handle null-lounge uniqueness with partial indexes
or equivalent, not ordinary NULL uniqueness. Mute suppresses provider/local alerts,
not unread counting or read/send rights. Read markers advance monotonically; reject
future/cross-conversation IDs. Bots never use these management/state endpoints.

Owner refinement, 2026-10-25: deleting a human cascades only their memberships
and personal state, preserving the existing R2 sender-message deletion behavior.
Bot and conversation references use RESTRICT. Creator kind/id is retained
provenance without a live-principal FK, so creator deletion cannot erase a group
or block human deletion. Explicit member removal deletes only that membership.
See the [schema story log](log/2026-10-25-UST-2.1.1.2.md) for the owner decision.
Migrations are additive, partial/repeated-up/down-up safe and non-destructive down.
