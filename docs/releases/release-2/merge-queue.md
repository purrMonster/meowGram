# Release merge queue — owner-approved split, 2026-10-20

Only the owner merges. R2 is API-only; R2.1 channels is planning only; media
is deferred to R3. #27–#33 are merged into their original parents. No published
branch is renamed or deleted. Do not merge epic 2.1/2.2 into release/r2.

## R2 queue and dependencies

Completed: #34–#38 are owner-merged. Identity is integrated in release/r2.

Completed #39 Message API sync PR: chore/r2-message-api-sync → feat/r2-epic-2.4-message-api | merge after nothing | merges release/r2 identity, preserves shared service and history, fixes SenderKind publication with bot/human regression. Use a merge commit to retain synchronization ancestry.
1. #40 feat/ust-2.4.2-http-send-idempotency → feat/r2-epic-2.4-message-api | merge after nothing (#39 merged) | verified authenticated send and durable idempotency.
2. #41 feat/ust-2.4.3-bot-sync-example → feat/r2-epic-2.4-message-api | stacked on/merge after #40 | verified bot sync, cursor edge cases, durable example and backoff; merge updated remote epic into task after #40.
4. Epic 2.4 doc-fold and verified epic → release/r2 | after API task merges | complete API implementation.
5. Sync integrated API into epic 2.5 through owner PR; UST-2.5.1/2.5.2 tasks → epic 2.5, doc-fold, verified epic → release/r2 | API-only acceptance, owner pilot instructions and release docs.
6. release/r2 → main | after API release gates | owner-reviewed release candidate; separately authorized pourOver live pilot.

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
