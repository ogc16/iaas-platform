# Architecture

This document describes how the IaaS Platform is built. It complements the README overview and is written to be accurate against the current `master`.

## Layered Design

The server is a single binary (`cmd/server`) with a deliberately thin HTTP layer:

```
HTTP (chi router + middleware)
  → Handlers (per-module, parse/validate HTTP, call services)
    → Services (business rules: membership, quotas, capacity, lifecycle, billing)
      → Repositories (pgx, SQL against PostgreSQL 16)
      → Providers (compute backend seam)
```

- **Handlers** own HTTP concerns: URL params, JSON, status codes, pagination (`internal/httpx`), and model validation (`internal/validate`, a dependency-free struct-tag validator).
- **Services** own the business rules. This is where multi-tenant isolation lives — every org-scoped call resolves the caller's membership before acting.
- **Repositories** are plain pgx queries; services hold interfaces so tests run against in-memory fakes (222 tests, 23 `_test.go` files, no DB or network needed).

## Modules

| Package | Responsibility |
|---------|----------------|
| `internal/auth` | JWT issue/verify (HS256), bcrypt, API-key generation & hash lookup, password reset, middleware |
| `internal/organizations` | Orgs, members, roles, invites, join requests, suspension |
| `internal/compute` | `Provider` interface, async instance lifecycle service, reconciler, `SimProvider` |
| `internal/billing` | Usage recording, 30-day usage summary, monthly invoice generation |
| `internal/database` | pgx repositories, embedded SQL migrations, `schema_migrations` |
| `internal/config` | Env-based config with production secret guards |
| `internal/middleware` | CORS, request logger, rate limiter, security headers |
| `internal/health` | `/healthz` liveness, `/readyz` readiness |
| `internal/mailer` | SMTP (STARTTLS) transactional email with `LogMailer` fallback |
| `internal/dashboard` | Embedded single-page dashboard (static assets served by the binary) |
| `internal/httpx` | JSON helpers, error mapping, pagination |
| `internal/validate` | Struct-tag validator (`required`, `email`, `min`, `max`, `oneof`, …) |
| `internal/models` | Shared types (user, org, compute, billing, policy) |
| `internal/router` | chi route registration and global middleware chain |
| `cmd/server` | Composition root: config, DB, DI wiring, HTTP server, graceful shutdown |
| `cmd/migrate` | Standalone migration runner for staged rollouts |

## Request Lifecycle

A request flows through the global middleware chain, then the auth middleware, then the route handler:

```
RequestID → RealIP → Logger → Recoverer → CORS → RateLimiter
  → auth.Middleware (Bearer JWT or X-API-Key)  [on /api/v1/** except auth routes]
    → Handler → Service → Repository → PostgreSQL
```

Middleware chain defined in `internal/router/router.go`. Claims are stashed in the request context (`auth.GetClaims`) and read by handlers; `POST /auth/*` and `GET /healthz|/readyz` are public.

## Authentication & Sessions

- **Signup** hashes the password with bcrypt (`BCRYPT_COST`, default 12), issues a JWT, and generates an API key (`iaas_<hex>`). The raw key is returned once; only its SHA-256 hash is persisted.
- **Login** verifies bcrypt and issues a JWT. No session state — tokens are stateless HS256 JWTs (`user_id`, `email`, `role`, issuer, 24h expiry).
- **API keys** are looked up by hash (`FindByAPIKeyHash`) and grant the same access as JWTs.
- **Password reset** issues a `pwr_` token (32 random bytes), stores its SHA-256 digest with a 24h TTL, deletes prior tokens, and marks it used on success. Delivery is via the mailer (`SMTP_HOST` set) or log output.

See `SECURITY.md` for the full security model.

## Multi-Tenancy

- `organizations` + `organization_members`; an org creator becomes the first `admin`.
- The universal access gate is `FindMember(orgID, userID)` — it returns 404 for anyone who is not an active member (suspended members are excluded by the SQL predicate).
- Roles gate admin operations; `ErrSelfAction` blocks self-removal/suspension; non-admin members only ever see their own membership record.
- Join requests let a signup with `org_slug` request admission; admins accept or revoke.

## Compute Lifecycle

Instances are rows whose state advances asynchronously:

```
pending ──► running ◄─► stopping ──► stopped
    └────────► terminating ──► terminated
any non-terminal ──► failed (if provider state is lost)
```

- User actions (`create/start/stop/terminate`) return `202 Accepted`; the **reconciler** (`internal/compute`) polls provider state and settles transient states.
- `Provider` is the extension seam: `Name, Provision, Start, Stop, Terminate, GetState`.
- Default `SimProvider` is a durable wall-clock simulation persisted in `provider_state`, so state survives restarts. Timings come from config (`PROVISIONING_DELAY_SECONDS`, `STOP_DELAY_SECONDS`, `RECONCILE_INTERVAL_SECONDS`).
- Quota (per org) and regional capacity are enforced at creation.

## Billing

- Usage is recorded as `usage_records` with resource types `cpu_hours`, `memory_gb_hours`, `disk_gb_hours` and fixed unit prices ($0.50/CPU-hr, $0.20/GB-hr, $0.01/GB-hr).
- The usage summary aggregates the last 30 days; invoice generation bills the current calendar month in cents (`status: pending|paid|overdue`) with line items.

## Data & Migrations

- PostgreSQL 16 via pgx/v5 (`pgxpool`).
- Migrations are embedded with `//go:embed migrations/*.sql`, applied in numeric order inside transactions, and recorded in `schema_migrations`. The server refuses to boot an older binary against a newer schema.
- Migrations also run standalone via `cmd/migrate` for staged rollouts. **13 migrations** cover users, orgs, instances, usage, invoices, region capacity, provider state, quotas, API-key hashing, join requests, suspension, and password resets.

## Concurrency & Background Work

- The reconciler runs as a goroutine in the server process (no separate worker binary yet).
- Rate limiting uses an in-memory token bucket — per-process state (relevant when scaling horizontally).
- Multi-step writes (instance/org creation, invoice generation) are currently **not** wrapped in DB transactions; tracked in `docs/ENTERPRISE_READINESS.md`.

## Extension Seams

1. **`compute.Provider`** — add a real backend (Docker, EC2, GCE) without touching the API or service layers.
2. **Repository interfaces** — services depend on interfaces, enabling both fakes in tests and alternative storage.
3. **Mailer interface** — swap `LogMailer`/`SMTPMailer` for any transactional email provider.
