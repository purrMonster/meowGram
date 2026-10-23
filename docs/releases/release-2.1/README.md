# Release 2.1 — lounge + groups

Owner decisions APPROVED, 2026-10-23. This revision is planning only: update
docs/r2.1-plan and draft PR #51 into main, then STOP for review. No implementation,
migration files, release/epic branches or changes to AGENTS.md/playbook.

## Approved scope

- Lounge is channel_id=null: every authenticated human and LDAP-eligible bot;
  no memberships/rename/delete; all existing history stays here.
- Everything else is a group, created with at least two typed members. A 1:1 is
  a two-member group, not a DM type. UUID channel_id remains opaque to clients.
- Current members have equal read/send rights, including retained pre-join
  history. Any human member can rename, add/remove members or leave. Bots only
  participate via GET sync/POST send, never manage or initiate conversations.
- Unnamed start-chat reuses the exact current member set; named groups always
  create fresh. Concurrent starts are race-safe. No member-set uniqueness index.
- Human conversation list/principal picker/management; per-user mute/read state;
  Flutter unread/offline/drafts and account/server/conversation-scoped caches.
- Explicit scope=member merged sync; omitted channel/scope remains lounge-only.
- One conversation per socket; removal invalidates group tickets and closes
  sockets through Hub. Single server, no per-frame generation barrier.
- Content-free per-conversation push excludes sender/muted users. Legacy devices
  receive lounge only. iPhone ntfy: protected per-user topic/relay, pinned synthetic
  fixture after owner version inventory, disabled by default, no iOS FCM in ntfy mode.
- Fail-closed startup APPROVED (ADR 001); exact pre-upgrade replay bytes exception
  APPROVED; quarantine unattributed old cache APPROVED.
- R2.1 playbook translations, docs-only planning evidence and R2 log/fold convention
  APPROVED. Implementation sessions require working Docker, Go 1.27.1 and Flutter;
  missing suites means stop before claiming implementation DONE.

Excluded: owner/admin roles, last-owner protection, audit rows, archive, hard
conversation deletion, invitations, block lists, media/Immich, structured mentions
and recovery UI. A group with no human members retains rows but denies ALL access.
Bot initiation, roles and archive remain future additive options.

## Estimate and risks

Base **21–32 days (168–256 hours)**; +25% **26.25–40 days (210–320 hours)**;
rounded up to half-days **26.5–40 days (212–320 hours)**. Fourteen implementation
stories include tests/review/folds, excluding this planning session and elapsed
owner/CI waits. Reserve 2–3 owner hours for separate timed live checks.

1. Legacy privacy and merged cursors: member scope must never become the default;
   membership changes require authorized backfill without exposing other groups.
2. Reuse concurrency: name/member edits must coordinate with exact-set creation
   without a schema uniqueness constraint.
3. Upgrade/recovery and devices: forced old binaries expose group rows; cache
   migration and locked-phone ntfy delivery need real evidence.

Removed roles/audit/archive/barriers reduce work; reuse, merged sync, server
unread/mute and expanded client flows add work. See the per-story arithmetic.

## Baseline and approved process

PR #51 remote head verified at 7991c0d; main e094dfa contains #50; v2.0.0 peels
to be15cec (#49). R2 publication/rollback evidence in runbook §§14.1–14.2 is
historical, not fresh tests or deployment verification.

Approved translations: R2.1 stories replace obsolete R2/media order; paths use
release-2.1; this docs PR targets main; matrix rows stay PLANNED until implemented;
task logs fold into runbook through epic PRs. Owner merges all layers. Future
release/r2.1 starts from fetched main after explicit authorization. No product
decision remains open; concrete API/edge-case choices are documented for review.

[Model ADR](adr-002-membership.md) · [Startup ADR](adr-001-schema-compatibility.md) ·
[Code map/API/privacy](channels-proposal.md) · [Delivery plan](delivery-plan.md) ·
[Verification matrix](verification-matrix.md) · [Owner checks](owner-checks.md) ·
[pourOver handoff](pourover-note.md) · [Merge queue](merge-queue.md).
