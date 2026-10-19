# UST-2.4.1 shared message service

Date: 2026-10-17. Owner requested proceeding while merges are deferred.

| Branch | Parent and eventual PR target |
|---|---|
| `refactor/ust-2.4.1-shared-message-service` | `feat/r2-epic-2.4-message-api` |
| `feat/r2-epic-2.4-message-api` | `release/r2` |

This task starts from its declared epic parent and does not import sibling
implementation. Release-wide planning and the directory investigation are
preserved in the named Git stash `R2 planning and UST-2.3.1 investigation before
UST-2.4.1`, created on `feat/ust-2.3.1-directory-identity`. Restore that stash on
its original branch after saving this task's work; do not pop it into this task.
Integrate this record into the release delivery plan after owner-merged planning.

The shared service accepts authenticated sender identity and text, applies the
principal budget and text validation, commits through the message repository,
then publishes through the Hub. Each adapter gets a service using the same
Hub-owned budget so a new adapter or socket cannot reset a principal's limit.
Push coalescing and slow-client eviction remain Hub responsibilities.

WebSocket decoding continues to accept raw text and the existing JSON envelope.
Empty and binary frames still consume the attempt budget but do not persist.
The five-second write deadline honors an earlier caller deadline. Typed service
errors distinguish empty text, length, throttling with retry duration,
persistence failure, and post-commit Hub shutdown. A committed result is retained
on publication failure so future HTTP handling can distinguish durability from
delivery. HTTP routing and transactional idempotency are later tasks; callers
must not blindly retry this service after a committed result.

Acceptance: service tests cover commit/publication ordering, failed/cancelled
writes, Unicode boundaries, shared concurrent budgets, principal isolation and
retained durable results after Hub shutdown. Existing real HTTP/WebSocket and
PostgreSQL integration tests exercise the adapter and repository together.
Run the repository scratch harness for Go race tests/vet, PostgreSQL, Flutter
analysis/tests and web checks. Record actual results in the runbook.
