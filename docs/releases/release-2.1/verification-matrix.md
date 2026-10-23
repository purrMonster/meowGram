# R2.1 verification matrix — revised planning draft

Owner-reviewed model, 2026-10-23. **All rows PLANNED**. Names are proposed tests,
not existing pass claims. Replace with real names/results at implementation;
no PLANNED automated row at release readiness. R2 historical results stay separate.

| ID / story | Required binary result / proposed test | Command | Status |
|---|---|---|---|
| V01 / 1.2 | TestConversationMigration: legacy rows/keys unchanged; partial/repeated up and migrate down/up; typed FKs, per-kind uniqueness, null personal-state uniqueness, no silent message/member cascades | B | PLANNED |
| V02 / 1.3 | TestHumanGroupManagement: any member rename/add/remove/leave; Basic denied; no-human group denies every path; surviving-human shrink; no roles/audit/archive routes | B,A | PLANNED |
| V03 / 1.3 | TestStartChatReuse: >=20 identical starts one group; typed exact current set; named always fresh; clear/rename/member edits race safely; duplicate unnamed matches deterministic; no set unique constraint | B,A | PLANNED |
| V04 / 1.3 | TestConversationListPicker: lounge first, last-activity order/ties, accurate bounded member summary, 403 nonmember, paging/minimal picker, Basic isolation, invalid/duplicate input and path encoding | B,A | PLANNED |
| V05 / 1.3 | TestPersonalMuteRead: per-user isolation; lounge/group state; monotonic valid read position, non-self counts, retained history baseline; mute never changes unread/read rights | B,A | PLANNED |
| V06 / 1.4 | TestMemberSync: omitted selectors lounge; UUID group; scope=member merged 1001 equal-time rows, 500 cap/[]/X-Has-More; no duplicate rows; aliases/offsets; conflicting/repeated/empty/null selectors 400 | B,A | PLANNED |
| V07 / 1.4 | TestMembershipSyncRevision: every page including empty authorized; removal/no-human filtering; addition exposes older history via revision/epoch replay; mid-page changes and stable checkpoint; count/mute metadata only authorized | B,A,C | PLANNED |
| V08 / 1.4 | TestConversationSendReplay: >=20 duplicates one row/publication/push; principal-wide channel/text conflict; requested/stored access before replay; revoked member 403; unknown body fields still rejected | B,A | PLANNED |
| V09 / 1.4 | TestLegacyReplayUpgrade: actual old stored bytes unchanged, missing channel interpreted lounge; null hash R2-compatible, versioned non-null; restart/expiry/failed-write/stopped-Hub durability | B,R | PLANNED |
| V10 / 1.4 | TestMemberBotBudgets: LDAP checked every page/retry, member/no-human 403, 20 polls+10 replies/min no steady 429; transient 429/503 Retry-After; bots denied every non-send/sync surface | B,A,D | PLANNED |
| V11 / 1.4 | TestBotConversationRouting: source channel copied, own-kind filter, two-member count versus >=3 prefix contract fixture, membership backfill/dedup, no unauthorized lounge fallback | B,A | PLANNED |
| V12 / 2.1 | TestConversationSocket: scope-bound ticket/recheck, no URL override, old null default, history/live/presence/notices scoped; malformed structured send no fallback; reconnect/slow eviction | B,A | PLANNED |
| V13 / 2.1 | TestMemberSocketRemoval: outstanding tickets invalidated, consume/register race, Hub closes all affected sockets including last-human case; queued writers terminate; in-flight allowance documented; no per-frame barrier | B,A | PLANNED |
| V14 / 2.2 | TestConversationPush: current members minus sender/muted, no-human denied, legacy capability lounge-only, >500 devices, expiry/reassignment, mute/revoke before retry, mixed-sender burst and trailing fairness | B,A | PLANNED |
| V15 / 3.1 | conversation_cache_test.dart: attributable lounge import/unattributed quarantine, restart-safe migration, private data never in v1, account/server switch, UTC/tie/dedup and retention | C | PLANNED |
| V16 / 3.1 | conversation_cursor_test.dart: merged versus per-group checkpoints, save before advance, history/live cannot skip gap, revision backfill, storage failure and stale-account callbacks | C | PLANNED |
| V17 / 3.2 | conversation_ui_test.dart: lounge pinned/activity order, named/start picker/reuse, rename-clear/add/remove/leave/mute, Bot labels, offline/403/drafts, no stale switch results, first-frame/800px/keyboard | C | PLANNED |
| V18 / 3.2 | conversation_unread_test.dart: server non-self count/read-watermark reconciliation, initial retained history, no replay double count, mute indicator, foreground-only read updates, stale offline badges | C | PLANNED |
| V19 / 3.3 | member_background_test.dart: scope=member paging, separate merged/per-group secure cursors, own/muted suppression, revision on empty page, logout/server cleanup, no Hive, iOS no-FCM ntfy mode | C | PLANNED |
| V20 / 4.1–4.2 | TestNtfyContract/Publisher: pinned synthetic server/relay ACL, generic body/no private metadata, TLS/topic/redirect errors, bounded timeout/429/5xx retry, disabled default, mapping revoke/mute | N,B | PLANNED |
| V21 / 2.2,4.2 | TestProviderIndependence: provider failures cannot block Hub/undo commit, retry reauthorization, no replay push, config completeness and no credential output | B,A,N | PLANNED |
| V22 / 5.1 | TestV200Privacy: actual old decoder/cache/background fixtures against real binary; lounge/A/B typed matrix, equal-time pages, old/new devices; forbidden canaries ABSENT and allowed messages PRESENT on every path | A,C | PLANNED |
| V23 / 5.2 | TestR21Rollback: actual v2.0.0 rejects ahead, zero-group safe recovery + forward reapply; refuse one group human/bot message or channel replay row; isolated forced-old-read leak proof | R | PLANNED |
| V24 / 5.2 | TestR21Restore: verified pre-upgrade dump restore loses post-backup data; old history/send works; unknown/dirty/partial schemas fail closed | R,B | PLANNED |
| V25 / 5.3 | Full regression, module tidy clean, config/runbook/versions/pinned artifacts; backup→upgrade→lounge→groups procedure and release packaging | B,C,D,A,R,N,F | PLANNED |
| V26 / owner | Protected ntfy topic/relay, physical locked iPhone, no duplicate FCM/local alerts | owner-checks.md | PLANNED |
| V27 / owner | Firebase restrictions; LLDAP reader/group, proxy Authorization/X-Forwarded-Proto, trusted CIDRs before pilot | owner-checks.md | PLANNED |

## Commands and evidence standard

B: docker compose -p <scratch-project> -f deploy/verify.compose.yml run --rm -e TZ=UTC server;
repeat with TZ=Asia/Kolkata. Runs go test -race -count=1 ./... and go vet with real
PostgreSQL /verification. C: same scratch compose run --build --rm client for
analyze/tests/web build, then restore/web-smoke. F: deploy/scripts/verify.ps1 or
sh deploy/scripts/verify.sh. D: verify-bot-auth.ps1/.sh; A: verify-api-acceptance.ps1/.sh;
R: verify-rollback.ps1/.sh. N: proposed verify-notifications.ps1/.sh in story 4.2.
Scripts must explicitly pass TZ to test/fixture containers for A/D/R/N in both
timezones; setting only host TZ is insufficient. No live Compose/env/volumes.

Use actual tag v2.0.0 code for legacy client/rollback fixtures. Real PostgreSQL for
persistence/reuse races; >=20 goroutines and -race. Real pinned LDAP for identity.
Pinned synthetic ntfy/upstream for provider contract; physical delivery owner-only.
Generic fixture skips are reported separately and dedicated harnesses must run.

Docs-only planning evidence is APPROVED. No runtime suites are claimed this session;
all R2.1 functionality remains unimplemented. Implementation must use working Docker,
Go 1.27.1 and Flutter and stop before DONE when suites unavailable. This exception
never carries from the planning PR into implementation. See the review log for
current static validation and the initial log for the earlier failed Docker attempt.
