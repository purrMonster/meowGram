# Lounge and member-stream bot example (R2.1)

A Go standard-library client for the authenticated lounge API. Run one instance
per state file and bot account. It polls every three seconds, walks timestamp/UUID
pages, and replies `@username received`. It ignores its own messages using BOTH
sender_kind and sender_id, not a username comparison. Human and bot names/IDs
are not required to be unique across kinds.

From `server`, set these environment variables and run `go run ./examples/lounge-bot`:

| Variable | Value |
|---|---|
| MEOWGRAM_SYNC_URL | `https://example.home.arpa/api/messages/sync` (actual configured HTTPS URL) |
| MEOWGRAM_SEND_URL | `https://example.home.arpa/api/messages` (actual configured HTTPS URL) |
| MEOWGRAM_USERNAME | Dedicated LLDAP login uid |
| MEOWGRAM_PASSWORD_FILE | Absolute path to a private, gitignored password file |
| MEOWGRAM_BOT_ID | Stable bot principal UUID, not LDAP UUID or display name |
| MEOWGRAM_STATE_FILE | Absolute path to a private, durable cursor JSON file outside tracked source |
| MEOWGRAM_CHANNEL | Empty; R2 rejects channel_id with 400 |
| MEOWGRAM_SCOPE | Empty for legacy lounge-only polling; `member` for R2.1 merged lounge and readable groups |

Use the principal's sender_id from a successful authenticated send response.
System roots verify HTTPS certificates; redirects are rejected. The example
never prints credentials or response bodies. Live accounts and pilot activation
remain owner steps in `docs/releases/release-2/owner-checks.md`.

A missing state file starts at the Unix epoch and processes existing lounge
history. For a live pilot, choose a starting timestamp by creating the state
file through an initial controlled run, or inspect the example before enabling
replies to old history. Keep the state file across process/container restarts.
The file is bound to the endpoint URLs, uid and principal ID; changed accounts
require a separate state file. Do not delete it to recover a transient failure.

Before sending, the client durably saves the exact body and
`Idempotency-Key: pourover-reply-<original message id>`. It advances the cursor
only after any 2xx response, or after skipping an own message. Restart drains
that pending operation first, so a lost response replays the same operation.
429 and 503 honor positive integer Retry-After, with exponential fallback
(3–60 seconds); transport failures also retry. Other non-2xx responses stop
with state retained. A pending operation older than 168 hours stops for manual
reconciliation because the minimum idempotency retention guarantee has ended.

Verification: `TestExampleScratchRestartAndBackoff` runs this client against
TLS httptest production routing/auth gates and real scratch PostgreSQL. It proves
429/503 delay handling, a committed send with a lost response, restart replay,
one publication/row, persistent cursor, own-kind/ID filtering, and expiry refusal.
The generic Go harness runs it with TEST_DATABASE_URL pointing to /verification.

## R2.1 member mode

Set `MEOWGRAM_SCOPE=member` and use a separate state file. Its account identity
includes the scope, preventing an old lounge checkpoint from silently skipping
older group history. A human must first add the existing bot principal to groups.
The bot calls only send/sync; it never lists or manages conversations.

Member polling sends `scope=member` and persists `X-Membership-Version` alongside
the merged cursor. A changed revision durably resets that cursor to the epoch;
the next poll backfills accessible history while retaining processed message IDs.
An absent revision fails closed without advancing the cursor. Per-conversation
processing positions are separate from the merged checkpoint. Keep the state
file across restarts and deliberately choose a history baseline before a pilot.
This small fixture retains processed IDs indefinitely; operators must account
for state-file growth instead of deleting deduplication history.

Replies copy the triggering message's `channel_id`, including null for lounge,
into the durable pending body. Retry/restart preserves that body and key exactly.
A private 403 stops with the pending operation retained; it never sends to lounge
as a fallback. The 168-hour ambiguous-send limit still applies.

This is an acknowledgement fixture: it replies to every non-self message, as
the R2 example did. It does not implement pourOver's trigger policy or infer group
size from historical senders. Message rows contain no member-count metadata.

`TestBotConversationRoutingRevisionRestart` uses a synthetic HTTPS server to
verify source-channel routing, lost-response replay, revision backfill across a
restart, deduplication and missing-revision refusal. `TestBotPrivateDenialNeverFallsBack`
checks a retained 403 operation across restart. The existing real-PostgreSQL test
continues to verify production send/sync routing and durable idempotency.
