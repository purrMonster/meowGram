# Release 2 merge queue

Only the owner merges. Queue verified 2026-10-18; task evidence is in each PR and story log.

1. #27 docs/r2-agent-playbook → release/r2 | merge after nothing | unblocks approved R2 workflow
2. #28 feat/ust-2.3.1-directory-identity → feat/r2-epic-2.3-bot-identity | merge after #27 | unblocks identity/auth integration
3. #29 feat/ust-2.3.3-bot-principals → feat/r2-epic-2.3-bot-identity | merge after #28 | unblocks bot principal persistence
4. #30 feat/ust-2.3.2-bot-auth → feat/r2-epic-2.3-bot-identity | merge after #29 | unblocks bot-identity epic integration
5. #31 refactor/ust-2.4.1-shared-message-service → feat/r2-epic-2.4-message-api | merge after the bot-identity epic PR described below | unblocks shared HTTP/WS send service

Between #30 and #31: open feat/r2-epic-2.3-bot-identity → release/r2 after #28–#30 are owner-merged and epic verification is green. It cannot be opened yet because the integration branch has no implementation commits. Fold the epic log fragments through its documentation task first where needed.

After #31 and the bot-identity epic merge: open release/r2 → feat/r2-epic-2.4-message-api as a sync PR. Its owner merge unblocks UST-2.4.2. No cross-epic cherry-picks or direct integration pushes.

Existing task PRs append to runbook.md. If an owner merge creates a conflict in a later task, the agent merges the remote epic into that task, preserves both entries, reruns checks and pushes the resolution. The owner need not resolve code conflicts.
