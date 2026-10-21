# R2 API and pourOver contract

Owner decisions recorded 2026-10-20. R2 is the authenticated message API for the
shared lounge; channels belong to R2.1 and media to R3. This is the implementation
acceptance contract, not a claim that pending HTTP routes already exist.

## Identity and authentication

Humans retain OIDC bearer authentication. Bots use Basic Auth over HTTPS only on
GET sync and POST send. Reject Basic on sockets/tickets, device registration and
all other routes, including future media. Reject malformed/ambiguous credentials;
failed bearer authentication never falls back to Basic. Validate bot passwords
and direct group membership freshly over verified LDAPS on every request,
including retries and every sync page. LLDAP 0.6.3 fixture does not support
StartTLS. No plaintext fallback, positive auth cache, local passwords or tokens.
Return 401 for invalid authentication, 403 for a valid nonmember, sanitized 503
with Retry-After for directory unavailability/overload. Human auth stays independent.

UST-2.3.4 fixes username to the verified LLDAP login ID (uid), not LDAP cn.
BotRepository.Resolve must stop overwriting username per request. cn may be kept
as a separate display_name if inexpensive. Display-name rename must not change
username; account recreation still gets a new principal. Preserve existing human
identities and messages. Human and bot usernames are NOT checked for uniqueness
against each other; pourOver relies on **sender_kind + sender_id**, never username.
Any migration must be additive, re-runnable after partial application and through
golang-migrate down/up, and preserve rows in a real scratch PostgreSQL regression.

## Sync — httptest acceptance

`GET /api/messages/sync?after=<RFC3339>&after_id=<uuid>`, with Basic Auth.

- Return 200 and a JSON array, oldest first by (created_at, id), at most 500 rows.
- Every message contains id, sender_id, username, sender_kind, text_content,
  created_at; timestamps are UTC instants, identity kinds distinguish bot/human.
- Set X-Has-More: true when more remain. A page of exactly 500 may also set it
  conservatively; a following empty page must return 200 with [] rather than null.
- Cursor uses both last processed timestamp and UUID. Equal created_at values
  across a page boundary must return every message exactly once.
- Accept an unencoded +00:00 suffix in after even when query decoding turns +
  into a space. Normalize that timezone suffix narrowly; malformed cursors still
  return 400. Test both encoded and unencoded offsets.
- pourOver polls every 3 seconds, persists its cursor after processing, deduplicates
  by message UUID and ignores its own messages using sender_kind + sender_id.

## Send — httptest acceptance

`POST /api/messages`, Basic Auth, Content-Type: application/json.
Header: `Idempotency-Key: pourover-reply-<original message id>`.
Body: `{"text_content": "…"}`.

Bots may post in the shared lounge. Replies are plain "@username" text; no
structured mention or notification semantics are implied. R2 does not require
channel_id. Human bearer callers use the same send service.

Idempotency-Key is an opaque bounded string, NOT a UUID-only field: the pourOver
prefix must be accepted. Proposed limit: 1–255 printable ASCII characters, no
control characters. Reject missing/invalid keys with 400. Authenticate before
replay; scope keys by sender_kind + sender_id. Reject spoofed sender/id/timestamp
fields. Reuse existing trim, Unicode 4,000-character and request-size validation.

Any 2xx means success to pourOver. Recommend 201 for initial commit and the SAME
stored 201, response body and application headers for replay (transport-generated
Date/request IDs excluded). Same key + same body must return an identical stored
response and exactly one message; do not use the older 201-first/200-replay design.
Same key + different body returns 409. Fingerprint the decoded text_content value
before message normalization so changed text cannot silently replay; JSON whitespace
and property formatting are not content changes. Persist result/key/message in one
transaction. At least 20 concurrent duplicate requests produce one row/broadcast
and at most one push. No publish on DB failure; crash-after-commit recovery uses
the stored response and sync. Test principal isolation, restart and expiry.

Proposed retention remains at least 168 hours, configurable. Exactly-once replay
is guaranteed only while the key is retained; test/document the expiry boundary.
After expiry clients must not assume retry deduplication. Replays consume auth/API
budgets but not a second message-write budget. WS and HTTP share message budgets.

## Backoff and budgets — httptest acceptance

- 429 includes integer Retry-After ≥ 1 (round fractional waits up).
- 503 includes Retry-After when overloaded or the directory is unavailable;
  recommend the same positive integer seconds convention for predictable clients.
- Bot request budgets must allow at least 30 requests/minute per bot. A lone bot
  polling every 3 seconds (20 polls/minute) must not hit the shared pre-auth limit.
  Allow reply traffic above that polling baseline and test a sustained polling
  schedule with a controlled clock; do not weaken abuse/concurrency protections.
- The example honors both statuses, persists cursors and retries sends with the
  same operation key/body. Do not classify 503 as a successful send.

Use httptest on real routing/auth middleware for every contract point above;
back persistence with scratch PostgreSQL and separately test real LLDAP. Fixtures
use synthetic identities. Maintain a named-test verification matrix in 2.5.1.

## Transport, configuration and rollout

Paths remain configurable with defaults /api/messages and /api/messages/sync.
Keep no-store responses, bounded bodies, fixed trusted proxy CIDRs, certificate
verification and sanitized errors. No secrets in URLs, logs or client config.
CORS permits Idempotency-Key for configured human web origins and exposes
X-Has-More and Retry-After where required; do not broaden allowed origins for bots.
Settings must be wired through server config, .env.example, Compose where needed
and the runbook at epic doc-fold. Basic stays disabled until integration is ready.
No live configuration is changed by this contract. See [owner checks](owner-checks.md).
