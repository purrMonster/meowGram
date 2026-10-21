# API verification matrix (in progress)

UST-2.4.2 entries below ran against real scratch PostgreSQL with -race; full
UTC/IST evidence is recorded in the story log before task completion. This is
not release sign-off; UST-2.4.3 and 2.5 must close the remaining rows.

| Requirement | Named test | Status |
|---|---|---|
| 20 duplicate sends, identical 201 and one row/publication | TestHTTPSendContract/concurrent_identical_response_one_publication | UTC/IST pass, 2026-10-21 |
| Durable replay, changed text 409 before normalization | TestHTTPSendContract/restart_replay_conflict_and_normalization | UTC/IST pass, 2026-10-21 |
| Real OIDC human and Basic bot, revocation before replay, kind isolation | TestHTTPSendContract/reauthentication_and_isolation | UTC/IST pass, 2026-10-21 |
| 429/503 positive Retry-After | TestHTTPSendContract/backoff | UTC/IST pass, 2026-10-21 |
| Strict body/key/size validation | TestHTTPSendContract/validation | UTC/IST pass, 2026-10-21 |
| Atomic failure rollback, committed hub failure replays without republishing | TestHTTPSendContract/failure_rollback_and_durable_publication_failure | UTC/IST pass, 2026-10-21 |
| Shared HTTP/WebSocket write allowance | TestHTTPSendContract/HTTP_and_WebSocket_share_human_write_budget | UTC/IST pass, 2026-10-21 |
| Idempotency-Key CORS | TestHTTPSendContract/cors | UTC/IST pass, 2026-10-21 |
| Additive 000006 up/down/up, direct repeated up, partial up, legacy rows preserved | TestIdempotencyMigrationRollbackAndPartialReapply | UTC/IST pass, 2026-10-21 |
| Expiry boundary, message/key transaction failure | TestIdempotencyExpiryAndTransactionFailure | UTC/IST pass, 2026-10-21 |
| Disabled default, retention bounds, minimum bot budgets, route configuration | TestHTTPSendConfiguration | UTC/IST pass, 2026-10-21 |
| Real LDAP behavior | TestLLDAP | PASS 2.79s, dedicated LLDAP 0.6.3 harness |
| Bot GET sync, paging/equal timestamps/unencoded offset/empty page | UST-2.4.3 | Pending implementation |
| Sustained polling plus replies, cursor/retry example | UST-2.4.3 | Pending implementation |
| Integrated release, owner pilot, native device | UST-2.5.1/2.5.2 and owner-checks.md | Pending integration |
| Human deletion cascades through HTTP-sent message and key | TestHTTPSendContract/user_deletion_cascades_idempotency | UTC/IST pass, 2026-10-21 |
| Transactional expiry cleanup bounded at 100; replay cleanup; active keys preserved | TestIdempotencyBoundedExpiredCleanup | UTC/IST pass, 2026-10-21 |
