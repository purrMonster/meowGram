# R2.1 delivery plan — approved product, implementation not started

Revised after owner review of #51, 2026-10-23. All implementation stories PLANNED.
P0 revision remains docs/r2.1-plan → main; STOP after updating #51. No new branch
layer or migration file this session.

## Stories and binary Done checks

Eight-hour focused days, including tests/review fixes, config/logs, parent-sync and
epic runbook folds. Excludes current planning and elapsed owner/CI waits. All new
named tests in the matrix are planned. Product decisions are settled, not re-opened.

| Story | Deliverable / pass-fail acceptance | Dependencies | Days |
|---|---|---|---:|
| UST-2.1.1.1 | Freeze executable API fixture contracts from approved design: exact route/body/status/scope matrix; legacy defaults and sync revision/count metadata specified; no new product decision gate | Implementation authorization | 0.5–1 |
| UST-2.1.1.2 | Additive conversations/memberships/personal mute/read state and nullable messages.channel_id; no roles/audit/archive; repeated/partial/down-up preserve rows, restrictive FKs and null-state uniqueness tested | 1.1 | 1.5–2 |
| UST-2.1.1.3 | Human list/create/start-chat/rename/member/leave/picker/mute/read APIs; >=20 identical unnamed starts one UUID; named fresh, rename-clear/current-set races; all-human management; no-human group denied; unread/mute and Basic isolation proven | 1.2 | 2.5–3.5 |
| UST-2.1.1.4 | Send/idempotency and explicit scope=member merged sync; 1001 equal-time rows with 500 cap; every-page access/revision backfill; old bytes/hash compatibility; >=20 duplicate sends one publication; 20 polls+10 replies/min without steady 429; docs-only pourOver handoff and in-repo bot fixture | 1.3 | 2–3 |
| UST-2.1.2.1 | Bound ticket/socket/history/live/presence; removal invalidates tickets and closes sockets via Hub, including consume/register race and last-human case; no per-frame barrier; eviction and reconnect tests | Epic 1 integrated and synced into epic 2 | 1–1.5 |
| UST-2.1.2.2 | Current member minus sender/muted push; old capability lounge-only; 500-device pages, token expiry/reassignment, busy-group fairness and trailing signal; no payload content; provider failure independent | 2.1 | 1.5–2 |
| UST-2.1.4.1 | ntfy contract spike first: owner server/iOS versions, pinned fixture digest, protected topic/relay generic payload proof; no live edits; mode config contract published | Owner version input, 1.1 contract | 1–1.5 |
| UST-2.1.3.1 | Account/server/conversation model/cache/cursors; quarantine unattributed v1; restart-safe import; merged checkpoint distinct from per-group state; revision replay and page-persist-before-cursor tests | Epics 1/2 integrated and synced into epic 3 | 2–3 |
| UST-2.1.3.2 | List/activity/unread/mute; named/create/start picker/reuse; rename/add/remove/leave; offline/403/drafts; no stale results, read-marker/count reconciliation; Bot/800px/keyboard tests | 3.1 | 2.5–4 |
| UST-2.1.3.3 | Android scope=member background with merged/per-conversation secure cursors and mute filtering; logout/server reset, no background Hive; device capability and iOS ntfy/no-FCM mode | 3.2; epic 4 integrated and synced to epic 3 | 1–1.5 |
| UST-2.1.4.2 | ntfy private mapping/publisher disabled by default, TLS/topic/redirect checks and bounded retry; member/mute/sender filters, synthetic fixture; no secret output | 4.1; epic 2 provider contract integrated and synced into epic 4 | 1.5–2.5 |
| UST-2.1.5.1 | Real-binary lounge/A/B canaries and positive controls on every legacy/new path, actual v2.0.0 decoder/cache; combined OIDC/LDAP/providers/reuse/mute/member sync; both timezones, no new fixture skips | Epics 1–4 integrated and synced into epic 5 | 1.5–2.5 |
| UST-2.1.5.2 | Actual v2.0.0 startup rejection, zero-group guarded recovery/forward preservation, refuse private rows/keys, isolated forced-read leak proof, real backup restore; both timezones | 5.1; approved ADR 001 | 1–1.5 |
| UST-2.1.5.3 | Matrix/runbook/config/artifact pin checks, backup→upgrade→lounge smoke→group rollout procedure; owner pilot gates and final draft release PR with passing required suites | 5.2 and all folds | 1.5–2.5 |
| **Base** | **168–256 hours** | | **21–32** |
| **Base + 25%** | **210–320 hours** | | **26.25–40** |
| **Rounded up to half-days** | **212–320 hours** | | **26.5–40** |

The old 20–31-day estimate is superseded. Simpler revocation and no roles/audit/
archive reduce backend work; exact-set reuse, merged-scope membership backfill,
persisted unread/mute and richer Flutter flows increase it. No external pourOver
implementation is included in these estimates. No live provisioning/console work.

## Exact proposed branch map

All parents are also PR targets except the explicitly stated planning origin.
Fetch first, use latest owner-merged remote parent; never rebase/force pushed history.
No implementation branch below exists by authorization from this session.

| Branch | Parent / PR target |
|---|---|
| docs/r2.1-plan (existing #51) | original origin/main e094dfa / main |
| release/r2.1 (after authorization) | updated origin/main / main |
| feat/r2.1-epic-1-conversations | release/r2.1 |
| feat/r2.1-epic-2-delivery | release/r2.1 |
| feat/r2.1-epic-3-client | release/r2.1 |
| feat/r2.1-epic-4-ntfy | release/r2.1 |
| test/r2.1-epic-5-verification | release/r2.1 |
| docs/ust-2.1.1.1-contracts | feat/r2.1-epic-1-conversations |
| feat/ust-2.1.1.2-schema | feat/r2.1-epic-1-conversations |
| feat/ust-2.1.1.3-conversations | feat/r2.1-epic-1-conversations |
| feat/ust-2.1.1.4-message-api | feat/r2.1-epic-1-conversations |
| feat/ust-2.1.2.1-sockets | feat/r2.1-epic-2-delivery |
| feat/ust-2.1.2.2-push | feat/r2.1-epic-2-delivery |
| feat/ust-2.1.3.1-cache | feat/r2.1-epic-3-client |
| feat/ust-2.1.3.2-conversation-ui | feat/r2.1-epic-3-client |
| feat/ust-2.1.3.3-background | feat/r2.1-epic-3-client |
| docs/ust-2.1.4.1-ntfy-contract | feat/r2.1-epic-4-ntfy |
| feat/ust-2.1.4.2-ntfy | feat/r2.1-epic-4-ntfy |
| test/ust-2.1.5.1-acceptance | test/r2.1-epic-5-verification |
| test/r2.1-verification-scaffold | test/r2.1-epic-5-verification (independent actual-v2.0.0 decoder probe; owner-authorized fast-track scaffolding only) |
| test/ust-2.1.5.2-rollback | test/r2.1-epic-5-verification |
| chore/ust-2.1.5.3-readiness | test/r2.1-epic-5-verification |

Each epic closes with docs/r2.1-epic-<n>-runbook from/into that epic. Fold task
logs there under approved R2 convention. Agent publishes initial integration refs
only after authorization; all subsequent integration changes are owner merges.

Owner merge sequence: epic 1 → release; sync release into epic 2; epic 2 → release;
sync release into epics 3/4; epic 4 (spike then publisher/fold) → release; sync release
into epic 3 before 3.3; epic 3 → release; sync release into epic 5; epic 5 → release;
draft release → main. Epic 3.1/3.2 may proceed while epic 4 finishes, but cannot
consume sibling commits. No need for partial epic-4 integration in this revised map.
Use chore/r2.1-epic-<n>-parent-sync from target epic, merge release there, PR to same
epic; or owner-merged direct release→epic sync PR. Never push implementation directly
to integration branches. Record actual PR numbers/parents in queue when created.

## Verification and stop rules

Planning doc-only evidence is APPROVED for #51. Runtime features are all PLANNED,
not tests passed. Implementation sessions must use an environment with Docker,
Go 1.27.1 and Flutter; if suites cannot run, stop before claiming DONE. Required:
affected package -race/vet, full verify, Go UTC and Asia/Kolkata, real PostgreSQL,
dedicated LDAP/acceptance/rollback/notification fixtures, Flutter analyze/test and
web build/smoke. Pin versions/digests; synthetic data/secrets only in scratch.
Record exact output/counts/skips; no green by skipping. Keep modules tidy/no diff.

New endpoints/defaults/config go through config.go/.env.example/Compose, client
app_config as needed and runbook fold. Owner alone changes live LLDAP/proxy/ntfy,
credentials, console restrictions, deployment or release tags/publication. No
AGENTS/playbook edits. Park green commits/logs pushed; no stashes; owner merges.
