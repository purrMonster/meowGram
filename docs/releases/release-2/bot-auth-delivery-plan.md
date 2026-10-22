# UST-2.3.2: group-gated bot authentication

## Branch and integration

| Branch | Parent / PR target |
|---|---|
| `release/r2` | `main` |
| `feat/r2-epic-2.3-bot-identity` | `release/r2` |
| `feat/ust-2.3.2-bot-auth` | `feat/r2-epic-2.3-bot-identity` |

Started from epic commit `0e413339ea0a1cc4d37f0e588bcd8704117a1106`
after fetching origin on 2026-10-18. Merges remain owner-deferred. No sibling
implementation was merged or cherry-picked. UST-2.3.1 established the verified
LLDAP 0.6.3 schema contract; UST-2.3.3 supplies the separate principal repository.

This task delivers the LDAP validator, restricted message-authentication gate,
configuration loader and isolated provider verification. **Basic Auth is not
activated on a production route.** `config.Load` rejects an enabled flag on this
standalone branch. Existing bearer authentication remains unchanged.

After owner integration, the message API tasks must:

1. Replace the temporary startup guard with construction of one shared Directory
   and message Gate when enabled. Initialize neither LDAP nor its reader secret
   when disabled.
2. Adapt `BotRepository.Resolve(ctx, namespace, externalID, displayName)` to
   `botauth.Resolver`, returning the local UUID and display label. This callback
   runs only after fresh directory validation and must honor its deadline.
3. Configure exact GET sync paths (including any alias) and POST send paths,
   sharing Directory/concurrency limits and budgets. `HumanOnly` is the existing
   OIDC plus human API-budget chain. Keep global ingress limiting outside both
   branches; give the gate a shared bounded bot pre-auth budget and bot API budget
   using existing configured global/API rate values.
4. Teach message handlers to accept `PrincipalFromContext` and use bot-aware
   persistence. Do not wrap ticket, socket, device or media routes. The gate
   checks exact method/path pairs if accidentally placed around a wider mux.
5. Run end-to-end HTTP send/read and identity tests before enabling. Configure
   trusted proxy peers and HTTPS ingress only with owner authorization.

## Security contract

- Exactly one Authorization header and scheme; no bearer-to-Basic fallback.
  Malformed/empty Basic credentials are rejected without LDAP.
- Bounded pre-auth token bucket and nonblocking semaphore precede directory
  work. Saturation/dependency failures return sanitized 503; budgets return 429
  with Retry-After. Principal resolution shares the gate's total deadline.
- Verified LDAPS only, TLS 1.2 minimum, system roots plus optional private CA.
  No StartTLS/plaintext fallback or certificate-verification bypass.
- Each request opens a reader connection and separate user connection. Search
  the configured people base using escaped `(uid=...)`, require exactly one
  result, bind its DN, then re-read UUID and membership. No referrals followed,
  nested groups inferred, or authorization grants cached.
- UUID and DN must agree before/after binding. Direct `memberOf` DNs are parsed
  and compared with the LDAP library. Valid nonmembers get 403; unknown account
  and bad password get the same 401. Malformed UUID, ambiguous results, reader
  bind, TLS and lookup errors fail closed. Group removal affects the next
  request; authorized in-flight work can finish. Reader-secret rotation requires
  restart. User passwords are checked anew on each request.
- One timeout covers dialing, TLS, binds and searches. Socket deadlines and
  cancellation terminate blocked I/O. No connection is shared across requests.
- Basic requires direct TLS or an explicitly trusted peer CIDR with exactly
  `X-Forwarded-Proto: https`. The proxy must overwrite that header, reject HTTP
  before forwarding, and be the only ingress. Never trust an entire shared
  network. Loopback publication remains required. Application rejection cannot
  undo a caller sending credentials over plaintext to ingress.
- Bot context is distinct from OIDC; no local token/session or stored bot
  password. Logs contain fixed reason codes and successful local principal UUID
  only, never provider errors or credentials. Gate responses use `no-store`.

## Verification

Run `deploy/scripts/verify.ps1` (or `.sh`) for Go race/vet, disposable PostgreSQL,
restore, Flutter and web regressions. Run `verify-bot-auth.ps1` (or `.sh`) for the
actual Go validator against digest-pinned LLDAP 0.6.3. CI runs both independently.
The provider runs non-root, with no external network or host ports, read-only
root filesystem and tmpfs-generated credentials, certificate and SQLite database.
Only source paths enter the Docker build context.

Unit tests cover escaping, ambiguity, mid-bind UUID changes, membership
revalidation, concurrency/cancellation, scheme dispatch, endpoint isolation,
proxy policy, budgets and sanitized responses/logs. Provider tests cover success,
unknown accounts, wrong passwords, display rename, group removal/restoration,
password rotation, deletion/recreation, untrusted TLS, cancellation and outage.
Live directory certificate/reader scope and ingress remain rollout checks.

Uses [go-ldap v3.4.15](https://pkg.go.dev/github.com/go-ldap/ldap/v3@v3.4.15).
Its required crypto/net/sync/text dependency updates are included in go.mod.
