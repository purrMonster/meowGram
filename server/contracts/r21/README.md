# UST-2.1.1.1 executable HTTP contract fixtures

`http.json` freezes the route × raw body × actor × status matrix from the approved
R2.1 design. Raw body strings preserve duplicate JSON keys; raw paths preserve
repeated query parameters. Default send/ticket omission or JSON null means lounge.
Default sync with neither selector means lounge ONLY; literal `null`, empty,
malformed, repeated or conflicting selectors are 400. Unknown and inaccessible
UUIDs have the same 403 JSON object, without existence-specific fields.

UST-2.1.1.4: all 78 frozen fixtures passed against the real binary in UTC and
Asia/Kolkata on 2026-10-27; markers now say `passing`. Status/body/header
expectations are unchanged.

Two execution modes remain deliberately separate:

- `go test ./contracts/r21` validates fixture integrity and participates in the
  default suite. It does not establish HTTP compliance.
- `./deploy/scripts/verify-r21-contracts.ps1` (or
  `sh deploy/scripts/verify-r21-contracts.sh`) builds the real server and tagged
  runner, starts scratch PostgreSQL plus pinned LLDAP 0.6.3 with verified LDAPS,
  and replays all 78 cases in UTC and Asia/Kolkata. The dedicated adapter provides
  a synthetic JWKS issuer and restarts the server and reseeds its database before
  every case, clearing replay keys, budgets and tickets. No production service,
  host port, persistent database or real credential is used. Setup failures fail
  loudly; this dedicated run never skips.

The `r21_contracts` build tag excludes the adapter from ordinary suites, avoiding
new generic-suite skips. The low-level runner is still independently executable:
`go test -tags=r21_contracts ./contracts/r21 -run '^TestHTTPContract$' -count=1`
with the explicit disposable adapter environment described below. Successful
fixture compilation alone does not establish runtime compliance.

## Seed adapter interface (test infrastructure, not an application API)

Set `MEOWGRAM_DISPOSABLE_R21_CONTRACT=1`, `R21_CONTRACT_URL` and
`R21_CONTRACT_CONTROL_URL` to explicit loopback HTTP origins for disposable
fixtures. The runner rejects non-loopback origins and redirects. Do not point it
at live services. Secrets are not embedded in fixtures or printed on failures.

Before EACH case the runner POSTs the controller's `/reset` with
`{"seed":"lounge-and-groups-v1","case":"<fixture-id>"}`. This must reset the
scratch DB, budgets, replay keys, ticket store and in-memory state. It returns
`{"human-member":"Bearer <synthetic-token>","human-peer":"Bearer <synthetic-peer-token>","bot-member":"Basic <synthetic-value>"}`.
These are synthetic credentials issued by the test fixture, never real accounts.
The controller is implemented in `integration/r21_contract_test.go`; it is test infrastructure, never a production endpoint.

Seed these records, all timestamps strictly between epoch and 2099:

- Provisioned human `fixture-member` (the human actor), human `fixture-peer`,
  and an LDAP-eligible bot (the bot actor). Group A has all three current members,
  is initially named, and is unmuted for the human actor. Both actors can read it.
- Group A `11111111-1111-4111-8111-111111111111`; inaccessible existing group B
  `22222222-2222-4222-8222-222222222222`; nonexistent group
  `33333333-3333-4333-8333-333333333333`; no-human group
  `44444444-4444-4444-8444-444444444444` with only the bot actor retained.
- Group C `55555555-5555-4555-8555-555555555555` is another readable group with
  both the human and bot actors as current members, initially without messages.
  Group B excludes both actors. `human-peer` authenticates as `fixture-peer`.
- Lounge message `aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa`; group A message
  `bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb`; private positive-control messages in
  B and the no-human group. Default sync must show lounge, never those canaries.
  Member sync must show both lounge and A, never B/no-human. Empty-page fixtures
  use 2099 and must still carry membership revision.
- Named creation is always fresh. Selected `fixture-peer` exists. Reset per case
  makes add-existing-member idempotent, remove/leave independent, and read markers
  refer to persisted messages in their own conversation.

Final requests carry JSON Content-Type and an Idempotency-Key derived from case ID unless explicitly set;
resets prevent cross-case replay contamination. When a fixture has setup sends,
execute them in order before its asserted request, retaining state and using the
same explicit `idempotency_key` throughout. Setup steps name their method, path,
body, actor and expected status; the key is optional for non-send steps (DELETE
uses no key and expects 204). Each setup send must finish with 201 before the
asserted request runs. This establishes an actual stored response for same-key
replay and changed-text/channel conflicts; an empty replay table is not evidence.
Both requested- and stored-channel authorization must precede conflict or replay, including a denied group
request using a key already stored for a readable channel. The stored-channel case sends to A, removes `fixture-member` from A
using `human-peer` (204), then reuses that key in lounge: stored-channel access
must still fail with 403 before a conflict or replay. Management and ticket
requests are tested with human bearer and rejected with bot Basic (401).

## Frozen response metadata and semantics

Message M fields: `id`, `sender_id`, `sender_kind`, `username`, `text_content`,
`created_at` (UTC RFC3339), `channel_id` (null or opaque UUID).
Only approved pre-upgrade exact replay bytes may omit channel_id; no row rewriting.

Owner correction, 2026-10-23: sync, send, history and live message rows contain
only M. They must not include `conversation_member_count`, `conversation_muted`,
conversation name, member summaries, unread counts or other per-viewer state.
Per-viewer state would become stale in an idempotent stored replay. This corrects
the earlier approved plan's message-row metadata without changing stored replay
bytes or granting bots new routes.

`X-Membership-Version` is an opaque, nonempty revision on every `scope=member`
page, including `[]`; expose it via CORS.
Changed/missing revision requires epoch replay/backfill as described in the plan.
`X-Has-More: true` marks a full 500-row page. Rows are deduplicated and ordered by
(created_at, UUID), with authorization applied BEFORE cursor/LIMIT. This runner
checks returned rows, positive controls, types, ordering, page cap and full-page
header; 1001-row/cursor/revocation concurrency tests remain later stories.

Conversation C: `channel_id`, `kind`, `name`, `member_summary` (`count`, `members`),
`last_activity_at`, `unread_count`, `muted`. List envelopes use `conversations` and
`next_cursor`; members/picker use `members`/`principals` and `next_cursor`.
Conversation metadata lives on `GET /api/conversations` and its members route
as appropriate. These remain human-only routes; bots cannot list or manage
conversations or call the members route. The earlier bot prefix assumption
based on `conversation_member_count` on sync rows is superseded; this correction
does not introduce an alternative bot count-discovery API.
Start-chat returns `{conversation:C,created:boolean}` with 200 for new or reused;
named creation returns C with 201. Ticket response contains `ticket`, `expires_in:30`.
No-store applies to all route responses. Default/custom configured aliases must
share these contracts; alias wiring tests land with endpoint implementation.

Names, reuse races, exact replay, unread/mute reconciliation, cursor backfill and
startup compatibility retain the approved design; this matrix does not replace
the broader verification matrix or claim those later behaviors are implemented.
