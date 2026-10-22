# Release 2: authenticated message API

Owner-approved scope split, 2026-10-20. Goal: **pourOver live in the shared lounge**.
This decision supersedes the older media-inclusive R2 scope, including scope
statements in the unchanged AGENT_PLAYBOOK.md. Proposed playbook edits are in
the scope-split PR body for owner approval; AGENTS.md is unchanged.

| Release | Scope | State |
|---|---|---|
| R2 | Epics 2.3 bot identity, 2.4 message API, 2.5 API verification/release | Implementation and integration remaining |
| R2.1 | Channels | Planning only; [proposal](../release-2.1/channels-proposal.md) |
| R3 | Epics 2.1 Immich backend and 2.2 Flutter media, unchanged | Deferred to R3; #32 contract stands |

#27–#32 are owner-merged into their respective parents. Bot identity and the
shared message service are implemented on their epic branches, not yet combined
on release/r2. Bot activation remains blocked until API integration. The media
spike is design evidence, not an implemented media feature.

Read the [delivery plan](delivery-plan.md), [API contract](api-design.md),
[merge queue](merge-queue.md), and [owner checks](owner-checks.md).
Next implementation: **UST-2.3.4 stable bot usernames**, before the identity epic
PR. Then integrate identity, sync the API epic, implement send/idempotency and
bot sync/examples, and complete API-only verification and rollout preparation.

R2 preserves human OIDC and adds bot Basic Auth only to GET sync and POST send,
with fresh directory credentials/group validation on every request. Bots may
post in the shared lounge. No bot OAuth, local passwords, new channels, media,
provisioning UI, granular bot roles or automatic human/bot linking is added.

Do not merge media epic 2.1/2.2 into release/r2. Preserve published branch names;
their eventual epic PRs target release/r3, created only when R3 starts. The
[contract from #32](https://github.com/purrMonster/meowGram/blob/feat/r2-epic-2.1-media-backend/docs/releases/release-2/media-contract.md)
remains authoritative for media despite its historical release-2 path.

Completion means API acceptance tests and release checks pass, the owner merges
the release candidate, and a separately authorized pourOver pilot succeeds.
Scope approval does not authorize live configuration or credential changes.
