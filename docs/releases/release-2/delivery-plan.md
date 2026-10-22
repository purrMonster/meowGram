# R2 API-only delivery and acceptance

Owner-approved revision: 2026-10-20. Outcome: pourOver live in the shared lounge.
R2 contains epics 2.3, 2.4 and API-only 2.5. R2.1 channels is planning only.
Media epics 2.1/2.2 are **deferred to R3**, unchanged; #32's contract stands.
This decision supersedes older scope statements in the unchanged playbook.

## Completed foundations

#27 workflow/playbook is merged into release/r2. #28 directory investigation,
#29 bot principals (including repeatable migration 000005), and #30 bot auth
are merged into epic 2.3. #31 shared message service is merged into epic 2.4.
#32 media contract is merged into epic 2.1, deferred to R3. #33 scope split and #36 queue are merged into release/r2; #34 stable usernames and #35 runbook fold are merged into epic 2.3. These task merges do not establish release readiness; evidence is recorded in story logs.

Identity epic integration is owner-merged as #38 (release/r2 at 3cfbfe8). Message API sync #39 and tasks #40/#41 are owner-merged into epic 2.4 at 6f98957. Send, sync and the durable example are implemented; #42 doc-fold and #43 epic integration are owner-merged into release/r2 at e74a1de. Combined real-server/LLDAP acceptance remains UST-2.5.1.

## Delivery status and remaining gates

| Story / step | Deliverable and Done check |
|---|---|
| Scope split — DONE (#33) | Docs-only PR into release/r2; pourOver contract, channels proposal and budget recorded |
| UST-2.3.4 — DONE (#34) | Stable bot username = verified LLDAP login ID (uid), not cn. Resolve must not rewrite it on each authentication. Display-name rename leaves username unchanged; deleted/recreated account gets a new principal; humans unaffected. Human and bot usernames are not cross-checked for uniqueness; identity is sender_kind + sender_id. Optional separate display_name only if cheap. Any migration is additive/re-runnable, with real golang-migrate down/up preservation tests and partial/repeated-up coverage equivalent to 000005. Document handling of existing cn-derived rows without destructive backfill. |
| Identity integration — DONE (#38) | Doc-fold task PR first if needed, then verified epic 2.3 → release/r2 PR, only after UST-2.3.4 is owner-merged |
| Message API sync — DONE (#39) | Owner-reviewed release/r2 → epic 2.4 sync; preserve SenderKind in MessageService.Send publication and SendHistory, with explicit bot/human publication regression |
| UST-2.4.2 — DONE (#40) | POST send for human bearer and bot Basic; transactional durable idempotency; identical replay response; 409 on changed body; ≥20 concurrent duplicates produce one row/broadcast and at most one push; restart/expiry/DB-failure tests; CORS Idempotency-Key support |
| UST-2.4.3 — DONE (#41) | Basic GET sync, fresh authorization per page, cursor compatibility and example with persistent cursor, deduplication, own-message filtering and 429/503 backoff; example runs in scratch infrastructure |
| Epic 2.4 closure — DONE (#42/#43) | docs/r2-epic-2.4-runbook → epic 2.4, then epic 2.4 → release/r2; fresh UTC/IST, Flutter, restore and web evidence |
| Epic 2.5 parent sync — review pending | Owner PR merges release/r2 into the existing test/r2-epic-2.5-verification (currently 0e41333); no task starts from old main |
| UST-2.5.1 — pending | One end-to-end scratch run of the release/r2 server binary, PostgreSQL and pinned LLDAP 0.6.3 over LDAPS, Basic enabled only in scratch: 401 wrong password, 403 nonmember/revoked member, 201 send, identical one-row replay, 409 conflict, bot sync discriminator, 429 integer Retry-After, directory-down 503, channel_id 400; human OIDC send/sync/WS and Flutter Bot-label/self-identity regressions. CI job if practical, otherwise scripted recorded harness. Matrix row per check with command/result/date |
| UST-2.5.2 — pending | API config/runbook/examples folded, statuses/queue updated, rollout/rollback/pourOver pilot prepared, previous-image tag/schema compatibility explicitly established or declared incompatible; final timed owner checks and pourOver placeholders/stop procedure; verified epic PRs, then release/r2 → main DRAFT for owner/Claude review; owner-only deployment/pilot |

Do not start a task unless it can finish or park this session with tests green,
branch pushed and a story log ending in "Where we stopped / next step".

## Branch hierarchy

| Branch | Immediate PR target |
|---|---|
| docs/r2-scope-split | release/r2 |
| chore/r2-verification-parent-sync (from remote epic 2.5 at 0e41333; merges release/r2 e74a1de) | test/r2-epic-2.5-verification |
| docs/r2-epic-2.4-runbook (from remote epic 2.4 at 6f98957) | feat/r2-epic-2.4-message-api |
| feat/ust-2.4.2-http-send-idempotency (merged #40) | feat/r2-epic-2.4-message-api |
| feat/ust-2.4.3-bot-sync-example (merged #41) | feat/r2-epic-2.4-message-api |
| chore/r2-message-api-sync (from remote epic 2.4, merges release/r2) | feat/r2-epic-2.4-message-api |
| feat/ust-2.3.4-stable-bot-usernames (merged #34) | feat/r2-epic-2.3-bot-identity |
| feat/r2-epic-2.3-bot-identity | release/r2, after 2.3.4 and doc-fold |
| feat/r2-epic-2.4-message-api | release/r2, after API tasks |
| test/r2-epic-2.5-verification | release/r2, after API integration |
| release/r2 | main |
| feat/r2-epic-2.1-media-backend | future release/r3; deferred to R3 |
| feat/r2-epic-2.2-media-client | future release/r3; deferred to R3 |

Published branch names stay unchanged; do not rename/delete them. Create
release/r3 only when R3 starts. Do not merge media into release/r2. New tasks
start from the fetched remote epic. Merge updated parents into tasks without
rebasing published history. No sibling implementation cherry-picks. Owner merges
all PRs. Reconcile this document with the older epic copies during integration;
retain this approved scope and the historical evidence from those copies.

## Required verification

Every [pourOver contract](api-design.md) row requires an httptest-level test
against the actual handler/auth path. Persistence claims additionally use real
scratch PostgreSQL; directory claims use pinned disposable LLDAP 0.6.3.

| Area | Required evidence |
|---|---|
| Sync | Basic GET; RFC3339 after and UUID after_id; 200 JSON array, oldest first, ≤500, all six fields; X-Has-More true when more remain; empty results are [] |
| Boundaries | Same created_at spanning pages returns each UUID once; unencoded +00:00 arriving as a space parses; exactly 500 may advertise more and next request returns 200 [] |
| Send | Idempotency-Key accepts pourover-reply-<original message id>; text_content body; 2xx on success; same key/body returns identical response and one message; different body gives 409 |
| Backoff/load | Integer Retry-After ≥1 on 429; Retry-After on overloaded/directory-unavailable 503; bot budgets ≥30 requests/min; single bot's 3-second polling is not throttled by shared pre-auth budgets |
| Identity | uid stability despite cn rename; recreated account distinct; humans unchanged; same-name humans/bots remain distinct; sender_kind + sender_id filters own messages |
| Security | Anonymous/malformed/wrong auth denied; nonmember 403; failed bearer never falls back to Basic; group/password/deletion/outage changes checked between requests, replay and pages; Basic denied outside GET sync/POST send; verified TLS and bounded LDAP work; no credential logging |
| Durability | Principal-isolated keys, concurrent duplicates, restart replay, retention boundary, write rollback, no broadcast on failure, no duplicate replay push; post-commit retry/sync recovery |
| Compatibility | Human OAuth, WS tickets, push, history/cache, Unicode text rules, UTC ordering, shared budgets, slow-client eviction and configured routes/CORS remain correct |

Run full Go -race and vet under UTC and Asia/Kolkata, restore, Flutter analysis
and tests, web build/smoke, and the provider harness. Report skips separately.
No media implementation or media fixture is an R2 acceptance requirement.

## Rollout and rollback

Owner-approved 2026-10-22: cellar will pull pinned GHCR images. UST-2.5.2 must implement the [GHCR rollout decision](ghcr-rollout-decision.md), including build override, version 2.0.0+4, off-node dump verification, image inspection before any public-package recommendation, digest recording and pre-bot-only rollback.

Basic stays disabled until the integrated API is reviewed. After explicit owner
authorization, validate reader/group visibility, TLS and cellar's proxy header
pass-through using [owner checks](owner-checks.md). Pilot pourOver every 3 seconds:
read a human message, reply once using its original-message key, retry safely,
restart with persisted cursor, ignore its own messages, remove membership and
confirm denial while human OAuth continues to work. Replies are plain @username
text, not structured mentions. Roll back by disabling Basic or removing bot group
membership, retaining additive schema and message history. Live deployment and
credential provisioning remain owner-gated.

## Original API-only estimate (2026-10-20)

Historical baseline, not today's remaining balance: 2.3.4 and 2.4 tasks are now complete; integration and API-only 2.5 remain. Focused engineering days of 8 hours, including tests/review fixes; excludes
elapsed owner/CI waiting. This replaces the prior media-inclusive estimate.

| Work | Days |
|---|---:|
| UST-2.3.4 stable usernames and regressions | 0.5–1 |
| Doc-fold, epic integration and SenderKind sync | 1–2 |
| UST-2.4.2 send/idempotency | 1.5–3 |
| UST-2.4.3 sync/example and cursor edge cases | 1–1.5 |
| UST-2.5.1 API integration/security verification | 1.5–2.5 |
| UST-2.5.2 release docs/candidate/pilot preparation | 1–2 |
| **Original base implementation** | **6.5–12 (52–96 hours)** |
| **With 25% contingency, rounded up to half-days** | **8.5–15 (68–120 hours)** |

Reserve 1–2 owner hours separately for the authorized API pilot/config checks.
R2.1 planning does not authorize implementation; its estimate is in the channels
proposal. R3 retains the #32 estimate of 8–12 days for media implementation,
subject to provider/platform verification when it starts. These are effort
budgets, not credit-consumption or release-date guarantees.
