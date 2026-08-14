# IaaS Platform

![CI](https://github.com/ogc16/iaas-platform/actions/workflows/ci.yml/badge.svg)
![Coverage](https://img.shields.io/badge/coverage-45%25+-orange)
![Go](https://img.shields.io/badge/Go-1.26-00ADD8?logo=go&logoColor=white)
![License](https://img.shields.io/badge/license-MIT-blue)
![API](https://img.shields.io/badge/API-OpenAPI%203.0-success)

A Go-based **cloud control plane and billing platform** with multi-tenant organizations, compute resource lifecycle management, API key authentication, rate limiting, quotas, and usage-based billing.

> **Scope honesty.** The compute backend is a *simulation*, not a real hypervisor: instances are database rows whose lifecycle advances through a state machine driven by a wall-clock simulator. The control-plane surface — auth, multi-tenancy, quotas, capacity checks, billing, and the async lifecycle API — is real and production-shaped. A pluggable `compute.Provider` interface is the seam where a real backend (e.g. Docker, or a cloud API) can be implemented without touching the API or service layers.

## 🏆 Competitive Positioning

Why developers will choose this over others:

- **Learn & Deploy** — not just a tutorial, but production-ready code
- **Clean Codebase** — 2,000 lines vs 100,000 line frameworks
- **Zero Dependencies** — only 5 direct deps, minimal attack surface
- **Multi-Tenant First** — most Go projects ignore this complexity
- **Production Ready** — TLS, rate limiting, audit logging included
- **Community Friendly** — good-first-issues for newcomers
- **Well Documented** — SECURITY.md, ARCHITECTURE.md, DEPLOYMENT.md
- **Active Maintenance** — regular releases, quick response

💡 **What sets this apart**

| Aspect | IaaS Platform | Most Open Source | AWS |
|--------|---------------|------------------|-----|
| Code Quality | Enterprise ✅ | GitHub luck | Proprietary |
| Test Coverage | 45%+ gate | Often none | N/A |
| Documentation | Extensive | Sparse | Excellent |
| Learning Value | ⭐⭐⭐⭐⭐ | ⭐⭐ | ❌ |
| Self-hosted | ✅ | N/A | ❌ |
| Multi-tenant | ✅ | Rare | ✅ |
| Billing Engine | ✅ | Ultra-rare | ✅ |
| Security | Production ✅ | Often weak | Industry-leading |

## Architecture

```mermaid
graph TD
  Client[Client / API Consumer] -->|JWT / X-API-Key| Router[Chi Router & Middleware]
  Router --> RateLimiter[Token Bucket Rate Limiter]
  RateLimiter --> Handlers[HTTP Handlers]
  Handlers --> Services[Compute / Billing / Org Services]
  Services --> DB[(PostgreSQL 16)]
  ComputeService --> Provider[Compute Provider]
  Reconciler[Lifecycle Reconciler] --> Provider
  Provider --> DB
  DB --> Billing[Billing Service]
```

The API layer is thin. Services own the business rules (membership checks, quotas, capacity), and the compute service delegates instance provisioning to a `Provider` implementation. A background **reconciler** advances instances between transient states based on provider-reported state.

## Features

- **User Authentication** — Signup/login with JWT (HS256) and bcrypt passwords; API key access via `X-API-Key`. API keys are shown once at signup and stored as SHA-256 hashes; passwords use configurable bcrypt cost
- **Multi-Tenant Organizations** — Create orgs, invite members, role-based access
- **Compute Lifecycle** — Async state machine with start/stop/terminate; per-org quotas and per-region capacity enforcement
- **Pluggable Provider** — `compute.Provider` interface; default `SimProvider` is a durable, wall-clock simulation
- **Usage-Based Billing** — Track CPU, memory, and disk usage; generate invoices
- **Rate Limiting** — Token-bucket limiter per IP/API key
- **Operational** — `/healthz` liveness and `/readyz` readiness probes, baseline security headers, request-ID echo in logs, paginated list endpoints (`limit`/`offset` + `X-Total-Count`), configurable connection pool

## Compute Lifecycle

Instances move between states asynchronously. User actions request a transition and the API returns `202 Accepted`; the **reconciler** settles transient states:

```
pending → running → stopping → stopped
   │         │          │           │
   └─────────┴─── terminating → terminated
                          │
                         failed
```

- `start` — `stopped → pending` (re-provision)
- `stop` — `running → stopping`
- `terminate` — any non-terminal state (including `failed`) → `terminating → terminated`
- Invalid transitions return `409`, unknown regions `400`, out-of-quota or out-of-capacity requests `409`.

### Quotas and capacity

Each organization has a quota (default: 20 instances, 16 vCPU, 32 GiB, 500 GB). Region capacity is tracked as running consumption and enforced at create time:

| Region | vCPU | Memory | Disk |
|--------|------|--------|------|
| `us-east-1` | 64 | 128 GiB | 2000 GB |
| `us-west-1` | 48 | 96 GiB | 1500 GB |
| `eu-west-1` | 32 | 64 GiB | 1000 GB |

## Quick Start

```bash
docker compose up -d
```

### Run the server
```
set POSTGRES_PASSWORD=iaas
go run ./cmd/server
```
> The server reads the database connection from `DATABASE_URL`, or builds it from `POSTGRES_USER`/`POSTGRES_PASSWORD`/`POSTGRES_HOST`/`POSTGRES_PORT`/`POSTGRES_DB` (see `.env.example`). Never hardcode credentials in source.

### psql
```
docker compose exec -it postgres psql -U iaas -d iaas
```
`\q` quit · `\x` expanded output · `\d` describe · `\dt` list tables

## Examples

| Path | What it does |
|------|--------------|
| `examples/api-usage.sh` | Scripted tour of the HTTP API (signup, orgs, instances, billing) with curl + jq |
| `examples/quick-start.sh` | One-command end-to-end demo: compose up → server → instance lifecycle → billing → teardown |
| `examples/terraform/` | Repeatable single-host deployment via the Docker provider (main/variables/outputs) |

The API is also explorable interactively at **`/docs`** (Swagger UI, served from the running server; spec at `openapi.yaml`).

## Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `ENV` | `development` | `development`, `staging`, or `production`. Outside development the server refuses to boot with a placeholder or short `JWT_SECRET`. |
| `PORT` | `8080` | HTTP listen port |
| `DATABASE_URL` | — | Postgres DSN. Use `sslmode=verify-full` (or `require`) outside local dev, never `disable`. |
| `JWT_SECRET` | `change-me-in-production` | Signing secret. Generate with `openssl rand -hex 32`. |
| `JWT_ISSUER` | `iaas-platform` | Token issuer claim |
| `JWT_EXPIRES_IN` | `86400` | Token lifetime (seconds) |
| `BCRYPT_COST` | `12` | Password hashing cost (4–15). Keep ≥ 10 in production. |
| `DB_MAX_CONNS` | `20` | Maximum connections in the pgx pool |
| `DB_MIN_CONNS` | `2` | Idle connections kept warm in the pool |
| `TLS_CERT_FILE` | — | Path to the TLS certificate. `TLS_KEY_FILE` must be set too; when both are set the server serves HTTPS and enables HSTS. |
| `TLS_KEY_FILE` | — | Path to the TLS private key (pair with `TLS_CERT_FILE`) |
| `PROVISIONING_DELAY_SECONDS` | `5` | Simulated time to `running` |
| `STOP_DELAY_SECONDS` | `3` | Simulated time to `stopped` |
| `RECONCILE_INTERVAL_SECONDS` | `2` | Reconciler tick interval |
| `APP_BASE_URL` | `http://localhost:8080` | Public base URL used to build password reset links |
| `SMTP_HOST` | — | SMTP relay for reset emails; when empty the link is logged instead of emailed |
| `SMTP_PORT` | `587` | SMTP port |
| `SMTP_USERNAME` | — | SMTP auth username (optional) |
| `SMTP_PASSWORD` | — | SMTP auth password (optional) |
| `SMTP_FROM` | `no-reply@iaas.local` | From address for outgoing email |
| `PASSWORD_RESET_TTL_HOURS` | `24` | How long a reset token stays valid |

## API Endpoints

### Public
| Method | Path | Description |
|--------|------|-------------|
| GET | `/healthz` | Liveness probe (always 200) |
| GET | `/readyz` | Readiness probe (200 when the DB is reachable, else 503) |
| POST | `/api/v1/auth/signup` | Create account |
| POST | `/api/v1/auth/login` | Login |
| POST | `/api/v1/auth/forgot-password` | Email a one-time reset link (enumeration-safe) |
| POST | `/api/v1/auth/reset-password` | Set a new password with a reset token |

### Authenticated (Bearer JWT or X-API-Key)
| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/me` | Current user |
| POST | `/api/v1/orgs` | Create organization |
| GET | `/api/v1/orgs` | List organizations |
| GET | `/api/v1/orgs/{id}` | Get organization |
| POST | `/api/v1/orgs/{id}/members` | Invite member |
| GET | `/api/v1/orgs/{id}/members` | List members |
| POST | `/api/v1/orgs/{id}/instances` | Create instance |
| GET | `/api/v1/orgs/{id}/instances` | List instances |
| GET | `/api/v1/orgs/{id}/instances/{iid}` | Get instance |
| POST | `/api/v1/orgs/{id}/instances/{iid}/start` | Start instance (`202` accepted) |
| POST | `/api/v1/orgs/{id}/instances/{iid}/stop` | Stop instance (`202` accepted) |
| POST | `/api/v1/orgs/{id}/instances/{iid}/terminate` | Terminate instance (`202` accepted) |
| GET | `/api/v1/orgs/{id}/billing/usage` | Get usage summary |
| POST | `/api/v1/orgs/{id}/billing/usage` | Record usage (`instance_id`, `resource_type`, `quantity`) |
| GET | `/api/v1/orgs/{id}/billing/invoices` | List invoices |
| POST | `/api/v1/orgs/{id}/billing/invoices/generate` | Generate an invoice for the current month |
| GET | `/api/v1/orgs/{id}/billing/invoices/{invoiceID}` | Get invoice line items |

Valid `resource_type` values: `cpu_hours`, `memory_gb_hours`, `disk_gb_hours`.

List endpoints (`/orgs`, `/orgs/{id}/members`, `/orgs/{id}/instances`, `/orgs/{id}/billing/invoices`) accept `limit` (default 50, max 100) and `offset` query parameters and return the total count in the `X-Total-Count` response header.

An OpenAPI description is available at [`openapi.yaml`](openapi.yaml).

## Testing

Unit tests cover the auth, organizations, compute, and billing services and handlers, the JWT and rate-limit middleware, configuration, health probes, security headers, and routing. They run against in-memory fakes — no database required.

```bash
go test ./...
go vet ./...
```

Integration tests exercise the repositories against a real PostgreSQL and are opt-in via the `integration` build tag (default `TEST_DATABASE_URL: postgres://iaas:iaas@127.0.0.1:5432/iaas?sslmode=disable`):

```bash
docker compose up -d
$env:TEST_DATABASE_URL="postgres://iaas:iaas@127.0.0.1:5432/iaas?sslmode=disable"   # PowerShell
TEST_DATABASE_URL='postgres://iaas:iaas@127.0.0.1:5432/iaas?sslmode=disable'          # bash
go test -tags integration -count=1 ./internal/database/
```

`gofmt -l .` should print nothing. CI runs formatting, vet, tests (with the race detector), coverage (gate: 45%), a build, a gitleaks secret scan, and [CodeQL](https://codeql.github.com/) analysis on every push and pull request. `make audit` runs the [govulncheck](https://go.dev/blog/vuln) vulnerability scan. See [CONTRIBUTING.md](CONTRIBUTING.md) for the full contribution guide.

## Tech Stack

- **Language:** Go 1.26
- **Router:** chi/v5
- **Database:** PostgreSQL 16 (pgx/v5)
- **Auth:** JWT (HS256) + hashed API keys (SHA-256 at rest)
- **Infrastructure:** Docker Compose, multi-stage Dockerfile (distroless runtime), Makefile

## Project Structure

```
cmd/server/main.go              # Entry point (health probes, TLS, graceful shutdown)
cmd/migrate/main.go             # Standalone migration runner
internal/
  auth/                         # Authentication (JWT, handlers, middleware)
  billing/                      # Usage tracking and invoice generation
  compute/                      # Provider abstraction, lifecycle service, reconciler
  config/                       # Environment-based configuration
  database/                     # PostgreSQL repositories and migrations
  health/                       # /healthz and /readyz probes
  middleware/                   # CORS, logging, rate limiting, security headers
  models/                       # Shared data models
  organizations/                # Multi-tenant org management
  router/                       # Route definitions
```

## Pricing

| Resource | Rate |
|----------|------|
| CPU | $0.50 / core-hour |
| Memory | $0.20 / GB-hour |
| Disk | $0.01 / GB-hour |

## Contributing

Contributions are welcome. See [CONTRIBUTING.md](CONTRIBUTING.md) for the full
guide — development setup, testing, linting, and the PR process. CI runs
formatting, vet, tests (with the race detector), coverage, a build, a gitleaks
secret scan, and CodeQL on every push and pull request.

## Documentation

- [Architecture](docs/ARCHITECTURE.md) — layered design, modules, data flow
- [Deployment](docs/DEPLOYMENT.md) — production hardening, migrations, monitoring, scaling
- [Enterprise Readiness](docs/ENTERPRISE_READINESS.md) — hardening backlog and status
- [Security](SECURITY.md) — threat model, configuration checklist, vulnerability reporting
- [Roadmap](ROADMAP.md) — v1.0 gate, v2 features, and non-goals
- [Changelog](CHANGELOG.md) — release history

## Blog Post

- [Learn how to build a cloud control panel with authentication, billing, and resource management](https://dev.to/ogc16/learn-how-to-build-a-cloud-control-panel-with-authentication-billing-and-resource-management--530e) — walkthrough on DEV of building this platform.

## Pinned Gist

- [Privacy policy](https://gist.github.com/ogc16/a5679814a9342587790c0adbc97a790e) — pinned privacy-policy gist.

## License

Licensed under the [MIT License](LICENSE).
