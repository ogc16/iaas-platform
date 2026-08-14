# Enterprise Readiness — Plan & Recommendations

Goal: take the IaaS control plane from a well-tested learning project to an
enterprise-ready MVP: deployable, observable, secure at rest, consistent, and
safe to operate by people who did not write it.

Principles:

- **Small, reviewable increments.** Each recommendation is implemented as one
  scoped commit with tests.
- **Honest scope.** The compute backend is a simulation. Enterprise readiness
  here means the *control plane* is production-shaped, not that it provisions
  real VMs.
- **No new runtime dependencies unless justified.** The Go dependency set stays
  tiny (chi, jwt, pgx, crypto). Observability/security needs are met with the
  standard library where practical.

---

## Tier 1 — Implement now (done in this pass)

| # | Change | Why it matters |
|---|--------|----------------|
| 1.1 | **Health endpoints** `/healthz` (liveness) and `/readyz` (readiness, pings Postgres) | Load balancers, container orchestration, and uptime probes have nothing to target today |
| 2.1 | **Security headers middleware** (CSP, nosniff, frame/iframe policy, HSTS outside dev) | Baseline hardening for any public web surface |
| 3.1 | **Echo `X-Request-ID`** on responses | Lets clients correlate failures with server logs (access logs already carry it) |
| 4.1 | **Hash API keys at rest** (SHA-256; raw key shown once at signup) + migration `010` to rehash existing rows | A DB dump no longer leaks working credentials |
| 5.1 | **Configurable bcrypt cost** (`BCRYPT_COST`, default 12) | Passwords are the cheapest thing to protect; default cost 10 is aging |
| 6.1 | **Pagination** (`limit`/`offset`, capped) on compute, org, member, and invoice list endpoints | Unbounded lists are a DoS and an anti-pattern for enterprise APIs |
| 7.1 | **TLS support** (`TLS_CERT_FILE`/`TLS_KEY_FILE`) | Serve HTTPS directly when terminated at the app (or note the LB requirement) |
| 8.1 | **Connection pool sizing** (`DB_MAX_CONNS`/`DB_MIN_CONNS`) | Prevent pool exhaustion and starved deploys under load |
| 9.1 | **Dockerfile** (multi-stage, non-root, distroless) + `.dockerignore` | A deployable, minimal, rootless image |
| 10.1 | **Makefile** (fmt/vet/test/race/build/migrate/docker) | Standardized developer + ops entry points |
| 11.1 | **CI: gitleaks scan + coverage summary; Dependabot** | Secrets can't land in CI; dependency drift is tracked |
| 12.1 | **Docs** — README config table, `.env.example`, `openapi.yaml` updated | Operators can run it without reading code |

## Tier 2 — Next (high value, larger refactors)

- **Database transactions.** `organizations.Create` (org + owner member),
  `billing.GenerateInvoice` (invoice + line items), and
  `compute.Create` (provider provision + insert) are not atomic. Refactor the
  repositories to depend on a `Querier` interface (`pgxpool.Pool` and `pgx.Tx`
  both satisfy it), add `database.InTx(ctx, pool, fn)`, and pass tx-bound
  repositories into the service calls. This is a wide but mechanical change.
- **Prometheus metrics** (`/metrics`): HTTP request totals/latency histograms,
  DB pool stats, reconciler loop time. Requires `prometheus/client_golang`.
- **CORS allowlist**: configurable `CORS_ALLOWED_ORIGINS` (comma-separated);
  defaults to `*` in development, refuses to boot in production without
  explicit origins.
- **Login rate limiting / lockout** (per-account, not just per-IP) and
  `Retry-After` on 429s.
- **Idempotency keys** on `POST /instances` and `POST /billing/usage` so
  retries cannot double-record.
- **Integration test job** in CI (Postgres service container) exercising the
  real repositories + migrations + handlers end-to-end.
- **`golangci-lint`** as a CI gate (currently only `gofmt` + `go vet`).
- **Structured error contract**: typed API errors with stable codes, not
  free-form strings.

## Tier 3 — Later (product / scale)

- Real compute provider backends behind the existing `compute.Provider` seam.
- Audit logging (who changed what, when) for multi-tenant governance.
- RBAC matrix expansion (roles beyond admin/member, fine-grained permissions).
- API key rotation endpoint and per-key scopes.
- Multi-zone/multi-region capacity; regional DR.
- Distributed rate limiting (shared store) for multi-instance deployments.
- Feature flags and canary/release gates wired to the deploy pipeline.

---

## Implemented changes (Tier 1) — summary

Each item lands as its own commit; see the git log for the full breakdown.
All code is verified with `gofmt -l .` (clean), `go build ./...`,
`go vet ./...`, and `go test ./...`.
