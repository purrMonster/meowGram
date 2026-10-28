> Historical archive copied for owner task A.2 on 2026-10-28. These notes are not current release requirements or implementation evidence. Source: rescue/feat/tabbytally-review-integration (archive only; never merge). No code or migrations were copied. The tabbyTally repository was inaccessible, so the owner-approved neutral documentation fallback is used. R2.1 requires trigger prefixes everywhere; historical prefix-free proposals below are superseded. Any historical relative runbook/release links refer to the source branch.
# pourOver: authenticated message API requirements

## Record and scope

- **Requesting project:** pourOver, FR-14; adapter S3.1.1 and requirements handoff S3.1.2.
- **Receiving project:** meowGram, Release 2 authenticated lounge message API.
- **Recorded:** 21 October 2026.
- **Status:** consumer adapter reported merged; provider HTTP integration and acceptance verification pending in the source records below. This document does not assert deployment readiness.
- **Scope:** record existing requests and the agreed response. No implementation, credentials, deployment or release merge is authorized by this document.

pourOver turns chat commands into Vikunja tasks and replies in the same chat.
meowGram supplies authenticated message transport and reliable sender identity;
pourOver owns command interpretation, sender-to-Vikunja mapping and task permissions.
Unknown senders must not gain task access merely by appearing in chat.

## Sources and precedence

This is a synthesis of local project records, read on 21 October 2026:

1. Sister repository `pourover-docs/runbook.md`, status and decisions dated
   19 October: polling adapter, configurable trigger/channel, proposed wire
   contract, merged PR #6 and historical test results.
2. Sister repository `pourover-docs/requirements.md` (FR-14),
   `planning/backlog.md` (S3.1.1/S3.1.2) and `architecture.md`: integration
   ownership and known-sender mapping.
3. meowGram's separate development checkout, `runbook.md` sections 13.6–13.8,
   dated 21 October: stable identity, approved release scope and exact pourOver
   acceptance contract. These newer sections are not yet folded into this parked
   integration branch.
4. That checkout's `docs/releases/release-2/stable-bot-usernames.md`: UST-2.3.4
   response to the stable bot username requirement (original R10).

The original external handoff is described by pourOver as ten requirements and
eight questions. Its full text was not inspected here. The PO identifiers below
are this document's traceability IDs, not a reconstruction of that original list.
The newer approved meowGram scope governs delivery: **R2 lounge API; R2.1 channels
proposal only; R3 media**. Consumer requests do not override that scope.

## Requested requirements and agreed response

| ID | pourOver need | meowGram response / delivery boundary |
| --- | --- | --- |
| PO-01 | A polling bot can read commands and post replies without interactive human sign-in. | R2 bot Basic auth on GET sync and POST send only; no inbound callback to pourOver required. |
| PO-02 | Identify human senders and ignore bots/unknown senders. | Return `sender_id`, `sender_kind` and `username`; use kind plus ID for identity, never username alone. pourOver owns its known-sender mapping. |
| PO-03 | Read new messages without pagination omissions. | Ordered timestamp/UUID cursor, bounded pages and continuation indication; see exact contract below. |
| PO-04 | Retrying a reply must not create a second message. | Durable send idempotency accepting `pourover-reply-<original id>`; matching replay and conflicting-payload behavior below. |
| PO-05 | Reply to the sender in the lounge. | Plain `@username` text in `text_content`; any successful 2xx send response is accepted by the consumer contract. |
| PO-06 | Poll every three seconds without routine throttling failure. | At least 30 requests/minute per bot and sufficient shared ingress capacity; 429 and transient 503 include integer `Retry-After` of at least one second. Consumer honors backoff. |
| PO-07 | Stable bot username despite display-name changes. | Directory-verified `uid`, not `cn` or caller-supplied Basic spelling; existing identity is not renamed by display changes. Recreated directory accounts get distinct identities. |
| PO-08 | Configurable channel and trigger, optionally no trigger in a dedicated channel. | Consumer records `MEOWGRAM_CHANNEL` and `MEOWGRAM_TRIGGER`, default trigger `@pourover`. Dedicated channels are deferred to the R2.1 proposal; the R2 pilot uses the lounge. |

### Sync contract: PO-02 / PO-03 / PO-06

```http
GET /api/messages/sync?after=<RFC3339>&after_id=<uuid>
Authorization: Basic <host-local bot credentials>
```

- Return 200 with a JSON array, oldest `created_at` then UUID first, at most 500
  messages per page. Each message includes `id`, `sender_id`, `username`,
  `sender_kind`, `text_content` and `created_at`.
- `X-Has-More` indicates continuation. An exactly full page may indicate more
  followed by a valid 200 empty array. Equal timestamps across pages must neither
  skip nor repeat messages when the composite cursor is advanced correctly.
- Accept an unencoded `+00:00` offset decoded as a space, as recorded in the
  approved compatibility contract.
- Consumer persists its cursor, starts at “now” on first run, ignores bots and
  unknown senders and uses its recorded at-most-once processing policy.
  Transport idempotency alone does not promise exactly-once Vikunja operations.

### Send contract: PO-04 / PO-05

```http
POST /api/messages
Authorization: Basic <host-local bot credentials>
Idempotency-Key: pourover-reply-<original-message-id>
Content-Type: application/json

{"text_content":"@<username> <reply>"}
```

The same key and body replay the identical response and create one message.
Reusing the key with a different body returns 409. Concurrent retries, process
restart, retention boundaries and failed writes must be covered by tests.
The prefixed key is deliberately **not UUID-only**. The parked tabbyTally callback
prototype is not the acceptance reference; future consumers must use the merged
R2 contract after its dependency gate passes.

### Channel request: PO-08

The proposal uses nullable UUID `channel_id` for lounge compatibility, a send
field and sync filter, membership authorization (403 for non-members), bot
membership administration, channel-scoped WebSocket/push and client cache migration.
These are planning requirements, not available R2 behavior. Do not enable channel
parameters against R2 until the eventual channel contract is implemented and tested.

## Completed work and evidence limits

- pourOver's 19 October runbook reports adapter PR #6 merged: three-second
  polling, at-most-once handling, first-run boundary, sender filtering, addressed
  replies, retry-safe keys and configurable trigger/channel. It reports **128
  tests passed, 6 skipped** on main. These are historical source results, not
  tests rerun for this document; the local pourOver checkout inspected here does
  not contain that adapter, so its implementation was not independently audited.
- meowGram's 21 October records describe bot identity foundations including
  verified stable usernames and shared message-service work. They explicitly
  leave HTTP send/idempotency and bot sync integration pending. Foundation tests
  do not certify the end-to-end pourOver contract.
- This documentation change adds the requirements register entry, consolidates
  the consumer/provider responsibilities and preserves the channel deferral.

## Planned work and acceptance

1. Finish and merge the R2 send/idempotency and bot sync contracts through the
   existing story workflow (UST-2.4.2 / UST-2.4.3), then perform API-only release
   verification. The owner controls merges and pilot authorization.
2. Verify Basic is accepted only on intended routes, invalid credentials fail,
   bot/human identities remain distinct and directory outages return retryable
   errors without changing identity. Verify display-name changes and recreated
   accounts do not relink historical senders.
3. Run sync tests for equal timestamps, multiple pages, exact 500 boundaries,
   empty results, restart cursor recovery and timezone-offset compatibility.
4. Run send tests with the actual prefixed key: sequential/concurrent replay,
   conflicting body, restart, failed persistence and retention boundaries.
5. Exercise a pourOver command and addressed reply with test identities, unknown
   sender refusal, bot-loop prevention, three-second polling and 429/503 backoff.
   Confirm the consumer's documented at-most-once crash behavior separately from
   provider reply deduplication.
6. Before an explicitly approved pilot, verify directory reader visibility of
   `uid` and bot group membership, and proxy forwarding of `Authorization` to
   `/api/*` without an SSO interception. Keep Basic disabled until integration
   and pilot approval. Preserve schema/history when disabling the feature.

Configuration examples remain placeholders; real credentials and endpoints stay
in ignored host-local configuration. No new callback hostname is needed. The
pourOver internal `/inbound` service interface is its own adapter boundary, not a
requirement for meowGram to expose or invoke an inbound callback.

## Where we stopped

Requirements recorded only. No API implementation, credentials, DNS or live
services changed. The tabbyTally integration branch stays parked until **both
UST-2.4.2 and UST-2.4.3 are merged into `release/r2`**. Reconcile the actual merged
contract with both consumers before resuming their integration. Channels remain
proposal-only.

See the [requirements register](README.md),
[tabbyTally requirements](tabbytally-review-bot.md) and [branch runbook](../../runbook.md).
