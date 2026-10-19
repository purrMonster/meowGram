# Release merge queue — owner-approved split, 2026-10-20

Only the owner merges. R2 is API-only; R2.1 channels is planning only; media
is deferred to R3. #27–#32 are merged into their original parents. No published
branch is renamed or deleted. Do not merge epic 2.1/2.2 into release/r2.

## R2 queue and dependencies

1. #33 Scope-split PR: docs/r2-scope-split → release/r2 | merge after nothing | records approved scope and pourOver contract.
2. NEW UST-2.3.4: task from fetched feat/r2-epic-2.3-bot-identity → that epic | before epic release PR | uid-based stable usernames; no per-resolve overwrite; rename/recreation/human regressions; any migration additive and re-runnable with real golang-migrate tests.
3. Epic 2.3 documentation fold, if needed: docs task → epic 2.3 | after 2.3.4 | fold logs/config, reconcile older release-plan files with the scope PR.
4. feat/r2-epic-2.3-bot-identity → release/r2 | after 2.3.4 and doc-fold owner merges, full epic verification | identity integration.
5. release/r2 → feat/r2-epic-2.4-message-api sync PR | after identity epic release merge; #31 already merged | unblocks HTTP API.
   Required resolution: MessageService.Send's published WSMessage carries `SenderKind: message.SenderKind`; SendHistory retains SenderKind. Add a store-returning-bot publishes bot / human publishes human regression. Preserve both behaviours when resolving client.go conflicts. Behaviour fixes get a separate fix commit/test; verify full suite. Use the approved sync PR workflow, no direct integration pushes or sibling cherry-picks.
6. UST-2.4.2 task → epic 2.4 | after sync | send + durable idempotency and pourOver httptest acceptance.
7. UST-2.4.3 task → epic 2.4 | after send | bot sync, cursor edge cases, example and backoff acceptance.
8. Epic 2.4 doc-fold and verified epic → release/r2 | after API task merges | complete API implementation.
9. Sync integrated API into epic 2.5 through owner PR; UST-2.5.1/2.5.2 tasks → epic 2.5, doc-fold, verified epic → release/r2 | API-only acceptance, owner pilot instructions and release docs.
10. release/r2 → main | after API release gates | owner-reviewed release candidate; separately authorized pourOver live pilot.

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
