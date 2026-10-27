# R2.1 verification matrix — revised planning draft

Owner-reviewed model, 2026-10-23. V01 verified 2026-10-25; V02–V05 verified
2026-10-26 for management surfaces. Other rows remain
PLANNED and their names are proposed tests, not existing pass claims. Replace with real names/results at implementation;
no PLANNED automated row at release readiness. R2 historical results stay separate.

| ID / story | Required binary result / proposed test | Command | Status |
|---|---|---|---|
| V01 / 1.2 | TestConversationMigrationFresh/LegacyAndDownUp/RepeatedUp/PartialPrefixes/Constraints/DeleteSemantics: legacy binary column bytes and reads unchanged; 24 DDL prefixes; typed/unique FKs; null lounge-state uniqueness; owner-approved human cascades and bot/conversation RESTRICT | B | PASS 2026-10-25: 56 cases in each timezone; [evidence](log/2026-10-25-UST-2.1.1.2.md) |
| V02 / 1.3 | TestConversationRepositoryRenameCurrentSetAndMembershipRevision / LastHumanAndDeletionRevisions; TestConversationBasicRejectedBeforeParsing / BasicGuardBeforeCORS; real-binary management_members_picker_persistence / management_basic_401_during_directory_outage / management_read_mute_leave | B,A | PASS UTC/IST 2026-10-26; management access and revision/cascade coverage; message/socket access remains V07/V13 |
| V03 / 1.3 | TestConversationRepositoryConcurrentReuseAndNamedFresh / ConcurrentMutationAndStart / RenameCurrentSetAndMembershipRevision; TestConversationTypedReuseAndEqualTimeRead; real-binary management_create_reuse_rename | B,A | PASS UTC/IST 2026-10-26; 24 simultaneous starts and 24-way rename/member races |
| V04 / 1.3 | TestConversationListTiesBoundedSummaryAndCursorBindings; TestConversationRepositoryPaginationAndPicker; TestConversationStrictInputs / UnicodeAliasEscapedPath; real-binary list/picker/uniform403 assertions | B,A | PASS UTC/IST 2026-10-26; [evidence](log/2026-10-26-UST-2.1.1.3.md) |
| V05 / 1.3 | TestConversationRepositoryUnreadWatermarkAndMute; TestConversationTypedReuseAndEqualTimeRead; real-binary management_read_mute_leave | B,A | PASS UTC/IST 2026-10-26; exact persisted read/mute and viewer isolation |
| V06 / 1.4 | TestScopedSyncEqualTimePrivacyRevisionBackfill; TestScopedSyncRejectsAmbiguousSelectorsBeforeStorage; TestHTTPContract: 1001 equal-time authorized rows interleaved with 1001 private rows, 500/500/1/empty pages, strict selectors and legacy lounge | B,H | PASS UTC/IST 2026-10-28; 78 real HTTP fixtures; no private rows or duplicates |
| V07 / 1.4 | TestScopedSyncEqualTimePrivacyRevisionBackfill: consistent page/revision snapshot, removal and last-human filtering, empty-page revision, rejoin/epoch retained-history backfill; TestBotConversationRoutingRevisionRestart | B,H | PASS UTC/IST 2026-10-28; per-viewer metadata stays on human conversation routes; Flutter recovery remains V16 |
| V08 / 1.4 | TestScopedHTTPConcurrentPublicationAndRevokedReplay; TestScopedSendConcurrentReplayConflictAndRevocation; TestHTTPSendContract; TestLegacyHubExcludesGroupWithLoungePositiveControl | B,H | PASS UTC/IST 2026-10-28; 24 private duplicates one row/publication; channel/text409 and requested/stored403-first; legacy Hub excludes groups; scoped delivery is epic2 |
| V09 / 1.4 | TestR21HTTPContract/actual_v2_stored_response_survives_upgrade; TestScopedSendLegacyExactBytesAndLoungeFingerprint; existing idempotency/HTTP expiry, failure and stopped-Hub assertions | B,H,R | PASS UTC/IST 2026-10-28: actual v2.0.0 response/storage bytes and raw-text hash unchanged, one row/key; frozen historical R2 drill5/5 |
| V10 / 1.4 | TestMemberBotBudgets; TestScopedHTTPConcurrentPublicationAndRevokedReplay; TestLLDAP; TestAPIEndToEnd; TestHTTPContract | B,H,A,D | PASS UTC/IST 2026-10-28 generic / 2026-10-27 dedicated A,D: 6 minutes x20 polls+10 replies, all180 auth-gate calls; real-LDAP denial/retry/route controls separately pass |
| V11 / 1.4 | TestBotConversationRoutingRevisionRestart; TestBotPrivateDenialNeverFallsBack; existing real-PG bot test; source-channel pending bodies, own-kind filter, revision/dedup/restart and no403 fallback | B | PASS UTC/IST 2026-10-28 for in-repo fixture; pourOver prefix-always handoff resolved by owner; external implementation/pilot O15 remains owner work |
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
R: verify-rollback.ps1/.sh. H: verify-r21-contracts.ps1/.sh (both timezones,
actual-tag upgrade replay plus 78 HTTP cases). N: proposed verify-notifications.ps1/.sh in story 4.2.
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
