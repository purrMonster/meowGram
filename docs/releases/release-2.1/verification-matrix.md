# R2.1 verification matrix — planning draft

2026-10-23. Every R2.1 test below is **PLANNED / NOT RUN**, including proposed
names. This is a test specification, not coverage evidence. Replace names with
actual test locations/results at implementation; no PLANNED row at release gate.
R2 historical evidence remains in ../release-2/verification-matrix.md.

| ID / story | Required result / proposed test | Command family | Status |
|---|---|---|---|
| V01 / 1.2 | TestChannelMigration: legacy rows/keys unchanged; repeated up, partial object creation, actual down/up; typed FKs/indexes and non-destructive down | B | PLANNED |
| V02 / 1.3 | TestChannelMembership: human creator/owner/member roles, Basic denied, unknown 403, last-owner concurrency, archive/read/send policy, audit and deletion constraints | B,A | PLANNED |
| V03 / 1.4 | TestChannelSync: omitted/empty lounge, A/B isolation, 1,001 equal timestamps, <=500, empty [], full-page flag, offset parsing, invalid/repeated params and custom aliases | B,A | PLANNED |
| V04 / 1.4 | TestChannelSendReplay: >=20 concurrent duplicates one row/broadcast/push; channel conflict, revoke-before-replay, stored-channel authorization, raw-text and UUID normalization, restart/expiry/failure | B,A | PLANNED |
| V05 / 1.4 | TestLegacyReplayUpgrade: seed using actual v2.0.0; same bytes/key retained; new null responses include field; non-null cannot match old hash; configured retention boundaries | B,R | PLANNED |
| V06 / 1.4 | TestChannelBotContract: global LDAP group on every request/page plus channel membership; lounge posting; bot sees 403 not []; Basic denied list/manage/ticket/push/media | B,A,D | PLANNED |
| V07 / 1.4 | TestChannelExampleRestart: channel included in durable state identity and pending body, independent cursors, one reply, 429/503 backoff, no self-kind collision | B,A | PLANNED |
| V08 / 2.1 | TestChannelSocketIsolation: bound ticket, recheck at consumption, scoped 50/history/live/presence/notices, forged scope denied, reconnect gap dedup, malformed JSON no lounge fallback | B,A | PLANNED |
| V09 / 2.1 | TestChannelRevocationBarrier: >=20 concurrent operations, queued/batched history, stalled writer, canceled HTTP, timeout/DB-down/restart; no post-ack dispatch; Hub remains non-blocking | B,A | PLANNED |
| V10 / 2.2 | TestChannelPush: active member devices only; legacy missing capability lounge-only; token reassignment/expiry/revocation; >500 recipient paging, channel fairness/trailing coalescing, generic payload | B,A | PLANNED |
| V11 / 3.1 | channel_cache_test.dart: legacy lounge migration/quarantine, interruption/retry, new box separate from v1, account/server change, pruning and stable equal-time ordering | C | PLANNED |
| V12 / 3.1 | channel_cursor_test.dart: page persistence before cursor, live/history cannot skip gap, independent channel sync, stale-account completion discarded, storage failure retry | C | PLANNED |
| V13 / 3.2 | channel_ui_test.dart: list/create/member controls, switching/offline/revoked state, channel-bound failed draft, first-frame cache, 800px/keyboard and Bot label | C | PLANNED |
| V14 / 3.2 | channel_unread_test.dart: non-self UUID count once, read watermark only foreground, sync/history/replay dedup, retention/restart and offline stale indicator | C | PLANNED |
| V15 / 3.3 | channel_background_test.dart: membership enumeration, scoped secure cursors, no Hive use, logout/server isolation, iOS FCM disabled in ntfy mode, Android/macOS unchanged | C | PLANNED |
| V16 / 4.1–4.2 | TestNtfyPublisher: fixed HTTPS origin, no redirects, ACL fixture, generic body, no sensitive metadata, wrong cert/topic denied, timeout/429/5xx bounded retry, revoked mapping/member canceled | N,B | PLANNED |
| V17 / 4.2 | TestNotificationProvidersIndependent: FCM/ntfy outage cannot block Hub or undo commit, no duplicate replay push; disabled ntfy needs no token; limits/default/config wiring | B,A,N | PLANNED |
| V18 / 5.1 | TestV200ClientPrivacy: unchanged legacy requests/decoder/cache/background fixtures; lounge/A/B canaries, allowed positive controls; no unauthorized HTTP/WS/provider/cache data | A,C | PLANNED |
| V19 / 5.2 | TestR21Rollback: actual v2.0.0 startup rejects ahead; clean no-channel recovery and forward preservation; refusal with private human/bot row or channel replay; forced old-read leak demonstrated only in scratch | R | PLANNED |
| V20 / 5.2 | TestR21Restore: restore pre-upgrade dump, post-backup changes absent, old read/send works; dirty/unknown/partial schema refuses unsafe startup | R,B | PLANNED |
| V21 / 5.3 | Existing suites plus new regressions; module tidy no diff, pinned artifacts, config completeness, source/layer inspection, release packaging/web smoke | B,C,D,A,R,N,F | PLANNED |
| V22 / owner | Locked iPhone ntfy, denied-topic user, actual relay, current membership, no duplicate alerts; real backup/device/provider checks | owner-checks.md | NOT RUN |
| V23 / owner | Firebase API/application restrictions appropriate to each used platform, retained required APIs, before/after smoke | owner-checks.md | NOT RUN |

## Reproduction command families

Use a new disposable project per run, never deploy/docker-compose.yml/live env.
B: docker compose -p <scratch-project> -f deploy/verify.compose.yml run --rm -e TZ=UTC server
and repeat with TZ=Asia/Kolkata. This runs go test -race -count=1 ./... and go vet
against real PostgreSQL /verification. Generic fixture skips are reported separately.
C: same project/compose, run --build --rm client (analyze/test/release web), then
restore and web-smoke services. F: deploy/scripts/verify.ps1 or sh deploy/scripts/verify.sh.
D: verify-bot-auth.ps1/.sh; A: verify-api-acceptance.ps1/.sh; R: verify-rollback.ps1/.sh.
N: proposed verify-notifications.ps1/.sh to be added in story 4.2, with pinned
synthetic ntfy and fake upstream; never point automated tests at the owner's service.
Run dedicated A/D/R/N under both TZ values as well; scripts must propagate TZ to
fixture/test containers explicitly (a host variable alone is insufficient).

The legacy privacy fixture must use source from tag v2.0.0, not a rewritten model
of what an old client might do. Check the pinned source commit in the harness.
Use actual server routing/OIDC and real PostgreSQL for persistence claims. Keep
synthetic credentials in disposable fixtures; no external provider credentials.
Owner real-device checks cannot be replaced with fake provider tests.

## This planning session

Static checks and exact results are in log/2026-10-23-P0-plan.md. Full verify.ps1
was attempted and failed before tests: Docker Desktop Linux engine pipe absent.
Go and Flutter are not on PATH. **No Go/Flutter/timezone pass count claimed.**
Dedicated integration/provider/rollback checks were not run for the same missing
container engine; R2.1 fixtures do not exist yet. Planning PR CI can verify unchanged
runtime baseline; it cannot prove any proposed R2.1 behavior. Request the scoped
docs-only evidence exception in README; keep this PR draft until review resolves it.
