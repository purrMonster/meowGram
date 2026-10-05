# meowGram Engineering Runbook & Architectural Decision Record (ADR)

> **Document Version**: 1.9.0  
> **Status**: APPROVED (Release 1 Feature-Frozen: Packaging Sprint & Post-Login UAT Hardening Complete)  
> **Author**: Lead Developer / Antigravity IDE  
> **Last Updated**: 2026-10-04  

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
  - **Dynamic Presence Broadcast**: `Hub` broadcasts active connected user rosters (`type: "presence"`) on client registration and unregistration.
- **Basic Chat UI Shell (UST-1.3.2)**:
  - Responsive Flutter UI powered by `flutter_bloc` managing the real-time message timeline, 50-message initial history hydration, and deduplication.
  - Mobile software keyboard safety with `SafeArea` and `MediaQuery.viewInsetsOf(context)`.
  - Dynamic `MessageBubble` styling distinguishing self messages (right-aligned, primary palette) from peer messages (left-aligned with `@username`) and system notices.
  - Automatic bottom scrolling triggered on inbound arrivals and outbound sends.
- **Responsive Adaptations & Offline Local Caching (UST-1.4.1 & UST-1.4.2)**:
  - Adaptive shell (`ResponsiveLayout`) switching between a persistent dual-pane layout for screens >= 800px (Desktop/Tablet) and a stacked navigation single-pane layout with modal drawer for screens < 800px (Mobile).
  - Hive NoSQL local storage (`hive_flutter`) enabling **Instant Offline Launch** (loading cached messages immediately on launch before live WebSocket connection).
  - Cache-to-Live Handoff: Seamless synchronization and deduplication between offline SQLite/Hive cache and live 50-message WebSocket bursts.
- **Catch-Up Synchronization Pipeline (UST-1.4.3)**:
  - Dedicated REST endpoint `GET /api/messages/sync?after={iso8601_timestamp}` querying PostgreSQL messages where `created_at > {timestamp}` chronologically (limit 500).
  - Client-side `SyncService` triggered automatically upon WebSocket reconnection, retrieving the newest cached timestamp and requesting gap-fill messages.
  - Strict timezone standardization: timestamps are normalized to UTC (`toUtc().toIso8601String()`) and parsed to UTC before executing PostgreSQL `TIMESTAMPTZ` comparisons.
  - Deduplicating merge in `ChatBloc`: incoming messages are deduplicated by PostgreSQL UUID against live WebSocket hydration bursts and active state, sorted chronologically, and persisted to Hive.
- **Post-Login UAT Hardening (Release 1)**:
  - **WebSocket Handshake Resilience**: Token signature verification directly against Authelia JWKS with trailing-slash normalization on `iss` and flexible audience verification (`aud`).
  - **Persistent Token Storage**: `flutter_secure_storage` encrypted Keychain/Keystore persistence with `first_unlock` accessibility, persisting across app backgrounding, termination, and device reboots.
  - **Friendly Username Extraction**: Prioritized hierarchy parsing `preferred_username` -> `name` -> `email prefix` -> `sub` from Authelia access tokens.
  - **Mock Data Elimination**: Ripped out hardcoded dummy presence members in favor of live presence rosters broadcasted over WebSocket.

---

## 2. Architectural Decisions & Design Rationale (ADR)

| Decision | Selected Technology / Pattern | Rationale & Alternatives Considered |
|---|---|---|
| **Client Auth Flow** | OAuth 2.0 Authorization Code with PKCE (RFC 7636) | Required for public clients where credentials cannot be embedded securely. Replaces legacy implicit flows with cryptographically bound `code_challenge` (S256). |
| **Desktop Redirect URI** | RFC 8252 Loopback HTTP (`http://127.0.0.1:8088/callback`) | Standard for native desktop apps on Windows/macOS/Linux. Eliminates OS custom URI scheme registry requirements during dev/testing. |
| **Web Redirect URI** | Host Origin (`Uri.base.origin`) | Seamless in-browser redirect on Web. URL inspection captures `code` and `state` parameters without local socket binding. |
| **Mobile Redirect URI** | Custom URL Scheme (`meowgram://callback`) | Deep link interception via `app_links` on iOS and Android. Replaces loopback sockets on mobile OSes. |
| **Token Transport to WS** | URL Query Parameter (`?token={access_token}`) | Standard web browsers do not allow arbitrary HTTP headers (such as `Authorization`) during WebSocket handshake (`new WebSocket(...)`). |
| **Auth State Management** | `AuthController` with `refreshListenable` GoRouter | Declarative route protection. Automatically redirects `/login` -> `/chat` on token acquisition and `/chat` -> `/login` on expiry/logout. |
| **Secure Token Persistence** | `flutter_secure_storage` (`TokenStorage`) | Persists tokens to iOS Keychain (`first_unlock`) and Android KeyStore immediately upon PKCE exchange. Restores session on app startup and foregrounding (`WidgetsBindingObserver`). |
| **Username Claim Resolution** | Token Claim Hierarchy | Resolves user display name from `preferred_username` -> `name` -> `email prefix` -> `sub` UUID fallback, preventing raw Authelia UUIDs in the UI. |
| **Chat Timeline State** | `flutter_bloc` (`ChatBloc`) | Unidirectional data flow cleanly separating WebSocket stream listening from UI presentation. Decouples event handling (history bursts, real-time broadcasts) from rendering. |
| **Responsive Breakpoint** | 800.0 Logical Pixels (`LayoutBuilder`) | Standard split point for tablets/desktops vs smartphones. Screens >= 800px show dual-pane (Sidebar + Chat); screens < 800px show single-pane with Drawer. |
| **Local Cache Database** | `hive_flutter` (`HiveLocalMessageRepository`) | Chosen over `sqflite` for unified cross-platform support across Web (IndexedDB), Windows, macOS, Linux, Android, and iOS without requiring C/FFI toolchains or WebAssembly build glue. |
| **Cache-to-Live Handoff** | Two-stage initialization in `ChatBloc` | Stage 1 loads cached messages from Hive instantly (zero UI wait). Stage 2 connects WebSocket and deduplicates against the 50-message server burst using PostgreSQL UUIDs. |
| **Catch-Up Synchronization** | Dedicated REST Endpoint (`GET /api/messages/sync`) | Fills message gaps after extended offline durations without overloading the initial WebSocket upgrade frame. Capped at 500 records per request to prevent payload bloat. |
| **Timezone Standardization** | ISO 8601 UTC / RFC 3339 (`.UTC()`) | Client exports `createdAt.toUtc().toIso8601String()`; Go backend normalizes any offset to UTC before querying PostgreSQL `TIMESTAMPTZ` column. Prevents timezone drift across global clients. |
| **Release 1 Packaging** | Multi-Platform Scripts (`build_all.ps1` / `build_all.sh`) | Unified build scripts inject production `--dart-define` flags targeting `purrbrews.cc`, producing Web, Android (APK/AAB), Windows, macOS, and iOS bundles. |
| **Android Keystore Separation** | Git-ignored `key.properties` & `*.jks` | Cryptographic signing keys remain strictly local/CI-injected; `build.gradle.kts` gracefully falls back to debug signing when `key.properties` is absent. |
| **Backend Docker Packaging** | Multi-stage static Alpine image (`meowgram:1.0.0`) | Stripped, non-root 8.7MB container embedding migrations, healthcheck, and CA certificates for production orchestration. |
| **Time Formatting** | `intl` (`DateFormat.jm()`) | Standardized localized time representation across Web and native mobile/desktop platforms. |
| **Mobile Keyboard Adaptation** | `SafeArea` + `viewInsets` in `ChatInputBar` | Prevents software keyboard from overlapping chat input bar on Android/iOS without double-padding on Web/Desktop. |
| **Message Ordering Guarantee** | Chronological Sorting & In-Memory Deduplication | Re-sorts by `createdAt` ascending and deduplicates by PostgreSQL UUID to guarantee deterministic ordering across network jitters. |
| **Token Refresh Lifecycle** | Proactive Background Timer (`expiresAt - 60s`) | Silently exchanges `refresh_token` for a fresh `access_token` prior to expiration, preventing WebSocket disconnects during active chat. |
| **Backend OIDC Verifier** | JWKS Remote KeySet + Normalized Claims | Cryptographically verifies incoming Bearer JWTs against Authelia's JWKS endpoint with trailing slash normalization and multi-audience matching. |
| **Broadcast Engine** | Go In-Memory `Hub` with Goroutine Channels | Highly performant, zero external messaging broker (Redis/RabbitMQ) dependency needed for single-node core. Scales efficiently across tens of thousands of concurrent connections. |
| **Real-Time Presence** | Hub Presence Frames (`type: "presence"`) | Dynamically tracks connected clients and broadcasts online roster on join/disconnect, eliminating hardcoded mock member lists. |
| **Message Ordering Guarantee** | **Persistence Before Broadcast** | `ReadPump` saves message to PostgreSQL *before* queueing into `hub.Broadcast`. If the database write fails or client drops mid-flight, uncommitted state never corrupts peer chat streams. |
| **Socket Thread Safety** | Gorilla WebSocket `ReadPump` & `WritePump` Split | Gorilla `*websocket.Conn` forbids concurrent writer calls. Isolating socket writes exclusively to `WritePump` while reads run on `ReadPump` eliminates data races without coarse mutex locks. |
| **Backpressure Protection** | Non-blocking Broadcast with Channel Eviction | `Hub` uses `select { case client.send <- msg: default: unregister }` with a 256-frame buffered channel. Slow or stalled clients cannot block the main broadcast loop or lag other peers. |
| **Ingress Path-Based Routing** | Traefik Router Rules (`PathPrefix`) + Priority | Evaluates backend routes (`/ws`, `/api`, `/healthz`) with priority 100 before frontend catch-all router on shared host domains. |
| **Database Migrations** | `golang-migrate/migrate/v4` | Automated `.up.sql` migrations executed on server container initialization. |
| **Root Route Auto-Login** | `GoRouter` Redirect | Automatically routes authenticated users hitting `/` directly to `/chat`, preventing dead-ends. |
| **Background Sync** | `workmanager` | Periodically wakes up to fetch gap messages via REST `sync` endpoint, keeping local cache warm. (Best-effort on iOS). |
| **Local Notifications** | `flutter_local_notifications` | Fires local OS alerts on message receipt. Used as a fallback/foreground notification mechanism. |
| **Push Notifications** | Firebase Cloud Messaging (FCM) | Chosen over direct APNs. Unifies Android, iOS, macOS, and Web under one backend API. Required due to Apple's strict background task and App Store restrictions against local polling for real-time chat. |

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

---

## 6. Responsive Shell & Local Offline Caching Architecture (UST-1.4.1 & UST-1.4.2)

```
       ┌────────────────────────────────────────────────────────┐
       │                Local Storage (Hive Box)                │
       │        (IndexedDB on Web, Binary Box on Native)        │
       └───────────────────────────▲────────────────────────────┘
                                   │  1. Instant Cached Messages
                                   ▼
       ┌────────────────────────────────────────────────────────┐
       │                       ChatBloc                         │
       │  Stage 1: Load cache immediately -> render UI          │
       │  Stage 2: Connect WebSocket -> hydrate 50 msgs burst   │
       │  Deduplication: by PostgreSQL UUID; re-sort by time    │
       │  Background Sync: write incoming frames to Hive box    │
       └───────────────────────────▲────────────────────────────┘
                                   │
                                   ▼
       ┌────────────────────────────────────────────────────────┐
       │              ResponsiveLayout (LayoutBuilder)          │
       │  Breakpoint: 800.0 logical pixels                      │
       ├───────────────────────────┬────────────────────────────┤
       │   Width >= 800px (Desktop)│    Width < 800px (Mobile)  │
       │   Persistent Dual-Pane    │    Single-Pane Stacked     │
       │   [Sidebar] + [ChatScreen]│    [ChatScreen] + [Drawer] │
       └───────────────────────────┴────────────────────────────┘
```

### 6.1 Responsive Breakpoint & Layout Logic
- **Breakpoint**: `800.0` logical pixels evaluated via `LayoutBuilder`.
- **Desktop/Tablet Mode (`>= 800px`)**:
  - Persistent left `Sidebar` (270px fixed width).
  - Channels directory (`# general-lounge`, `# cat-memes`, `# treat-discussions`).
  - Active lounge members list with online status indicator dots.
  - User profile badge in footer with logout action.
  - Main chat viewport (`ChatScreen`) fills remaining horizontal space.
- **Mobile Mode (`< 800px`)**:
  - Full-screen `ChatScreen` with push/pop stacked navigation.
  - AppBar leading hamburger menu button (`Icons.menu_rounded`) opens `Sidebar` in a modal `Drawer`.

### 6.2 Local Storage Strategy (Hive vs. sqflite)
- **Technology Chosen**: `hive: ^2.2.3` and `hive_flutter: ^1.1.0`.
- **Why Hive?**:
  - `sqflite` requires native C SQLite compilation (not supported on Web without complex WASM toolchains and worker files).
  - `hive` provides unified key-value NoSQL storage using IndexedDB on Web and memory-mapped binary files on Windows, macOS, Linux, Android, and iOS.
  - Zero platform-specific native C FFI dependencies.
- **Interface Contract**: `LocalMessageRepository` with production `HiveLocalMessageRepository` and test `MemoryLocalMessageRepository`.

### 6.3 Cache-to-Live Handoff & Synchronization Mechanics
- **Two-Stage Initialization**:
  1. On launch, `ChatBloc` dispatches `ChatInitializeRequested`. It queries `localRepo.getCachedMessages()` and immediately emits `isLoadedFromCache: true`. The UI renders instant history without waiting for network handshakes.
  2. In the background, `socketService.connect()` establishes the WebSocket connection. The backend streams the latest 50 messages (`SendHistory`).
- **Conflict Resolution & Deduplication**:
  - Incoming server messages are checked against existing items by PostgreSQL UUID (`ChatMessage.id`).
  - Duplicate frames are discarded; new frames are appended and re-sorted ascending by `createdAt`.
  - Inbound messages are saved to Hive asynchronously in the background (`unawaited(_localRepo.saveMessage(incoming))`), guaranteeing persistent offline continuity.

---

## 7. Catch-Up Synchronization Architecture (UST-1.4.3)

```
┌──────────────┐          ┌─────────────┐        ┌─────────────┐       ┌────────────┐       ┌────────────┐
│ WebSocket    │          │ SyncService │        │ Hive Cache  │       │ ChatBloc   │       │ Go Backend │
│ Service      │          │ (Client)    │        │ Repository  │       │ State      │       │ REST API   │
└──────┬───────┘          └──────┬──────┘        └──────┬──────┘       └─────┬──────┘       └─────┬──────┘
       │                         │                      │                    │                    │
       │ (1) Status: connected   │                      │                    │                    │
       ├────────────────────────►│                      │                    │                    │
       │                         │ (2) getNewestMessage │                    │                    │
       │                         ├─────────────────────►│                    │                    │
       │                         │◄─────────────────────┤                    │                    │
       │                         │     DateTime?        │                    │                    │
       │                         │                      │                    │                    │
       │                         │ (3) GET /api/messages/sync?after={UTC_ISO}│                    │
       │                         ├───────────────────────────────────────────────────────────────►│
       │                         │                                           │                    │ (4) created_at > $1
       │                         │                                           │                    │     LIMIT 500
       │                         │◄───────────────────────────────────────────────────────────────┤
       │                         │     JSON: List<Message>                   │                    │
       │                         │                                           │                    │
       │                         │ (5) SyncCompleted(messages)               │                    │
       │                         ├──────────────────────────────────────────►│                    │
       │                         │                      │                    │                    │
       │                         │                      │                    │ (6) Deduplicate by │
       │                         │                      │                    │     PostgreSQL UUID│
       │                         │                      │                    │     Sort Chrono ASC│
       │                         │                      │ (7) saveMessages   │                    │
       │                         │                      │◄───────────────────┤                    │
       │                         │                      │     (batch Hive)   │                    │
       │                         │                      │                    │                    │
```

### 7.1 Gap Analysis & The Need for Catch-Up Sync
- **The Problem**: When a user was offline for minutes or hours (e.g. laptop closed or device in airplane mode), dozens or hundreds of messages may have been broadcast. The default WebSocket reconnection handshake emits a burst of the 50 most recent messages (`SendHistory(ctx, 50)`). If more than 50 messages were sent while offline, a permanent gap remains between the user's latest local message and the 50-message burst.
- **The Solution**: A dedicated REST endpoint (`GET /api/messages/sync?after={timestamp}`) allows the client to fetch all messages created strictly after the newest locally cached message, up to 500 messages per request.

### 7.2 Endpoint Contract & Timezone Standardization
- **Endpoint**: `GET /api/messages/sync?after={iso8601_timestamp}`
- **Security**: Authenticated via Authelia OIDC Bearer token (`Authorization: Bearer <token>`) or `?token=<token>`.
- **Query Parameter**: `after` (mandatory, ISO 8601 / RFC 3339 formatted).
- **Timezone Standardization Architecture**:
  - **Client**: `newest.createdAt.toUtc().toIso8601String()` produces RFC 3339 with `Z` suffix (e.g. `2026-10-03T13:40:00.123456Z`).
  - **URL Encoding**: `Uri.replace(queryParameters: {'after': afterIso})` safely percent-encodes colons and plus signs.
  - **Go Backend Parser (`ParseSyncTimestamp`)**: Supports `RFC3339Nano`, `RFC3339`, ISO variants, and numeric epoch timestamps. Resolves space-encoded `+` characters in timezone offsets.
  - **Database Query**: Converted to `.UTC()` before parameter binding (`$1`). PostgreSQL stores `created_at` as `TIMESTAMPTZ` (UTC internally), guaranteeing mathematically exact comparison (`created_at > $1`).
- **Response**: Array of message objects `[]*model.Message` serialized as JSON (HTTP 200 OK).

### 7.3 Conflict Resolution & In-Memory Deduplication
- Because both the WebSocket hydration burst (50 messages) and the REST sync response can deliver overlapping messages, `ChatBloc` uses PostgreSQL UUID (`ChatMessage.id`) as a unique deduplication key:
  1. Messages already present in `state.messages` are filtered out.
  2. Non-duplicate messages are merged into the timeline.
  3. The timeline is re-sorted chronologically ascending (`createdAt.compareTo`).
  4. Only newly discovered messages are batch persisted to local Hive storage (`_localRepo.saveMessages(newMessages)`).

---

## 8. Environment & Compile-Time Configuration Contract

### Backend Environment Variables (`deploy/.env.example` / `.env`)

| Variable | Dev Default | Staging/Prod Example | Description |
|---|---|---|---|
| `APP_DOMAIN` | `localhost` | `meowgram.purrbrews.cc` | Primary routing domain. Injected into Traefik host rules and link generation. |
| `ENVIRONMENT` | `development` | `production` | Environment tier (`development`, `staging`, `production`). |
| `APP_ENV` | `development` | `production` | Alias for `ENVIRONMENT`. |
| `LOG_LEVEL` | `debug` | `info` | Minimum log verbosity (`debug`, `info`, `warn`, `error`). |
| `USE_SECURE_SCHEMES` | `false` | `true` | Enforces `https://` and `wss://` protocols. |
| `HOST_HTTP_PORT` | `8080` | `8080` | Host port exposed on the host machine by Docker Compose. |
| `HTTP_PORT` | `8080` | `8080` | Internal container port bound by the Go HTTP server. |
| `WS_PORT` | `8080` | `8080` | WebSocket endpoint port (unified with `HTTP_PORT` over `/ws`). |
| `API_BASE_URL` | *(Computed)* | `https://meowgram.purrbrews.cc` | Full REST API base URL override. |
| `WS_BASE_URL` | *(Computed)* | `wss://meowgram.purrbrews.cc/ws` | Full WebSocket base URL override. |
| `WS_ENDPOINT` | `/ws` | `/ws` | WebSocket upgrade route path. |
| `SYNC_ENDPOINT` | `/api/messages/sync` | `/api/messages/sync` | Catch-up synchronization REST route path. |
| `HEALTH_ENDPOINT` | `/healthz` | `/healthz` | Health check probe route path. |
| `AUTHELIA_DOMAIN` | `localhost:9091` | `auth.purrbrews.cc` | FQDN or host:port for Authelia OIDC identity provider. |
| `AUTHELIA_ISSUER` | *(Derived)* | `https://auth.purrbrews.cc` | Base Issuer URL for Authelia OIDC provider. Derived from `AUTHELIA_DOMAIN`. |
| `AUTHELIA_ISSUER_URL` | *(Derived)* | `https://auth.purrbrews.cc` | Alias for `AUTHELIA_ISSUER`. |
| `AUTHELIA_CLIENT_ID` | `meowgram` | `meowgram` / `meowgram-client` | Client ID expected in JWT aud or token validation. |
| `AUTHELIA_AUDIENCE` | *(Optional)* | `https://meowgram.purrbrews.cc` | Expected audience (`aud`) in access tokens. Defaults to `AUTHELIA_CLIENT_ID` or `APP_DOMAIN`. |
| `AUTHELIA_JWKS_URL` | *(Derived)* | `https://auth.purrbrews.cc/jwks.json` | URL for Authelia public keys (JWKS). Derived from `AUTHELIA_ISSUER`. |
| `POSTGRES_USER` | `meowgram` | `meowgram_prod` | PostgreSQL user account. |
| `POSTGRES_PASSWORD` | `meowgram_secret_dev...` | *(Strong secret)* | PostgreSQL password. |
| `POSTGRES_DB` | `meowgram` | `meowgram` | PostgreSQL database name. |
| `DATABASE_URL` | `postgres://...` | `postgres://...` | Full connection string for Go backend. |
| `CORS_ORIGINS` | *(Localhost list)* | `https://meowgram.purrbrews.cc` | Comma-separated list of allowed client origins. |
| `TRAEFIK_ENTRYPOINTS` | `web` | `web` / `websecure` | Traefik entrypoint(s) bound to backend router (`meowgram-server`). |

### Flutter Client Environment Variables & `.env` Controllability

All endpoints can be controlled either via `--dart-define-from-file=.env`, `--dart-define=KEY=VAL`, or by placing a `.env` file in the project or `client/` folder (automatically loaded at startup via `AppConfig.initialize()`):

| Variable / Flag | Dev Default | Description |
|---|---|---|
| `APP_DOMAIN` | `localhost` | Target backend host domain. |
| `HTTP_PORT` | `8080` | Target backend HTTP port. |
| `APP_ENV` | `development` | Runtime environment name (`development`, `staging`, `production`). |
| `USE_SECURE_SCHEMES` | `false` | Whether to force `https://` and `wss://`. |
| `API_BASE_URL` | *(Computed)* | Direct override for HTTP REST API base URL. |
| `WS_BASE_URL` | *(Computed)* | Direct override for WebSocket base URL. |
| `WS_ENDPOINT` | `/ws` | WebSocket relative route path. |
| `SYNC_ENDPOINT` | `/api/messages/sync` | Catch-up sync endpoint path or full URL. |
| `HEALTH_ENDPOINT` | `/healthz` | Health check endpoint path or full URL. |
| `AUTHELIA_DOMAIN` | `localhost:9091` | Authelia identity provider domain. Setting this auto-derives all OIDC endpoints! |
| `AUTHELIA_ISSUER_URL` | *(Derived)* | Authelia OIDC base issuer URL for discovery. |
| `AUTHELIA_CLIENT_ID` | `meowgram-client` | Public client identifier registered in Authelia. |
| `AUTHELIA_JWKS_URL` | *(Derived)* | Authelia cryptographic public keys URL. |
| `AUTHELIA_DISCOVERY_URL` | *(Derived)* | OpenID configuration discovery endpoint URL. |
| `AUTHELIA_AUTHORIZATION_ENDPOINT` | *(Derived)* | OIDC PKCE authorization endpoint. |
| `AUTHELIA_TOKEN_ENDPOINT` | *(Derived)* | OIDC token exchange endpoint. |
| `AUTHELIA_USERINFO_ENDPOINT` | *(Derived)* | OIDC userinfo endpoint. |
| `AUTHELIA_REVOCATION_ENDPOINT` | *(Derived)* | OIDC token revocation endpoint. |
| `AUTH_REDIRECT_URI` | *(Dynamic)* | OAuth redirect URI (default: RFC 8252 loopback on desktop, origin on web). |
| `IMMICH_DOMAIN` | `immich.purrbrews.cc` | Immich media service domain (Release 2). |
| `IMMICH_API_URL` | *(Derived)* | Immich REST API base URL (Release 2). |

---

## 9. Operations & Developer Playbook

### 8.1 Starting the Infrastructure (PostgreSQL + Go Backend)

```powershell
cd c:\Users\jyotirmoyc\Desktop\Projects\meowGram

# 1. Initialize environment file if not present
if (!(Test-Path deploy/.env)) { Copy-Item deploy/.env.example deploy/.env }

# 2. Build and launch services in detached mode
docker compose -f deploy/docker-compose.yml up -d --build

# 3. Verify health status
curl http://localhost:8080/healthz
```

### 8.2 Launching the Flutter Client with OIDC PKCE

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

### 8.3 Automated Testing

```powershell
# Run Flutter client unit and widget test suite
cd c:\Users\jyotirmoyc\Desktop\Projects\meowGram\client
flutter test

# Run Go backend test suite (unit tests for Hub, migrations, repos)
cd c:\Users\jyotirmoyc\Desktop\Projects\meowGram\server
go test -v ./...
```

---

## 10. Release 1 Build & Packaging Procedures

### 10.1 Production Environment Target: `purrbrews.cc`
All production binaries must be compiled with `--dart-define` flags targeting the production infrastructure:
```bash
--dart-define=APP_ENV=production \
--dart-define=APP_DOMAIN=meowgram.purrbrews.cc \
--dart-define=HTTP_PORT=443 \
--dart-define=USE_SECURE_SCHEMES=true \
--dart-define=AUTHELIA_ISSUER_URL=https://auth.purrbrews.cc \
--dart-define=AUTHELIA_CLIENT_ID=meowgram-client \
--dart-define=API_BASE_URL=https://meowgram.purrbrews.cc \
--dart-define=WS_BASE_URL=wss://meowgram.purrbrews.cc/ws
```

### 10.2 Automated Build Execution Scripts
Run the automated build script for sequential compilation:
- **On Windows (PowerShell)**:
  ```powershell
  deploy/scripts/build_all.ps1 -Target all
  ```
- **On Linux / macOS (Bash / CI/CD)**:
  ```bash
  chmod +x deploy/scripts/build_all.sh
  ./deploy/scripts/build_all.sh all
  ```

### 10.3 Android Signing & Keystore Generation
To publish to Google Play or generate official release APKs:
1. **Generate Local Keystore** (execute once on local secure machine):
   ```bash
   keytool -genkey -v -keystore upload-keystore.jks -storetype JKS -keyalg RSA -keysize 2048 -validity 10000 -alias upload
   ```
2. **Configure `android/key.properties`**:
   Copy `android/key.properties.example` to `android/key.properties` and fill in:
   ```properties
   storePassword=your_keystore_password
   keyPassword=your_key_password
   keyAlias=upload
   storeFile=../upload-keystore.jks
   ```
   > [!CAUTION]
   > NEVER commit `upload-keystore.jks` or `key.properties` to version control. Both patterns are enforced in `.gitignore`.
3. **Build Artifacts**:
   - Google Play Bundle: `flutter build appbundle --release` &rarr; `build/app/outputs/bundle/release/app-release.aab`
   - Direct Install APK: `flutter build apk --release` &rarr; `build/app/outputs/flutter-apk/app-release.apk`

### 10.4 iOS & macOS Code Signing Procedures (Manual Xcode Steps)
Apple platforms enforce cryptographic code signing and provisioning profiles that **cannot be fully automated via command-line scripts without an active Apple Developer Team**:
1. **Open Workspace**:
   - iOS: Open `client/ios/Runner.xcworkspace` in Xcode.
   - macOS: Open `client/macos/Runner.xcworkspace` in Xcode.
2. **Signing & Capabilities**:
   - Select the `Runner` target.
   - Under the `Signing & Capabilities` tab, select your team (`Team: PurrBrews LLC`).
   - Bundle Identifier is locked to `com.purrbrews.meowgram`.
   - Check `Automatically manage signing` (recommended) or import manual Distribution Provisioning Profiles.
3. **App Sandbox & Hardened Runtime (macOS)**:
   - Verify `Incoming Connections (Server)` and `Outgoing Connections (Client)` are checked in `Runner.entitlements` to permit WebSocket and HTTP connectivity.
4. **Archive & Distribution**:
   - In Xcode menu, select `Product > Archive`.
   - In the Organizer window, click `Distribute App` &rarr; `App Store Connect` (or `Direct Distribution / Developer ID` for notarized macOS `.dmg`).

#### 10.4.1 Day 1 iOS Testing: Progressive Web App (PWA) Mode
To test on iOS devices without waiting for Apple Developer Team certificate provisioning:
1. Navigate to `https://meowgram.purrbrews.cc` in Safari on iOS.
2. Tap the Share button &rarr; **Add to Home Screen**.
3. Launch **meowGram** from the Home Screen. The app runs in standalone fullscreen (`apple-mobile-web-app-capable: yes`, `black-translucent` status bar) with CanvasKit and core assets cached by `sw.js` for instant subsequent cold starts.

#### 10.4.2 iOS Custom URL Scheme & Mobile Deep Link Interception
For native mobile (iOS & Android) UAT and Authelia OIDC callback interception:
1. **Custom URL Scheme Registration (`CFBundleURLTypes`)**:
   - `client/ios/Runner/Info.plist` registers the custom URL scheme `meowgram`:
     ```xml
     <key>CFBundleURLTypes</key>
     <array>
       <dict>
         <key>CFBundleTypeRole</key>
         <string>Editor</string>
         <key>CFBundleURLSchemes</key>
         <array>
           <string>meowgram</string>
         </array>
       </dict>
     </array>
     ```
   - `client/android/app/src/main/AndroidManifest.xml` configures the matching `VIEW` intent-filter with `android:scheme="meowgram"` and `android:host="callback"`.
2. **Mobile Deep Link Interception (`app_links` & `OidcPlatformIoHelper`)**:
   - On mobile (`Platform.isIOS || Platform.isAndroid`), `OidcPlatformIoHelper` subscribes to `AppLinks().uriLinkStream` and inspects `getLatestLink()` / `getInitialLink()` instead of attempting to bind a loopback HTTP server.
   - Deep links matching `meowgram://` validate `state == expectedState` for CSRF protection and ignore stale links from prior sessions.
   - Extracts the authorization `code` and hands off to `AuthController` for PKCE token exchange.
3. **Cross-Platform Discrepancies & Background Survival**:
   - **Browser Launch Mode**: `AuthController` uses `LaunchMode.externalApplication` to open the system browser (Safari on iOS, Chrome on Android) outside of an embedded webview, allowing clean session isolation and automatic callback bounce.
   - **Background Survival**: `AuthController` retains `_pendingPkce` and `_pendingRedirectUri` on the controller instance, preventing state loss during OS app suspension while Safari is in the foreground.
   - **Persistent Token Storage**: Tokens are immediately persisted to secure native storage (`flutter_secure_storage` with Keychain `first_unlock` accessibility on iOS/macOS and EncryptedSharedPreferences on Android) upon PKCE token exchange, prior to routing to `/chat`. Rehydration occurs on app cold start and when returning to the foreground (`AppLifecycleState.resumed`).
   - **Redirect URI Auto-Resolution**: `AppConfig.authRedirectUri` automatically resolves to `meowgram://callback` on iOS/Android, and `http://127.0.0.1:8088/callback` on Desktop.
4. **Physical Device Launch Script (`deploy/scripts/run_ios_physical.sh`)**:
   - Launches meowGram on a connected iOS device using `--dart-define-from-file`.
   - Supports `--clean` / `-c` (or `CLEAN=true`) to invalidate Flutter and Xcode build caches (`flutter clean && flutter pub get`), forcing Xcode to re-index the updated `Info.plist`.
   - Usage:
     ```bash
     # Launch in production release mode with clean cache
     ./deploy/scripts/run_ios_physical.sh production release --clean

     # Launch on specific device ID
     ./deploy/scripts/run_ios_physical.sh production release --clean -d <DEVICE_ID>
     ```
5. **Deep Link Verification**:
   - **Simulator**: `xcrun simctl openurl booted "meowgram://callback?code=mock_code&state=mock_state"`
   - **Physical Device**: Open Safari and navigate to `meowgram://callback?code=test&state=test_state`. Safari prompts to open "meowGram", confirming native scheme registration and deep link routing.

#### 10.4.3 Post-Login UAT Hardening & Release 1 Fixes
During Release 1 iOS UAT, four post-login blockers were identified and resolved:
1. **WebSocket Handshake Validation (RS256 & Claim Normalization)**:
   - *Problem*: Authelia signs JWTs with RS256 and often includes trailing slashes in the issuer (`https://auth.domain/`), causing strict string match verification to fail with HTTP 401 during the WebSocket upgrade handshake. Additionally, audience (`aud`) claims were rejected if not strictly matched.
   - *Fix*: The Go backend `OIDCVerifier` leverages `keySet.VerifySignature(ctx, rawToken)` against Authelia's JWKS for cryptographic integrity, followed by explicit normalized claim validation (`validateIssuer` with trailing-slash tolerance, `validateAudience` accepting client ID, audience, or backend domain, and 1-minute expiration clock skew).
2. **App Backgrounding & Session Persistence**:
   - *Problem*: Backgrounding the app on iOS cleared transient in-memory state or prompted route guards to redirect back to `/login`.
   - *Fix*: Integrated `flutter_secure_storage` via `TokenStorage`. Tokens are written immediately after PKCE exchange completion, before navigation to `/chat`. `MeowGramApp` observes `WidgetsBindingObserver.didChangeAppLifecycleState` to trigger `authController.checkSession()` when resuming foreground activity.
3. **Friendly Username Resolution**:
   - *Problem*: Authelia access tokens use raw UUIDs in the `sub` claim (e.g. `@141a5dfa...`), which rendered directly in the UI instead of the user's nickname.
   - *Fix*: `UserProfile.fromTokens()` and `fromJwt()` decode access token payload claims hierarchically: `preferred_username` -> `name` -> `email prefix` -> `sub`, providing human-friendly chat identities.
4. **Elimination of Hardcoded Presence Data**:
   - *Problem*: The drawer presence roster showed static mock cats (Mittens, Felix, Garfield, Luna).
   - *Fix*: The Go `Hub` broadcasts dynamic `presence` frames (`type: "presence"`, `users: []UserPresence`) whenever clients connect or disconnect. `ChatBloc` tracks `activeUsers` in state, and `Sidebar` renders live member counts, loading states, and active users.
5. **Traefik Path-Based Routing & Ingress Separation**:
   - *Problem*: In deployments where the Flutter frontend PWA and Go backend share the same domain (e.g. `meow.whiskertreat.fyi`), Traefik's default host routing routed all traffic to the frontend container, silently dropping WebSocket handshake (`/ws`) and API requests.
   - *Fix*: Updated `meowgram-server` Traefik router labels in `deploy/docker-compose.yml` with rule `Host(\`${APP_DOMAIN:-localhost}\`) && (PathPrefix(\`/ws\`) || PathPrefix(\`/api\`) || PathPrefix(\`/healthz\`))` and priority `100`, guaranteeing backend endpoints take precedence over the frontend catch-all router.

### 10.5 Windows Packaging & Signing
1. Binary outputs compile to `build/windows/x64/runner/Release/meowGram.exe`.
2. Sign with Authenticode EV certificate via `signtool.exe`:
   ```cmd
   signtool sign /tr http://timestamp.digicert.com /td sha256 /fd sha256 /a "build\windows\x64\runner\Release\meowGram.exe"
   ```

### 10.6 Backend Docker Image Rollout (`meowgram:1.0.0`)
1. **Build and Tag Image**:
   ```bash
   docker build -t meowgram:1.0.0 -t meowgram:latest -f server/Dockerfile server
   ```
2. **Push to Production Registry**:
   ```bash
   docker tag meowgram:1.0.0 registry.purrbrews.cc/meowgram:1.0.0
   docker push registry.purrbrews.cc/meowgram:1.0.0
   ```
3. **Deploy to Production Swarm / Compose**:
   ```bash
   # On production node
   docker compose -f deploy/docker-compose.yml pull
   docker compose -f deploy/docker-compose.yml up -d --no-deps backend
   ```
4. **Health Check Verification**:
   ```bash
   curl -f https://meowgram.purrbrews.cc/healthz
   ```

---

## 11. Review Gate & Verification Checklist

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
- [x] `ResponsiveLayout` responsive wrapper with 800.0 logical pixel breakpoint distinguishing desktop dual-pane from mobile drawer.
- [x] `Sidebar` desktop navigation widget displaying chat channels, active online members, and user profile footer.
- [x] `HiveLocalMessageRepository` cross-platform offline message cache using `hive_flutter` with fallback in-memory repository.
- [x] Cache-to-live handoff in `ChatBloc` delivering instant offline launch before live WebSocket hydration.
- [x] Version string bumped to `1.0.0+1` in `client/pubspec.yaml`.
- [x] Application uniformly named "meowGram" across Android, iOS, macOS, Windows, and Web.
- [x] Launcher icons generated across all OS platforms using `flutter_launcher_icons`.
- [x] Automated build scripts created: `deploy/scripts/build_all.ps1` and `deploy/scripts/build_all.sh`.
- [x] Production `--dart-define` parameters target `purrbrews.cc` environment with secure schemes.
- [x] Android release signing configured with `key.properties` and keystore documentation provided.
- [x] Backend Docker image `meowgram:1.0.0` built and verified (8.7MB static Alpine binary).
- [x] Manual Xcode code signing steps documented for iOS/macOS App Store and TestFlight distribution.
- [x] Full client test suite (40/40 tests across `chat_ui_test.dart`, `responsive_layout_test.dart`, `sync_service_test.dart`, `widget_test.dart`, and `post_login_uat_test.dart`) passing with 100% success rate.
- [x] Go backend test suite (`sync_test.go`, `hub_test.go`, `oidc_test.go`) passing with 100% success rate.
- [x] Production Web build compiled successfully to `client/build/web`.
- [x] Go backend `OIDCVerifier` cryptographically verifies RS256 Authelia tokens against remote JWKS, with trailing slash normalization and multi-audience tolerance.
- [x] Token storage persisted via `flutter_secure_storage` immediately upon PKCE exchange; rehydrated on startup and app foregrounding (`AppLifecycleState.resumed`).
- [x] Authelia user profile extraction resolves `preferred_username` -> `name` -> `email prefix` -> `sub` to avoid raw UUID display in UI.
- [x] Mock presence data removed from `Sidebar`; Go `Hub` presence frame (`type: "presence"`) drives dynamic active roster in `ChatBloc`.
- [x] Traefik router labels in `deploy/docker-compose.yml` updated with path prefixes (`/ws`, `/api`, `/healthz`) and priority 100 to prevent shared-domain collision with frontend container.

---

## 12. Push Notifications ADR: FCM vs. APNs

### Context
meowGram is a real-time chat application requiring instant message delivery across all supported platforms (Web, Android, iOS, macOS, Windows). Initially, a background sync approach via `workmanager` combined with `flutter_local_notifications` was explored to simulate push notifications on mobile.

### Problem
Apple imposes strict restrictions on background execution on iOS:
1. `BGAppRefreshTask` (used by `workmanager`) is scheduled based on proprietary OS heuristics (battery life, user habits) and does not guarantee execution intervals.
2. If an app is force-quit, iOS suspends background execution entirely.
3. Apple's App Store Guidelines prohibit using background execution purely to poll for notifications. 
To achieve real-time alerts, Apple mandates the use of Remote Push Notifications.

### Options Considered

#### 1. Firebase Cloud Messaging (FCM)
FCM provides a unified cross-platform push infrastructure. For iOS, it wraps APNs under the hood. For Android, it uses Google Play Services natively.
- **Pros**:
  - Unified Go backend API (one payload format).
  - Built-in "Topics" (e.g., subscribing to a chat room), removing the need for complex device token fan-out logic in Go.
  - Mature Flutter integration (`firebase_messaging`).
- **Cons**:
  - Adds a Google dependency and requires Firebase configuration (`google-services.json`, `GoogleService-Info.plist`).

#### 2. Direct APNs (Apple Push Notification service)
The Go backend integrates directly with Apple's servers using p8 certificates.
- **Pros**:
  - Eliminates Google as a middleman for Apple devices, maximizing privacy.
- **Cons**:
  - Because Android natively requires FCM, this approach forces the Go backend to implement and maintain **both** FCM and APNs.
  - Requires maintaining device types in PostgreSQL and routing payloads manually (no Topics support for cross-platform broadcasts).

### Decision
**Firebase Cloud Messaging (FCM)** was selected as the push notification infrastructure. The overhead of maintaining dual push implementations (APNs for Apple + FCM for Android/Web) in the Go backend significantly outweighed the privacy benefits of bypassing Google for iOS devices, especially given that meowGram already relies on FCM for Android devices anyway. FCM provides a unified cross-platform API and handles topic subscriptions natively, aligning with the project's requirement for a streamlined, maintainable core.
