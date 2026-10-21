# AGENTS.md: rules for every agent working on meowGram

Release 2 work: read docs/releases/release-2/AGENT_PLAYBOOK.md before every session; its [OVERRIDE] rules are owner-approved.

meowGram is the purrBrews household's cat-lounge chat: a Flutter client (web,
Android, iOS, Windows, macOS) and a Go backend with PostgreSQL, deployed as
containers. Read this file first, then [`runbook.md`](runbook.md) (current design
and decisions) before changing anything. `README.md` provides a project overview;
where the two disagree, the runbook wins.

## 1. Workflow

1. **Every change gets its own new branch.** Never commit to `main` directly.
   - For standalone changes, branch from up-to-date `main`: `git fetch origin`, then
     `git switch -c <type>/<short-name> origin/main`.
   - **For work divided into releases, epics and tasks, use matching branch
     layers.** Create `release/<release>` from updated `origin/main`; create
     `<type>/<release>-epic-<number>-<name>` from the release branch; create
     individual story/task branches from their epic branch. Omit unused layers
     for two-level work. Git has no literal sub-branches: starting points and
     PR bases establish the hierarchy.
   - Record each branch's parent and PR target in the delivery plan. Fetch
     before starting a child and use the latest owner-approved parent state
     (the remote parent once published). Release work must not branch from
     `main` or a sibling instead of its declared parent.
   - PRs move one layer at a time: task -> epic -> release -> `main`.
     Release-wide planning/workflow branches may target the release directly.
     Integration branches collect owner-merged PRs; do not commit implementation
     directly to them or merge your own PRs at any layer. Shared dependencies
     must reach the common integration parent before other epics incorporate
     them. Update children without resetting them or rewriting published history.
     Cross-layer exceptions require the owner's instruction and a recorded reason.
   - Name it `<type>/<what>` with the commit types below, e.g.
     `feat/ust-1.5.1-typing-indicator`, `fix/ws-reconnect-backoff`,
     `docs/agents-guide`. One topic per task branch; integration branches group
     only their declared release or epic scope. The current R2 branch map lives
     in `docs/releases/release-2/delivery-plan.md`.
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
  **Approved Release 2 exception (implemented; disabled by default (BOT_BASIC_AUTH_ENABLED=false) until the authorized rollout):** dedicated bot
  accounts may use HTTP Basic Auth on message send/read endpoints over HTTPS.
  Validate passwords and membership in a configured LLDAP group on every
  request over encrypted LDAP; never persist passwords or issue local tokens.
  Human OAuth remains unchanged. Basic Auth must not grant WebSocket tickets,
  push-device access or media access. See
  [Release 2 API design](docs/releases/release-2/api-design.md).
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
  `deploy/.env` holds the server's secret and the loopback port bindings. Compose
  requires a database password and defaults published ports to loopback.

## 4. Known gaps (fix these on their own branches)

Where the code doesn't yet meet the parameters above (updated for release 1.0.1;
details in runbook §12.9):

- **Client ID:** this repository now uses canonical ID `meowgram` per §2. The
  Authelia client registration in `purrbrews-containers` must be updated before
  deploying this branch; do not change that external configuration without owner
  authorization.
- **WebSocket token URL:** resolved in the review-hardening branch. The client
  exchanges its access token for an authenticated, one-use ticket that expires
  after 30 seconds.
- **Push:** authenticated per-device registration replaces public topics on
  `feat/authenticated-device-push`. Provider delivery still needs a live-device
  check after deployment; scratch tests emulate the provider without credentials.
- **Mobile sign-in:** `meowgram://callback` custom scheme works; an App Link /
  Universal Link is stronger and must also be registered in Authelia.
- **Web build:** a Flutter web container and frontend catch-all route were added
  in the review-hardening branch. Rebuild and start it before deploying.
- **Database backups:** a daily local `pg_dump` service and retention policy were
  added in the review-hardening branch. Copy dumps off-host and verify restores
  before relying on them for recovery from host or disk loss.

Resolved in 1.0.1: audience/expiry validation, hardcoded production values in
tracked files (use gitignored `client/config/*.local.json`), compose dev defaults
(password now required, ports on `127.0.0.1`).
