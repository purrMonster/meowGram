# meowGram Engineering Runbook & Architectural Decision Record (ADR)

> **Document Version**: 1.13.0
> **Status**: Full codebase re-audit is in progress on `refactor/full-codebase-audit`; see [§12.10](#1210-review-hardening) and [§12.11](#1211-full-codebase-re-audit) for changes and deployment prerequisites. Release 1.0.1 remains the latest deployed release.
> **Author**: Lead Developer / Antigravity IDE; §12 and corrections by Claude  
> **Last Updated**: 2026-10-10

> [!IMPORTANT]
> Statements in this runbook that the 2026-10-07 code review found to be inaccurate have been corrected in place and marked **⚠ Correction**; §§12.10–12.11 record code changes that are not yet deployed. Real domains have been replaced with `example.home.arpa` placeholders, per AGENTS.md ("No real domain in any tracked file"); real values belong in gitignored env files.

---

## 1. Executive Summary & TL;DR

meowGram is a cross-platform realtime cat-themed chat lounge organized as a monorepo consisting of a Go backend service, a Flutter cross-platform client (Web, Desktop, Mobile), and Docker Compose orchestration infrastructure.

### Architectural Mandate Updates
- **Backend OIDC Pivot (Epic 1.2)**: All custom local authentication, password storage, user registration, and local JWT issuance are completely eliminated in favor of **Authelia OpenID Connect (OIDC)**.
- **Client OIDC PKCE Integration (UST-1.2.3)**: The Flutter client implements the OAuth 2.0 Authorization Code Flow with Proof Key for Code Exchange (PKCE, RFC 7636). The client is purely public and stores **zero client secrets**.
- **Real-Time WebSocket Core (Epic 1.3 - UST-1.3.1, UST-1.3.3, UST-1.3.4)**:
  - In-memory Go `Hub` manages active client registries via synchronized channels (`register`, `unregister`, `broadcast`).
  - Strict **Persistence Before Broadcast**: Inbound messages are committed to PostgreSQL (`messages` table) before entering the broadcast pipeline, so nothing is broadcast that isn't stored. (This guarantees durability of accepted messages, not delivery: a frame lost on a dying socket before the server reads it is lost silently, since there is no client acknowledgement.)
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
  - Cache-to-Live Handoff: Seamless synchronization and deduplication between the offline Hive cache and live 50-message WebSocket bursts. **⚠ Correction**: the first frame is currently still blocked by auth refresh and Firebase setup in `main.dart` (§12, H6). ✅ **Fixed in 1.0.1**: only Hive and the stored session load before `runApp`; refresh, notifications, Firebase and background sync run afterwards.
- **Catch-Up Synchronization Pipeline (UST-1.4.3)**:
  - Dedicated REST endpoint `GET /api/messages/sync?after={iso8601_timestamp}&after_id={uuid}` querying messages with a stable `(created_at, id)` cursor (limit 500).
  - Client-side `SyncService` is triggered on WebSocket reconnection, uses the last cached chat message as its resume cursor, and pages until caught up. The backend background sync also follows every full page. Older timestamp-only cursors remain accepted for upgrade compatibility.
  - Strict timezone standardization: timestamps are normalized to UTC (`toUtc().toIso8601String()`) and parsed to UTC before executing PostgreSQL `TIMESTAMPTZ` comparisons.
  - Deduplicating merge in `ChatBloc`: incoming messages are deduplicated by PostgreSQL UUID against live WebSocket hydration bursts and active state, sorted chronologically, and persisted to Hive.
- **Post-Login UAT Hardening (Release 1)**:
  - **WebSocket Handshake Resilience**: Token signature verification directly against Authelia JWKS with trailing-slash normalization on `iss` and flexible audience verification (`aud`). **⚠ Correction**: the "flexible" audience check is a security hole; tokens with no `aud` or no `exp` are accepted (§12, H3). ✅ **Fixed in 1.0.1**: `exp` and `aud` are mandatory.
  - **Persistent Token Storage**: `flutter_secure_storage` encrypted Keychain/Keystore persistence with `first_unlock` accessibility, persisting across app backgrounding, termination, and device reboots.
  - **Friendly Username Extraction**: Prioritized hierarchy parsing `preferred_username` -> `name` -> `email prefix` -> `sub` from Authelia access tokens.
  - **Mock Data Elimination**: Ripped out hardcoded dummy presence members in favor of live presence rosters broadcasted over WebSocket.

---

## 2. Architectural Decisions & Design Rationale (ADR)

| Decision | Selected Technology / Pattern | Rationale & Alternatives Considered |
|---|---|---|
| **Client Auth Flow** | OAuth 2.0 Authorization Code with PKCE (RFC 7636) | Required for public clients where credentials cannot be embedded securely. Replaces legacy implicit flows with cryptographically bound `code_challenge` (S256). |
| **Desktop Redirect URI** | RFC 8252 Loopback HTTP (`http://127.0.0.1:8088/callback`) | Standard for native desktop apps on Windows/macOS/Linux. Eliminates OS custom URI scheme registry requirements during dev/testing. |
| **Web Redirect URI** | Host Origin (`Uri.base.origin`) | Seamless in-browser redirect on Web. URL inspection captures `code` and `state` parameters without local socket binding. **⚠ Correction**: not working; the URL is inspected before the browser opens and the PKCE verifier is lost on the redirect page load (§12, H7). ✅ **Fixed in 1.0.1**: same-tab redirect with PKCE state in `sessionStorage`; the code is exchanged on startup. |
| **Mobile Redirect URI** | Custom URL Scheme (`meowgram://callback`) | Deep link interception via `app_links` on iOS and Android. Replaces loopback sockets on mobile OSes. |
| **Token Transport to WS** | Authenticated ticket exchange (`POST /api/ws-ticket`) | The client exchanges its access token for a random, one-use 30-second ticket; only the ticket appears in the WebSocket URL. The ticket is hashed in server memory and consumed atomically. |
| **Auth State Management** | `AuthController` with `refreshListenable` GoRouter | Declarative route protection. Automatically redirects `/login` -> `/chat` on token acquisition and `/chat` -> `/login` on expiry/logout. |
| **Secure Token Persistence** | `flutter_secure_storage` (`TokenStorage`) | Persists tokens to iOS Keychain (`first_unlock`) and Android KeyStore immediately upon PKCE exchange. Restores session on app startup and foregrounding (`WidgetsBindingObserver`). |
| **Username Claim Resolution** | Token Claim Hierarchy | Resolves user display name from `preferred_username` -> `name` -> `email prefix` -> `sub` UUID fallback, preventing raw Authelia UUIDs in the UI. |
| **Chat Timeline State** | `flutter_bloc` (`ChatBloc`) | Unidirectional data flow cleanly separating WebSocket stream listening from UI presentation. Decouples event handling (history bursts, real-time broadcasts) from rendering. |
| **Responsive Breakpoint** | 800.0 Logical Pixels (`LayoutBuilder`) | Standard split point for tablets/desktops vs smartphones. Screens >= 800px show dual-pane (Sidebar + Chat); screens < 800px show single-pane with Drawer. |
| **Local Cache Database** | `hive_flutter` (`HiveLocalMessageRepository`) | Chosen over `sqflite` for unified cross-platform support across Web (IndexedDB), Windows, macOS, Linux, Android, and iOS without requiring C/FFI toolchains or WebAssembly build glue. |
| **Cache-to-Live Handoff** | Two-stage initialization in `ChatBloc` | Stage 1 loads cached messages from Hive instantly (zero UI wait). Stage 2 connects WebSocket and deduplicates against the 50-message server burst using PostgreSQL UUIDs. |
| **Catch-Up Synchronization** | Dedicated REST Endpoint (`GET /api/messages/sync`) | Fills message gaps after extended offline durations without overloading the initial WebSocket upgrade frame. Capped at 500 records per request to prevent payload bloat. |
| **Timezone Standardization** | ISO 8601 UTC / RFC 3339 (`.UTC()`) | Client exports `createdAt.toUtc().toIso8601String()`; Go backend normalizes any offset to UTC before querying PostgreSQL `TIMESTAMPTZ` column. Prevents timezone drift across global clients. |
| **Release 1 Packaging** | Multi-Platform Scripts (`build_all.ps1` / `build_all.sh`) | Unified build scripts inject production `--dart-define` flags targeting `example.home.arpa`, producing Web, Android (APK/AAB), Windows, macOS, and iOS bundles. |
| **Android Keystore Separation** | Git-ignored `key.properties` & `*.jks` | Cryptographic signing keys remain strictly local/CI-injected; `build.gradle.kts` gracefully falls back to debug signing when `key.properties` is absent. |
| **Backend Docker Packaging** | Multi-stage static Alpine image (`meowgram:1.0.0`) | Stripped, non-root 8.7MB container embedding migrations, healthcheck, and CA certificates for production orchestration. |
| **Time Formatting** | `intl` (`DateFormat.jm()`) | Standardized localized time representation across Web and native mobile/desktop platforms. |
| **Mobile Keyboard Adaptation** | `SafeArea` + `viewInsets` in `ChatInputBar` | Prevents software keyboard from overlapping chat input bar on Android/iOS without double-padding on Web/Desktop. |
| **Message Ordering Guarantee** | Chronological Sorting & In-Memory Deduplication | Re-sorts by `createdAt` ascending and deduplicates by PostgreSQL UUID to guarantee deterministic ordering across network jitters. |
| **Token Refresh Lifecycle** | Proactive Background Timer (`expiresAt - 60s`) | Silently exchanges `refresh_token` for a fresh `access_token` prior to expiration, preventing WebSocket disconnects during active chat. **⚠ Correction**: any refresh failure, including being offline, triggers `logout()`, and timer and app-resume refreshes can race (§12, H6). ✅ **Fixed in 1.0.1**: refresh is single-flight, and only an HTTP 400/401 rejection ends the session; network errors retry. |
| **Backend OIDC Verifier** | JWKS Remote KeySet + strict API audience | Cryptographically verifies signed JWT access tokens against Authelia's JWKS and requires `sub`, `exp`, and one configured API audience. The OIDC client ID is never an API audience. Authelia must issue JWT access tokens with the API audience. |
| **Broadcast Engine** | Go In-Memory `Hub` with Goroutine Channels | Highly performant, zero external messaging broker (Redis/RabbitMQ) dependency needed for single-node core. Suited to household scale; no load testing has been done, so throughput claims are unverified. |
| **Real-Time Presence** | Hub Presence Frames (`type: "presence"`) | Dynamically tracks connected clients and broadcasts online roster on join/disconnect, eliminating hardcoded mock member lists. |
| **Write Durability** | **Persistence Before Broadcast** | `ReadPump` saves message to PostgreSQL *before* queueing into `hub.Broadcast`. If the database write fails or client drops mid-flight, uncommitted state never corrupts peer chat streams. |
| **Socket Thread Safety** | Gorilla WebSocket `ReadPump` & `WritePump` Split | Gorilla `*websocket.Conn` forbids concurrent writer calls. Isolating socket writes exclusively to `WritePump` while reads run on `ReadPump` eliminates data races without coarse mutex locks. |
| **Backpressure Protection** | Non-blocking Broadcast with Channel Eviction | `Hub` uses `select { case client.send <- msg: default: unregister }` with a 256-frame buffered channel. Slow or stalled clients cannot block the main broadcast loop or lag other peers. |
| **Ingress Path-Based Routing** | Traefik Router Rules (`PathPrefix`) + Priority | Evaluates backend routes (`/ws`, `/api`, `/healthz`) with priority 100 before frontend catch-all router on shared host domains. |
| **Database Migrations** | `golang-migrate/migrate/v4` | Automated `.up.sql` migrations executed on server container initialization. |
| **Root Route Auto-Login** | `GoRouter` Redirect | Automatically routes authenticated users hitting `/` directly to `/chat`, preventing dead-ends. |
| **Background Sync** | `workmanager` (opt-in, Android only) | Periodically checks the REST `sync` endpoint and shows one "new messages" notification. **1.0.1**: off unless built with `--dart-define=ENABLE_BACKGROUND_SYNC=true`; never opens the Hive cache from the background isolate (Hive is not multi-isolate safe); keeps its own cursor in secure storage. Not on iOS (needs BGTaskScheduler setup; Apple discourages polling). |
| **Local Notifications** | `flutter_local_notifications` | OS alerts for **live chat from other people while the app is in the background** (1.0.1; previously every frame, including the 50-message history burst, notified). Android, iOS, macOS. |
| **Push Notifications** | Firebase Cloud Messaging (FCM) topics | Chosen over direct APNs (see §10.9). **1.0.1**: content-free payload, coalesced and collapsed; subscribe only while signed in. Per-device tokens are the planned replacement for topics. |

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
  - Bound to WebSocket read loop. Enforces `pongWait` (60s), `maxMessageSize` (**512 KB**, i.e. `512 * 1024`; there is no smaller app-level text cap), and `SetReadDeadline`.
  - Accepts either a raw text frame or a JSON envelope `{"text_content": "Hello!"}`. Any `type` field sent by the client is ignored.
  - Injects verified `sender_id` (Authelia `sub`) and triggers `messageRepo.Create(...)`.
  - Upon successful DB insert, sends message payload to `hub.Broadcast`.
  - Defers `hub.Unregister` and `conn.Close()` on socket termination.
- **`Client.WritePump()`**:
  - Exclusively handles all socket write operations.
  - Listens to `client.Send` channel and `ticker` (ping interval 54s).
  - Encodes payloads into WebSocket JSON text frames and sends ping control frames to keep network connections alive.
  - **Batching**: messages already queued in `send` are flushed into the *same* text frame, separated by `\n`. Clients must split frames on newlines before JSON-decoding (`ChatWebSocketService._handleIncomingData` does).

### 3.2 Database Schema: Messages Table (`migrations/000002_create_messages_table.up.sql`)

**⚠ Correction**: the previous snippet here did not match the migration file. This is the actual SQL:

```sql
-- Ensure users.authelia_sub has an explicit UNIQUE constraint for foreign key linkage
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'uq_users_authelia_sub'
    ) THEN
        ALTER TABLE users ADD CONSTRAINT uq_users_authelia_sub UNIQUE (authelia_sub);
    END IF;
END $$;

-- Persistent message ledger
CREATE TABLE IF NOT EXISTS messages (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    sender_id VARCHAR(255) NOT NULL REFERENCES users(authelia_sub) ON DELETE CASCADE,
    text_content TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_messages_created_at ON messages (created_at DESC);
CREATE INDEX IF NOT EXISTS idx_messages_sender_id ON messages (sender_id);
```

Notes:
- Migration 000001 already creates the unique index `idx_users_authelia_sub`, so `authelia_sub` ends up with **two** unique indexes.
- `ON DELETE CASCADE` means deleting a user deletes their entire message history.
- `users.username` is `VARCHAR(100)`: a longer `preferred_username` makes auto-provisioning fail with HTTP 500.

### 3.3 WebSocket JSON Protocol Specification
- **Inbound Client Frame** (a plain text frame is also accepted):
  ```json
  {
    "text_content": "Purr purr! Hello meowGram lounge."
  }
  ```
- **Outbound Broadcast Frame**:
  ```json
  {
    "type": "chat",
    "id": "7f8b91a2-3c4d-5e6f-7a8b-9c0d1e2f3a4b",
    "sender_id": "usr_authelia_sub_12345",
    "username": "sir_purrsalot",
    "text_content": "Purr purr! Hello meowGram lounge.",
    "created_at": "2026-10-03T12:30:00Z"
  }
  ```
- **Other frame types**: `system` (join/leave notices, no `id`), `presence` (`users: [{username, sub, is_online}]`, broadcast to everyone, so every member's Authelia `sub` is visible to all clients), `error` (sent to the sender when the DB write fails), `history`.
- **Connection Hydration (Recent History)**:
  On successful upgrade and registration, the server streams the last 50 persisted messages to the connecting client formatted as `"type": "history"`. Because registration happens first, the client's own "pounced into the lounge" `system` notice (timestamped *now*) and the `presence` frame usually arrive **before** the history burst.

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
   - **⚠ Correction (does not work today)**: `OidcPlatformWebHelper.listenForAuthCode` reads `Uri.base` synchronously *before* the browser is launched, so it returns `null` at once and login reports "cancelled". When Authelia redirects back, the page reloads, the in-memory PKCE verifier is gone, and no startup code exchanges the `code`. The `state` check is also skipped when `state` is absent. The fix is to persist verifier and state in `sessionStorage`, redirect in the same tab, and handle `?code=` at startup. ✅ **Fixed in 1.0.1**: implemented exactly this way (`oidc_platform_web.dart`, `AuthController.initialize`); a missing or mismatched `state` is rejected.
2. **Desktop (Windows, macOS, Linux)**:
   - Redirect URI default: `http://127.0.0.1:8088/callback`.
   - Callback mechanism: Ephemeral local loopback server (`HttpServer.bind(InternetAddress.loopbackIPv4, 8088)`) listens for the redirect, responds with a confirmation HTML page, and yields the authorization code. The port is **fixed** at 8088 (not dynamically allocated). Any request to `/callback` with a wrong `state` aborts the login, and the error page interpolates the `error` query parameter into HTML without escaping.
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
  - Channels directory (`# general-lounge`, `# cat-memes`, `# treat-discussions`, …). **⚠ Correction**: these rooms and their unread counts are hardcoded mock data in `sidebar.dart`. The backend has a single room; selecting another room only changes the header text. ✅ **Fixed in 1.0.1**: the sidebar lists only the real lounge.
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
  - Inbound messages are saved to Hive asynchronously in the background (`unawaited(_localRepo.saveMessage(incoming))`).
- **⚠ Known limitations**:
  - `system`/`error` frames (no `id`) are cached too, under synthetic keys, and accumulate forever.
  - The Hive box is never pruned and **never cleared on logout**: the next user on a shared device, or the same browser profile on web, sees the previous user's history.
  - `getCachedMessages()`/`getNewestMessage()` decode and sort the whole box every call, so cost grows with history size.
  - `ChatInitializeRequested` is dispatched twice (by `ResponsiveLayout` and by `ChatScreen`), and again whenever the window crosses the 800 px breakpoint.
  - `ChatState ==` compares only list *lengths*, so same-length changes (e.g. a roster swap) are swallowed by Bloc.
  - There is no automatic WebSocket reconnect: after a drop, only the manual refresh button reconnects.
- ✅ **Fixed in 1.0.1**: only server-confirmed chat messages are cached (legacy junk purged); the cache is capped at 2,000 and cleared on logout; init is idempotent; `ChatState` compares contents; the socket reconnects with backoff (1–30 s), on resume and after a token refresh. The full-box decode remains, but is bounded by the cap.

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
       │                         │ (3) GET /api/messages/sync?after={UTC_ISO}&after_id={UUID}│       │
       │                         ├───────────────────────────────────────────────────────────────►│
       │                         │                                           │                    │ (4) created_at > $1
       │                         │                                           │                    │ OR (created_at = $1 AND id > $2)
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
- **The Solution**: A dedicated REST endpoint (`GET /api/messages/sync?after={timestamp}&after_id={uuid}`) lets the client page through messages after a stable timestamp and UUID cursor, up to 500 messages per request.
- **Cursor and paging**: The cursor is `(created_at, id)` so messages sharing a timestamp are not skipped. The client captures the newest cached chat message before reconnecting and requests pages until one is shorter than 500 messages. Failed pages retain the cursor and retry after 15 seconds.

### 7.2 Endpoint Contract & Timezone Standardization
- **Endpoint**: `GET /api/messages/sync?after={iso8601_timestamp}&after_id={uuid}`
- **Security**: HTTP sync and ticket exchange require an Authelia OIDC Bearer token (`Authorization: Bearer <token>`). The WebSocket authenticates with the short-lived one-use ticket from `POST /api/ws-ticket`; access tokens are never accepted in its URL.
- **Query Parameters**: `after` (mandatory, ISO 8601 / RFC 3339 formatted); `after_id` (optional UUID used to disambiguate messages with the same timestamp; omitted for old timestamp-only cursors).
- **Timezone Standardization Architecture**:
  - **Client**: `newest.createdAt.toUtc().toIso8601String()` produces RFC 3339 with `Z` suffix (e.g. `2026-10-03T13:40:00.123456Z`).
  - **URL Encoding**: `Uri.replace(queryParameters: {'after': afterIso})` safely percent-encodes colons and plus signs.
  - **Go Backend Parser (`ParseSyncTimestamp`)**: Supports `RFC3339Nano`, `RFC3339`, ISO variants, and numeric epoch timestamps. Resolves space-encoded `+` characters in timezone offsets. **⚠ Correction**: the `2006-01-02 15:04:05` (space-separated) layout can never match, because the `+`-restoration step rewrites the space first. Verified: `"2026-10-03 13:40:00"` → error. ✅ **Fixed in 1.0.1**: only a trailing ` HH:MM` after a `T` time is treated as an offset.
  - **Database Query**: Converted to `.UTC()` before parameter binding (`$1`). PostgreSQL stores `created_at` as `TIMESTAMPTZ` (UTC internally); pagination compares `created_at`, then UUID, with the same ordering as the client timeline.
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
| `APP_DOMAIN` | `localhost` | `meow.example.home.arpa` | Primary routing domain. Injected into Traefik host rules and link generation. |
| `ENVIRONMENT` | `development` | `production` | Environment tier (`development`, `staging`, `production`). |
| `APP_ENV` | `development` | `production` | Alias for `ENVIRONMENT`. |
| `LOG_LEVEL` | `debug` | `info` | Minimum log verbosity (`debug`, `info`, `warn`, `error`). |
| `USE_SECURE_SCHEMES` | `false` | `true` | Enforces `https://` and `wss://` protocols. |
| `HOST_HTTP_PORT` | `127.0.0.1:8080` | `127.0.0.1:8080` | Loopback host binding for the backend. |
| `HOST_WEB_PORT` | `127.0.0.1:8081` | `127.0.0.1:8081` | Loopback host binding for the Flutter web client. |
| `HTTP_PORT` | `8080` | `8080` | Internal container port bound by the Go HTTP server. |
| `WS_PORT` | `8080` | `8080` | WebSocket endpoint port (unified with `HTTP_PORT` over `/ws`). |
| `API_BASE_URL` | *(Computed)* | `https://meow.example.home.arpa` | Full REST API base URL override. |
| `WS_BASE_URL` | *(Computed)* | `wss://meow.example.home.arpa/ws` | Full WebSocket base URL override. |
| `WS_ENDPOINT` | `/ws` | `/ws` | WebSocket upgrade route path. |
| `WS_TICKET_ENDPOINT` | `/api/ws-ticket` | `/api/ws-ticket` | Authenticated endpoint that issues short-lived one-use WebSocket tickets. |
| `SYNC_ENDPOINT` | `/api/messages/sync` | `/api/messages/sync` | Catch-up synchronization REST route path. |
| `HEALTH_ENDPOINT` | `/healthz` | `/healthz` | Health check probe route path. |
| `AUTHELIA_DOMAIN` | `localhost:9091` | `auth.example.home.arpa` | FQDN or host:port for Authelia OIDC identity provider. |
| `AUTHELIA_ISSUER` | *(Derived)* | `https://auth.example.home.arpa` | Base Issuer URL for Authelia OIDC provider. Derived from `AUTHELIA_DOMAIN`. |
| `AUTHELIA_ISSUER_URL` | *(Derived)* | `https://auth.example.home.arpa` | Alias for `AUTHELIA_ISSUER`. |
| `AUTHELIA_CLIENT_ID` | `meowgram` | `meowgram` | Public OIDC client ID. The Authelia registration in `purrbrews-containers` must be aligned before deploying this branch. |
| `AUTHELIA_AUDIENCE` | `http://localhost:8080` | `https://meow.example.home.arpa` | Required API audience. The backend accepts only this value; do not set it to the OIDC client ID. Configure Authelia to issue signed JWT access tokens with this audience. |
| `AUTHELIA_DOMAIN` | *(unset)* | `auth.example.home.arpa` | Also widens issuer matching to `http(s)://<domain>`. Passed through by Compose. |
| `GOOGLE_APPLICATION_CREDENTIALS` | *(unset)* | `/run/secrets/fcm-service-account.json` | Path to FCM service-account JSON. Compose passes the path and mounts `deploy/secrets` read-only. |
| `MIGRATIONS_PATH` | `/migrations` (image) | `/migrations` | Directory of `golang-migrate` SQL files; migrations run at every startup. |
| `READ_TIMEOUT_SECONDS` / `WRITE_TIMEOUT_SECONDS` / `IDLE_TIMEOUT_SECONDS` | `15` / `15` / `60` | same | `http.Server` timeouts. |
| `IMMICH_DOMAIN` / `IMMICH_API_URL` | *(unset)* | deployment-specific | Release 2. No default; the API key belongs on the server, never in a client define. |
| `AUTHELIA_JWKS_URL` | *(Derived)* | `https://auth.example.home.arpa/jwks.json` | URL for Authelia public keys (JWKS). Derived from `AUTHELIA_ISSUER`. |
| `POSTGRES_USER` | `meowgram` | `meowgram_prod` | PostgreSQL user account. |
| `POSTGRES_PASSWORD` | `meowgram_secret_dev...` | *(Strong secret)* | PostgreSQL password. |
| `POSTGRES_DB` | `meowgram` | `meowgram` | PostgreSQL database name. |
| `DATABASE_URL` | `postgres://...` | `postgres://...` | Full connection string for Go backend. |
| `CORS_ORIGINS` | *(Derived: local origins in development; HTTPS app origin in staging/production)* | `https://meow.example.home.arpa` | Comma-separated allow-list. Set explicitly when serving the app from another origin. |
| `TRAEFIK_ENTRYPOINTS` | `web` | `web` / `websecure` | Traefik entrypoint(s) used by the backend and web routers. |
| `BACKUP_RETENTION_DAYS` | `14` | `14` or more | Number of days of local PostgreSQL backups to retain. |

### Flutter Client Environment Variables & `.env` Controllability

All endpoints are **compile-time only**, set via `--dart-define-from-file=<file>` or `--dart-define=KEY=VAL`. **⚠ Correction**: there is no `AppConfig.initialize()` and no runtime `.env` loading; a `.env` file placed next to the app has no effect.

- `OIDC_SCOPES` appears in the example files but is **not read**; scopes are hardcoded in `AppConfig.oidcScopes`.
- Never put secrets in `--dart-define`: they are compiled into every binary and into the web JavaScript. This applies to the planned `IMMICH_API_KEY`, which must live on the server.

| Variable / Flag | Dev Default | Description |
|---|---|---|
| `APP_DOMAIN` | `localhost` | Target backend host domain. |
| `HTTP_PORT` | `8080` | Target backend HTTP port. |
| `APP_ENV` | `development` | Runtime environment name (`development`, `staging`, `production`). |
| `USE_SECURE_SCHEMES` | `false` | Whether to force `https://` and `wss://`. |
| `API_BASE_URL` | *(Computed)* | Direct override for HTTP REST API base URL. |
| `WS_BASE_URL` | *(Computed)* | Direct override for WebSocket base URL. |
| `WS_ENDPOINT` | `/ws` | WebSocket relative route path. |
| `WS_TICKET_ENDPOINT` | `/api/ws-ticket` | Authenticated endpoint for short-lived WebSocket tickets. |
| `SYNC_ENDPOINT` | `/api/messages/sync` | Catch-up sync endpoint path or full URL. |
| `HEALTH_ENDPOINT` | `/healthz` | Health check endpoint path or full URL. |
| `AUTHELIA_DOMAIN` | `localhost:9091` | Authelia identity provider domain. Setting this auto-derives all OIDC endpoints! |
| `AUTHELIA_ISSUER_URL` | *(Derived)* | Authelia OIDC base issuer URL for discovery. |
| `AUTHELIA_CLIENT_ID` | `meowgram` | Public client identifier registered in Authelia. This repository now uses the canonical ID; align the Authelia registration in `purrbrews-containers` before deploying. |
| `AUTHELIA_JWKS_URL` | *(Derived)* | Authelia cryptographic public keys URL. |
| `AUTHELIA_DISCOVERY_URL` | *(Derived)* | OpenID configuration discovery endpoint URL. |
| `AUTHELIA_AUTHORIZATION_ENDPOINT` | *(Derived)* | OIDC PKCE authorization endpoint. |
| `AUTHELIA_TOKEN_ENDPOINT` | *(Derived)* | OIDC token exchange endpoint. |
| `AUTHELIA_USERINFO_ENDPOINT` | *(Derived)* | OIDC userinfo endpoint. |
| `AUTHELIA_REVOCATION_ENDPOINT` | *(Derived)* | OIDC token revocation endpoint. |
| `AUTH_REDIRECT_URI` | *(Dynamic)* | OAuth redirect URI (default: RFC 8252 loopback on desktop, origin on web). |
| `IMMICH_DOMAIN` | *(real domain hardcoded in `app_config.dart`; should be empty)* | Immich media service domain (Release 2). |
| `IMMICH_API_URL` | *(Derived)* | Immich REST API base URL (Release 2). |
| `IMMICH_API_KEY` | *(empty)* | **Do not use.** A client-side dart-define ships the key to every user; proxy Immich through the Go server instead. |

---

## 9. Operations & Developer Playbook

### 9.1 Starting the Infrastructure (PostgreSQL + Go Backend)

```powershell
cd c:\Users\jyotirmoyc\Desktop\Projects\meowGram

# 1. Initialize environment files if not present (then set a real POSTGRES_PASSWORD)
if (!(Test-Path .env)) { Copy-Item .env.example .env }
if (!(Test-Path deploy/.env)) { Copy-Item deploy/.env.example deploy/.env }

# 2. Build and launch services in detached mode (same invocation as roastery, per AGENTS.md §3)
docker compose --env-file .env --env-file deploy/.env -f deploy/docker-compose.yml up -d --build

# 3. Verify health status
curl http://localhost:8080/healthz
```

> [!NOTE]
> ✅ **Fixed in 1.0.1**: compose now **requires** `POSTGRES_PASSWORD` (it refuses to start without it) and binds both published ports to `127.0.0.1` by default. Set `POSTGRES_PASSWORD` in `deploy/.env` before the first `up`.

### 9.2 Launching the Flutter Client with OIDC PKCE

#### On Google Chrome (Web):
```powershell
cd c:\Users\jyotirmoyc\Desktop\Projects\meowGram\client

flutter run -d chrome `
  --dart-define=APP_DOMAIN=localhost `
  --dart-define=HTTP_PORT=8080 `
  --dart-define=AUTHELIA_CLIENT_ID=meowgram `
  --dart-define=AUTHELIA_ISSUER_URL=http://localhost:9091 `
  --dart-define=APP_ENV=development
```

#### On Windows Desktop:
```powershell
flutter run -d windows `
  --dart-define=APP_DOMAIN=localhost `
  --dart-define=HTTP_PORT=8080 `
  --dart-define=AUTHELIA_CLIENT_ID=meowgram `
  --dart-define=AUTHELIA_ISSUER_URL=http://localhost:9091 `
  --dart-define=APP_ENV=development
```

### 9.3 Automated Testing

```powershell
# Run Flutter client unit and widget test suite
cd c:\Users\jyotirmoyc\Desktop\Projects\meowGram\client
flutter test

# Run Go backend test suite (prefer -race: the hub is concurrency-heavy)
cd c:\Users\jyotirmoyc\Desktop\Projects\meowGram\server
go test -race ./...
```

**⚠ Correction**: Go tests cover `auth` (verifier with a mock key set), `chat` (hub lifecycle), `config` and `handler` (sync parsing and validation). There are **no** tests for migrations, repositories, `ReadPump`/`WritePump`, the auth middleware or CORS. As of 2026-10-07 on `feat/push-notifications`, the `chat` package **does not compile** (`hub_test.go` calls `NewHub(logger)`; the signature is now `NewHub(logger, fcmService)`). ✅ **Fixed in 1.0.1**: the suite compiles and passes under `-race`, with new tests for hub safety (closed-channel panic, shutdown, content-free push), mandatory OIDC claims, strict audience and timestamp parsing. Client: 53 tests, including `release_regression_test.dart`.

---

## 10. Release 1 Build & Packaging Procedures

### 10.1 Production Environment Target
All production binaries must be compiled with `--dart-define` flags targeting the production infrastructure. Keep the real values in a **gitignored** `--dart-define-from-file` (AGENTS.md known gap: `client/config/production.json` and `build_all.*` defaults currently hardcode real domains and are tracked). The deployment on roastery is `meow.<domain>`, not `meowgram.<domain>` as the old defaults assumed.
```bash
--dart-define=APP_ENV=production \
--dart-define=APP_DOMAIN=meow.example.home.arpa \
--dart-define=HTTP_PORT=443 \
--dart-define=USE_SECURE_SCHEMES=true \
--dart-define=AUTHELIA_ISSUER_URL=https://auth.example.home.arpa \
--dart-define=AUTHELIA_CLIENT_ID=meowgram \
--dart-define=API_BASE_URL=https://meow.example.home.arpa \
--dart-define=WS_BASE_URL=wss://meow.example.home.arpa/ws
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
   - Verify `Incoming Connections (Server)` and `Outgoing Connections (Client)` are checked to permit WebSocket/HTTP connectivity and the 8088 loopback login listener.
   - **⚠ Correction**: there is no `Runner.entitlements`. The files are `DebugProfile.entitlements` (has `network.server`, **missing `network.client`**) and `Release.entitlements` (**missing both**). As committed, a sandboxed macOS build cannot reach the backend or Authelia, and the release build cannot bind the login listener. `flutter_secure_storage` on macOS may also need a Keychain Sharing (`keychain-access-groups`) entitlement. ✅ **Fixed in 1.0.1**: `network.client` is added to both files and `network.server` to Release. The Keychain question still needs verifying on a Mac.
4. **Archive & Distribution**:
   - In Xcode menu, select `Product > Archive`.
   - In the Organizer window, click `Distribute App` &rarr; `App Store Connect` (or `Direct Distribution / Developer ID` for notarized macOS `.dmg`).

#### 10.4.1 Day 1 iOS Testing: Progressive Web App (PWA) Mode
> [!WARNING]
> The review-hardening branch adds a Flutter web image to Compose. A deployment must rebuild and start it before the root URL is served. Local bind is `127.0.0.1:8081`; the optional development Traefik routes `/` to the frontend and `/api`, `/ws`, and `/healthz` to the backend.

To test on iOS devices without waiting for Apple Developer Team certificate provisioning:
1. Navigate to `https://meow.example.home.arpa` in Safari on iOS.
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
   - **Background Survival**: `AuthController` retains `_pendingPkce` and `_pendingRedirectUri` on the controller instance, preventing state loss during OS app suspension while Safari is in the foreground. This is in-memory only: if the OS *terminates* the app while the browser is open, the verifier is lost and the login must be restarted.
   - **Custom scheme caveat**: `meowgram://` can be claimed by any app on Android. PKCE protects the code exchange, but an App Link or Universal Link is the stronger option (AGENTS.md known gap).
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
   - **⚠ Correction**: the fix over-corrected. `validateAudience` accepts tokens with **no `aud`** (unless `AUTHELIA_AUDIENCE` is set) and always accepts the hardcoded strings `meowgram` and `meowgram-client`; `exp` is only checked *if present*. See §12, H3. ✅ **Fixed in 1.0.1**.
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
   - *Problem*: In deployments where the Flutter frontend PWA and Go backend share the same domain (e.g. `meow.example.home.arpa`), Traefik's default host routing routed all traffic to the frontend container, silently dropping WebSocket handshake (`/ws`) and API requests.
   - *Fix*: Updated `meowgram-server` Traefik router labels in `deploy/docker-compose.yml` with rule `Host(\`${APP_DOMAIN:-localhost}\`) && (PathPrefix(\`/ws\`) || PathPrefix(\`/api\`) || PathPrefix(\`/healthz\`))` and priority `100`, guaranteeing backend endpoints take precedence over the frontend catch-all router.

### 10.5 Windows Packaging & Signing
1. Binary outputs compile to `build/windows/x64/runner/Release/meowGram.exe`.
2. **Metadata Configuration**: `client/windows/runner/Runner.rc` is configured with the target metadata (`CompanyName: com.purrbrews`, `ProductName: meowGram`, `FileDescription: meowGram`).
3. Sign with Authenticode EV certificate via `signtool.exe`:
   ```cmd
   signtool sign /tr http://timestamp.digicert.com /td sha256 /fd sha256 /a "build\windows\x64\runner\Release\meowGram.exe"
   ```

### 10.6 Backend Docker Image Rollout

**⚠ Correction**: the previous registry-push procedure conflicted with AGENTS.md ("Git-only deployment on Docker: what runs is built from this repo through `deploy/docker-compose.yml`"). It also targeted a compose service named `backend`, which doesn't exist; the service is `server`, and its image is `meowgram-server:latest`. The supported rollout is:

1. **On the host (roastery), update the checkout**:
   ```bash
   git fetch origin && git switch main && git pull --ff-only
   ```
2. **Rebuild and restart the server only**:
   ```bash
   docker compose --env-file .env --env-file deploy/.env -f deploy/docker-compose.yml up -d --build --no-deps server
   ```
3. **Health Check Verification**:
   ```bash
   curl -f https://meow.example.home.arpa/healthz
   ```

`deploy/scripts/build_all.*` still tags `meowgram:1.0.0`, which is useful for local verification but not what compose runs. Migrations run automatically at server start, so take a database backup first: there are **no backups** on roastery yet (AGENTS.md known gap).

### 10.7 FCM Push Notification Architecture

> [!NOTE]
> PR #15 (`feat/push-notifications`) was merged on 2026-10-07 and hardened in release 1.0.1: content-free pushes, subscribe only while signed in, credentials mounted via compose. See §12.9.

During the transition from Apple Push Notifications (APN) direct integration to Firebase Cloud Messaging (FCM), several key decisions were made:
- **FCM Over APN**: We chose FCM because it abstracts away Apple's strict background notification constraints and unifies our push notification code across Android, iOS, and Web platforms using a single publisher.
- **Go Backend Publisher**: The Go backend utilizes the `firebase.google.com/go/v4/messaging` SDK. The Hub uses a separate goroutine to publish to the `room_lounge` topic whenever a message of type `chat` is broadcasted, ensuring no blocking of the main WebSocket fan-out loop.
- **Zero Hardcoding Compliance**: The Go server reads `GOOGLE_APPLICATION_CREDENTIALS` for the Service Account JSON.
  - **⚠ Correction**: the file is *not* covered by `.gitignore`, which only ignores `*.pem`, `*.key`, `*.cert` and `*.crt`. Add an explicit pattern (e.g. `*service-account*.json`, `deploy/secrets/`) before the file ever enters the tree. ✅ **Fixed in 1.0.1**.
  - **⚠ Correction**: `deploy/docker-compose.yml` neither passes the variable nor mounts the file, so push is silently disabled in every Docker deployment. ✅ **Fixed in 1.0.1**: compose mounts `deploy/secrets/` read-only at `/run/secrets` and passes `GOOGLE_APPLICATION_CREDENTIALS` (see `deploy/secrets/README.md`).
- **Flutter Client Configuration**: The client employs `flutterfire_cli` to auto-generate `firebase_options.dart`, meaning API Keys are embedded publicly but do not compromise the backend's secret Service Account key. The client calls `FirebaseMessaging.instance.subscribeToTopic('room_lounge')` to receive messages, handling them in both foreground and background states.
- **⚠ Privacy design flaw (release-blocking)**:
  - The client subscribes to `room_lounge` in `main.dart` **before login** and never unsubscribes on logout.
  - The server puts the **full message text** in the notification body.
  - FCM topics have no access control, so any install of the app, logged in or not (e.g. a sideloaded APK), receives every lounge message. This bypasses Authelia entirely and sends household chat content through Google.
  - Fix: register per-device FCM tokens through an authenticated endpoint and send **data-only** "new message" pings with no text; the app then fetches over the authenticated API. Interim minimum: subscribe after login, unsubscribe on logout, drop the body.
  - ✅ **Fixed in 1.0.1** (interim): pushes are content-free ("New messages in the lounge 🐾"), with no text and no sender; they collapse per topic and are throttled to one per 30 s. Devices subscribe only while signed in, unsubscribe on logout, and old pre-login subscriptions are removed. **Still open:** an unauthenticated install can still learn *that* the lounge is active. Per-device tokens remain the proper fix.
- **Other FCM gaps**:
  - Senders receive pushes for their own messages, and pushes are sent even when every user is online.
  - FCM rejects payloads over 4 KB, while the WebSocket accepts up to 512 KB.
  - `created_at` is sent via `time.Time.String()`, not RFC 3339.
  - `requestPermission()` and `subscribeToTopic()` are awaited before `runApp`, so the OS permission prompt and any offline stall block the first frame.
  - `firebase_messaging` has no Windows support; the web build has no `firebase-messaging-sw.js` or VAPID key.
- **Pending Apple Notifications Setup**: 
  - **Backend**: `server/internal/fcm/service.go` currently lacks the `APNSConfig` payload. This needs to be added for iOS devices to play sounds and update badge counts.
  - **Xcode**: The `aps-environment` entitlement must be added via Xcode (Signing & Capabilities -> Push Notifications).
  - **Apple Developer Portal**: An APNs Auth Key (.p8) needs to be generated and uploaded to the Firebase Console for production use.

### 10.8 JSON Configuration for Build Environments
To clean up lengthy `--dart-define` build commands, we migrated to using `--dart-define-from-file=config/production.json`.
- **`client/config/production.json`**: Centralizes production URLs and ports. Passed to the Flutter CLI during compilation. **⚠ Correction**: this file is tracked and contains real domains (and the `meowgram-client` client ID), which violates AGENTS.md. Move it to a gitignored `config/production.local.json` and keep a placeholder-only template under version control. ✅ **Fixed in 1.0.1**: the scripts prefer `config/<env>.local.json` and refuse release builds that would use placeholder domains.
- **Testability**: `client/test/production_config_test.dart` ensures the `AppConfig` properly reads and cascades these configuration values.

### 10.9 ADR: Push Notifications — FCM vs. APNs

*Added in PR #19 (`docs/adr-updates`); renumbered from §12 and annotated in 1.0.1.*

#### Context
meowGram is a real-time chat application requiring instant message delivery across all supported platforms (Web, Android, iOS, macOS, Windows). Initially, a background sync approach via `workmanager` combined with `flutter_local_notifications` was explored to simulate push notifications on mobile.

#### Problem
Apple imposes strict restrictions on background execution on iOS:
1. `BGAppRefreshTask` (used by `workmanager`) is scheduled based on proprietary OS heuristics (battery life, user habits) and does not guarantee execution intervals.
2. If an app is force-quit, iOS suspends background execution entirely.
3. Apple's App Store Guidelines prohibit using background execution purely to poll for notifications.
To achieve real-time alerts, Apple mandates the use of Remote Push Notifications.

#### Options Considered

**1. Firebase Cloud Messaging (FCM).** FCM provides a unified cross-platform push infrastructure. For iOS, it wraps APNs under the hood. For Android, it uses Google Play Services natively.
- Pros: unified Go backend API (one payload format); built-in "Topics" (e.g. subscribing to a chat room), removing the need for device-token fan-out logic in Go; mature Flutter integration (`firebase_messaging`).
- Cons: adds a Google dependency and requires Firebase configuration (`google-services.json`, `GoogleService-Info.plist`).

**2. Direct APNs.** The Go backend integrates directly with Apple's servers using p8 certificates.
- Pros: eliminates Google as a middleman for Apple devices, maximizing privacy.
- Cons: Android natively requires FCM, so the backend would maintain **both** FCM and APNs; requires device types in PostgreSQL and manual routing (no Topics).

#### Decision
**Firebase Cloud Messaging (FCM)** was selected. Maintaining dual push implementations outweighed the privacy benefit of bypassing Google for iOS, given that Android needs FCM anyway.

#### Consequences (added in 1.0.1)
- **Topics have no access control**: any install of the app can subscribe to `room_lounge`, signed in or not. Therefore the payload must never carry message text or sender identity. 1.0.1 sends a content-free, collapsed "New messages in the lounge" notice.
- The "no device-token fan-out" advantage is exactly what causes the exposure. The planned follow-up is per-device tokens registered through an authenticated endpoint, with data-only pings, so only signed-in devices are notified.
- Web push (service worker + VAPID key) and Windows (no FCM plugin) are not supported; Windows relies on the live WebSocket while the app runs.

---

## 11. Review Gate & Verification Checklist

> Re-verified 2026-10-07 against branch `feat/push-notifications` (§12). Items that were ticked but don't hold have been unticked, with the reason.

- [x] Database migration `000002_create_messages_table.up.sql` created and verified.
- [x] Foreign key constraint properly mapped to `users.authelia_sub`.
- [x] In-memory Go `Hub` maintains thread-safe registry of connected clients via goroutines and channels. *(`Hub.ClientCount()` reads the map from outside the hub goroutine; it's unused in production but unsafe.)*
- [x] Separate `ReadPump` and `WritePump` goroutines guarantee Gorilla WebSocket concurrency safety. *(Exception: `ReadPump` writes error notices to `client.send` directly, which can panic after eviction; §12, M9.)*
- [x] Strict **Persistence Before Broadcast**: Inbound messages are inserted into PostgreSQL before dispatching to `hub.Broadcast`.
- [x] WebSocket handler authenticates connections via OIDC token and sends last 50 historical messages upon connection.
- [x] Unit test suite (`hub_test.go`, `hub_safety_test.go`) validates Hub lifecycle, registration, unregistration, broadcasting, eviction safety, shutdown and push payloads. (Was broken by `a709637`; fixed in 1.0.1.)
- [x] `ChatMessage` model with JSON wire protocol parsing, time formatting (`intl`), and `isFromSelf` matching.
- [x] `ChatBloc` state management with `flutter_bloc` managing message list, deduplication, and chronological sorting. *(`ChatState ==` compares lengths only; §12, M12.)*
- [x] `MessageBubble` dynamically styled (right-aligned primary for self, left-aligned for peers with `@username`, centered pills for system events).
- [x] `ChatInputBar` responsive keyboard handling with `SafeArea` and `MediaQuery.viewInsetsOf`.
- [x] Timeline auto-scrolling with `ScrollController` on message receipt and submission.
- [x] `ResponsiveLayout` responsive wrapper with 800.0 logical pixel breakpoint distinguishing desktop dual-pane from mobile drawer.
- [x] `Sidebar` desktop navigation widget displaying the lounge, active online members, and user profile footer (1.0.1: mock rooms and fake unread counts removed).
- [x] `HiveLocalMessageRepository` cross-platform offline message cache using `hive_flutter` with fallback in-memory repository.
- [x] Cache-to-live handoff in `ChatBloc` delivering instant offline launch before live WebSocket hydration. (1.0.1: nothing network-bound before `runApp`; offline refresh failures keep the session.)
- [x] Version string bumped to `1.0.0+1` in `client/pubspec.yaml`.
- [x] Application uniformly named "meowGram" across Android, iOS, macOS, Windows, and Web.
- [x] Launcher icons generated across all OS platforms using `flutter_launcher_icons`.
- [x] Automated build scripts created: `deploy/scripts/build_all.ps1` and `deploy/scripts/build_all.sh`.
- [x] Production `--dart-define` parameters with secure schemes, sourced from the gitignored `config/production.local.json` (tracked `production.json` = placeholders).
- [x] Android release signing configured with `key.properties` and keystore documentation provided.
- [x] Backend Docker image builds (`go build ./...` verified 2026-10-07; image size not re-measured).
- [x] Manual Xcode code signing steps documented for iOS/macOS App Store and TestFlight distribution.
- [x] Full client test suite passing: **53/53** (`flutter test`, Flutter 3.47.6, release 1.0.1). `flutter analyze`: **no issues**. `flutter build web --release`: succeeds (wasm dry run OK).
- [x] Go backend test suite passing under `go test -race -count=3 ./...`; `go vet` and `gofmt` clean.
- [x] Production Web build compiled successfully to `client/build/web`.
- [ ] Web client usable end-to-end. *Web OIDC login fixed in 1.0.1; the web bundle is still not served by any container (404).*
- [x] Go backend `OIDCVerifier` enforces signature, issuer, **mandatory audience and expiry** (1.0.1).
- [x] Token storage persisted via `flutter_secure_storage` immediately upon PKCE exchange; rehydrated on startup and app foregrounding (`AppLifecycleState.resumed`).
- [x] Authelia user profile extraction resolves `preferred_username` -> `name` -> `email prefix` -> `sub` to avoid raw UUID display in UI.
- [x] Mock presence data removed from `Sidebar`; Go `Hub` presence frame (`type: "presence"`) drives dynamic active roster in `ChatBloc`.
- [x] Traefik router labels in `deploy/docker-compose.yml` updated with path prefixes (`/ws`, `/api`, `/healthz`) and priority 100 to prevent shared-domain collision with frontend container.
- [x] macOS sandbox entitlements allow outgoing and incoming network (1.0.1; verify on a Mac).
- [x] Push notifications do not leak message content or sender to unauthenticated installs (1.0.1 content-free payload; activity itself is still visible to any subscriber until per-device tokens land).
- [x] Compose binds published ports to `127.0.0.1` and requires `POSTGRES_PASSWORD` (1.0.1).
- [ ] Database backups exist for roastery (AGENTS.md known gap).

---

## 12. Analysis by Claude (2026-10-07)

> Independent code review of the whole monorepo (server, client, deploy, docs), performed on branch `feat/push-notifications` at `417e30d` (= `main` at `7fd1b19` + PR #15) and re-checked against `origin/main` at `0162395`. The only change on `main` since then (`7267bdd`: OIDC errors propagated to the UI, login timeout 180 s → 600 s) does not affect these findings. Findings marked **verified** were reproduced by running code; the rest come from reading the source. **H1 and H2 exist only on the `feat/push-notifications` branch**; everything else applies to `main`. Line numbers refer to `417e30d` and may be off by a few lines on `main`.

### 12.1 Method & verification results

| Check | Environment | Result |
|---|---|---|
| `go build ./...` | Go 1.27.1 (toolchain per `go.mod`), Linux | ✅ builds |
| `go vet ./...` | same | ❌ `internal/chat/hub_test.go:15`: not enough arguments in call to `NewHub` |
| `go test -race -count=1 ./...` | same | ❌ `chat` [build failed]; ✅ `auth`, `config`, `handler`. With the one-line test fix, `chat` passes and **no data races** were reported. |
| `gofmt -l .` | same | 16 files flagged, all due to CRLF line endings (no `.gitattributes`) |
| `flutter test` | Flutter 3.47.6 stable, Windows | ✅ 40/40 |
| `flutter analyze` | same | 45 info-level (mostly `withOpacity` → `withValues` deprecations and unnecessary imports), 0 warnings or errors |
| Secrets in git | `git ls-files` | ✅ no `.env`, keystore or `key.properties` tracked; only public Firebase config |
| Probe: token with no `exp` and no `aud` | ad-hoc Go test against `OIDCVerifier` | ❌ **accepted** |
| Probe: `aud=meowgram-client` with strict `AUTHELIA_AUDIENCE` set | same | ❌ **accepted** |
| Probe: `ParseSyncTimestamp("2026-10-03 13:40:00")` | ad-hoc Go test | ❌ rejected (dead layout) |

### 12.2 What is solid

- **Persistence before broadcast** is implemented as specified: `ReadPump` inserts, then sends to `hub.Broadcast`, and a failed insert means nothing is broadcast.
- **Single reader and single writer per connection**, with write deadlines and pings in `WritePump` and read deadlines and a pong handler in `ReadPump`.
- **Non-blocking fan-out with eviction** at 256 frames.
- **PKCE** is correct: S256, a 64-character verifier from `Random.secure()`, and state checked on desktop and mobile. The app holds no client secret.
- **Tokens** live in `flutter_secure_storage`; nothing sensitive is logged server-side (the request logger records the path only, never the query string).
- **Container** is multi-stage, non-root (uid 10001) and static, with migrations embedded and a healthcheck.
- The client test suite is broad: sync, layout, UAT regressions and deep links.

### 12.3 Findings

Severity: **High** = security/privacy exposure, data loss, or a platform that doesn't work. **Medium** = correctness or robustness bug with real user impact. **Low** = hygiene.

#### High

| ID | Finding | Where | Fix direction |
|---|---|---|---|
| **H1** | Go test suite broken on the current branch, against AGENTS.md workflow rule 3 (verified). | `server/internal/chat/hub_test.go:15` | `NewHub(logger, nil)`; run `go test -race ./...` before every PR. |
| **H2** | Push notifications leak all chat content outside Authelia: topic subscribed before login and never unsubscribed, full text in the body, and topics have no access control. | `client/lib/src/push/fcm_service.dart:41`, `client/lib/main.dart:34-41`, `server/internal/chat/hub.go:113-129` | Per-device tokens registered via an authenticated endpoint plus data-only pushes. See §10.7. |
| **H3** | Token validation is permissive (verified): a missing `exp` means the token never expires; a missing `aud` is accepted unless `AUTHELIA_AUDIENCE` is set (compose never sets it); `meowgram`/`meowgram-client` are always accepted; ID tokens are accepted as bearer tokens (no `typ` check). | `server/internal/auth/oidc.go:104-114, 157-200` | Require `exp`; compare `aud` to one configured value only; remove the hardcoded list; pass `AUTHELIA_AUDIENCE` in compose; decide whether the client sends access tokens (Authelia must issue JWT access tokens) or ID tokens, and check `typ` accordingly. |
| **H4** | Access token rendered on screen: the status banner prints `Endpoint: $target`, where `target` is the WS URL including `?token=<JWT>`. | `client/lib/src/screens/chat_screen.dart:185-208` | Show the URL without its query, or remove the banner from release builds. Long term, move the token out of the URL (AGENTS.md gap). |
| **H5** | Catch-up sync can leave permanent gaps: racy cursor, `system` items count as newest, one page only, silent failures. | `client/lib/src/services/sync_service.dart:71-129`, `client/lib/src/storage/local_message_repository.dart:65-69` | See §7.1 "Fix direction". |
| **H6** | Offline-first is broken by auth and push setup: `runApp` waits on `authController.initialize()` (refresh POST with **no timeout**) and on Firebase permission and subscription; any refresh failure, including offline, calls `logout()` and wipes tokens; timer and resume refreshes can race and reuse a rotated refresh token. | `client/lib/main.dart:27-46`, `client/lib/src/auth/auth_controller.dart:84-93, 169-186, 217-230`, `client/lib/src/auth/oidc_service.dart:76-125` | Start the UI first and init in the background; add timeouts; log out only on `invalid_grant`/401; make refresh single-flight (share one in-flight `Future`). |
| **H7** | Web login cannot complete (`Uri.base` read before the browser opens; verifier lost on the redirect reload; state check optional). Combined with "web build not served", the web client is unusable. | `client/lib/src/auth/oidc_platform_web.dart:13-39`, `client/lib/src/auth/auth_controller.dart:104-166` | `sessionStorage` for verifier and state, same-tab redirect, exchange on startup, mandatory state; serve `build/web` (separate container or Go `FileServer`). |
| **H8** | macOS sandbox entitlements are missing `network.client` (Debug and Release) and `network.server` (Release), so the app can't reach the backend or Authelia and the release build can't bind the 8088 login listener. | `client/macos/Runner/*.entitlements` | Add both keys and verify on a Mac (couldn't be run in this review). |

#### Medium

| ID | Finding | Where | Fix direction |
|---|---|---|---|
| **M9** | Possible server crash: after the hub evicts a slow client (`close(client.send)`), `ReadPump` can still send an error notice into that closed channel. A `select` does not protect against a closed channel, so it panics and takes down the process. Separately, shutdown order is reversed: `hubCancel()` runs before `srv.Shutdown`, so new upgrades block forever on `Register` and `ReadPump`'s deferred `Unregister` never returns. | `server/internal/chat/client.go:140-150`, `server/cmd/server/main.go:162-171` | Route all client-bound frames through the hub (or guard with a `closed` flag the hub owns); call `srv.Shutdown` first, then cancel the hub. |
| **M10** | No automatic WebSocket reconnect: after a drop, status stays `disconnected` until the user taps refresh. `connect()` also overwrites `_subscription` without cancelling it. | `client/lib/src/services/chat_websocket_service.dart:46-105` | Exponential-backoff reconnect with a fresh token; reconnect on app resume; cancel the old subscription and channel. |
| **M11** | `ChatInitializeRequested` is dispatched twice, and again on every 800 px breakpoint crossing; Bloc runs these concurrently. | `client/lib/src/screens/responsive_layout.dart:58-60`, `client/lib/src/screens/chat_screen.dart:49-51` | Initialize only in `ResponsiveLayout`; use a `droppable`/`restartable` transformer. |
| **M12** | `ChatState ==` compares list lengths only, so Bloc drops same-length updates (roster swaps, username changes, future edits). | `client/lib/src/bloc/chat_bloc.dart:112-123` | Use `listEquals` or `Equatable` with full lists. |
| **M13** | Hive cache is unbounded, includes system/error notices, is fully decoded and sorted on every read, and is **not cleared on logout** (a privacy issue on shared devices and web). | `client/lib/src/storage/local_message_repository.dart`, `client/lib/src/auth/auth_controller.dart:189-202` | Cache only `chat` items with an `id`; cap (e.g. last 1000); keep a stored cursor; clear on logout. |
| **M14** | Client ID mismatch: `meowgram` (AGENTS.md, server) vs `meowgram-client` (client, configs, scripts), masked by the hardcoded audience list. | `server/internal/config/config.go:129-131`, `client/lib/src/config/app_config.dart:57-60`, `client/config/*.json`, `deploy/scripts/build_all.*` | Standardize on `meowgram` per AGENTS.md and update the Authelia client in `purrbrews-containers`. |
| **M15** | Real domains in tracked files (`client/config/production.json`, both `.env.example` files, `client/.env.example`, `build_all.*` defaults, test fixtures, `config.go:172` and `app_config.dart:282` Immich defaults). | various | Placeholders in tracked files; real values in gitignored env/define files. |
| **M16** | Compose: Postgres and server published on all interfaces with a default DB password; `AUTHELIA_AUDIENCE`, `AUTHELIA_CLIENT_ID`, `AUTHELIA_DOMAIN` and `GOOGLE_APPLICATION_CREDENTIALS` not passed; the dev Traefik profile exposes an insecure dashboard plus the Docker socket. | `deploy/docker-compose.yml` | `127.0.0.1:` bindings, `${POSTGRES_PASSWORD:?}`, pass the auth vars, mount the FCM secret read-only. |
| **M17** | Input limits: 512 KB WS frames with no app-level text cap (also breaks FCM's 4 KB limit); `preferred_username` longer than 100 characters overflows `VARCHAR(100)`, so login fails with 500; no rate limiting. | `server/internal/chat/client.go:27`, `server/migrations/000001_*.up.sql:9` | Cap text (e.g. 4,000 characters) with an error frame; truncate usernames; simple per-client rate limit. |
| **M18** | `IMMICH_API_KEY` is designed as a client `--dart-define`, so the key would ship in every binary and in the web JavaScript. | `client/lib/src/config/app_config.dart:302-307` | Server-side Immich proxy for Release 2. |

#### Low

- `ParseSyncTimestamp`: the space-separated layout is unreachable (verified). (`server/internal/handler/sync.go:126-141`)
- Duplicate unique index on `users.authelia_sub` (migration 001 index plus migration 002 constraint).
- Desktop loopback page interpolates `error` into HTML unescaped; a stray `/callback` request with a wrong state aborts the login. (`oidc_platform_io.dart:157-170, 252-266`)
- The OIDC `nonce` is sent but never validated; ID-token claims are decoded without verification on the client. This is acceptable over TLS with PKCE, but drop the nonce or validate it.
- `dotenv.go` loads `../.env` before `deploy/.env`, and only the first file found, so a bare-metal run from `server/` ignores `deploy/.env`.
- Dockerfile: `golang:alpine` unpinned; `-extldflags '-static'` is a no-op with `CGO_ENABLED=0`.
- Presence frames broadcast every member's Authelia `sub`; the app bar shows the user's own `sub`.
- The double-tick (`done_all`) icon implies read receipts that don't exist.
- `client/android/gradlew` and `gradlew.bat` are gitignored; they are normally committed.
- `OIDC_SCOPES` env var is unused; `golang-migrate` pulls in `lib/pq` alongside `pgx` (could use the `pgx/v5` migrate driver).
- No `.gitattributes`: add `* text=auto eol=lf` (at least for `*.go`, `*.sh`, `*.sql`) so `gofmt` and shell scripts behave.
- 45 `flutter analyze` infos (`withOpacity` deprecations, unnecessary imports).

### 12.4 Status of AGENTS.md §4 "Known gaps"

| Gap | Status on 2026-10-07 |
|---|---|
| Audience not checked (`SkipClientIDCheck`) | **Description outdated.** `SkipClientIDCheck` is gone, but the replacement check is permissive (H3). Update AGENTS.md wording. |
| Hardcoded production values in `build_all.*` and runbook | Runbook fixed in this revision (placeholders). Still open in `build_all.*`, `client/config/production.json` and `.env.example` files (M15). |
| Dev defaults in compose | **Open** (M16). |
| Token in WebSocket URL | **Open**, and worse than described: the URL is shown on screen (H4). |
| Mobile sign-in | **Partly addressed**: `meowgram://callback` custom scheme on iOS/Android (§10.4.2). An App Link / Universal Link is still preferable, and the scheme must be registered in Authelia. |
| Web build not served | **Open**; web login is also broken (H7). |
| No backups | **Open**. |

### 12.5 Documentation drift

- **`roadmap.html`** references files that don't exist (`deploy/traefik/`, `deploy/authelia/configuration.yml`, `server/internal/auth/verifier.go`, `server/internal/hub/*`, `client/lib/src/services/auth_service.dart`, `client/lib/src/blocs/`). It also claims "dynamic port allocation" for the loopback (actually fixed at 8088), "mutex synchronization" in the hub (AGENTS.md explicitly forbids it), Let's Encrypt and HTTP→HTTPS in compose (absent), macOS network-client entitlements (absent), and "22/22 tests" (actually 40 client tests, with the Go suite broken). Treat it as aspirational until it's regenerated from the code.
- **`README.md`** still describes the echo-server phase (AGENTS.md acknowledges this; it should be rewritten or reduced to a pointer here).
- **This runbook** previously claimed tests were passing, `Runner.entitlements`, `AppConfig.initialize()`/runtime `.env` loading, a 512-byte message limit, a schema snippet that didn't match the migration, gitignored FCM credentials, compose injection of FCM credentials, and a registry-push rollout. All of these are corrected above.

### 12.6 Recommended order of work at release 1.0.1

This is the plan recorded before review hardening; items addressed by the current
branch are summarized in §12.10.

Items 1–6 of the original plan were completed in release 1.0.1 (§12.9). Remaining, each on its own branch per AGENTS.md §1:

1. `feat/fcm-device-tokens`: per-device push tokens via an authenticated endpoint, with data-only pings (replaces topic pushes).
2. `feat/ws-ticket-auth`: move the access token out of the WebSocket URL (`Sec-WebSocket-Protocol` or a short-lived ticket).
3. `feat/serve-web`: serve `client/build/web` (separate container or Go `FileServer`) so `https://meow.<domain>/` works.
4. `chore/db-backups`: scheduled `pg_dump` for roastery (AGENTS.md gap).
5. Owner decision on the client ID (`meowgram` vs `meowgram-client`), then align AGENTS.md, Authelia and the defaults.
6. Low-priority items in §12.9 (rate limiting, `.gitattributes`, Docker base-image pin, duplicate index, `gradlew`).

### 12.7 Secrets scan (2026-10-07)

**Scope.**
- [gitleaks](https://github.com/gitleaks/gitleaks) (latest Docker image), run with the repo mounted **read-only** and `--redact`; no secret values were printed.
- `gitleaks git --log-opts=--all`: every commit on all 21 local branches (including unpushed ones) and the fetched remote branches.
- `gitleaks dir`: the full working tree, including gitignored files.
- `git log --all --diff-filter=A`: checked for sensitive file types ever added to history.

**Result: no real secret has ever been committed.**

| Finding | Location | Verdict |
|---|---|---|
| 6 × `gcp-api-key` | `client/lib/firebase_options.dart` (5), `client/android/app/google-services.json` (1), since `72e3115` (PR #15 only) | Firebase **client** keys, public by design. Restrict them in Google Cloud console: Android → package + SHA-1, iOS → bundle ID, Web → HTTP referrer, all → Firebase APIs only. They are also what lets any install subscribe to the `room_lounge` topic (H2). |
| 1 × `jwt` | `client/test/widget_test.dart` | False positive: fake test token (`sub` + `preferred_username` only, signature `dummySignature`). |
| `POSTGRES_PASSWORD` | `deploy/.env` (gitignored, never committed) | Expected location. Strong (64 chars), not the dev default; `deploy/.env` also binds ports to `127.0.0.1`. |
| Copies of the above | `client/build/test_cache/*.dill` | Build output, gitignored; removed by `flutter clean`. |
| Dev password `meowgram_secret_dev_change_in_production` | `.env.example` files, compose defaults | Placeholder, not live; still replace the compose fallback with `${POSTGRES_PASSWORD:?}` (M16). |
| Sensitive files ever added | — | None. Only `*.example` templates have ever been committed. |

**Gap found:** the root `.gitignore` did not cover the FCM service-account JSON or APNs `.p8` keys that PR #15 requires (keystores and `key.properties` are covered by `client/android/.gitignore`). Fixed on `chore/secrets-hygiene`, which also adds a `.gitleaksignore` for the 7 reviewed findings so future scans only show new hits. **Recommended:** enable GitHub secret scanning with push protection on `purrMonster/meowGram`.

### 12.8 Branch state

All six branches listed in the original review (`feat/push-notifications`, `docs/adr-updates`, `feat/auto-login`, `feat/background-sync`, `feat/notifications`; `fix/traefik-ws-routing` was never pushed) were merged into `main` on 2026-10-07 as PRs #15–#19. That merge left `client/lib/main.dart` uncompilable (an unclosed `catch` from combining the FCM and background-sync startup code). Release 1.0.1 starts from that `main` (`5370cf9`) and fixes it; the assessments of those branches are addressed in §12.9.

The local-only `fix/traefik-ws-routing` branch is superseded by PR #13 and should be deleted.

### 12.9 Release 1.0.1 resolution

Verification for 1.0.1:
- **Server**: `go vet` clean; `gofmt` clean; `go test -race -count=3 ./...` passes.
- **Client**: `flutter analyze` reports no issues; `flutter test` passes 54/54; `flutter build web --release` succeeds.
- **Secrets**: gitleaks over all refs and the working tree reports no leaks (with the `.gitleaksignore` baseline).
- **Compose**: `docker compose config` validates and refuses to run without `POSTGRES_PASSWORD`.

| ID | Status | Resolution |
|---|---|---|
| H1 Go tests don't compile | ✅ Fixed | `NewHub(logger, publisher)`; tests updated, plus new hub safety tests |
| H2 Push leaks chat content | ✅ Fixed (interim) | Content-free, collapsed, throttled pushes; subscribe only while signed in; legacy subscriptions removed. **Open:** per-device tokens |
| H3 Permissive token validation | ✅ Fixed | `exp` + `aud` mandatory; strict `AUTHELIA_AUDIENCE`; no hardcoded audiences; default client ID `meowgram-client` |
| H4 Token rendered on screen | ✅ Fixed | WS URL redacted everywhere it is shown. **Open:** token still travels in the WS query string (AGENTS gap) |
| H5 Sync gaps | ✅ Fixed | Cursor snapshot before connect/after drop, paging, retry, `X-Has-More` |
| H6 Offline-first broken by auth | ✅ Fixed | Local-only startup; single-flight refresh; logout only on 400/401; timeouts |
| H7 Web login | ✅ Fixed | `sessionStorage` PKCE + same-tab redirect + exchange on startup. **Open:** web bundle not served |
| H8 macOS entitlements | ✅ Fixed | `network.client`/`network.server`. **Verify on a Mac** (incl. Keychain) |
| M9 Hub panic / shutdown deadlock | ✅ Fixed | Only the Hub goroutine sends to/closes client channels; non-blocking ops after `Done()`; listener stops first |
| M10 No auto-reconnect | ✅ Fixed | Backoff 1–30 s with fresh token; reconnect on resume and token refresh |
| M11 Double init | ✅ Fixed | Idempotent `ChatInitializeRequested` |
| M12 `ChatState` equality | ✅ Fixed | Compares contents (`listEquals`) |
| M13 Cache hygiene / logout | ✅ Fixed | Chat-only, capped 2,000, purged legacy entries, cleared on logout |
| M14 Client ID mismatch | ⚠️ Partly | Server default aligned with the deployed `meowgram-client`; AGENTS.md says `meowgram`. **Owner decision** |
| M15 Real domains in tracked files | ✅ Fixed (tip) | Placeholders + gitignored `*.local.json`. History still contains them |
| M16 Compose defaults | ✅ Fixed | Required password, loopback ports, auth/FCM vars, secrets mount |
| M17 Input limits | ✅ Fixed | 4,000-character messages; usernames truncated to 100. **Open:** rate limiting |
| M18 Client-side Immich key | ✅ Fixed | Define removed; must be a server-side proxy |
| Merge: `main.dart` doesn't compile | ✅ Fixed | Startup rewritten (local-only before `runApp`) |
| Merge: notification spam | ✅ Fixed | Live chat from others, background only |
| Merge: background sync unsafe | ✅ Fixed | Opt-in, Android only, never touches Hive in the background isolate |
| Merge: failing deep-link test | ✅ Fixed | Test updated to the intended error propagation (`7267bdd`) |
| Low: timestamp layout, HTML escaping, stray callbacks, `withOpacity`, mock rooms, config test isolation | ✅ Fixed | |
| Low: duplicate unique index, nonce not validated, `gradlew` gitignored, CRLF (`.gitattributes`), `golang:alpine` unpinned | Open | Low risk; left out of a stability release on purpose |

**Open at release 1.0.1**: token in the WebSocket URL; web bundle not served; no database backups; per-device push tokens; rate limiting; the `google-services` Gradle plugin `4.3.15` alongside AGP `9.1.0`. The current branch's status is in §12.10.

### 12.10 Review hardening

The changes below are prepared on `fix/review-hardening` and have not been
deployed. They address the findings from the 2026-10-10 whole-codebase review.

| Finding | Change in this branch | Remaining deployment action |
|---|---|---|
| Bearer token in WebSocket URL | `POST /api/ws-ticket` verifies the bearer token and issues a random, hashed-in-memory ticket that is one-use and expires after 30 seconds. The WebSocket consumes the ticket; the client redacts auth query values from displayed URLs. | Rebuild both client and server together. Existing access-token WebSocket URLs stop working. |
| ID tokens accepted as API credentials or loose token matching | `AUTHELIA_AUDIENCE` is required at startup and is the only accepted audience. Issuer and audience claims are compared exactly (apart from a trailing issuer slash); alternate issuer schemes, domain aliases, and query-string bearer tokens are rejected. | Configure Authelia to sign JWT access tokens for the app API audience. Authelia defaults to opaque access tokens, which this JWKS verifier cannot validate; see [Authelia OIDC integration](https://www.authelia.com/integration/openid-connect/introduction/). |
| Sync gaps at page boundaries and large catch-up windows | Sync cursor is `(created_at, id)`, and both foreground and Android background sync continue through full pages. | Existing background timestamp-only cursors are accepted and may replay same-time messages once; UUID deduplication makes that safe. |
| Web frontend had no deployment route | Compose builds Flutter web assets from the repo, serves them with unprivileged Nginx, and routes the host catch-all to the frontend while backend paths retain priority. Local web bind defaults to `127.0.0.1:8081`. | Rebuild and start the `web` service. Configure production `APP_DOMAIN`, `API_BASE_URL`, `WS_BASE_URL`, and Authelia issuer in the ignored deploy environment. |
| No scheduled database dump | Compose starts a daily `pg_dump` service with owner-only files and 14-day local retention by default. Restore instructions are in `deploy/backups/README.md`. | Backups share the deployment host; configure off-host copying and verify restores before relying on disaster recovery. |
| Development proxy exposed insecure dashboard/socket | Optional development Traefik now uses static file routing, binds to loopback, disables the dashboard, and has no Docker socket mount. | None for the dev profile. Production continues to use the externally managed reverse proxy. |
| CORS implicitly allowed the app hostname over HTTP | Origin checks now accept only configured origins; default production/staging origins use HTTPS, while development defaults include localhost ports used by the web client. | Set `CORS_ORIGINS` explicitly for every deployed web origin. |
| Empty environment values could be overwritten by ignored `.env` files | Dotenv loading now respects environment-variable presence, even when the value is explicitly empty; this also keeps config tests isolated from local secrets. | None. |
| Manual reconnect could be ignored while connected or start duplicate handshakes | Reconnect now updates the current token/endpoint and starts exactly one new ticket and socket attempt; stream errors also schedule recovery. A concurrent sync no longer clears its pending cursor. | None. |
| PowerShell build could claim success after failure | The packaging script checks every Flutter/Docker native exit code and stops on error. | None. |
| Client ID mismatch | Client, server, examples, and build defaults now use `meowgram`, as specified in AGENTS.md §2. | Update the Authelia client registration in `purrbrews-containers` before deploying; this branch does not alter that repo or the live deployment. |

The original review-hardening commit was prepared without running the Go and
Flutter suites. Follow-up verification for this branch is recorded in §12.11.
Both suites must pass before requesting a merge, per AGENTS.md §1.3. No live
deployment or external Authelia configuration was changed.

### 12.11 Full codebase re-audit

Follow-up refactors on `refactor/full-codebase-audit`:

| Finding | Change | Remaining action |
|---|---|---|
| OIDC issuer aliases and audience normalization accepted values other than the configured claims | Issuer matching now requires the configured issuer (trailing slash normalized); audience matching is exact and case-sensitive. Future `iat` values outside clock skew are rejected. | Configure Authelia with the exact issuer and API audience values. |
| HTTP auth still accepted access tokens in a query parameter | Protected HTTP routes now accept bearer tokens only in `Authorization`; URL query tokens no longer authenticate. | None. |
| CORS automatically allowed the app hostname over HTTP, including in production | Only explicitly configured origins match. Production/staging defaults are HTTPS; Compose leaves an empty allow-list for environment-aware defaults. | Set `CORS_ORIGINS` explicitly for deployed origins. |
| Empty environment variables were overwritten from ignored dotenv files | Dotenv loading now respects variables present in the process even when empty, making environment precedence predictable and config tests isolated. | None. |
| Manual reconnect could be ignored or start duplicate ticket handshakes | Reconnect configures token and endpoint before starting one forced reconnect; stream errors schedule recovery. Concurrent sync attempts retain the pending cursor. | None. |
| Sync SQL relied on implicit parameter type inference for an empty UUID cursor | The cursor parameter is explicitly treated as text and converted with `NULLIF` before UUID comparison. | None. |
| Local OIDC issuer detection matched hostnames containing the word `localhost` | Development HTTP is now selected only for an exact loopback hostname/IP; lookalike public hostnames default to HTTPS in both client and server config. | Use an explicit issuer URL when Authelia is behind a nonstandard local proxy. |
| User identity uniqueness had two equivalent indexes | Migration `000003` removes the standalone index while preserving the unique constraint and all rows; its down migration recreates the index. The Go builder image now pins its toolchain to 1.27.1. | The additive migration runs on normal server startup; no manual data operation is required. |

Remaining gaps confirmed during the re-audit and still tracked in AGENTS.md/runbook: per-device FCM tokens (topic subscriptions remain public), OIDC nonce persistence and validation in the client, App Links / Universal Links registration with Authelia, off-host backup and restore drills, API rate limiting, and Android Google Services plugin compatibility. These need separate implementation or deployment work; no external identity configuration was changed here.

Verification attempted for this audit: `go test ./...` and `flutter test` could
not start because Go and Flutter are not installed or available on PATH in the
review environment. Docker Compose configuration validation, PowerShell script
parsing, and `git diff --check` passed. Shell syntax parsing could not run
because the available Bash launcher was denied by the environment. The graph
utility launcher also failed before extraction; the audit continued through
direct source inspection. No deployment or external Authelia configuration was
changed.

### 12.12 Reproducible scratch verification (2026-10-11)

`test/scratch-verification` includes the two previous hardening commits and adds
`deploy/verify.compose.yml`, PowerShell/POSIX entry scripts and GitHub Actions.
The stack has an internal, disposable PostgreSQL database with no published ports;
trust authentication is confined to this scratch network. It never loads deploy
credentials or mounts a live data volume. Go 1.27.1 and Flutter 3.44.4 run in
containers; the Flutter lockfile now matches that pinned SDK. The wrapper scripts
clean up their own project and volumes on exit. `.gitattributes` enforces LF for
shell scripts and source files.

Real execution found and repaired two failures missed in the earlier audit:
- Background sync dereferenced a nullable cursor and did not compile.
- Migration 000003 could not drop an index referenced by the messages foreign key.
  It now recreates that foreign key atomically against the remaining unique
  constraint, validates it, and preserves all rows. This migration was exercised
  only in the disposable database; live migration execution is not authorized.

Regression coverage includes ticket identity isolation, expiry, concurrent
single-use consumption, endpoint authentication/cache headers, a real loopback
WebSocket ticket/reconnect exchange and PostgreSQL pagination across 1001 messages
sharing one timestamp. A dump is restored into a second scratch database and
checked against a probe plus message count. This validates the restore mechanism;
it does not establish off-host storage or production recovery readiness.

Results: Go tests with race detection and Go vet passed; PostgreSQL pagination
and migration tests passed; dump/restore drill passed; Flutter analysis reported
no issues; all 55 Flutter tests passed; release web build passed. The web build
reports an existing optional Cupertino font warning. Native OS builds and live
service configuration are tracked separately.

### 12.13 OIDC login binding

`fix/oidc-nonce` persists the random nonce alongside PKCE state in web session
storage and requires it for native and web code exchanges. Before saving a new
session, the client verifies the ID token's RS256 signature against configured
JWKS, exact issuer, client audience/authorized party, subject, expiry, issuance
time, optional not-before and nonce. Unsigned, malformed, forged and mismatched
responses fail closed with a generic error that cannot include token contents.
The implementation follows OpenID Connect Core §3.1.3.7:
https://openid.net/specs/openid-connect-core-1_0.html#IDTokenValidation

No endpoint or credential is added. Existing AUTHELIA_JWKS_URL must be reachable
from the client; for web this also requires the provider's CORS policy to permit
the app origin. Old in-flight web sign-ins without a saved nonce must restart.
Tests generate ephemeral RSA keys and emulate token/JWKS responses, requiring no
live provider. Flutter analysis is clean and all 66 tests pass.
Release web build also passed with the nonce and signature checks enabled.
