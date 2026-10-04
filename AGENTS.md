# AGENTS.md: rules for every agent working on meowGram

meowGram is the purrBrews household's cat-lounge chat: a Flutter client (web,
Android, iOS, Windows, macOS) and a Go backend with PostgreSQL, deployed as
containers. Read this file first, then [`runbook.md`](runbook.md) (current design
and decisions) before changing anything. `README.md` still describes the early echo
server; where the two disagree, the runbook wins.

## 1. Workflow

1. **Every change gets its own new branch.** Never commit to `main` directly.
   - Branch from an up-to-date `main`: `git fetch origin`, then
     `git switch -c <type>/<short-name> origin/main`.
   - Name it `<type>/<what>` with the commit types below, e.g.
     `feat/ust-1.5.1-typing-indicator`, `fix/ws-reconnect-backoff`,
     `docs/agents-guide`. One topic per branch.
   - It reaches `main` only through a pull request the owner merges. Don't merge
     your own branch, and don't push to `main`.
   - Never rewrite history that's been pushed (no force-push, no rebase of a
     pushed branch) without the owner's go-ahead.
2. **Commits** follow Conventional Commits, as the history does:
   `type(scope): what changed`. Types: `feat`, `fix`, `docs`, `chore`, `refactor`,
   `test`. Author `jyotirmoyc <jyotirmoy.github@jyotirmoy.cc>`.
3. **Before asking for a merge, run both suites** and say what passed:
   `cd server; go test ./...` and `cd client; flutter test`.
4. **Keep `runbook.md` current**: a decision, a new endpoint, a new env variable,
   a changed default, in the same branch as the code. If code and runbook disagree,
   fix one of them; don't leave both.
5. **Stop and ask the owner** before: anything touching the live deployment on
   roastery or the fleet, Authelia's configuration (it lives in the
   `purrbrews-containers` repo, not here), a database migration that drops or
   rewrites data, or any credential.

## 2. Design parameters (non-negotiable)

### Deployment and infrastructure

- **Git-only deployment on Docker.** What runs is built from this repo through
  `deploy/docker-compose.yml`. No hand-edited containers, no files copied onto a
  host outside Git (gitignored env files excepted).
- **Zero hardcoding.** Every hostname, port, URL, origin and endpoint comes from
  environment variables: `server/internal/config/config.go` and `.env.example` on
  the server, `--dart-define` / `--dart-define-from-file` (read in
  `client/lib/src/config/app_config.dart`) on the client. A new setting is added
  to both and to the runbook's table. **No real domain in any tracked file**:
  use `localhost`, `example.home.arpa` or a placeholder; the real values live in
  gitignored env files.
- **Secrets are generated on each host, never committed or shared.** Database
  passwords and the like are made on the machine that runs the service and kept in
  gitignored env files (`.env`, `deploy/.env`). Compose defaults like
  `meowgram_secret_dev_…` are for a laptop only and must never reach a deployment.
  Never print, log or paste a secret, in code, output or chat.

### Security and identity

- **No local authentication.** No passwords, user registration or self-issued
  tokens in meowGram. Identity is Authelia OpenID Connect; users are
  auto-provisioned from the token's `sub`.
- **The Flutter app holds no client secret.** It's a public OAuth 2.0 client
  using Authorization Code + PKCE (S256), with client ID `meowgram`. The backend
  verifies access tokens (JWTs) against Authelia's JWKS: issuer
  `https://authelia.<domain>`, and the audience must be the app's own URL.
- **Expose nothing directly.** Published ports bind to `127.0.0.1`; the only way
  in is the reverse proxy (Traefik) over HTTPS. Postgres is never reachable from
  the LAN.

### Real-time data integrity

- **Persistence before broadcast.** An inbound message is committed to PostgreSQL
  (`messages`) before it enters the hub's broadcast channel. If the write fails,
  nothing is broadcast. Zero data loss outranks latency.
- **Goroutine isolation.** Each client has a `ReadPump` (the only reader) and a
  `WritePump` (the only writer, including pings). Gorilla WebSocket's concurrency
  rules are met by that split, not by mutexes around the connection. Only the
  `Hub` goroutine touches its client registry.
- **Backpressure by eviction.** Broadcasting never blocks: a non-blocking send to
  each client's buffered channel (256 frames); a client whose buffer is full is
  unregistered and dropped. A slow client must never lag the broadcast loop or
  other clients.

### Cross-platform client behaviour

- **One ordering, one identity.** The timeline is sorted by `createdAt`
  ascending, in UTC, and deduplicated by the message's PostgreSQL UUID. This is
  what reconciles the local cache, the 50-message history burst, catch-up sync and
  the live stream; any new message source goes through the same merge.
- **Offline-first.** On launch the cached messages (Hive) render before the
  WebSocket connects; the network then fills in and catch-up sync closes any gap.
  Never block the first frame on the network.
- **Adaptive layout.** The chat input respects `SafeArea` and
  `MediaQuery.viewInsetsOf(context)` so the software keyboard never covers it on
  mobile, with no extra padding on web and desktop. The 800 px breakpoint splits
  the dual-pane (desktop, tablet) and drawer (phone) layouts.

## 3. How it's deployed now

- Runs on **roastery** in Docker Desktop, behind roastery's Traefik as
  `meow.<domain>`, with sign-in through Authelia on percolator. The route and the
  Authelia client are defined in `purrbrews-containers`
  (`stacks/roastery/meowgram/README.md`).
- Started with
  `docker compose --env-file .env --env-file deploy/.env -f deploy/docker-compose.yml up -d`.
  `deploy/.env` holds the server's secret and the loopback port bindings; without
  it the compose falls back to open ports and the dev password.

## 4. Known gaps (fix these on their own branches)

Where the code doesn't yet meet the parameters above:

- **Audience not checked:** `server/internal/auth/oidc.go` uses
  `SkipClientIDCheck`, so any JWT Authelia signs is accepted, including other
  apps' ID tokens. Verify `aud` against a configured value.
- **Hardcoded production values:** `deploy/scripts/build_all.*` and the runbook
  hardcode a domain that isn't the fleet's, an `auth.` host and client ID
  `meowgram-client`. Read them from a gitignored `--dart-define-from-file` instead.
- **Dev defaults in `deploy/docker-compose.yml`:** a default Postgres password and
  ports on all interfaces. Make the password required (`${POSTGRES_PASSWORD:?}`)
  and the default bindings `127.0.0.1`.
- **Token in the WebSocket URL** (`/ws?token=`): visible to anything that logs
  full URLs. Move it to `Sec-WebSocket-Protocol` or a short-lived ticket.
- **Mobile sign-in:** the loopback redirect is desktop-only; Android and iOS need
  an app link or custom scheme (registered in Authelia too).
- **Web build not served:** the backend serves no static files, so
  `https://meow.<domain>/` is 404.
- **No backups** of the database while it runs on roastery.
