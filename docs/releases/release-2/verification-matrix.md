# API verification matrix

Component rows below were rerun in UST-2.5.1 against real scratch PostgreSQL
with -race under UTC and Asia/Kolkata. Combined binary/LDAPS evidence follows.
This is API acceptance evidence, not live rollout or release-candidate sign-off.

| Requirement | Named test | Command | Status |
|---|---|---|---|
| 20 duplicate sends, identical 201 and one row/publication | TestHTTPSendContract/concurrent_identical_response_one_publication | B | UTC/IST PASS, 2026-10-22 |
| Durable replay, changed text 409 before normalization | TestHTTPSendContract/restart_replay_conflict_and_normalization | B | UTC/IST PASS, 2026-10-22 |
| Real OIDC human and Basic bot, revocation before replay, kind isolation | TestHTTPSendContract/reauthentication_and_isolation | B | UTC/IST PASS, 2026-10-22 |
| 429/503 positive Retry-After | TestHTTPSendContract/backoff | B | UTC/IST PASS, 2026-10-22 |
| Strict body/key/size validation | TestHTTPSendContract/validation | B | UTC/IST PASS, 2026-10-22 |
| Atomic failure rollback, committed hub failure replays without republishing | TestHTTPSendContract/failure_rollback_and_durable_publication_failure | B | UTC/IST PASS, 2026-10-22 |
| Shared HTTP/WebSocket write allowance | TestHTTPSendContract/HTTP_and_WebSocket_share_human_write_budget | B | UTC/IST PASS, 2026-10-22 |
| Idempotency-Key CORS | TestHTTPSendContract/cors | B | UTC/IST PASS, 2026-10-22 |
| Additive 000006 up/down/up, direct repeated up, partial up, legacy rows preserved | TestIdempotencyMigrationRollbackAndPartialReapply | B | UTC/IST PASS, 2026-10-22 |
| Expiry boundary, message/key transaction failure | TestIdempotencyExpiryAndTransactionFailure | B | UTC/IST PASS, 2026-10-22 |
| Disabled default, retention bounds, minimum bot budgets, route configuration | TestHTTPSendConfiguration | B | UTC/IST PASS, 2026-10-22 |
| Real LDAP behavior | TestLLDAP | D | PASS 2.66s, 2026-10-22; dedicated LLDAP 0.6.3 |
| Bot GET sync, 1000 equal-time rows, unencoded offset, empty page, per-page group checks | TestBotSyncPagingAndPolling | B | UTC/IST PASS, 2026-10-22 |
| 20 polls + 10 replies/min with minimum budgets | TestBotSyncPagingAndPolling | B | UTC/IST PASS, 2026-10-22 |
| Release readiness, owner pilot, native device | UST-2.5.2 and owner-checks.md | Owner procedures | Not run; release integration/live access required |
| Human deletion cascades through HTTP-sent message and key | TestHTTPSendContract/user_deletion_cascades_idempotency | B | UTC/IST PASS, 2026-10-22 |
| Transactional expiry cleanup bounded at 100; replay cleanup; active keys preserved | TestIdempotencyBoundedExpiredCleanup | B | UTC/IST PASS, 2026-10-22 |
| Default/custom sync aliases, human access, exact Basic methods | TestSyncRouteAliasesAndBasicIsolation | B | UTC/IST PASS, 2026-10-22 |
| Example durable cursor, lost-response replay, own filtering, 429/503, retention cutoff | TestExampleScratchRestartAndBackoff | B | UTC/IST PASS, 2026-10-22 |
| Example HTTPS endpoint boundaries | TestExampleRequiresSecureEndpoints | B | UTC/IST PASS, 2026-10-22 |

## UST-2.5.1 combined acceptance — 2026-10-22

Command **A**: `deploy/scripts/verify-api-acceptance.ps1` (Windows), or
`sh deploy/scripts/verify-api-acceptance.sh` (CI/Linux). Runs TestAPIEndToEnd
against the actual server binary, PostgreSQL 16 and digest-pinned LLDAP 0.6.3
with Basic enabled only inside scratch. Initial application sources equal
release/r2 e74a1de. The synthetic proxy provides HTTPS; LDAPS validates a generated
CA. Human OIDC uses a synthetic RSA/JWKS issuer through the production verifier.

| Check | Command | Named subtest under TestAPIEndToEnd | Result | Date |
|---|---|---|---|---|
| Wrong bot password → 401 | A | wrong_password_401 | PASS | 2026-10-22 |
| Valid nonmember → 403 | A | nonmember_403 | PASS | 2026-10-22 |
| Human OIDC HTTP send and sync unchanged | A | human_oidc_send_sync | PASS | 2026-10-22 |
| Human ticket/WS send and live delivery | A | human_websocket_send | PASS | 2026-10-22 |
| Bot send → 201; live WS carries bot kind | A | bot_send_201_live_kind | PASS | 2026-10-22 |
| Same key/body → identical response, one stored message | A | identical_replay_one_row | PASS | 2026-10-22 |
| Same key/different text → 409 | A | changed_body_409 | PASS | 2026-10-22 |
| Sync includes bot message with bot kind and principal ID | A | bot_sync_kind | PASS | 2026-10-22 |
| channel_id body field → 400 | A | channel_id_400 | PASS | 2026-10-22 |
| Basic denied ticket endpoint and HEAD sync | A | basic_route_isolation | PASS | 2026-10-22 |
| Group removal → 403 on very next request | A | group_removal_next_request_403 | PASS | 2026-10-22 |
| Burst → 429 with integer Retry-After >=1 | A | burst_429_integer_retry | PASS | 2026-10-22 |
| Stopped directory → 503 with Retry-After | A | directory_down_503_retry | PASS | 2026-10-22 |
| Human HTTP send/sync continue during directory outage | A | human_unaffected_by_directory_down | PASS | 2026-10-22 |
| Binary startup runs migrations to version 6, not dirty | A | startup_migrations_clean | PASS | 2026-10-22 |

Combined result: 15 subchecks plus parent test passed (8.12s). Generic Go skips
this fixture-dependent test; it is NOT counted as a generic pass. Dedicated
api-acceptance CI job runs command A and fails on any failed check. The existing
LDAP lifecycle harness remains separate for password rotation/recreation/TLS cases.

### Reproduction commands for component and client rows

**B**: from repository root, use a fresh disposable project name, then run
`docker compose -p <scratch-project> -f deploy/verify.compose.yml run --rm -e TZ=UTC server`
and the same command with `TZ=Asia/Kolkata`. The server service runs full
`go test -race -count=1 ./...` and `go vet ./...` against real PostgreSQL.
**C**: `docker compose -p <scratch-project> -f deploy/verify.compose.yml run --build --rm client`
(Flutter analysis, tests and release web build).
**D**: `deploy/scripts/verify-bot-auth.ps1` or its `.sh` counterpart.
Restore/web smoke run through the same project's `restore` / `web-smoke` services;
finish with `docker compose ... down --volumes --remove-orphans` on that scratch
project only. These commands never use deploy/docker-compose.yml or live env files.

| Additional acceptance area | Named tests | Command | Result | Date |
|---|---|---|---|---|
| uid/cn stability; recreated account distinct; human unchanged | TestGateStableUsernameAndRecreatedPrincipal; TestDirectoryUsesVerifiedUIDNotDisplayName | B | UTC/IST PASS | 2026-10-22 |
| OIDC signature/issuer/audience/expiry; bearer-only input | TestOIDCVerifier_Rejections; TestOIDCVerifier_MandatoryClaims; TestOIDCVerifier_StrictConfiguredAudience; TestExtractToken_RequiresBearerHeader | B | UTC/IST PASS | 2026-10-22 |
| Gate method/path/proxy boundaries; bounded concurrency/deadline | TestMessageGateBoundaries; TestConcurrencyAndCancellation; TestTransportDeadlineAndConfiguration | B | UTC/IST PASS | 2026-10-22 |
| One-use tickets under concurrency and expiry | TestWSTicketSingleUseConcurrent; TestWSTicketExpiryAndInvalidInput | B | UTC/IST PASS | 2026-10-22 |
| Persistence before publication; Unicode/failed writes; stopped hub | TestMessageServiceCommitBeforePublication; TestMessageServiceRejectsWithoutSideEffects; TestMessageServiceWriteFailureAndCancellation; TestMessageServiceStoppedHubRetainsCommittedResult | B | UTC/IST PASS | 2026-10-22 |
| Safe slow-client eviction and content-free coalesced push | TestHub_SendToEvictedClientDoesNotPanic; TestHub_PushIsContentFreeAndCoalesced | B | UTC/IST PASS | 2026-10-22 |
| Migration 000005 down/up, repeated and partial reapply, lifecycle | TestBotPrincipalsMigrationRollbackReapply; TestBotPrincipalsMigrationRepeatedUp; TestBotPrincipalsMigrationPartialReapply; TestBotIdentityMigrationAndLifecycle | B | UTC/IST PASS | 2026-10-22 |
| Bot shows as a peer with the · Bot label, never You | bot_sender_test.dart: bot with colliding subject is labeled as a peer | C | PASS | 2026-10-22 |
| Cache preserves kind; matching human/bot IDs are distinct | bot_sender_test.dart: sender kind survives cache and separates identical human/bot IDs | C | PASS | 2026-10-22 |
| Human login/refresh, catch-up/cache/ordering and notifications | release_regression_test.dart; chat_ui_test.dart | C | PASS (83 total Flutter tests) | 2026-10-22 |

## UST-2.5.2 recovery and release evidence (2026-10-22)

Dedicated command D: `deploy/scripts/verify-rollback.ps1` locally, or
`sh deploy/scripts/verify-rollback.sh` in CI. Uses actual v1.1.0/current binaries
and isolated PostgreSQL; generic suite skips TestReleaseRollback deliberately.

| Check / TestReleaseRollback subtest | Command | Result | Date |
|---|---|---|---|
| previous_binary_rejects_version_six | D | PASS: expected migration startup failure | 2026-10-22 |
| keep_data_force_four_previous_history_and_send | D | PASS: human history + WS send; idempotency retained | 2026-10-22 |
| roll_forward_reapplies_five_six_and_preserves_rows | D | PASS: clean version 6; 3 messages + 1 key retained | 2026-10-22 |
| bot_row_breaks_previous_history_after_force_four | D | PASS: old history 500, current history 200 | 2026-10-22 |
| full_restore_previous_backup_loses_post_upgrade_messages | D | PASS: old history restored; post-backup data absent; new send works | 2026-10-22 |
| Full Go -race and vet | B, UTC and Asia/Kolkata | 166 passes each; 3 fixture skips each | 2026-10-22 |
| Flutter analysis/test, restore, web build/smoke | C and verify Compose restore/web-smoke | 83 tests; analysis, restore and runtime PASS | 2026-10-22 |
| Real LLDAP and combined acceptance rerun | verify-bot-auth.ps1; verify-api-acceptance.ps1 | PASS; 15 API subchecks + parent | 2026-10-22 |
| Production image history/config/all layers | image-inspection.md commands | PASS within documented candidate scope | 2026-10-22 |
| Non-publishing release packaging | release.yml workflow_dispatch, run 39785349125 | PASS: 7 build/verify jobs; publication intentionally skipped | 2026-10-22 |

No live check or production-dump verification is counted as a scratch pass.
