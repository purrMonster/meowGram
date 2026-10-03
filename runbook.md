# meowGram Engineering Runbook & Architectural Decision Record (ADR)

> **Document Version**: 1.0.0  
> **Status**: APPROVED (Scaffolding Complete, Foundation Operational)  
> **Author**: Lead Developer / Antigravity IDE  
> **Last Updated**: 2026-10-03  

---

## 1. Executive Summary & TL;DR

meowGram is a cross-platform realtime cat-themed chat lounge organized as a monorepo consisting of a Go backend service, a Flutter cross-platform client (Web, Desktop, Mobile), and Docker Compose orchestration infrastructure.

### What Was Built
1. **Monorepo Foundation**: Standardized repository layout separating `/client`, `/server`, and `/deploy` with strict `.gitignore` rules preventing secret commits.
2. **Go Backend Service (`/server`)**:
   - Zero hardcoding contract loaded strictly from environment variables via `internal/config`.
   - Healthcheck endpoint: `GET /healthz`.
   - Bidirectional WebSocket echo endpoint: `GET /ws` using `gorilla/websocket`.
   - Structured JSON logging using standard library `log/slog`.
   - Graceful shutdown handling `SIGINT`/`SIGTERM` with 10-second context timeout.
   - Dynamic CORS middleware inspecting origins against environment contract.
   - Multi-stage `Dockerfile` producing an unprivileged, minimal Alpine image (~15MB).
3. **Flutter Client Shell (`/client`)**:
   - Declarative navigation via `go_router` with initial `/login` and `/chat` routes.
   - `AppConfig` contract reading all endpoints via compile-time `--dart-define` parameters.
   - `ChatWebSocketService` managing connection lifecycles, reconnection, and streaming.
   - Interactive UI with live connection status pill, quick action chips, and message echo history.
4. **Deployment Orchestration (`/deploy`)**:
   - `docker-compose.yml` with healthchecks, environment variable interpolation, and **Traefik** reverse proxy labels.
   - Comprehensive `deploy/.env.example` documenting all configuration keys.

---

## 2. Architectural Decisions & Design Rationale

| Decision | Selected Technology / Pattern | Rationale & Alternatives Considered |
|---|---|---|
| **Repository Pattern** | Monorepo (`/client`, `/server`, `/deploy`) | Keeps client protocol types, backend handlers, and container configs aligned in atomic commits. Avoids multi-repo synchronization lag. |
| **Domain & Host Binding** | Zero Hardcoded Domains (`.env` + `--dart-define`) | Eliminates environment leakage. Enables deploying to local development (`localhost`), staging (`staging.meowgram.local`), or production without rebuilding code. |
| **Backend Framework** | Go 1.22+ Standard Library Router (`http.NewServeMux`) + `gorilla/websocket` | Standard library routing handles method and path matching natively (`GET /healthz`). Gorilla WebSocket provides battle-tested framed WebSocket streaming and ping/pong keepalive. |
| **Server Logging** | Go Standard Library `log/slog` | Structured JSON logging with configurable log level (`LOG_LEVEL`), avoiding external dependency bloat. |
| **Container Image** | Multi-Stage Build -> Alpine 3.21 Runtime | Strips Go build toolchain. Uses non-root user `appuser:10001` for security. Retains `wget` and CA certificates for health checks. |
| **Client Routing** | Flutter `go_router` | Declarative URL-based routing compatible with browser deep-linking, browser history, and cross-platform route stacks. |
| **Reverse Proxy** | Traefik v3 via Docker Compose labels | Automated Docker service discovery. Ready for containerized ingress with Zero SSL / Let's Encrypt in production. |

---

## 3. Issues Encountered & Post-Mortem Analysis

During initial integration and client startup, four distinct issues were encountered and resolved:

### Issue 1: Missing `AppConfig.appName`
- **Symptom**: `lib/main.dart:18:24: Error: Member not found: 'appName'`.
- **Root Cause**: `main.dart` referenced `AppConfig.appName` in `MaterialApp.router`, but `AppConfig` had omitted this field.
- **Resolution**: Added `static const String appName = 'meowGram';` to `client/lib/src/config/app_config.dart`.

### Issue 2: Import Namespace Shadowing in WebSocket Service
- **Symptom**: `Error: The getter 'normalClosure' isn't defined for the type 'SocketStatus'`.
- **Root Cause**: In `chat_websocket_service.dart`, `package:web_socket_channel/status.dart` was imported as `status`. Inside `ChatWebSocketService`, a class getter named `SocketStatus get status` shadowed the import prefix, causing Dart to evaluate `status.normalClosure` against `SocketStatus`.
- **Resolution**: Renamed the import alias from `status` to `ws_status` (`import .../status.dart as ws_status;`) and referenced `ws_status.normalClosure`.

### Issue 3: Go WebSocket Upgrade Failure (`http.Hijacker` Missing)
- **Symptom**: Client reported `WebSocketChannelException: Failed to connect WebSocket`. Server logged:
  ```json
  {"level":"ERROR","msg":"Failed to upgrade WebSocket connection","error":"websocket: response does not implement http.Hijacker","remote_addr":"..."}
  ```
- **Root Cause**: The custom `responseWriterWrapper` in `server/internal/middleware/logging.go` wrapped `http.ResponseWriter` without implementing the `http.Hijacker` interface. `gorilla/websocket` requires the underlying writer to support `Hijack() (net.Conn, *bufio.ReadWriter, error)` to take over the raw TCP socket from HTTP.
- **Resolution**: Implemented `Hijack()`, `Flush()`, and `Unwrap()` methods on `responseWriterWrapper` in `server/internal/middleware/logging.go`.

### Issue 4: Unhandled Asynchronous Error on Web WebSocket Connection
- **Symptom**: In the Chrome Flutter web runner, `RethrownDartError` printed repeatedly in the console.
- **Root Cause**: `package:web_socket_channel` v3 provides a `channel.ready` Future. When connection fails, this Future produces an error. If unhandled, Dart's root zone treats it as an uncaught exception. Furthermore, `_status = SocketStatus.connected` was being set prematurely before the connection completed.
- **Resolution**: In `chat_websocket_service.dart`, bound `channel.ready.then((_) => _setStatus(SocketStatus.connected)).catchError((e) => _setStatus(SocketStatus.error))`.

---

## 4. Environment Contract (`deploy/.env.example`)

| Variable | Dev Default | Staging/Prod Example | Description |
|---|---|---|---|
| `APP_DOMAIN` | `localhost` | `meowgram.chat` | Primary routing domain. Injected into Traefik host rules and healthcheck payloads. |
| `ENVIRONMENT` | `development` | `production` | Environment tier. |
| `LOG_LEVEL` | `debug` | `info` | Minimum log verbosity (`debug`, `info`, `warn`, `error`). |
| `HOST_HTTP_PORT` | `8080` | `8080` | Host port exposed on the host machine by Docker Compose. |
| `HTTP_PORT` | `8080` | `8080` | Internal container port bound by the Go HTTP server. |
| `WS_PORT` | `8080` | `8080` | WebSocket endpoint port (unified with `HTTP_PORT` over `/ws`). |
| `CORS_ORIGINS` | `http://localhost:8080,...` | `https://meowgram.chat` | Allowed browser origins for CORS preflight and WebSocket handshake. |
| `READ_TIMEOUT_SECONDS` | `15` | `15` | Maximum duration for reading incoming request headers/body. |
| `WRITE_TIMEOUT_SECONDS` | `15` | `15` | Maximum duration for writing response. |
| `IDLE_TIMEOUT_SECONDS` | `60` | `60` | Keep-alive idle connection timeout. |

---

## 5. Operations & Developer Playbook

### 5.1 Starting the Backend Service

#### Via Docker Compose (Recommended)
```powershell
# Navigate to project root
cd c:\Users\jyotirmoyc\Desktop\Projects\meowGram

# 1. Initialize environment file if not present
if (!(Test-Path deploy/.env)) { Copy-Item deploy/.env.example deploy/.env }

# 2. Build and run in detached mode
docker compose -f deploy/docker-compose.yml up -d --build

# 3. Verify health status
curl http://localhost:8080/healthz

# 4. Stream structured logs
docker compose -f deploy/docker-compose.yml logs -f server

# 5. Stop services
docker compose -f deploy/docker-compose.yml down
```

#### Running Bare-Metal (Native Go)
```powershell
cd c:\Users\jyotirmoyc\Desktop\Projects\meowGram\server
$env:APP_DOMAIN="localhost"
$env:PORT="8080"
$env:CORS_ORIGINS="http://localhost:8080,http://localhost"
go run ./cmd/server
```

---

### 5.2 Running the Frontend Client

The Flutter app reads its target backend configuration strictly via `--dart-define` parameters:

```powershell
cd c:\Users\jyotirmoyc\Desktop\Projects\meowGram\client

# Install dependencies
flutter pub get

# Launch on Chrome targeting local dev backend
flutter run -d chrome `
  --dart-define=APP_DOMAIN=localhost `
  --dart-define=HTTP_PORT=8080 `
  --dart-define=APP_ENV=development
```

#### Launching on Native Windows Desktop:
```powershell
flutter run -d windows `
  --dart-define=APP_DOMAIN=localhost `
  --dart-define=HTTP_PORT=8080 `
  --dart-define=APP_ENV=development
```

---

### 5.3 Automated Testing

```powershell
# Run Flutter unit and widget tests
cd c:\Users\jyotirmoyc\Desktop\Projects\meowGram\client
flutter test

# Format all client code
docker run --rm -v "${PWD}:/app" -w /app dart:stable dart format .

# Validate Go server compilation & tests
docker run --rm -v "${PWD}/server:/app" -w /app golang:alpine go test -v ./...
```

---

## 6. Verification Checklist

- [x] Monorepo directory structure matches architectural specification.
- [x] No hardcoded hostnames or ports exist in client or server code.
- [x] Server reads all configurations from environment variables.
- [x] Multi-stage `Dockerfile` compiles cleanly and executes as non-root user.
- [x] Docker Compose configuration validates cleanly with healthchecks.
- [x] Healthcheck endpoint (`GET /healthz`) returns 200 OK with server timestamp.
- [x] Echo WebSocket (`/ws`) successfully upgrades, accepts text/binary frames, and echoes back.
- [x] CORS origin validation enforces configured origins while allowing native clients.
- [x] Client `go_router` transitions smoothly between `/login` and `/chat`.
- [x] Client handles WebSocket connection lifecycle and displays live badge state.
- [x] Automated widget and unit tests verify `AppConfig` and UI structure.

---

## 7. Review Gate & Next Steps

> [!IMPORTANT]
> Per the PM and Technical Lead architectural mandate, **implementation of PostgreSQL database schemas, SQL migrations, and session/JWT authentication is on hold** pending review of this scaffolding and operational runbook.
