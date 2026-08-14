# Deployment Guide

This guide covers running the IaaS Platform locally and hardening it for production. For architecture details, see [ARCHITECTURE.md](ARCHITECTURE.md). For copy-paste deployment walkthroughs (systemd, Railway, Kubernetes, Terraform), see the [Deployment Playbook](DEPLOYMENT_PLAYBOOK.md).

## Prerequisites

- Go 1.26+
- PostgreSQL 16 (or Docker)
- Make (optional; targets in the `Makefile`)

## Local Development

```bash
# 1. Start PostgreSQL
docker compose up -d          # postgres:16-alpine, user/password/db = iaas

# 2. Create .env from the template (dev defaults are safe for local use)
cp .env.example .env

# 3. Run the server (migrations apply automatically at boot)
go run ./cmd/server
# 4. (Optional) run migrations explicitly
go run ./cmd/migrate
```

The server listens on `:8080`; the dashboard is served at `/` and the API under `/api/v1`. Liveness/readiness: `GET /healthz`, `GET /readyz`.

## Building

```bash
make build        # -> bin/server, bin/migrate
make test         # unit tests against in-memory fakes (no DB needed)
make race         # race-detector test run (matches CI)
docker build -t iaas-platform:latest .
```

The Dockerfile is multi-stage: a `golang:1.26-alpine` build with `CGO_ENABLED=0` and `-trimpath -ldflags="-s -w"`, running as the `nonroot` distroless image. Both `server` and `migrate` binaries are produced.

## Production Configuration

Set these via the environment (or a secret store). All defaults and meanings are in `.env.example` and `internal/config/config.go`.

### Required

| Variable | Value |
|----------|-------|
| `ENV` | `production` (non-dev refuses weak `JWT_SECRET`) |
| `JWT_SECRET` | random value ≥ 32 bytes; never a placeholder |
| `DATABASE_URL` | Postgres DSN (or `POSTGRES_*` vars) |

### Recommended

| Variable | Recommendation |
|----------|----------------|
| `BCRYPT_COST` | 12 (default) or higher |
| `APP_BASE_URL` | the public HTTPS origin (used in password-reset links) |
| `TLS_CERT_FILE` / `TLS_KEY_FILE` | enable in-process TLS **or** terminate TLS at a reverse proxy |
| `SMTP_HOST`/`SMTP_PORT`/`SMTP_USERNAME`/`SMTP_PASSWORD`/`SMTP_FROM` | transactional email; without SMTP, reset links are logged to stdout |
| `PASSWORD_RESET_TTL_HOURS` | 24 (default) |
| `DB_MAX_CONNS` / `DB_MIN_CONNS` | sized to workload; default 20/2 |
| `PROVISIONING_DELAY_SECONDS` / `STOP_DELAY_SECONDS` / `RECONCILE_INTERVAL_SECONDS` | simulated compute timing |

### Hardening Checklist

- [ ] `ENV=production` and a strong `JWT_SECRET`
- [ ] TLS in-process **or** TLS at the reverse proxy with HSTS
- [ ] Dedicated least-privilege DB user; restrict network access to the DB
- [ ] Database credentials, JWT secret, and SMTP credentials from a secret manager, never in the repo (`.env` is gitignored and excluded from the Docker build context)
- [ ] Run the server as an unprivileged user (the distroless image already does)
- [ ] Reverse-proxy layer adds a request-body size limit (no in-app limit exists yet; see `SECURITY.md`)
- [ ] Run `go run github.com/gitleaks/gitleaks/v8@latest detect --config .gitleaks.toml` before every push (CI enforces it)

## Migrations in Production

Migrations run automatically at server boot. For staged/rolling deployments, run them explicitly **before** starting the new binary:

```bash
DATABASE_URL='postgres://...' go run ./cmd/migrate
```

The server refuses to boot against a newer schema than its binary understands, so apply migrations before rolling out the app.

## Monitoring

- `GET /healthz` — liveness; `GET /readyz` — readiness (DB check).
- Structured JSON logs via `slog`; each request logs with its `X-Request-ID`, enabling correlation across hops.
- `GET /metrics` — Prometheus-compatible text exposition format (version 0.0.4). The endpoint is mounted outside the auth middleware so internal scrapers don't need a bearer token. When `METRICS_TOKEN` is set, scrapes must present it as `Authorization: Bearer <token>`.

### Recorded metrics

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `iaas_http_requests_total` | counter | `method`, `route`, `status` | Total HTTP requests handled |
| `iaas_http_request_duration_seconds` | histogram | `method`, `route` | Request latency (seconds) |
| `iaas_webhook_deliveries_total` | counter | — | Webhook deliveries completed |
| `iaas_webhook_failures_total` | counter | — | Webhook deliveries that exhausted retries |
| `iaas_webhook_retries_total` | counter | — | Webhook delivery retry attempts |

### Example scrape config

```yaml
scrape_configs:
  - job_name: iaas-platform
    metrics_path: /metrics
    static_configs:
      - targets: ["localhost:8080"]
    # If METRICS_TOKEN is set:
    # authorization:
    #   credentials: <token>
```

The registry is a zero-dependency, in-process Prometheus text renderer — no `client_golang` dependency is required.

## Scaling Notes

- The server is stateless except for the in-memory **rate limiter** (token bucket keyed by IP/API key). Horizontal scaling is fine for correctness, but rate limiting is per-process unless you front it with a shared store (e.g. Redis) or sticky sessions.
- The compute **reconciler** is a per-process goroutine. Run a single instance, or make reconciler ownership explicit (future work).
- Tune `DB_MAX_CONNS`/`DB_MIN_CONNS` per replica; don't exceed what PostgreSQL can serve.
- Database is the single source of truth (instances, provider state, usage, invoices). Back up PostgreSQL regularly (`pg_dump`); restore testing is part of a healthy runbook.

## Backups

```bash
docker compose exec -T postgres pg_dump -U iaas iaas > iaas-$(date +%F).sql
```

Rotate backups off-host and periodically test a restore into a scratch database.

## Example: systemd unit

```ini
[Unit]
Description=IaaS Platform control plane
After=network-online.target postgresql.service
Wants=network-online.target

[Service]
User=iaas
Group=iaas
EnvironmentFile=/etc/iaas-platform/env
ExecStart=/opt/iaas-platform/bin/server
Restart=on-failure
RestartSec=5
LimitNOFILE=65536

[Install]
WantedBy=multi-user.target
```

## Example: Kubernetes (Deployment)

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: iaas-platform
spec:
  replicas: 1
  selector:
    matchLabels: { app: iaas-platform }
  template:
    metadata:
      labels: { app: iaas-platform }
    spec:
      securityContext:
        runAsNonRoot: true
        runAsUser: 65532
      containers:
        - name: server
          image: iaas-platform:latest
          ports: [{ containerPort: 8080 }]
          envFrom:
            - secretRef: { name: iaas-platform-secrets }
          livenessProbe:
            httpGet: { path: /healthz, port: 8080 }
          readinessProbe:
            httpGet: { path: /readyz, port: 8080 }
```

> Run one replica for now (in-process reconciler and rate limiter). Multi-replica with a shared rate-limiter store is on the roadmap.
