# meowGram

<p align="center">
  <strong>Cross-Platform Realtime Cat Lounge</strong><br>
  Monorepo containing Flutter cross-platform client and Go backend service.
</p>

---

## 🐾 Architectural Overview & Principles

- **Monorepo Architecture**: Clean separation between the cross-platform client (`/client`), the high-performance Go backend (`/server`), and deployment infrastructure (`/deploy`).
- **Zero Hardcoded Domains**: All hostnames, ports, CORS origins, and domain-dependent routing are sourced strictly from environment variables via a standardized `.env` contract (`deploy/.env.example`) and Flutter compile-time declarations (`--dart-define`).
- **Containerized & Reverse-Proxy Ready**: The Go backend produces a minimal multi-stage Alpine container ready for deployment behind **Traefik** reverse proxy or direct mapped development.
- **Strict Security & Secret Isolation**: Secrets and environment configurations are excluded from version control (`.env` in `.gitignore`).

---

## 📁 Repository Layout

```text
meowGram/
├── .gitignore                    # Monorepo root ignore rules (secrets, OS, artifacts)
├── README.md                     # Architectural documentation & dev instructions
├── client/                       # Flutter cross-platform app (Android, iOS, macOS, Windows, Web)
│   ├── .gitignore
│   ├── pubspec.yaml              # Dependencies (go_router, web_socket_channel)
│   ├── analysis_options.yaml     # Strict static analysis & lint configurations
│   ├── lib/
│   │   ├── main.dart             # Application root (MaterialApp.router & theme)
│   │   └── src/
│   │       ├── config/
│   │       │   └── app_config.dart          # Environment contract reader (--dart-define)
│   │       ├── routes/
│   │       │   └── app_router.dart          # go_router setup (/login, /chat)
│   │       ├── screens/
│   │       │   ├── login_screen.dart        # Login & environment contract inspect screen
│   │       │   └── chat_screen.dart         # Interactive WebSocket echo chat screen
│   │       ├── services/
│   │       │   └── chat_websocket_service.dart # Realtime WebSocket client & stream manager
│   │       └── widgets/
│   │           └── connection_badge.dart    # Live connection state indicator
│   ├── test/
│   │   └── widget_test.dart      # Client widget smoke test
│   └── web/
│       ├── index.html            # Web platform runner
│       └── manifest.json
├── server/                       # Go backend service
│   ├── .dockerignore
│   ├── Dockerfile                # Multi-stage production build (Alpine, unprivileged user)
│   ├── go.mod                    # Go module (meowgram/server)
│   ├── go.sum                    # Dependency checksums (gorilla/websocket)
│   ├── cmd/
│   │   └── server/
│   │       └── main.go           # Entrypoint: graceful shutdown & routing
│   └── internal/
│       ├── config/
│       │   └── config.go         # Environment config loader with fallback defaults
│       ├── handler/
│       │   ├── health.go         # GET /healthz JSON health check endpoint
│       │   └── websocket.go      # /ws bidirectional echo WebSocket handler
│       └── middleware/
│           ├── cors.go           # Dynamic CORS enforcement from environment
│           └── logging.go        # Structured HTTP request logger (log/slog)
└── deploy/                       # Orchestration & deployment configurations
    ├── .env.example              # Canonical environment variable contract
    └── docker-compose.yml        # Docker Compose service definition with Traefik labels
```

---

## ⚙️ Environment Variables Contract (`deploy/.env.example`)

| Variable | Default | Purpose |
|---|---|---|
| `APP_DOMAIN` | `localhost` | Canonical domain used for host routing, link generation, and Traefik rules |
| `ENVIRONMENT` | `development` | Deployment tier: `development`, `staging`, or `production` |
| `LOG_LEVEL` | `debug` | Structured logger verbosity: `debug`, `info`, `warn`, or `error` |
| `HOST_HTTP_PORT` | `8080` | Host port exposed on the host machine via Docker Compose |
| `HTTP_PORT` | `8080` | Internal container port bound by the Go HTTP server |
| `WS_PORT` | `8080` | WebSocket endpoint port (unified over `/ws` HTTP upgrade) |
| `CORS_ORIGINS` | `http://localhost:8080,...` | Comma-separated list of allowed browser origins for CORS & WS handshake |
| `READ_TIMEOUT_SECONDS` | `15` | Server maximum duration for reading request headers/body |
| `WRITE_TIMEOUT_SECONDS` | `15` | Server maximum duration before timing out writes |
| `IDLE_TIMEOUT_SECONDS` | `60` | Server maximum idle keep-alive connection timeout |
| `TRAEFIK_HTTP_PORT` | `80` | Ingress port when running with `--profile proxy` |
| `TRAEFIK_DASHBOARD_PORT`| `8081` | Traefik web dashboard port |

---

## 🚀 Running Locally

### 1. Backend Service (`/server`)

#### Option A: Running via Docker Compose (Recommended)

1. Create your local `.env` file:
   ```bash
   cp deploy/.env.example deploy/.env
   ```

2. Start the Go backend container:
   ```bash
   docker compose -f deploy/docker-compose.yml up --build -d
   ```

3. Verify service health:
   ```bash
   curl -i http://localhost:8080/healthz
   ```
   Expected response:
   ```json
   {
     "status": "ok",
     "service": "meowgram-server",
     "domain": "localhost",
     "environment": "development",
     "timestamp": "2026-10-03T09:06:59Z"
   }
   ```

4. View structured logs:
   ```bash
   docker compose -f deploy/docker-compose.yml logs -f server
   ```

5. (Optional) Run behind Traefik reverse proxy:
   ```bash
   docker compose -f deploy/docker-compose.yml --profile proxy up -d
   ```

#### Option B: Running Bare-Metal / Native Go

```bash
cd server
go run ./cmd/server
```

---

### 2. Frontend Client (`/client`)

The Flutter client reads its target backend domain and API/WS URLs at build or run time via `--dart-define` parameters to prevent hardcoded domains.

#### Running on Web / Desktop / Mobile:

```bash
cd client

# Install Flutter dependencies
flutter pub get

# Run on Chrome with default development backend
flutter run -d chrome \
  --dart-define=APP_DOMAIN=localhost \
  --dart-define=HTTP_PORT=8080 \
  --dart-define=APP_ENV=development

# Run with custom API and WebSocket targets (e.g. staging or production)
flutter run -d chrome \
  --dart-define=API_BASE_URL=https://api.meowgram.example.com \
  --dart-define=WS_BASE_URL=wss://api.meowgram.example.com/ws \
  --dart-define=APP_ENV=staging
```

#### Navigating the Client:
1. **`/login` Screen**:
   - Inspects the active target domain, HTTP API, and WebSocket URLs.
   - Enter a display name or cat handle.
   - Press **"Enter Chat Lounge"** to transition to `/chat`.
2. **`/chat` Screen**:
   - Automatically establishes a WebSocket connection to `AppConfig.wsBaseUrl` (`/ws`).
   - Displays real-time connection badge (Connected, Connecting, Disconnected).
   - Enter text or tap action chips (**Meow!**, **Ping**) to verify bidirectional WebSocket echo streaming.

---

## 🔒 Security & Deployment Notes

- **CORS & Origin Validation**: Non-browser native clients (iOS/Android/Desktop) are permitted by default, while browser origins (`Origin` header) are strictly validated against `CORS_ORIGINS` and `APP_DOMAIN`.
- **Zero Hardcoding**: Ensure any new endpoints or external services are registered in `server/internal/config/config.go` and `deploy/.env.example`.
- **Next Phase**: Awaiting PM and Technical Lead review before introducing PostgreSQL database storage, migrations, and session/token authentication.
