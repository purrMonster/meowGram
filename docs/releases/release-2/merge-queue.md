# Release merge queue — owner-approved split, 2026-10-20

Only the owner merges. R2 is API-only; R2.1 channels is planning only; media
is deferred to R3. #27–#33 are merged into their original parents. No published
branch is renamed or deleted. Do not merge epic 2.1/2.2 into release/r2.

## R2 queue and dependencies

Completed: #34–#41 are owner-merged. Identity is in release/r2; #39 synchronized
identity into epic 2.4, and #40/#41 complete its API tasks at 6f98957.

1. Epic 2.4 doc-fold: docs/r2-epic-2.4-runbook → feat/r2-epic-2.4-message-api | merge first | runbook, delivery status, approved AGENTS wording.
2. Epic 2.4 → release/r2 | merge after doc-fold | integrated API; STOP for owner merges before Step B.
3. release/r2 → test/r2-epic-2.5-verification sync via child PR | after step 2 | replace old-main ancestry by merging, not rebasing; owner merge before tasks.
4. UST-2.5.1 task → epic 2.5 | after sync | combined real-server/PostgreSQL/LDAPS acceptance and matrix with command/result/date.
5. UST-2.5.2 task → epic 2.5 | after acceptance | rollout, rollback/image compatibility, final timed owner checks and pourOver pilot.
6. Epic 2.5 doc-fold → epic 2.5, then epic 2.5 → release/r2 | after task merges | STOP for owner merges.
7. release/r2 → main as DRAFT | after API release gates | owner/Claude review; never mark ready automatically. No deployment/live services/credential/DNS changes.

Merge updated remote parents INTO task branches, never rebase/force-push.
Preserve directory AND bot-auth CI jobs ahead of verify, retain chronological
runbook entries, and require go mod tidy to produce no module diff. #30's prior
conflict resolution is complete. Keep approved scope/contract when older epic
copies of README, delivery-plan or api-design conflict with this scope PR.
Do not start work that cannot finish or park green, pushed and logged this session.

## R2.1 — channels planning only

See ../release-2.1/channels-proposal.md. No channel schema, code or implementation
branch is authorized by this planning step. Field name is channel_id, recommend
UUID internally and opaque string to pourOver. Lounge remains implicit/null.

## R3 — deferred media integration requirements

Epics 2.1 and 2.2 are deferred to R3 unchanged. #32's contract stands on
feat/r2-epic-2.1-media-backend at its existing release-2/media-contract.md path.
Preserve branch names; future epic PRs target release/r3, created ONLY when R3
starts. Their current ancestry does not authorize merging them into release/r2.

At R3 integration, combine owner-checks.md into one file: identity/ingress checks
1–3 first (and retain later API checks), then media M1–M4. Preserve roadmap's
2.3.1 status line AND #32's /api/server/version and placeholder-host corrections,
plus the approved R2/R2.1/R3 split. Resolve these through R3 task/sync PRs and run
appropriate checks. The old ephemeral-media roadmap is an unassigned backlog,
not part of the newly approved R3 Immich scope.
