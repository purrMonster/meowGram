# 2026-10-21 message API release sync

owner: "merged." (#38)

Branch chore/r2-message-api-sync starts from remote epic 2.4; PR target is
feat/r2-epic-2.4-message-api. Merge 539c6c5 brings origin/release/r2 at 3cfbfe8
into this child, retaining owner-only integration merges and published history.

- Resolved client.go by retaining the shared MessageService.Send path and the
  release's SenderKind in SendHistory. No duplicate inline broadcast remains.
- Retained both runbook histories; the shared-service historical record sits
  with the other 2026-10-17 entries. Approved API-only scope is unchanged.
- Separate behavior fix 0689c98 adds SenderKind: message.SenderKind to service
  publication. TestMessageServicePublishesStoredSenderKind/bot and /human assert
  exactly one publication retaining the stored sender kind and IDs.
- Focused go test -race -count=1 ./internal/chat: ok (1.294s); go vet passed.
- go mod tidy on an isolated source copy produces no content difference in
  go.mod/go.sum (comparison ignores CRLF versus LF); tracked module files unchanged.
- Formatting initially hit the harness read-only mount; used a standalone
  writable formatter mount. Tests retain the normal read-only source harness.
- Dedicated LLDAP 0.6.3 TestLLDAP: PASS (3.09s). Generic suite skips will be
  reported separately. Both directory and bot-auth CI jobs remain ahead of verify.

- Full suite: UTC 144 test/subtest passes and IST 144 passes, both with -race and
  go vet clean. Each skipped TestLLDAP once; it passed in the dedicated harness
  above, not counted among generic passes. Real scratch PostgreSQL tests ran.
- Backup/restore drill passed (probe and message count match).
- Flutter: No issues found! (10.4s); 00:07 +83: All tests passed!; Built build/web.
- Full evidence: local Temp/meowgram-message-api-sync-{UTC,Asia-Kolkata,restore,
  client,web-smoke}.log; provider evidence meowgram-api-sync-provider.log.

- Web runtime passed: non-root, SPA fallback, no-store HTML and bootstrap asset.

Where we stopped / next step: DONE; publish verified sync PR for owner merge. UST-2.4.2 begins from the updated remote epic after that merge, per playbook 5.6.
