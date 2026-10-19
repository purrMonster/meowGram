# Release 2 merge queue

Only the owner merges. Queue verified 2026-10-18; task evidence is in each PR and story log.

1. #27 docs/r2-agent-playbook → release/r2 | merge after nothing | unblocks approved R2 workflow
2. #28 feat/ust-2.3.1-directory-identity → feat/r2-epic-2.3-bot-identity | merge after #27 | unblocks identity/auth integration
3. #29 feat/ust-2.3.3-bot-principals → feat/r2-epic-2.3-bot-identity | merge after #28 | unblocks bot principal persistence
4. #30 feat/ust-2.3.2-bot-auth → feat/r2-epic-2.3-bot-identity | merge after #29 | unblocks bot-identity epic integration
5. #31 refactor/ust-2.4.1-shared-message-service → feat/r2-epic-2.4-message-api | merge after the bot-identity epic PR described below | unblocks shared HTTP/WS send service

6. #32 docs/ust-2.1.0-immich-contract → feat/r2-epic-2.1-media-backend | merge after #27 | unblocks UST-2.1.1–2.2.2; independent media lane

Between #30 and #31: open feat/r2-epic-2.3-bot-identity → release/r2 after #28–#30 are owner-merged and epic verification is green. It cannot be opened yet because the integration branch has no implementation commits. Fold the epic log fragments through its documentation task first where needed.

After #31 and the bot-identity epic merge: open release/r2 → feat/r2-epic-2.4-message-api as a sync PR. Its owner merge unblocks UST-2.4.2. No cross-epic cherry-picks or direct integration pushes.

Existing task PRs append to runbook.md. If an owner merge creates a conflict in a later task, the agent merges the remote epic into that task, preserves both entries, reruns checks and pushes the resolution. The owner need not resolve code conflicts.

## Required resolutions from the 2026-10-19 review

Perform each resolution only after the owner merges its prerequisite. Fetch and merge the updated remote parent INTO the task branch; never rebase or force-push. Preserve both behaviours, rerun checks and push.

- After #28 merges, update #29 from origin/feat/r2-epic-2.3-bot-identity. Preserve both runbook.md entries in date order.
- After #29 merges, update #30 from origin/feat/r2-epic-2.3-bot-identity. In .github/workflows/verify.yml retain BOTH directory (2.3.1) and bot-auth (2.3.2) jobs ahead of verify. Preserve both runbook entries. Run go mod tidy and require no diff in server/go.mod or server/go.sum.
- **At the release/r2 → feat/r2-epic-2.4-message-api sync step above:** reconcile the client.go broadcast move into MessageService.Send with bot principal sender kinds. The WSMessage published by MessageService.Send MUST include `SenderKind: message.SenderKind`; SendHistory MUST retain SenderKind. Add a regression test proving a store returning SenderKind "bot" publishes "bot", and a human message publishes "human". Keep this behaviour fix and its test in a separate fix commit if it goes beyond mechanical conflict resolution. Run the full harness before the owner merges the sync PR.
- When epic 2.1 (including #32) reaches release/r2: merge the two owner-checks.md additions into ONE file, identity/ingress checks 1–3 first, media checks M1–M4 after them. In roadmap.html preserve the 2.3.1 status line AND #32's endpoint and placeholder-host corrections.
