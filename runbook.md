# meowGram Engineering Runbook & Architectural Decision Record (ADR)

> **Document Version**: 1.5.0  
> **Status**: APPROVED (Epic 1.2 OIDC Auth, Epic 1.3 WebSocket Core, & UST-1.3.2 Chat UI Shell Complete)  
> **Author**: Lead Developer / Antigravity IDE  
> **Last Updated**: 2026-10-03  

---

## 1. Executive Summary & TL;DR

meowGram is a cross-platform realtime cat-themed chat lounge organized as a monorepo consisting of a Go backend service, a Flutter cross-platform client (Web, Desktop, Mobile), and Docker Compose orchestration infrastructure.

### Architectural Mandate Updates
- **Backend OIDC Pivot (Epic 1.2)**: All custom local authentication, password storage, user registration, and local JWT issuance are completely eliminated in favor of **Authelia OpenID Connect (OIDC)**.
- **Client OIDC PKCE Integration (UST-1.2.3)**: The Flutter client implements the OAuth 2.0 Authorization Code Flow with Proof Key for Code Exchange (PKCE, RFC 7636). The client is purely public and stores **zero client secrets**.
- **Real-Time WebSocket Core (Epic 1.3 - UST-1.3.1, UST-1.3.3, UST-1.3.4)**:
  - In-memory Go `Hub` manages active client registries via synchronized channels (`register`, `unregister`, `broadcast`).
  - Strict **Persistence Before Broadcast**: Inbound messages are committed to PostgreSQL (`messages` table) before entering the broadcast pipeline, guaranteeing zero dropped messages.
  - Goroutine Isolation: Per-client `ReadPump` and `WritePump` goroutines guarantee Gorilla WebSocket concurrency safety and prevent head-of-line blocking.
- **Basic Chat UI Shell (UST-1.3.2)**:
  - Responsive Flutter UI powered by `flutter_bloc` managing the real-time message timeline, 50-message initial history hydration, and deduplication.
  - Mobile software keyboard safety with `SafeArea` and `MediaQuery.viewInsetsOf(context)`.
  - Dynamic `MessageBubble` styling distinguishing self messages (right-aligned, primary palette) from peer messages (left-aligned with `@username`) and system notices.
  - Automatic bottom scrolling triggered on inbound arrivals and outbound sends.

---

## 2. Architectural Decisions & Design Rationale (ADR)

| Decision | Selected Technology / Pattern | Rationale & Alternatives Considered |
|---|---|---|
| **Client Auth Flow** | OAuth 2.0 Authorization Code with PKCE (RFC 7636) | Required for public clients where credentials cannot be embedded securely. Replaces legacy implicit flows with cryptographically bound `code_challenge` (S256). |
| **Desktop Redirect URI** | RFC 8252 Loopback HTTP (`http://127.0.0.1:8088/callback`) | Standard for native desktop apps on Windows/macOS/Linux. Eliminates OS custom URI scheme registry requirements during dev/testing. |
| **Web Redirect URI** | Host Origin (`Uri.base.origin`) | Seamless in-browser redirect on Web. URL inspection captures `code` and `state` parameters without local socket binding. |
| **Token Transport to WS** | URL Query Parameter (`?token={access_token}`) | Standard web browsers do not allow arbitrary HTTP headers (such as `Authorization`) during WebSocket handshake (`new WebSocket(...)`). |
| **Auth State Management** | `AuthController` with `refreshListenable` GoRouter | Declarative route protection. Automatically redirects `/login` -> `/chat` on token acquisition and `/chat` -> `/login` on expiry/logout. |
| **Chat Timeline State** | `flutter_bloc` (`ChatBloc`) | Unidirectional data flow cleanly separating WebSocket stream listening from UI presentation. Decouples event handling (history bursts, real-time broadcasts) from rendering. |
| **Time Formatting** | `intl` (`DateFormat.jm()`) | Standardized localized time representation across Web and native mobile/desktop platforms. |
| **Mobile Keyboard Adaptation** | `SafeArea` + `viewInsets` in `ChatInputBar` | Prevents software keyboard from overlapping chat input bar on Android/iOS without double-padding on Web/Desktop. |
| **Message Ordering Guarantee** | Chronological Sorting & In-Memory Deduplication | Re-sorts by `createdAt` ascending and deduplicates by PostgreSQL UUID to guarantee deterministic ordering across network jitters. |
| **Token Refresh Lifecycle** | Proactive Background Timer (`expiresAt - 60s`) | Silently exchanges `refresh_token` for a fresh `access_token` prior to expiration, preventing WebSocket disconnects during active chat. |
| **Backend OIDC Verifier** | `coreos/go-oidc/v3` with Remote KeySet | Cryptographically verifies incoming Bearer JWTs against Authelia's JWKS endpoint without handling user credentials. |
| **Broadcast Engine** | Go In-Memory `Hub` with Goroutine Channels | Highly performant, zero external messaging broker (Redis/RabbitMQ) dependency needed for single-node core. Scales efficiently across tens of thousands of concurrent connections. |
| **Message Ordering Guarantee** | **Persistence Before Broadcast** | `ReadPump` saves message to PostgreSQL *before* queueing into `hub.Broadcast`. If the database write fails or client drops mid-flight, uncommitted state never corrupts peer chat streams. |
| **Socket Thread Safety** | Gorilla WebSocket `ReadPump` & `WritePump` Split | Gorilla `*websocket.Conn` forbids concurrent writer calls. Isolating socket writes exclusively to `WritePump` while reads run on `ReadPump` eliminates data races without coarse mutex locks. |
| **Backpressure Protection** | Non-blocking Broadcast with Channel Eviction | `Hub` uses `select { case client.send <- msg: default: unregister }` with a 256-frame buffered channel. Slow or stalled clients cannot block the main broadcast loop or lag other peers. |
| **Database Migrations** | `golang-migrate/migrate/v4` | Automated `.up.sql` migrations executed on server container initialization. |

---

## 3. Real-Time WebSocket Core Architecture (Epic 1.3)

```
                            ┌──────────────────────────────────────────────┐
                            │               PostgreSQL                     │
                            │           (Table: `messages`)                │
                            └──────────────────────▲───────────────────────┘
                                                   │
                                      (1) Save Message (SQL INSERT)
                                                   │
 ┌──────────────────────┐        ┌─────────────────┴────────┐       ┌──────────────────────┐
 │ Client A (WebSocket) │        │ Client A ReadPump (Go)   │       │ Client B (WebSocket) │
 └──────────┬───────────┘        └────────────┬─────────────┘       └──────────▲───────────┘
            │                                 │                                │
      JSON Message                       (2) Enqueue                           │
            │                                 │                           JSON Message
            ▼                                 ▼                                │
  [ Gorilla WebSocket ] ───────►    [ Hub.Broadcast Channel ]                  │
                                              │                                │
                                         (3) Fan-out                           │
                                              ▼                                │
                                    ┌───────────────────┐                      │
                                    │ Client B Send Ch  │ ─────────────────────┘
                                    │ (buffered: 256)   │    (4) Client B WritePump
                                    └───────────────────┘
```

### 3.1 Concurrency Model & Channel Synchronization
- **`Hub.Run(ctx context.Context)`**:
  - Runs in a background goroutine started during server initialization.
  - Listens on `Register` (`chan *Client`), `Unregister` (`chan *Client`), and `Broadcast` (`chan *model.WSMessage`).
  - Maintains private `clients map[*Client]bool` safely confined to its single goroutine event loop.
- **`Client.ReadPump()`**:
  - Bound to WebSocket read loop. Enforces `pongWait` (60s), `maxMessageSize` (512 bytes), and `SetReadDeadline`.
  - Parses inbound JSON: `{"type": "chat", "text_content": "Hello!"}`.
  - Injects verified `sender_id` (Authelia `sub`) and triggers `messageRepo.Create(...)`.
  - Upon successful DB insert, sends message payload to `hub.Broadcast`.
  - Defers `hub.Unregister` and `conn.Close()` on socket termination.
- **`Client.WritePump()`**:
  - Exclusively handles all socket write operations.
  - Listens to `client.Send` channel and `ticker` (ping interval 54s).
  - Encodes payloads into WebSocket JSON text frames and sends ping control frames to keep network connections alive.

### 3.2 Database Schema: Messages Table (`migrations/000002_create_messages_table.up.sql`)

```sql
-- Ensure unique constraint exists for foreign key reference
ALTER TABLE users ADD CONSTRAINT uq_users_authelia_sub UNIQUE (authelia_sub);

-- Messages storage table
CREATE TABLE IF NOT EXISTS messages (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    sender_id VARCHAR(255) NOT NULL,
    text_content TEXT NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_messages_sender FOREIGN KEY (sender_id) 
        REFERENCES users(authelia_sub) ON DELETE CASCADE
);

-- Fast reverse-chronological message history lookups
CREATE INDEX IF NOT EXISTS idx_messages_created_at_desc ON messages (created_at DESC);
CREATE INDEX IF NOT EXISTS idx_messages_sender_id ON messages (sender_id);
```

### 3.3 WebSocket JSON Protocol Specification
- **Inbound Client Frame**:
  ```json
  {
    "type": "chat",
    "text_content": "Purr purr! Hello meowGram lounge."
  }
  ```
- **Outbound Broadcast Frame**:
  ```json
  {
    "id": "7f8b91a2-3c4d-5e6f-7a8b-9c0d1e2f3a4b",
    "sender_id": "usr_authelia_sub_12345",
    "text_content": "Purr purr! Hello meowGram lounge.",
    "created_at": "2026-10-03T12:30:00Z",
    "type": "chat"
  }
  ```
- **Connection Hydration (Recent History)**:
  On successful upgrade and registration, the server streams the last 50 persisted messages to the connecting client formatted as `"type": "history"`.

---

## 4. Cross-Platform Redirect URI Mechanics & Tradeoffs

```
                  ┌────────────────────────────────────────┐
                  │          Authelia OIDC Login           │
                  │       (System Default Browser)         │
                  └──────────────────┬─────────────────────┘
                                     │
                    Redirect with ?code=...&state=...
                                     │
           ┌─────────────────────────┴─────────────────────────┐
           ▼                                                   ▼
   [Web Platform]                                    [Desktop / Native]
   - Redirects to current browser origin             - Redirects to http://127.0.0.1:8088/callback
   - Web application captures query params           - Ephemeral Loopback HttpServer captures GET
   - No local socket binding required                - Renders success HTML, closes server
```

### Windows vs. Web Platform Differences:
1. **Web (`kIsWeb`)**:
   - Redirect URI default: Current browser origin (`Uri.base.origin`).
   - Callback mechanism: Native browser navigation updates the address bar with `code` and `state`.
2. **Desktop (Windows, macOS, Linux)**:
   - Redirect URI default: `http://127.0.0.1:8088/callback`.
   - Callback mechanism: Ephemeral local loopback server (`HttpServer.bind(InternetAddress.loopbackIPv4, 8088)`) listens for the redirect, responds with a confirmation HTML page, and yields the authorization code.
   - **Tradeoff**: Windows loopback requires firewall permission for loopback interface (typically granted automatically for `127.0.0.1`). If custom protocol schemes (e.g. `meowgram://`) are preferred in production, an MSIX package or Windows registry registration (`HKEY_CURRENT_USER\Software\Classes\meowgram`) must be configured.

---

---

## 5. Flutter Real-Time Chat Shell Architecture (UST-1.3.2)

```
       ┌────────────────────────────────────────────────────────┐
       │                 ChatWebSocketService                   │
       │    (Streams: messageStream, statusStream; sink.add)    │
       └───────────────────────────▲────────────────────────────┘
                                   │  Streams & Commands
                                   ▼
       ┌────────────────────────────────────────────────────────┐
       │                       ChatBloc                         │
       │  - Events: Connect, Received, Send, StatusChanged       │
       │  - State: messages, status, lastError, connectedUrl    │
       │  - Deduplication: by PostgreSQL UUID                   │
       │  - Ordering: strictly ascending by createdAt           │
       └───────────────────────────▲────────────────────────────┘
                                   │  BlocBuilder / BlocConsumer
                                   ▼
       ┌────────────────────────────────────────────────────────┐
       │                      ChatScreen                        │
       │  - Timeline: ListView.builder with ScrollController    │
       │  - Auto-scrolling: triggers on inbound & outbound msgs │
       │  - Bubbles: MessageBubble (Self: right/primary,        │
       │             Peers: left/@username, System: pills)      │
       │  - Input: ChatInputBar (SafeArea + viewInsets aware)   │
       └────────────────────────────────────────────────────────┘
```

### 5.1 New Flutter Dependencies Added
- **`flutter_bloc: ^9.1.1`** (with `bloc: ^9.2.1`): Implements unidirectional data flow for WebSocket streams, separating transport mechanics from widget rendering.
- **`intl: ^0.20.3`**: Provides standardized, localized time formatting (`DateFormat.jm()`) for message timestamps across Web, Desktop, and Mobile.

### 5.2 Responsive Software Keyboard Handling
Mobile software keyboards require dynamic layout insets to avoid obstructing the chat input bar:
- `Scaffold(resizeToAvoidBottomInset: true)` naturally contracts viewport height when the keyboard activates.
- `ChatInputBar` leverages `SafeArea(top: false, bottom: !isKeyboardOpen)` and checks `MediaQuery.viewInsetsOf(context).bottom > 0` to adjust vertical padding dynamically.
- Desktop and Web platforms experience zero layout shift because `viewInsets.bottom` remains `0`.

### 5.3 Auto-Scrolling Mechanics
`ScrollController` is bound to the `ListView.builder`. `BlocConsumer.listener` detects when `state.messages.length > _lastMessageCount` (e.g. during initial 50-message history hydration burst or live broadcast arrivals) and automatically invokes `_scrollToBottom()`, animating smoothly with `Curves.easeOutCubic`.

---

## 6. Environment & Compile-Time Configuration Contract

### Backend Environment Variables (`deploy/.env.example`)

| Variable | Dev Default | Staging/Prod Example | Description |
|---|---|---|---|
| `APP_DOMAIN` | `localhost` | `meowgram.chat` | Primary routing domain. Injected into Traefik host rules and healthcheck payloads. |
| `ENVIRONMENT` | `development` | `production` | Environment tier. |
| `LOG_LEVEL` | `debug` | `info` | Minimum log verbosity (`debug`, `info`, `warn`, `error`). |
| `HOST_HTTP_PORT` | `8080` | `8080` | Host port exposed on the host machine by Docker Compose. |
| `HTTP_PORT` | `8080` | `8080` | Internal container port bound by the Go HTTP server. |
| `WS_PORT` | `8080` | `8080` | WebSocket endpoint port (unified with `HTTP_PORT` over `/ws`). |
| `POSTGRES_USER` | `meowgram` | `meowgram_prod` | PostgreSQL user account. |
| `POSTGRES_PASSWORD` | `meowgram_secret_dev...` | *(Strong secret)* | PostgreSQL password. |
| `POSTGRES_DB` | `meowgram` | `meowgram` | PostgreSQL database name. |
| `DATABASE_URL` | `postgres://...` | `postgres://...` | Full connection string for Go backend. |
| `AUTHELIA_ISSUER` | `http://localhost:9091` | `https://auth.example.com` | Base Issuer URL for Authelia OIDC provider. |
| `AUTHELIA_JWKS_URL` | `http://localhost:9091/jwks.json` | `https://auth.example.com/jwks.json` | URL for Authelia cryptographic public keys (JWKS). |

### Flutter Client Compile-Time Flags (`--dart-define`)

| Flag | Dev Default | Purpose |
|---|---|---|
| `APP_DOMAIN` | `localhost` | Target backend host domain. |
| `HTTP_PORT` | `8080` | Target backend HTTP port. |
| `APP_ENV` | `development` | Runtime environment name. |
| `AUTHELIA_CLIENT_ID` | `meowgram-client` | Public client identifier registered in Authelia. |
| `AUTHELIA_ISSUER_URL` | `http://localhost:9091` | Authelia OIDC base issuer URL for discovery. |
| `AUTH_REDIRECT_URI` | *(Dynamic)* | Optional explicit redirect override (default: loopback on desktop, origin on web). |
| `API_BASE_URL` | *(Computed)* | Direct override for HTTP REST API base URL. |
| `WS_BASE_URL` | *(Computed)* | Direct override for WebSocket base URL. |

---

## 7. Operations & Developer Playbook

### 7.1 Starting the Infrastructure (PostgreSQL + Go Backend)

```powershell
cd c:\Users\jyotirmoyc\Desktop\Projects\meowGram

# 1. Initialize environment file if not present
if (!(Test-Path deploy/.env)) { Copy-Item deploy/.env.example deploy/.env }

# 2. Build and launch services in detached mode
docker compose -f deploy/docker-compose.yml up -d --build

# 3. Verify health status
curl http://localhost:8080/healthz
```

### 7.2 Launching the Flutter Client with OIDC PKCE

#### On Google Chrome (Web):
```powershell
cd c:\Users\jyotirmoyc\Desktop\Projects\meowGram\client

flutter run -d chrome `
  --dart-define=APP_DOMAIN=localhost `
  --dart-define=HTTP_PORT=8080 `
  --dart-define=AUTHELIA_CLIENT_ID=meowgram-client `
  --dart-define=AUTHELIA_ISSUER_URL=http://localhost:9091 `
  --dart-define=APP_ENV=development
```

#### On Windows Desktop:
```powershell
flutter run -d windows `
  --dart-define=APP_DOMAIN=localhost `
  --dart-define=HTTP_PORT=8080 `
  --dart-define=AUTHELIA_CLIENT_ID=meowgram-client `
  --dart-define=AUTHELIA_ISSUER_URL=http://localhost:9091 `
  --dart-define=APP_ENV=development
```

### 7.3 Automated Testing

```powershell
# Run Flutter client unit and widget test suite
cd c:\Users\jyotirmoyc\Desktop\Projects\meowGram\client
flutter test

# Run Go backend test suite (unit tests for Hub, migrations, repos)
cd c:\Users\jyotirmoyc\Desktop\Projects\meowGram\server
go test -v ./...
```

---

## 8. Review Gate & Verification Checklist

- [x] Database migration `000002_create_messages_table.up.sql` created and verified.
- [x] Foreign key constraint properly mapped to `users.authelia_sub`.
- [x] In-memory Go `Hub` maintains thread-safe registry of connected clients via goroutines and channels.
- [x] Separate `ReadPump` and `WritePump` goroutines guarantee Gorilla WebSocket concurrency safety.
- [x] Strict **Persistence Before Broadcast**: Inbound messages are inserted into PostgreSQL before dispatching to `hub.Broadcast`.
- [x] WebSocket handler authenticates connections via OIDC token and sends last 50 historical messages upon connection.
- [x] Unit test suite (`hub_test.go`) validates Hub lifecycle, registration, unregistration, and broadcasting.
- [x] `ChatMessage` model with JSON wire protocol parsing, time formatting (`intl`), and `isFromSelf` matching.
- [x] `ChatBloc` state management with `flutter_bloc` managing message list, deduplication, and chronological sorting.
- [x] `MessageBubble` dynamically styled (right-aligned primary for self, left-aligned for peers with `@username`, centered pills for system events).
- [x] `ChatInputBar` responsive keyboard handling with `SafeArea` and `MediaQuery.viewInsetsOf`.
- [x] Timeline auto-scrolling with `ScrollController` on message receipt and submission.
- [x] Comprehensive client test suite (14/14 tests) passing with 100% test success rate.
- [x] Flutter Web production build verified with `flutter build web`.
- [x] Backend compiles cleanly with zero warnings or errors.
