# meowGram Engineering Runbook & Architectural Decision Record (ADR)

> **Document Version**: 1.3.0  
> **Status**: APPROVED (Epic 1.2 & UST-1.2.3 Client OIDC PKCE Complete)  
> **Author**: Lead Developer / Antigravity IDE  
> **Last Updated**: 2026-10-03  

---

## 1. Executive Summary & TL;DR

meowGram is a cross-platform realtime cat-themed chat lounge organized as a monorepo consisting of a Go backend service, a Flutter cross-platform client (Web, Desktop, Mobile), and Docker Compose orchestration infrastructure.

### Architectural Mandate Updates
- **Backend OIDC Pivot (Epic 1.2)**: All custom local authentication, password storage, user registration, and local JWT issuance are completely eliminated in favor of **Authelia OpenID Connect (OIDC)**.
- **Client OIDC PKCE Integration (UST-1.2.3)**: The Flutter client implements the OAuth 2.0 Authorization Code Flow with Proof Key for Code Exchange (PKCE, RFC 7636). The client is purely public and stores **zero client secrets**.
- **Auto-Provisioning**: On first successful authenticated connection, the user's `sub` and username are atomically recorded in the local PostgreSQL `users` table.

---

## 2. Architectural Decisions & Design Rationale (ADR)

| Decision | Selected Technology / Pattern | Rationale & Alternatives Considered |
|---|---|---|
| **Client Auth Flow** | OAuth 2.0 Authorization Code with PKCE (RFC 7636) | Required for public clients where credentials cannot be embedded securely. Replaces legacy implicit flows with cryptographically bound `code_challenge` (S256). |
| **Desktop Redirect URI** | RFC 8252 Loopback HTTP (`http://127.0.0.1:8088/callback`) | Standard for native desktop apps on Windows/macOS/Linux. Eliminates OS custom URI scheme registry requirements during dev/testing. |
| **Web Redirect URI** | Host Origin (`Uri.base.origin`) | Seamless in-browser redirect on Web. URL inspection captures `code` and `state` parameters without local socket binding. |
| **Token Transport to WS** | URL Query Parameter (`?token={access_token}`) | Standard web browsers do not allow arbitrary HTTP headers (such as `Authorization`) during WebSocket handshake (`new WebSocket(...)`). |
| **Auth State Management** | `AuthController` with `refreshListenable` GoRouter | Declarative route protection. Automatically redirects `/login` -> `/chat` on token acquisition and `/chat` -> `/login` on expiry/logout. |
| **Token Refresh Lifecycle** | Proactive Background Timer (`expiresAt - 60s`) | Silently exchanges `refresh_token` for a fresh `access_token` prior to expiration, preventing WebSocket disconnects during active chat. |
| **Backend OIDC Verifier** | `coreos/go-oidc/v3` with Remote KeySet | Cryptographically verifies incoming Bearer JWTs against Authelia's JWKS endpoint without handling user credentials. |
| **Database Migrations** | `golang-migrate/migrate/v4` | Automated `.up.sql` migrations executed on server container initialization. |

---

## 3. Cross-Platform Redirect URI Mechanics & Tradeoffs

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

## 4. Environment & Compile-Time Configuration Contract

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

## 5. Operations & Developer Playbook

### 5.1 Starting the Infrastructure (PostgreSQL + Go Backend)

```powershell
cd c:\Users\jyotirmoyc\Desktop\Projects\meowGram

# 1. Initialize environment file if not present
if (!(Test-Path deploy/.env)) { Copy-Item deploy/.env.example deploy/.env }

# 2. Build and launch services in detached mode
docker compose -f deploy/docker-compose.yml up -d --build

# 3. Verify health status
curl http://localhost:8080/healthz
```

### 5.2 Launching the Flutter Client with OIDC PKCE

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

### 5.3 Automated Testing

```powershell
# Run Flutter client unit and widget test suite
cd c:\Users\jyotirmoyc\Desktop\Projects\meowGram\client
flutter test

# Run Go backend test suite
cd c:\Users\jyotirmoyc\Desktop\Projects\meowGram\server
go test -v ./...
```

---

## 6. Review Gate & Verification Checklist

- [x] Flutter client generates RFC 7636 compliant PKCE code verifier and S256 challenge.
- [x] Client contains zero client secrets (public client model).
- [x] OIDC state controller manages session tokens, claims parsing, and auto-refresh timers.
- [x] Cross-platform redirect architecture handles Web origin inspection and Desktop loopback server.
- [x] Login screen updated with single "Login with purrBrews" button and OIDC contract card.
- [x] GoRouter declarative route guards enforce redirect between `/login` and `/chat`.
- [x] `ChatWebSocketService` injects verified Bearer token into `?token={token}` query parameter.
- [x] Automated test suite passing with 100% test success rate.
- [x] Flutter Web production build verified with `flutter build web`.
