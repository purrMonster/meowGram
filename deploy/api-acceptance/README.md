# Combined API acceptance

Run `deploy/scripts/verify-api-acceptance.ps1` on Windows or
`sh deploy/scripts/verify-api-acceptance.sh` on Linux. CI runs the same POSIX wrapper
in the `api-acceptance` job alongside directory, bot-auth and generic verification.

The build compiles the actual server main and a Go test driver. Runtime combines
that binary, PostgreSQL 16, and the digest-pinned LLDAP 0.6.3 provider. A generated
CA signs LDAPS; the real reader/bot bind and group queries go through the server.
A synthetic HTTPS reverse proxy overwrites X-Forwarded-Proto and is the only
trusted peer. A synthetic RSA/JWKS issuer drives the normal human OIDC verifier.
No handler, directory, repository, budget or publication mocks are used.

All runtime networking is internal to the disposable Compose project, with no
published host ports. Generated passwords/certificates live in tmpfs; child logs
and response bodies are suppressed. The wrapper removes scratch resources on
exit. Do not supply live credentials or production env files. The DB name guard
requires /verification. Production deploy/verify.compose.yml is unchanged.

TestAPIEndToEnd has named checks for auth errors, human HTTP/WS compatibility,
bot send/live discriminator, exact replay and one stored row, conflict, bot sync,
unknown channel field, Basic route isolation, group revocation, real budget burst,
directory outage, human independence and clean startup migration version 6.
Generic `go test ./...` explicitly skips this test because the binary/provider
fixture is absent; the dedicated job must pass, and its passes are reported
separately. Flutter's existing bot_sender_test.dart verifies the Bot label and
that a bot with a colliding human ID is never rendered as the user's own message.

The initial recorded run builds application sources identical to release/r2
(e74a1de). Later CI runs build their checkout, so regressions are detected before
integration. Live Authelia, proxy, phone and push-provider checks remain owner work.
