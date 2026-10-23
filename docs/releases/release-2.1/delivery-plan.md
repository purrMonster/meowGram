# R2.1 delivery plan

PROPOSED, 2026-10-23. All implementation stories are NOT STARTED. This session's
P0 is docs-only, from main e094dfa, with one PR into main. Approval of this plan
is required before any release/epic creation or migration 000007.

## Stories, checks and estimates

Eight-hour focused engineering days; includes each story's tests, review fixes,
configuration docs and log fragment. Waiting for owner merges/CI is excluded.
Story IDs use 2.1.<epic>.<story> to distinguish R2.1 from deferred R2 media IDs.
Checks named below are proposed, not existing passing tests; see the matrix.

| Story | Deliverable and binary acceptance check | Depends on | Days |
|---|---|---|---:|
| UST-2.1.1.1 | Resolve ADRs/playbook translations and contract details before schema; accepted decisions recorded, unknown/dirty/ahead-version rejection tests specified, replay exception settled | Owner plan approval | 0.5–1 |
| UST-2.1.1.2 | Additive channel/membership/audit/index schema; legacy human/bot rows and keys unchanged through partial/repeated up and migrate down/up; last-owner integrity and typed uniqueness tested with real PostgreSQL | 1.1 | 1.5–2 |
| UST-2.1.1.3 | Human create/list/archive/member/owner APIs and bounded principal lookup; 403 nonmembers, Basic denied, concurrent last-owner protections and audit verified; no auto-grant by name | 1.2 | 1.5–2.5 |
| UST-2.1.1.4 | Scoped HTTP send/sync, durable fingerprints/replay and bot example; >=20 duplicates one row/publication; equal-time >500 pagination; revoke-before-replay; legacy keys preserved; pourOver channel state isolated | 1.3 | 1.5–2.5 |
| UST-2.1.2.1 | Channel-bound ticket/socket/history/live/presence; final-frame revocation barrier, reconnect and slow-client eviction; old socket receives lounge only, 20-way races under -race | Epic 1 merged to release and synced into epic 2 | 2–3 |
| UST-2.1.2.2 | Recipient-aware FCM and bounded per-channel push coalescing; old capability lounge-only; expiry/reassignment/revocation and provider-failure tests; generic payload | 2.1 | 1.5–2 |
| UST-2.1.4.1 | ntfy contract spike: exact server/iOS version inventory supplied by owner, fixture version/digest pinned, protected-topic/relay contract and payload captured using synthetic service; no live edits | 1.1; owner version input | 1–1.5 |
| UST-2.1.3.1 | Model/cache migration and per-channel completed/read cursors; restart-safe legacy import or quarantine; account/server switch, gap-before-history and storage-limit tests | Epics 1/2 integrated and synced into epic 3 | 1.5–2.5 |
| UST-2.1.3.2 | Channel list/switch/management UI, unread/offline/403/draft states; no stale results after switching; keyboard/800px regressions; no duplicate unread on replay | 3.1 | 2–3 |
| UST-2.1.3.3 | Scoped Android background cursor and device capability; logout/server-change isolation; iOS ntfy mode disables FCM path and duplicate local alerts; no Hive background isolate use | 3.2 and 4.1 contract | 1–1.5 |
| UST-2.1.4.2 | ntfy publisher and private mapping config, bounded retry/timeouts; TLS/redirect/topic validation; member-only generic alerts; disabled-by-default and provider-failure tests | 4.1 and epic 2 shared provider interface integrated/synced | 1.5–2.5 |
| UST-2.1.5.1 | Real-binary channel/legacy-client acceptance, OIDC/LDAP and provider fixtures, every privacy path with allowed-delivery controls; UTC/Asia-Kolkata green; no added fixture skips | Epics 1–4 owner-merged to release and synced into epic 5 | 1.5–2.5 |
| UST-2.1.5.2 | Actual v2.0.0 rollback/forward/restore drill, refusal after private traffic, guarded owner recovery procedure; dirty/ahead schema cases | 5.1, ADR 001 | 1–1.5 |
| UST-2.1.5.3 | Final matrix, runbook folds, config tables, pinned artifacts, rollout/pilot instructions and draft release PR; all automated gates green, owner-only checks explicit | 5.2 and all doc folds | 2–3 |
| **Base** | **160–248 hours** | | **20–31** |
| **Base + 25%** | **200–310 hours before rounding** | | **25–38.75** |
| **Budget rounded up to half-days** | **200–312 hours** | | **25–39** |

P0 analysis/docs is the present session, excluded from future implementation effort.
Epic doc folds and parent-sync review work are included in story estimates, not a
free unbudgeted phase. If the provider spike invalidates the assumed ntfy setup or
revocation needs distributed deployment support, return to the owner scope gate;
do not silently consume contingency on a different product.

## Exact branch map

The following branches are proposals only. Every listed parent is also the PR
base unless explicitly stated. Fetch first and use the latest owner-merged remote
parent. Published history is updated by merge, never force/rebase.

| Branch | Parent / PR target |
|---|---|
| docs/r2.1-plan (this session only) | origin/main e094dfa / main |
| release/r2.1 (after approval) | updated origin/main / main |
| feat/r2.1-epic-1-channels | release/r2.1 |
| feat/r2.1-epic-2-delivery | release/r2.1 |
| feat/r2.1-epic-3-client | release/r2.1 |
| feat/r2.1-epic-4-ntfy | release/r2.1 |
| test/r2.1-epic-5-verification | release/r2.1 |
| docs/ust-2.1.1.1-contracts | feat/r2.1-epic-1-channels |
| feat/ust-2.1.1.2-schema | feat/r2.1-epic-1-channels |
| feat/ust-2.1.1.3-membership | feat/r2.1-epic-1-channels |
| feat/ust-2.1.1.4-message-api | feat/r2.1-epic-1-channels |
| feat/ust-2.1.2.1-sockets | feat/r2.1-epic-2-delivery |
| feat/ust-2.1.2.2-push | feat/r2.1-epic-2-delivery |
| feat/ust-2.1.3.1-cache | feat/r2.1-epic-3-client |
| feat/ust-2.1.3.2-channel-ui | feat/r2.1-epic-3-client |
| feat/ust-2.1.3.3-background | feat/r2.1-epic-3-client |
| docs/ust-2.1.4.1-ntfy-contract | feat/r2.1-epic-4-ntfy |
| feat/ust-2.1.4.2-ntfy | feat/r2.1-epic-4-ntfy |
| test/ust-2.1.5.1-acceptance | test/r2.1-epic-5-verification |
| test/ust-2.1.5.2-rollback | test/r2.1-epic-5-verification |
| chore/ust-2.1.5.3-readiness | test/r2.1-epic-5-verification |

Each epic closes with docs/r2.1-epic-<n>-runbook from/into its epic, folding task
log fragments. Integration branches receive owner merges only. Agent may publish
their initial refs once after approval, never push implementation directly to them.

Cross-epic integration order: epic 1 → release; owner parent-sync release → epic 2;
epic 2 → release; sync release → epic 3 and epic 4; epic 4 contract must reach release
before epic 3's dependent background story incorporates it; epics 3/4 → release;
sync release → epic 5; epic 5 → release; draft release → main. Implementations
consume contracts only after owner merge to the common release parent. If a partial
epic-4 contract integration is needed, merge that reviewed slice before starting
its implementation tasks, then refresh those tasks from the updated epic.

Use chore/r2.1-epic-<n>-parent-sync from the target epic, merge latest release into
that task and PR to the same epic; owner merges. A direct release → epic sync PR
is also allowed by the playbook, without agent pushes to integration branches.
No sibling cherry-picks, cross-epic stacks, reset or silent branch-parent changes.

## Done and stop gates

Each story: affected tests/race/vet; full verify harness, Go UTC and Asia/Kolkata,
real PostgreSQL and dedicated provider harness as applicable; Flutter analyze/test,
web build/smoke. Record exact commands, pass counts, skips and reasons in its log.
Checkpoint/push green work and log Where we stopped; do not stash or merge PRs.
At release readiness no matrix row may still be planned; owner-only checks remain
explicitly separate. Fixtures must use synthetic values, pinned versions/digests,
no host ports/live volumes. Keep credentials and actual deployment values private.

Config additions go through config.go, .env.example, Compose, client app_config
where appropriate, and runbook fold. No AGENTS/playbook changes. Release artifact
version/tag changes, publication, live deployment, external config and credentials
remain separately owner-authorized. Firebase restrictions are owner-only with no
source changes. Stop this session after the one planning PR is pushed and logged.
