# IaaS Platform

A Go-based Infrastructure-as-a-Service platform with multi-tenant organizations, compute resource management, API key authentication, rate limiting, and usage-based billing.

## Features

- **User Authentication** — Signup/login with JWT and bcrypt passwords
- **API Key Auth** — Programmatic access via `X-API-Key` header for IaaS endpoints
- **Multi-Tenant Organizations** — Create orgs, invite members, role-based access
- **Compute Resources** — Create and manage VMs and containers with full lifecycle (start/stop/terminate)
- **Usage-Based Billing** — Track CPU, memory, and disk usage; auto-generate invoices
- **Rate Limiting** — Token-bucket rate limiter per IP/API key

## Quick Start

```bash
docker compose up -d
```

### Run the server
```
go run ./cmd/server

```
### open psql in cmd
```
docker compose exec -it postgres psql -U iaas -d iaas
```
### psql operations
```
\q | quit
\x | format output
\d | show table
\dt | list tables & relations
```

## API Endpoints

### Public
| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/v1/auth/signup` | Create account |
| POST | `/api/v1/auth/login` | Login |

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
| POST | `/api/v1/orgs/{id}/instances/{iid}/start` | Start instance |
| POST | `/api/v1/orgs/{id}/instances/{iid}/stop` | Stop instance |
| POST | `/api/v1/orgs/{id}/instances/{iid}/terminate` | Terminate instance |
| GET | `/api/v1/orgs/{id}/billing/usage` | Get usage summary |
| GET | `/api/v1/orgs/{id}/billing/invoices` | List invoices |

## Tech Stack

- **Language:** Go 1.26
- **Router:** chi/v5
- **Database:** PostgreSQL 16 (pgx/v5)
- **Auth:** JWT (HS256) + API keys
- **Infrastructure:** Docker Compose

## Project Structure

```
cmd/server/main.go              # Entry point
internal/
  auth/                         # Authentication (JWT, handlers, middleware)
  billing/                      # Usage tracking and invoice generation
  compute/                      # VM/container lifecycle management
  config/                       # Environment-based configuration
  database/                     # PostgreSQL repositories and migrations
  middleware/                   # CORS, logging, rate limiting
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
