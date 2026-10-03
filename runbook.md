# meowGram Engineering Runbook & Architectural Decision Record (ADR)

> **Document Version**: 1.2.0  
> **Status**: APPROVED (Epic 1.2 OIDC & PostgreSQL Scaffold Complete)  
> **Author**: Lead Developer / Antigravity IDE  
> **Last Updated**: 2026-10-03  

---

## 1. Executive Summary & TL;DR

meowGram is a cross-platform realtime cat-themed chat lounge organized as a monorepo consisting of a Go backend service, a Flutter cross-platform client (Web, Desktop, Mobile), and Docker Compose orchestration infrastructure.

### Architectural Mandate Update (Epic 1.2: Identity & Authentication)
- **Pivot Decision**: All custom local authentication, password storage, user registration, and local JWT issuance are completely eliminated in favor of **Authelia OpenID Connect (OIDC)**.
- **Zero Credentials Backend**: The Go backend does not store hashes, passwords, or manage registration flows. It solely validates signed JWTs against Authelia's JWKS endpoint.
- **Auto-Provisioning**: On first successful authenticated connection, the user's `sub` (and preferred username) is atomically recorded in the local PostgreSQL `users` table.

---

## 2. Architectural Decisions & Design Rationale (ADR)

| Decision | Selected Technology / Pattern | Rationale & Alternatives Considered |
|---|---|---|
| **Identity & Authentication** | Authelia OIDC (`coreos/go-oidc/v3`) | Offloads 100% of password management, 2FA/MFA, and user registration to Authelia. Backend only performs cryptographic verification against Authelia's JWKS. |
| **Token Transport** | Dual Bearer Header + `?token=` Query Param | Standard HTTP endpoints use `Authorization: Bearer <token>`. WebSockets cannot send custom HTTP headers during browser handshake, so `?token=<jwt>` is supported for `/ws`. |
| **User Persistence & Auto-Provisioning** | PostgreSQL + Atomic `ON CONFLICT (authelia_sub)` | Synchronizes external identity without registration friction. Atomic insert prevents race conditions on concurrent first connections. |
| **Database Migrations** | `golang-migrate/migrate/v4` | Declarative, versioned `.up.sql` and `.down.sql` migrations executed automatically on server startup. |
| **Database Driver** | `jackc/pgx/v5` via standard `database/sql` | Modern, high-performance PostgreSQL driver with robust connection pooling. |
| **Repository Pattern** | Monorepo (`/client`, `/server`, `/deploy`) | Atomic commits across backend protocols, database migrations, and client shells. |
| **Domain & Host Binding** | Zero Hardcoded Domains (`.env` + `--dart-define`) | Eliminates hardcoded URLs. Enables seamless deployment across local dev, staging, and prod. |
| **Container Topology** | Docker Compose with PostgreSQL & Traefik | Isolated network (`meowgram-net`), persistent database volume (`postgres_data`), and declarative healthchecks. |

---

## 3. Database Schema & Migrations (`server/migrations`)

### `000001_create_users_table.up.sql`
```sql
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    authelia_sub VARCHAR(255) NOT NULL,
    username VARCHAR(100) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_users_authelia_sub ON users (authelia_sub);
```

### Key Design Notes:
- **No Password Fields**: Credentials never touch the application database.
- **UUID Primary Key**: Internal identifiers use UUIDv4 (`gen_random_uuid()`).
- **Unique `authelia_sub` Index**: Ensures strict 1:1 mapping with Authelia identity subjects.

---

## 4. OIDC Middleware & WebSocket Lifecycle

```
[Client (Flutter / Web / Mobile)]
   │
   ├── 1. HTTP Request: Authorization: Bearer <jwt>  OR
   │   WebSocket Handshake: GET /ws?token=<jwt>
   │
   ▼
[OIDC Middleware (auth.Middleware)]
   │
   ├── 2. Extract Token (Header or Query Param)
   ├── 3. Verify Signature against Authelia JWKS (RemoteKeySet)
   ├── 4. Validate Claims (iss == AUTHELIA_ISSUER, exp, sub)
   │
   ▼
[Auto-Provisioning (UserRepository.GetOrCreateBySub)]
   │
   ├── 5. Query PostgreSQL: SELECT id, username WHERE authelia_sub = $1
   └── 6. If Not Found: INSERT INTO users (authelia_sub, username) ...
   │
   ▼
[Request Context Injection]
   │
   ├── 7. ctx = context.WithValue(ctx, UserContextKey, user)
   └── 8. ctx = context.WithValue(ctx, SubContextKey, sub)
   │
   ▼
[EchoWebSocketHandler (/ws)]
   │
   └── 9. Read user identity from context, log with user_id, stream echo messages
```

---

## 5. Environment Variables Contract (`deploy/.env.example`)

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
| `HOST_POSTGRES_PORT`| `5432` | `5432` | Host port exposed for PostgreSQL. |
| `DATABASE_URL` | `postgres://...` | `postgres://...` | Full connection string for Go backend. |
| `AUTHELIA_ISSUER` | `http://localhost:9091` | `https://auth.example.com` | Base Issuer URL for Authelia OIDC provider. |
| `AUTHELIA_JWKS_URL` | `http://localhost:9091/jwks.json` | `https://auth.example.com/jwks.json` | URL for Authelia cryptographic public keys (JWKS). |
| `CORS_ORIGINS` | `http://localhost:8080,...` | `https://meowgram.chat` | Allowed browser origins for CORS preflight and WebSocket handshake. |

---

## 6. Operations & Developer Playbook

### 6.1 Starting the Complete Stack (PostgreSQL + Server)

```powershell
cd c:\Users\jyotirmoyc\Desktop\Projects\meowGram

# 1. Initialize environment file if not present
if (!(Test-Path deploy/.env)) { Copy-Item deploy/.env.example deploy/.env }

# 2. Build and launch services in detached mode
docker compose -f deploy/docker-compose.yml up -d --build

# 3. Verify PostgreSQL and Server health
docker compose -f deploy/docker-compose.yml ps

# 4. View database migrations and server startup logs
docker compose -f deploy/docker-compose.yml logs server
```

### 6.2 Running Automated Tests & Verification

```powershell
# Go backend tests & compilation check
docker run --rm -v "${PWD}/server:/app" -w /app golang:alpine go test -v ./...

# Client unit & widget tests
cd c:\Users\jyotirmoyc\Desktop\Projects\meowGram\client
flutter test
```

---

## 7. Review Gate & Next Steps

> [!IMPORTANT]
> The PostgreSQL schema, migration scripts, OIDC authentication middleware, user auto-provisioning repository, and Docker Compose configurations are fully implemented and verified.
>
> **Per the architectural mandate, we are awaiting PM and Technical Lead review before modifying the Flutter client OIDC authentication flow.**
