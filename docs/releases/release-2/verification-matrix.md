# API verification matrix (in progress)

UST-2.4.2 entries below ran against real scratch PostgreSQL with -race; full
UTC/IST evidence is recorded in the story log before task completion. This is
not release sign-off; UST-2.4.3 and 2.5 must close the remaining rows.

| Requirement | Named test | Status |
|---|---|---|
| 20 duplicate sends, identical 201 and one row/publication | TestHTTPSendContract/concurrent_identical_response_one_publication | Focused pass |
| Durable replay, changed text 409 before normalization | TestHTTPSendContract/restart_replay_conflict_and_normalization | Focused pass |
| Real OIDC human and Basic bot, revocation before replay, kind isolation | TestHTTPSendContract/reauthentication_and_isolation | Focused pass |
| 429/503 positive Retry-After | TestHTTPSendContract/backoff | Focused pass |
| Strict body/key/size validation | TestHTTPSendContract/validation | Focused pass |
| Atomic failure rollback, committed hub failure replays without republishing | TestHTTPSendContract/failure_rollback_and_durable_publication_failure | Focused pass |
| Shared HTTP/WebSocket write allowance | TestHTTPSendContract/HTTP_and_WebSocket_share_human_write_budget | Focused pass |
| Idempotency-Key CORS | TestHTTPSendContract/cors | Focused pass |
| Additive 000006 up/down/up, direct repeated up, partial up, legacy rows preserved | TestIdempotencyMigrationRollbackAndPartialReapply | Focused pass |
| Expiry boundary, message/key transaction failure | TestIdempotencyExpiryAndTransactionFailure | Focused pass |
| Disabled default, retention bounds, minimum bot budgets, route configuration | TestHTTPSendConfiguration | Focused pass |
| Real LDAP behavior | TestLLDAP | Dedicated provider harness required |
| Bot GET sync, paging/equal timestamps/unencoded offset/empty page | UST-2.4.3 | Pending implementation |
| Sustained polling plus replies, cursor/retry example | UST-2.4.3 | Pending implementation |
| Integrated release, owner pilot, native device | UST-2.5.1/2.5.2 and owner-checks.md | Pending integration |
