# Lounge bot example (R2)

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
