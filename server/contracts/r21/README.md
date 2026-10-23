# UST-2.1.1.1 executable HTTP contract fixtures

`http.json` freezes the route × raw body × actor × status matrix from the approved
R2.1 design. Raw body strings preserve duplicate JSON keys; raw paths preserve
repeated query parameters. Default send/ticket omission or JSON null means lounge.
Default sync with neither selector means lounge ONLY; literal `null`, empty,
malformed, repeated or conflicting selectors are 400. Unknown and inaccessible
UUIDs have the same 403 JSON object, without existence-specific fields.

Two intentionally separate execution modes:

- `go test ./contracts/r21` validates fixture integrity. This is green now and is
  included in existing `go test ./...`; it does NOT establish server compliance.
- `go test -tags=r21_contracts ./contracts/r21 -run '^TestHTTPContract$' -count=1`
  replays every fixture against an explicitly seeded disposable HTTP server.
  **PENDING IMPLEMENTATION / EXPECTED TO FAIL on R2.0**. The build tag is never
  enabled in default CI. Missing fixture setup fails loudly, never skips. No
  runtime R2.1 pass is claimed by this contract-only story.

The tagged runner compiles now. Future implementation stories supply the seed
adapter and make these assertions pass; they must not weaken expected statuses.
No production routes, migrations, authentication or configuration are changed.

## Seed adapter interface (test infrastructure, not an application API)

Set `MEOWGRAM_DISPOSABLE_R21_CONTRACT=1`, `R21_CONTRACT_URL` and
`R21_CONTRACT_CONTROL_URL` to explicit loopback HTTP origins for disposable
fixtures. The runner rejects non-loopback origins and redirects. Do not point it
at live services. Secrets are not embedded in fixtures or printed on failures.

Before EACH case the runner POSTs the controller's `/reset` with
`{"seed":"lounge-and-groups-v1","case":"<fixture-id>"}`. This must reset the
scratch DB, budgets, replay keys, ticket store and in-memory state. It returns
`{"human-member":"Bearer <synthetic-token>","bot-member":"Basic <synthetic-value>"}`.
These are synthetic credentials issued by the test fixture, never real accounts.
The controller is not part of this story or the production route contract.

Seed these records, all timestamps strictly between epoch and 2099:

- Provisioned human `fixture-member` (the human actor), human `fixture-peer`,
  and an LDAP-eligible bot (the bot actor). Group A has all three current members,
  is initially named, and is unmuted for the human actor. Both actors can read it.
- Group A `11111111-1111-4111-8111-111111111111`; inaccessible existing group B
  `22222222-2222-4222-8222-222222222222`; nonexistent group
  `33333333-3333-4333-8333-333333333333`; no-human group
  `44444444-4444-4444-8444-444444444444` with only the bot actor retained.
- Lounge message `aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa`; group A message
  `bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb`; private positive-control messages in
  B and the no-human group. Default sync must show lounge, never those canaries.
  Member sync must show both lounge and A, never B/no-human. Empty-page fixtures
  use 2099 and must still carry membership revision.
- Named creation is always fresh. Selected `fixture-peer` exists. Reset per case
  makes add-existing-member idempotent, remove/leave independent, and read markers
  refer to persisted messages in their own conversation.

All requests carry JSON Content-Type and a unique Idempotency-Key derived from
case ID; resets prevent cross-case replay contamination. Management and ticket
requests are tested with human bearer and rejected with bot Basic (401).

## Frozen response metadata and semantics

Message M fields: `id`, `sender_id`, `sender_kind`, `username`, `text_content`,
`created_at` (UTC RFC3339), `channel_id` (null or opaque UUID).
Only approved pre-upgrade exact replay bytes may omit channel_id; no row rewriting.

Sync adds `conversation_member_count`: integer for a group (three in this seed),
null for lounge. Human sync additionally adds boolean `conversation_muted`; bot
sync does not expose that human state. `X-Membership-Version` is an opaque,
nonempty revision on every `scope=member` page, including `[]`; expose it via CORS.
Changed/missing revision requires epoch replay/backfill as described in the plan.
`X-Has-More: true` marks a full 500-row page. Rows are deduplicated and ordered by
(created_at, UUID), with authorization applied BEFORE cursor/LIMIT. This runner
checks returned rows, positive controls, types, ordering, page cap and full-page
header; 1001-row/cursor/revocation concurrency tests remain later stories.

Conversation C: `channel_id`, `kind`, `name`, `member_summary` (`count`, `members`),
`last_activity_at`, `unread_count`, `muted`. List envelopes use `conversations` and
`next_cursor`; members/picker use `members`/`principals` and `next_cursor`.
Start-chat returns `{conversation:C,created:boolean}` with 200 for new or reused;
named creation returns C with 201. Ticket response contains `ticket`, `expires_in:30`.
No-store applies to all route responses. Default/custom configured aliases must
share these contracts; alias wiring tests land with endpoint implementation.

Names, reuse races, exact replay, unread/mute reconciliation, cursor backfill and
startup compatibility retain the approved design; this matrix does not replace
the broader verification matrix or claim those later behaviors are implemented.
