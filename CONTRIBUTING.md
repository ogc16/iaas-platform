# Contributing to IaaS Platform

Thanks for wanting to contribute! This guide covers how to set up the project locally, what good looks like, and the process for getting changes merged.

## Project overview

A multi-tenant IaaS platform written in Go. The server exposes a REST API (`/api/v1`) plus an embedded dashboard served at `/`. See [README.md](README.md) for features, the API surface, and project layout.

## Prerequisites

- **Go 1.26+** — the version is pinned in `go.mod`; CI reads it from there.
- **Docker** (or a local PostgreSQL 16) for running the database.
- **Git** and a GitHub account.

## Local development

1. Clone the repository:

   ```bash
   git clone git@github.com:ogc16/iaas-platform.git
   cd iaas-platform
   ```

2. Start PostgreSQL:

   ```bash
   docker compose up -d
   ```

3. Run the server:

   ```bash
   go run ./cmd/server
   ```

   The server reads `DATABASE_URL`, or builds one from `POSTGRES_USER`/`POSTGRES_PASSWORD`/`POSTGRES_HOST`/`POSTGRES_PORT`/`POSTGRES_DB` (see `.env.example`). Never hardcode credentials in source.

4. Hit the API:

   ```bash
   curl -X POST localhost:8080/api/v1/auth/signup \
     -H 'Content-Type: application/json' \
     -d '{"email":"you@example.com","password":"hunter2","name":"You"}'
   ```

## Running checks

Run these locally before pushing; CI enforces them too.

```bash
gofmt -l .        # format check (should print nothing)
go vet ./...      # static analysis
go test ./...     # unit tests
go test -race ./...  # same, with the race detector
```

All tests are dependency-free unit tests (services are tested against in-memory fakes), so `go test ./...` works without a database or network access.

## Project layout

```
cmd/server/                # entry point, wiring, graceful shutdown
internal/
  auth/                    # JWT, password hashing, auth middleware + handlers
  billing/                 # usage recording, invoice generation
  compute/                 # VM/container lifecycle
  config/                  # environment-based configuration
  dashboard/               # embedded web UI
  database/                # pgx repositories + migrations
  httpx/                   # JSON response helpers
  middleware/              # CORS, logging, rate limiting
  models/                  # shared data types
  organizations/           # org + membership management
  router/                  # route definitions
```

## Architecture conventions

- **Handlers** own HTTP concerns: parse the request, validate input, map service errors to status codes, write JSON via `internal/httpx.WriteJSON`.
- **Services** hold business logic. They depend on **interfaces** (defined next to the service), not concrete repositories — this is what makes them testable. Keep it that way when you add features.
- **Repositories** (`internal/database`) implement the SQL. Return `database.ErrNotFound` (wrapping `pgx.ErrNoRows`) when a record is absent so callers can distinguish "not found" from real failures.
- **Never swallow errors.** If a lookup fails and the error is not `database.ErrNotFound`, propagate it wrapped with context (`fmt.Errorf("...: %w", err)`).

## Writing tests

- Every service and handler should have tests. Use small in-memory fakes that implement the service interfaces — see `internal/organizations/service_test.go` and `internal/billing/service_test.go` for the pattern.
- Prefer table-driven tests where it helps readability.
- Name tests `TestXxx_Behavior` so the intent is clear from the failure output.
- Verify behavior, not implementation: assert on returned values, status codes, and persisted state in the fakes — not that a specific helper was called.
- Don't write tests that pass on every outcome. A test that accepts both 500 and 401 is a test that asserts nothing.

## Making a change

1. Create a branch off `master`:

   ```bash
   git checkout -b feat/your-feature
   ```

2. Make your change, add or update tests, and run the checks above.
3. Commit with a concise, descriptive message. Follow the existing history style (e.g. `Fix SonarQube findings: ...`).
4. Push and open a pull request against `master`.

## Pull request checklist

- [ ] `gofmt -l .` prints nothing
- [ ] `go vet ./...` passes
- [ ] `go test -race ./...` passes
- [ ] New behavior has test coverage
- [ ] No new swallowed errors or hardcoded credentials
- [ ] README updated if the public API or configuration changed

## Reporting issues

Open a GitHub issue with:
- What you expected and what happened.
- Steps to reproduce, including request/response bodies.
- Go version and any relevant logs.

## Code of conduct

Be kind and constructive. Assume good intent; focus feedback on the code, not the person.
