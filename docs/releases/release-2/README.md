# Release 2: authenticated message API

Owner-approved scope split, 2026-10-20. Goal: **pourOver live in the shared lounge**.
This decision supersedes the older media-inclusive R2 scope, including scope
statements in the unchanged AGENT_PLAYBOOK.md. Proposed playbook edits are in
the scope-split PR body for owner approval; AGENTS.md is unchanged.

| Release | Scope | State |
|---|---|---|
| R2 | Epics 2.3 bot identity, 2.4 message API, 2.5 API verification/release | Implementation complete; epic 2.5 fold/integration and draft RC review remain |
| R2.1 | Channels | Planning only; [proposal](../release-2.1/channels-proposal.md) |
| R3 | Epics 2.1 Immich backend and 2.2 Flutter media, unchanged | Deferred to R3; #32 contract stands |

#34–#46 are owner-merged into their declared parents. Identity and message API
are combined on release/r2 at e74a1de. Epic 2.5 at b9bfc99 includes the release
parent sync, real-server/PostgreSQL/LDAPS acceptance, rollback proofs, GHCR
configuration, image inspection and owner rollout procedures. Basic remains
**disabled by default** until a separately authorized rollout/pilot.

Read the [delivery plan](delivery-plan.md), [API contract](api-design.md),
[merge queue](merge-queue.md), [verification matrix](verification-matrix.md),
[rollout and recovery](rollout.md), and [owner checks](owner-checks.md).
Next: owner merge the epic 2.5 documentation fold, then epic 2.5 → release/r2.
Only then open release/r2 → main as a **DRAFT** for owner/Claude review.
The candidate version is 2.0.0+4; eventual owner tag v2.0.0. No deployment,
publication or live pilot has occurred. Old-image rollback requires the guarded
human-only schema-version reset; after bot messages it requires full restore,
losing post-backup changes. Disabling Basic on the current image needs neither.

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
