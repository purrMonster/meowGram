# Release 2 delivery and acceptance

Status: updated 2026-10-17. UST-2.3.1 has passing isolated LLDAP v0.6.3
evidence and a selected identity design, matching the owner's reported deployed
version. Live transport remains a rollout gate. Other stories remain planned. See [scope](README.md),
[API contract](api-design.md) and [investigation](directory-identity.md).

## Work breakdown

| Story | Deliverable | Acceptance / dependency |
|---|---|---|
| UST-2.1.1–2.1.2 | Existing Immich authentication/proxy and asset persistence stories | Retain roadmap scope; review current provider contract before implementation; secrets stay server-side |
| UST-2.2.1–2.2.2 | Existing media picker and feed rendering stories | Depend on media contracts; verify supported platforms and offline behavior |
| UST-2.3.1 | LLDAP compatibility and identity design | Isolated v0.6.3 probe passed, matching owner-reported version; LDAPS, direct membership, display-name stability and recreation verified. Login-ID rename is unsupported by the tested API. Identity design selected; live transport remains a rollout gate. See investigation. |
| UST-2.3.2 | Group-gated Basic authentication | Depends on 2.3.1; only message routes accept it; credentials and membership checked per request; directory failure denies bots without affecting OAuth callers |
| UST-2.3.3 | Principal mapping and lifecycle | Depends on 2.3.1; no OIDC collision or username linking; group removal blocks subsequent requests; history survives revocation |
| UST-2.4.1 | Shared message service | Existing WS semantics retained; HTTP and WS use one validation, budget, persistence and broadcast path |
| UST-2.4.2 | HTTP send and durable idempotency | Depends on 2.3.2–2.3.3 and 2.4.1; concurrent retries yield one row and original response; payload conflicts return 409 |
| UST-2.4.3 | Authenticated bot sync and examples | Depends on 2.3.2–2.3.3; existing pagination contract retained; every page checks access; examples persist cursors and honor retry limits |
| UST-2.5.1 | Integrated regression and security checks | All API acceptance cases below pass in disposable infrastructure |
| UST-2.5.2 | Release documentation and authorized pilot | Configuration/runbook/examples complete; Go and Flutter suites pass; owner authorizes external provisioning and deployment |

Recommended API sequence: 2.3.1 → identity/auth implementation and shared
message service → HTTP send/read → integration checks → authorized pilot.
Use the release/epic/task hierarchy below and owner-merged PRs. Maintain
Conventional Commits and the required repository author. Do not force-push or
merge implementation branches on the owner's behalf.

## Branch hierarchy

Owner-directed workflow, recorded 2026-10-17. These branches are local until
published; naming does not imply an existing PR. Integration branches initially
share the current `origin/main` baseline. Existing uncommitted planning and
investigation files remain in the active task checkout; they are not yet commits
on the integration branches.

| Branch | Parent / PR target | Scope |
|---|---|---|
| `release/r2` | `main` | Complete R2 integration, including media and API |
| `feat/r2-epic-2.1-media-backend` | `release/r2` | Immich backend |
| `feat/r2-epic-2.2-media-client` | `release/r2` | Flutter media experience |
| `feat/r2-epic-2.3-bot-identity` | `release/r2` | Directory authentication and identity |
| `feat/r2-epic-2.4-message-api` | `release/r2` | Shared message service, HTTP send/read |
| `test/r2-epic-2.5-verification` | `release/r2` | Integrated verification and release readiness |
| `feat/ust-2.3.1-directory-identity` | `feat/r2-epic-2.3-bot-identity` | Current directory investigation |
| `docs/release-2-planning` | `release/r2` | Release-wide planning and workflow foundation |

Before publishing, separate the existing uncommitted release-wide foundation
from the investigation into their respective branches/PRs. Merge the foundation
through owner review before integrating the investigation. New task branches
start from their latest approved epic parent; epic PRs target `release/r2`.
The completed release reaches `main` through one final owner-merged PR.
For example, UST-2.4.1 starts from `feat/r2-epic-2.4-message-api`, not `main`.
Shared dependencies reach `release/r2` through their epic PR before other epics
incorporate them; do not cherry-pick sibling implementation as an implicit shortcut.

## Required verification

| Area | Evidence required |
|---|---|
| Authentication | Anonymous, empty, malformed and incorrect credentials rejected; valid nonmember denied; member accepted; ambiguous Authorization rejected; bearer failure cannot fall back to Basic |
| Transport | Invalid LDAP certificate and failed StartTLS fail closed; insecure public ingress rejected; no secrets in logs or diagnostics |
| Route isolation | Basic cannot obtain WS tickets, open sockets, register push devices or access future media routes; OAuth regression coverage remains green |
| Lifecycle | Group removal, password rotation, deletion and directory outage tested between requests, including idempotent replays and successive read pages |
| Identity | Rename preserves sender identity; deleted/recreated account does not inherit old identity; same-name OIDC account remains distinct; old messages remain readable |
| Persistence | Concurrent duplicate keys create one message; changed content conflicts; principal isolation, restart replay, expiry boundary and rollback on failed writes verified against PostgreSQL |
| Delivery | Committed HTTP message reaches connected clients; DB failure emits nothing; post-commit failure recoverable via retry/sync; no duplicate push on replay |
| Reading | More than 500 messages, equal timestamps, empty pages, restarts and UUID deduplication exercised |
| Budgets | LDAP concurrency/timeouts, pre-auth budgets, per-principal limits and 429 Retry-After behavior verified; slow clients do not block broadcast |
| Compatibility | Existing Flutter login, history, live chat and cache remain functional; configured endpoints and web CORS idempotency header work |

Run `go test ./...` from `server` and `flutter test` from `client` before asking
for merge. Use the repository scratch harness for race detection, Go vet,
PostgreSQL migrations, Flutter analysis and web build as appropriate. Extend
it with a disposable directory fixture; use synthetic identities, no live
environment files or credentials. Record actual results, not planned passes.

Documentation-only changes require link/consistency and whitespace checks;
this plan does not claim application suites have run for implementation that
does not exist yet.

## Release gates

1. Directory and identity design resolved with reproducible evidence; remaining
   proposed configuration defaults settled and recorded.
2. API and media stories complete, or owner explicitly approves a narrower
   release. API v1 completion alone is not Release 2 completion.
3. Required suites and integration checks pass; examples explain idempotency
   retention, cursor handling and access revocation semantics.
4. Additive migrations preserve existing users/messages and support disabling
   the feature without deleting data. Review compatibility before any binary
   rollback; do not run destructive down migrations as a rollback shortcut.
5. Owner authorizes group/account/reader-credential provisioning and any live
   deployment or external identity changes. Secrets are generated on their
   target host and retained only in ignored environment files or secret mounts.
6. Pilot one dedicated bot: send, read, retry, remove group membership, confirm
   denial, and verify a human OAuth session remains operational. Any restoration
   of group membership is part of the approved pilot procedure.

## Rollout and rollback design

Deploy with Basic disabled by default. Validate configured HTTPS ingress and
encrypted directory access, then enable for the pilot group only after owner
approval. Monitor sanitized auth outcomes, LDAP latency, 429/503 rates and
message insertion counts. Define acceptable operating thresholds during the
pilot; no throughput claims are made here.

The primary rollback is disabling Basic Auth while retaining human OAuth and
persisted messages. Group removal stops an individual bot. Preserve additive
identity/idempotency data for investigation and a subsequent forward fix.
No credential, account, live server or database changes are performed by
creating these planning documents.

## Estimate

API work is provisionally 1–2 days for directory/identity validation, 2–4 days
for authentication and identity persistence, 2–4 days for send/read and retry
handling, and 1–3 days for integration/docs: roughly 6–13 focused engineering
days. These are estimates, not a deadline; schema constraints may change them.
Existing media work is additional and needs separate estimation after provider
and platform review.
